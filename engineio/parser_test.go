package engineio

import (
	"reflect"
	"testing"
)

func TestEncodeDecodeString(t *testing.T) {
	cases := []struct {
		packet Packet
		wire   string
	}{
		{Packet{Type: Open, Data: `{"sid":"abc"}`}, `0{"sid":"abc"}`},
		{Packet{Type: Message, Data: "2[\"ev\"]"}, `42["ev"]`},
		{Packet{Type: Ping}, "2"},
		{Packet{Type: Pong}, "3"},
		{Packet{Type: Close}, "1"},
		{Packet{Type: Noop}, "6"},
	}
	for _, c := range cases {
		if got := c.packet.Encode(); got != c.wire {
			t.Errorf("Encode(%v) = %q, want %q", c.packet, got, c.wire)
		}
		dec, err := Decode(c.wire)
		if err != nil {
			t.Fatalf("Decode(%q): %v", c.wire, err)
		}
		if dec.Type != c.packet.Type || dec.Data != c.packet.Data {
			t.Errorf("Decode(%q) = %+v, want %+v", c.wire, dec, c.packet)
		}
	}
}

func TestBinaryRoundTrip(t *testing.T) {
	p := Packet{Type: Message, Binary: []byte{0x00, 0x01, 0xff, 0x7e}}
	wire := p.Encode()
	if wire[0] != 'b' {
		t.Fatalf("binary packet should start with 'b', got %q", wire)
	}
	dec, err := Decode(wire)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(dec.Binary, p.Binary) {
		t.Fatalf("binary round trip = %v, want %v", dec.Binary, p.Binary)
	}
}

func TestPayloadRoundTrip(t *testing.T) {
	packets := []Packet{
		{Type: Message, Data: "0"},
		{Type: Message, Data: `2["hello","world"]`},
		{Type: Ping},
	}
	payload := EncodePayload(packets)
	// Packets must be separated by 0x1e.
	if want := "40\x1e42[\"hello\",\"world\"]\x1e2"; payload != want {
		t.Fatalf("EncodePayload = %q, want %q", payload, want)
	}
	got, err := DecodePayload(payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(packets) {
		t.Fatalf("decoded %d packets, want %d", len(got), len(packets))
	}
	for i := range packets {
		if got[i].Type != packets[i].Type || got[i].Data != packets[i].Data {
			t.Errorf("packet %d = %+v, want %+v", i, got[i], packets[i])
		}
	}
}

func TestDecodeErrors(t *testing.T) {
	if _, err := Decode(""); err == nil {
		t.Error("expected error for empty packet")
	}
	if _, err := Decode("9foo"); err == nil {
		t.Error("expected error for invalid packet type")
	}
}
