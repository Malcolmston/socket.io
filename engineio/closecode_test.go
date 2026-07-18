package engineio

import (
	"reflect"
	"testing"
)

func TestCloseCodeString(t *testing.T) {
	cases := map[CloseCode]string{
		CloseNormalClosure:   "normal closure",
		CloseGoingAway:       "going away",
		CloseMessageTooBig:   "message too big",
		CloseTryAgainLater:   "try again later",
		CloseCode(4000):      "close code 4000",
		CloseAbnormalClosure: "abnormal closure",
	}
	for code, want := range cases {
		if got := code.String(); got != want {
			t.Errorf("CloseCode(%d).String() = %q, want %q", code, got, want)
		}
	}
}

func TestCloseCodeIsValid(t *testing.T) {
	cases := map[CloseCode]bool{
		CloseNormalClosure:    true,
		CloseTryAgainLater:    true,
		CloseNoStatusReceived: false, // 1005 reserved
		CloseAbnormalClosure:  false, // 1006 reserved
		CloseTLSHandshake:     false, // 1015 reserved
		CloseCode(999):        false,
		CloseCode(1014):       false,
		CloseCode(2000):       false,
		CloseCode(3000):       true, // library use
		CloseCode(4999):       true, // application use
		CloseCode(5000):       false,
	}
	for code, want := range cases {
		if got := code.IsValid(); got != want {
			t.Errorf("CloseCode(%d).IsValid() = %v, want %v", code, got, want)
		}
	}
}

func TestEncodeDecodeCloseFrame(t *testing.T) {
	payload := EncodeCloseFrame(CloseMessageTooBig, "too big")
	want := []byte{0x03, 0xf1, 't', 'o', 'o', ' ', 'b', 'i', 'g'} // 1009 = 0x03f1
	if !reflect.DeepEqual(payload, want) {
		t.Fatalf("EncodeCloseFrame = %v, want %v", payload, want)
	}
	code, reason, err := DecodeCloseFrame(payload)
	if err != nil {
		t.Fatal(err)
	}
	if code != CloseMessageTooBig || reason != "too big" {
		t.Fatalf("decoded = (%d, %q)", code, reason)
	}
}

func TestDecodeCloseFrameEdgeCases(t *testing.T) {
	// Empty payload -> no status received, no error.
	code, reason, err := DecodeCloseFrame(nil)
	if err != nil || code != CloseNoStatusReceived || reason != "" {
		t.Fatalf("empty: (%d, %q, %v)", code, reason, err)
	}
	// Single byte -> invalid.
	if _, _, err := DecodeCloseFrame([]byte{0x03}); err != ErrInvalidCloseFrame {
		t.Fatalf("one byte: err = %v, want ErrInvalidCloseFrame", err)
	}
	// Code-only frame (no reason).
	code, reason, err = DecodeCloseFrame(EncodeCloseFrame(CloseNormalClosure, ""))
	if err != nil || code != CloseNormalClosure || reason != "" {
		t.Fatalf("code only: (%d, %q, %v)", code, reason, err)
	}
}
