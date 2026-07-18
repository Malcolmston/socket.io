package socketio

import (
	"reflect"
	"testing"
)

func TestIsReservedEvent(t *testing.T) {
	cases := map[string]bool{
		"connect":        true,
		"connect_error":  true,
		"disconnect":     true,
		"disconnecting":  true,
		"newListener":    true,
		"removeListener": true,
		"chat":           false,
		"":               false,
		"Connect":        false, // case-sensitive
	}
	for name, want := range cases {
		if got := IsReservedEvent(name); got != want {
			t.Errorf("IsReservedEvent(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestValidateEventName(t *testing.T) {
	if err := ValidateEventName(""); err != ErrEmptyEvent {
		t.Errorf("empty: err = %v, want ErrEmptyEvent", err)
	}
	if err := ValidateEventName("disconnect"); err != ErrReservedEvent {
		t.Errorf("reserved: err = %v, want ErrReservedEvent", err)
	}
	if err := ValidateEventName("message"); err != nil {
		t.Errorf("valid: err = %v, want nil", err)
	}
}

func TestReservedEventsSorted(t *testing.T) {
	got := ReservedEvents()
	want := []string{"connect", "connect_error", "disconnect", "disconnecting", "newListener", "removeListener"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReservedEvents() = %v, want %v", got, want)
	}
}
