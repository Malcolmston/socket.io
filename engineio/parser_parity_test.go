package engineio

// Upstream-parity tests for the Engine.IO v4 packet/payload codec.
//
// The vectors are transcribed from engine.io-parser's own test suite, now
// living in the socket.io monorepo at packages/engine.io-parser/test
// (github.com/socketio/socket.io, branch main):
//
//   - test/index.ts — the string single-packet round-trip, the malformed-packet
//     cases, the "all packet types" payload, and the malformed-payload cases.
//   - test/node.ts  — the Buffer→base64 single packet and the string+Buffer
//     mixed payload.
//
// The exact wire strings the JavaScript suite asserts are reproduced here as
// known-answer values.

import (
	"bytes"
	"reflect"
	"testing"
)

// TestParityEIOStringPacket mirrors index.ts "should encode/decode a string":
// {message,"test"} <-> "4test".
func TestParityEIOStringPacket(t *testing.T) {
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

// TestParityEIOMalformedPacket mirrors index.ts "should fail to decode a
// malformed packet": "" and "a123" are both parser errors.
func TestParityEIOMalformedPacket(t *testing.T) {
	for _, s := range []string{"", "a123"} {
		if _, err := Decode(s); err == nil {
			t.Errorf("Decode(%q) = nil error, want error", s)
		}
	}
}

// TestParityEIOPayloadAllTypes mirrors index.ts "should encode/decode all packet
// types": the five text packet types join into
// "0\x1e1\x1e2probe\x1e3probe\x1e4test".
func TestParityEIOPayloadAllTypes(t *testing.T) {
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

// TestParityEIOMalformedPayload mirrors index.ts "should fail to decode a
// malformed payload": "{", "{}", and '["a123", "a456"]' are all rejected. (In
// the JS suite these decode to a single {error,"parser error"} packet; the Go
// codec surfaces the same condition as a decode error.)
func TestParityEIOMalformedPayload(t *testing.T) {
	for _, s := range []string{"{", "{}", `["a123", "a456"]`} {
		if _, err := DecodePayload(s); err == nil {
			t.Errorf("DecodePayload(%q) = nil error, want error", s)
		}
	}
}

// TestParityEIOBufferBase64 mirrors node.ts "should encode/decode a Buffer as
// base64": Buffer[1,2,3,4] <-> "bAQIDBA==".
func TestParityEIOBufferBase64(t *testing.T) {
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

// TestParityEIOStringBufferPayload mirrors node.ts "should encode/decode a
// string + Buffer payload": the mixed payload is "4test\x1ebAQIDBA==".
func TestParityEIOStringBufferPayload(t *testing.T) {
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
