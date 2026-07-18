package engineio

import (
	"reflect"
	"testing"
)

func TestPacketConstructors(t *testing.T) {
	cases := []struct {
		got  Packet
		want Packet
		enc  string
	}{
		{NewPing("probe"), Packet{Type: Ping, Data: "probe"}, "2probe"},
		{NewPong("probe"), Packet{Type: Pong, Data: "probe"}, "3probe"},
		{NewClose(), Packet{Type: Close}, "1"},
		{NewUpgrade(), Packet{Type: Upgrade}, "5"},
		{NewNoop(), Packet{Type: Noop}, "6"},
		{NewMessage("hi"), Packet{Type: Message, Data: "hi"}, "4hi"},
	}
	for _, c := range cases {
		if !reflect.DeepEqual(c.got, c.want) {
			t.Errorf("constructor = %+v, want %+v", c.got, c.want)
		}
		if got := c.got.Encode(); got != c.enc {
			t.Errorf("Encode() = %q, want %q", got, c.enc)
		}
	}
}

func TestNewBinaryMessageAndIsBinary(t *testing.T) {
	p := NewBinaryMessage([]byte{0x01, 0x02})
	if p.Type != Message || !p.IsBinary() {
		t.Fatalf("NewBinaryMessage: %+v IsBinary=%v", p, p.IsBinary())
	}
	// base64 of {0x01,0x02} is "AQI=", prefixed with "b".
	if got := p.Encode(); got != "bAQI=" {
		t.Errorf("Encode() = %q, want bAQI=", got)
	}
	if NewMessage("text").IsBinary() {
		t.Error("text message should not report IsBinary")
	}

	// Round-trips through Decode as a binary Message.
	dec, err := Decode("bAQI=")
	if err != nil {
		t.Fatal(err)
	}
	if !dec.IsBinary() || !reflect.DeepEqual(dec.Binary, []byte{0x01, 0x02}) {
		t.Fatalf("decoded = %+v", dec)
	}
}
