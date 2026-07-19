package engineio

// Upstream-parity known-answer tests for the Engine.IO v4 packet/payload codec,
// transcribed from the ORIGINAL engine.io-parser JavaScript/TypeScript suite.
//
// Source of truth (github.com/socketio/engine.io-parser, branch "master",
// package version 5.0.3):
//
//   - https://raw.githubusercontent.com/socketio/engine.io-parser/master/test/index.ts
//       * "should encode/decode a string": {message,"test"} <-> "4test"
//       * "should fail to decode a malformed packet": "" and "a123" -> error
//       * "should encode/decode all packet types":
//             [open, close, ping"probe", pong"probe", message"test"]
//             <-> "0\x1e1\x1e2probe\x1e3probe\x1e4test"
//       * "should fail to decode a malformed payload": "{", "{}",
//             '["a123", "a456"]' -> error
//   - https://raw.githubusercontent.com/socketio/engine.io-parser/master/test/node.ts
//       * "should encode/decode a Buffer as base64":
//             Buffer[1,2,3,4] <-> "bAQIDBA=="
//       * "should encode/decode a string + Buffer payload":
//             [message"test", message Buffer[1,2,3,4]] <-> "4test\x1ebAQIDBA=="
//   - https://raw.githubusercontent.com/socketio/engine.io-parser/master/lib/commons.ts
//       * the canonical PACKET_TYPES table: open=0 close=1 ping=2 pong=3
//             message=4 upgrade=5 noop=6
//   - https://raw.githubusercontent.com/socketio/engine.io-parser/master/lib/decodePacket.ts
//       * decodePacket of a single-char string ("0") yields {type} with no data
//       * an unknown type char yields the ERROR_PACKET
//
// The wire strings below are the exact values the JavaScript suite asserts.

import (
	"bytes"
	"reflect"
	"testing"
)

// TestParityUpstreamStringPacket mirrors index.ts "should encode/decode a
// string": {message,"test"} <-> "4test".
func TestParityUpstreamStringPacket(t *testing.T) {
	p := Packet{Type: Message, Data: "test"}
	if got := p.Encode(); got != "4test" {
		t.Fatalf("Encode = %q, want %q", got, "4test")
	}
	dec, err := Decode("4test")
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if dec.Type != Message || dec.Data != "test" || dec.Binary != nil {
		t.Fatalf("Decode = %+v, want {Message test}", dec)
	}
}

// TestParityUpstreamPacketTypeTable pins the canonical PACKET_TYPES mapping from
// commons.ts: each packet type encodes to its single ASCII digit prefix, and
// decodes back to the same type. This is the table every other vector rests on.
func TestParityUpstreamPacketTypeTable(t *testing.T) {
	cases := []struct {
		typ  PacketType
		data string
		wire string
	}{
		{Open, "", "0"},
		{Close, "", "1"},
		{Ping, "probe", "2probe"},
		{Pong, "probe", "3probe"},
		{Message, "test", "4test"},
		{Upgrade, "", "5"},
		{Noop, "", "6"},
	}
	for _, c := range cases {
		p := Packet{Type: c.typ, Data: c.data}
		if got := p.Encode(); got != c.wire {
			t.Errorf("Encode(%s,%q) = %q, want %q", c.typ, c.data, got, c.wire)
		}
		dec, err := Decode(c.wire)
		if err != nil {
			t.Errorf("Decode(%q): %v", c.wire, err)
			continue
		}
		if dec.Type != c.typ || dec.Data != c.data {
			t.Errorf("Decode(%q) = %+v, want {%s %q}", c.wire, dec, c.typ, c.data)
		}
	}
}

// TestParityUpstreamDecodeSingleChar mirrors decodePacket.ts: a single-character
// encoded packet ("0") decodes to just a typed packet with empty data.
func TestParityUpstreamDecodeSingleChar(t *testing.T) {
	dec, err := Decode("0")
	if err != nil {
		t.Fatalf("Decode(%q): %v", "0", err)
	}
	if dec.Type != Open || dec.Data != "" || dec.Binary != nil {
		t.Fatalf("Decode(%q) = %+v, want {Open}", "0", dec)
	}
}

// TestParityUpstreamMalformedPacket mirrors index.ts "should fail to decode a
// malformed packet": "" and "a123" are parser errors. (Upstream returns an
// {error,"parser error"} packet; the Go codec surfaces the same condition as a
// decode error.)
func TestParityUpstreamMalformedPacket(t *testing.T) {
	for _, s := range []string{"", "a123"} {
		if _, err := Decode(s); err == nil {
			t.Errorf("Decode(%q) = nil error, want error", s)
		}
	}
}

// TestParityUpstreamPayloadAllTypes mirrors index.ts "should encode/decode all
// packet types".
func TestParityUpstreamPayloadAllTypes(t *testing.T) {
	packets := []Packet{
		{Type: Open},
		{Type: Close},
		{Type: Ping, Data: "probe"},
		{Type: Pong, Data: "probe"},
		{Type: Message, Data: "test"},
	}
	const want = "0\x1e1\x1e2probe\x1e3probe\x1e4test"
	if got := EncodePayload(packets); got != want {
		t.Fatalf("EncodePayload = %q, want %q", got, want)
	}
	dec, err := DecodePayload(want)
	if err != nil {
		t.Fatalf("DecodePayload: %v", err)
	}
	if !reflect.DeepEqual(dec, packets) {
		t.Fatalf("DecodePayload = %+v, want %+v", dec, packets)
	}
}

// TestParityUpstreamMalformedPayload mirrors index.ts "should fail to decode a
// malformed payload": "{", "{}", and '["a123", "a456"]' are all rejected.
func TestParityUpstreamMalformedPayload(t *testing.T) {
	for _, s := range []string{"{", "{}", `["a123", "a456"]`} {
		if _, err := DecodePayload(s); err == nil {
			t.Errorf("DecodePayload(%q) = nil error, want error", s)
		}
	}
}

// TestParityUpstreamBufferBase64 mirrors node.ts "should encode/decode a Buffer
// as base64": Buffer[1,2,3,4] <-> "bAQIDBA==".
func TestParityUpstreamBufferBase64(t *testing.T) {
	p := Packet{Type: Message, Binary: []byte{1, 2, 3, 4}}
	if got := p.Encode(); got != "bAQIDBA==" {
		t.Fatalf("Encode = %q, want %q", got, "bAQIDBA==")
	}
	dec, err := Decode("bAQIDBA==")
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if dec.Type != Message || !bytes.Equal(dec.Binary, []byte{1, 2, 3, 4}) {
		t.Fatalf("Decode = %+v, want Message binary [1 2 3 4]", dec)
	}
}

// TestParityUpstreamStringBufferPayload mirrors node.ts "should encode/decode a
// string + Buffer payload": the mixed payload is "4test\x1ebAQIDBA==".
func TestParityUpstreamStringBufferPayload(t *testing.T) {
	packets := []Packet{
		{Type: Message, Data: "test"},
		{Type: Message, Binary: []byte{1, 2, 3, 4}},
	}
	const want = "4test\x1ebAQIDBA=="
	if got := EncodePayload(packets); got != want {
		t.Fatalf("EncodePayload = %q, want %q", got, want)
	}
	dec, err := DecodePayload(want)
	if err != nil {
		t.Fatalf("DecodePayload: %v", err)
	}
	if len(dec) != 2 {
		t.Fatalf("len = %d, want 2", len(dec))
	}
	if dec[0].Type != Message || dec[0].Data != "test" {
		t.Errorf("packet 0 = %+v", dec[0])
	}
	if dec[1].Type != Message || !bytes.Equal(dec[1].Binary, []byte{1, 2, 3, 4}) {
		t.Errorf("packet 1 = %+v", dec[1])
	}
}
