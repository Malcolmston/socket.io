//go:build js && wasm

// Command socketio (wasm) exposes the socket.io module's pure wire-protocol
// codecs to JavaScript. Built with GOOS=js GOARCH=wasm it registers a
// `__mgo_socketio` object on the JS global holding the Engine.IO and Socket.IO
// packet encode/decode functions — no net/http, no Server/Socket — so the very
// same Go codec that powers the Go server runs in the browser or Node and can
// read and write the identical wire format. See socketio.mjs for the idiomatic
// JS wrapper.
package main

import (
	"encoding/base64"
	"syscall/js"

	socketio "github.com/malcolmston/socketio"
	"github.com/malcolmston/socketio/engineio"
)

func main() {
	obj := js.Global().Get("Object").New()

	// Engine.IO codec (the transport framing layer).
	obj.Set("engineEncode", js.FuncOf(engineEncodeFn))
	obj.Set("engineDecode", js.FuncOf(engineDecodeFn))
	obj.Set("engineEncodePayload", js.FuncOf(engineEncodePayloadFn))
	obj.Set("engineDecodePayload", js.FuncOf(engineDecodePayloadFn))

	// Socket.IO codec (the application packet layer that rides on Engine.IO
	// MESSAGE packets).
	obj.Set("sioEncode", js.FuncOf(sioEncodeFn))
	obj.Set("sioDecode", js.FuncOf(sioDecodeFn))

	// Packet-type name -> number maps, so JS callers need not hardcode digits.
	obj.Set("engineTypes", stringNumMap(map[string]int{
		"open": int(engineio.Open), "close": int(engineio.Close),
		"ping": int(engineio.Ping), "pong": int(engineio.Pong),
		"message": int(engineio.Message), "upgrade": int(engineio.Upgrade),
		"noop": int(engineio.Noop),
	}))
	obj.Set("sioTypes", stringNumMap(map[string]int{
		"CONNECT": int(socketio.Connect), "DISCONNECT": int(socketio.Disconnect),
		"EVENT": int(socketio.Event), "ACK": int(socketio.Ack),
		"CONNECT_ERROR": int(socketio.ConnectError),
		"BINARY_EVENT":  int(socketio.BinaryEvent),
		"BINARY_ACK":    int(socketio.BinaryAck),
	}))

	obj.Set("protocolVersion", socketio.ProtocolVersion)
	obj.Set("engineProtocol", engineio.Protocol)

	js.Global().Set("__mgo_socketio", obj)

	select {} // keep the Go runtime alive so the exported funcs stay callable
}

// engineEncodeFn(typeNum, data?) -> string renders an Engine.IO packet to its
// string wire form (e.g. engineEncode(4, `2["ev"]`) -> `42["ev"]`).
func engineEncodeFn(_ js.Value, a []js.Value) any {
	if len(a) == 0 {
		return errObj("engineEncode: type required")
	}
	p := engineio.Packet{Type: engineio.PacketType(a[0].Int())}
	if len(a) > 1 && a[1].Type() == js.TypeString {
		p.Data = a[1].String()
	}
	return p.Encode()
}

// engineDecodeFn(str) -> {type, typeName, data} parses one Engine.IO packet.
// A binary ("b<base64>") packet yields {type, typeName, binary:true, base64}.
func engineDecodeFn(_ js.Value, a []js.Value) any {
	if len(a) == 0 || a[0].Type() != js.TypeString {
		return errObj("engineDecode: string required")
	}
	p, err := engineio.Decode(a[0].String())
	if err != nil {
		return errObj(err.Error())
	}
	return engineToJS(p)
}

// engineEncodePayloadFn(array of {type, data}) -> string joins several packets
// into a single polling payload (separated by the 0x1e record separator).
func engineEncodePayloadFn(_ js.Value, a []js.Value) any {
	if len(a) == 0 || a[0].Type() != js.TypeObject {
		return errObj("engineEncodePayload: array required")
	}
	arr := a[0]
	n := arr.Length()
	packets := make([]engineio.Packet, n)
	for i := 0; i < n; i++ {
		el := arr.Index(i)
		p := engineio.Packet{}
		if t := el.Get("type"); t.Type() == js.TypeNumber {
			p.Type = engineio.PacketType(t.Int())
		}
		if d := el.Get("data"); d.Type() == js.TypeString {
			p.Data = d.String()
		}
		packets[i] = p
	}
	return engineio.EncodePayload(packets)
}

// engineDecodePayloadFn(str) -> array of {type, typeName, data} splits a polling
// payload into its constituent packets.
func engineDecodePayloadFn(_ js.Value, a []js.Value) any {
	if len(a) == 0 || a[0].Type() != js.TypeString {
		return errObj("engineDecodePayload: string required")
	}
	packets, err := engineio.DecodePayload(a[0].String())
	if err != nil {
		return errObj(err.Error())
	}
	out := js.Global().Get("Array").New(len(packets))
	for i, p := range packets {
		out.SetIndex(i, engineToJS(p))
	}
	return out
}

// sioEncodeFn(packetObj) -> string renders a Socket.IO packet to the string
// carried inside an Engine.IO MESSAGE packet. packetObj = {type:number,
// namespace?:string, id?:number, data?:any}.
func sioEncodeFn(_ js.Value, a []js.Value) any {
	if len(a) == 0 || a[0].Type() != js.TypeObject {
		return errObj("sioEncode: packet object required")
	}
	o := a[0]
	p := socketio.Packet{}
	if t := o.Get("type"); t.Type() == js.TypeNumber {
		p.Type = socketio.PacketType(t.Int())
	}
	if ns := o.Get("namespace"); ns.Type() == js.TypeString {
		p.Namespace = ns.String()
	}
	if id := o.Get("id"); id.Type() == js.TypeNumber {
		v := uint64(id.Int())
		p.ID = &v
	}
	if d := o.Get("data"); !d.IsUndefined() && !d.IsNull() {
		p.Data = jsToAny(d)
	}
	s, err := p.Encode()
	if err != nil {
		return errObj(err.Error())
	}
	return s
}

// sioDecodeFn(str) -> packetObj parses a Socket.IO packet. The returned object
// carries {type, typeName, namespace, id, data, attachments} plus, for
// Event/BinaryEvent packets, the convenience fields {eventName, args}.
func sioDecodeFn(_ js.Value, a []js.Value) any {
	if len(a) == 0 || a[0].Type() != js.TypeString {
		return errObj("sioDecode: string required")
	}
	p, err := socketio.DecodePacket(a[0].String())
	if err != nil {
		return errObj(err.Error())
	}
	out := js.Global().Get("Object").New()
	out.Set("type", int(p.Type))
	out.Set("typeName", p.Type.String())
	out.Set("namespace", p.Namespace)
	if p.ID != nil {
		out.Set("id", float64(*p.ID))
	} else {
		out.Set("id", js.Null())
	}
	out.Set("data", anyToJS(p.Data))
	out.Set("attachments", p.Attachments())
	if p.Type == socketio.Event || p.Type == socketio.BinaryEvent {
		out.Set("eventName", p.EventName())
		out.Set("args", anyToJS(p.Args()))
	}
	return out
}

// engineToJS converts a decoded Engine.IO packet into a JS object.
func engineToJS(p engineio.Packet) js.Value {
	out := js.Global().Get("Object").New()
	out.Set("type", int(p.Type))
	out.Set("typeName", p.Type.String())
	if p.Binary != nil {
		out.Set("binary", true)
		out.Set("base64", encodeBase64(p.Binary))
	} else {
		out.Set("data", p.Data)
	}
	return out
}

// jsToAny recursively converts a JS value into a Go value that json.Marshal
// understands, mirroring what JSON.parse would produce.
func jsToAny(v js.Value) any {
	switch v.Type() {
	case js.TypeBoolean:
		return v.Bool()
	case js.TypeNumber:
		return v.Float()
	case js.TypeString:
		return v.String()
	case js.TypeObject:
		if js.Global().Get("Array").Call("isArray", v).Bool() {
			n := v.Length()
			arr := make([]any, n)
			for i := 0; i < n; i++ {
				arr[i] = jsToAny(v.Index(i))
			}
			return arr
		}
		keys := js.Global().Get("Object").Call("keys", v)
		m := make(map[string]any, keys.Length())
		for i := 0; i < keys.Length(); i++ {
			k := keys.Index(i).String()
			m[k] = jsToAny(v.Get(k))
		}
		return m
	default: // Undefined, Null, Function, Symbol
		return nil
	}
}

// anyToJS converts a Go value produced by json.Unmarshal into a JS value.
func anyToJS(v any) any {
	switch t := v.(type) {
	case nil:
		return js.Null()
	case bool, string, float64, int:
		return t
	case []any:
		arr := js.Global().Get("Array").New(len(t))
		for i, e := range t {
			arr.SetIndex(i, anyToJS(e))
		}
		return arr
	case map[string]any:
		o := js.Global().Get("Object").New()
		for k, e := range t {
			o.Set(k, anyToJS(e))
		}
		return o
	default:
		return js.Null()
	}
}

// errObj wraps an error message in an object with a distinctive __error field
// so the JS wrapper can detect failures and throw without ambiguity.
func errObj(msg string) js.Value {
	o := js.Global().Get("Object").New()
	o.Set("__error", msg)
	return o
}

// stringNumMap builds a frozen JS object from a name -> number map.
func stringNumMap(m map[string]int) js.Value {
	o := js.Global().Get("Object").New()
	for k, v := range m {
		o.Set(k, v)
	}
	return o
}

// encodeBase64 renders raw bytes as standard base64 (matching the "b" wire form).
func encodeBase64(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}
