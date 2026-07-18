package socketio

import (
	"testing"
)

func TestOnAnyReceivesEveryEvent(t *testing.T) {
	s := New()
	_, _, sock := newBoundSocket(t, s, "/")

	var seen []string
	sock.OnAny(func(event string, args []any) {
		seen = append(seen, event)
	})
	// A named handler still fires alongside the catch-all.
	named := 0
	sock.On("chat", func(args []any) []any { named++; return nil })

	sock.dispatch(newEvent("/", "chat", []any{"hi"}, nil))
	sock.dispatch(newEvent("/", "news", []any{"x"}, nil))

	if len(seen) != 2 || seen[0] != "chat" || seen[1] != "news" {
		t.Fatalf("catch-all saw %v, want [chat news]", seen)
	}
	if named != 1 {
		t.Fatalf("named handler fired %d times, want 1", named)
	}
}

func TestPrependAnyOrder(t *testing.T) {
	s := New()
	_, _, sock := newBoundSocket(t, s, "/")

	var order []string
	sock.OnAny(func(event string, args []any) { order = append(order, "second") })
	sock.PrependAny(func(event string, args []any) { order = append(order, "first") })

	sock.dispatch(newEvent("/", "ev", nil, nil))

	if len(order) != 2 || order[0] != "first" || order[1] != "second" {
		t.Fatalf("order = %v, want [first second]", order)
	}
}

func TestOffAnyClears(t *testing.T) {
	s := New()
	_, _, sock := newBoundSocket(t, s, "/")

	sock.OnAny(func(event string, args []any) {})
	if len(sock.ListenersAny()) != 1 {
		t.Fatalf("expected 1 catch-all listener")
	}
	sock.OffAny()
	if len(sock.ListenersAny()) != 0 {
		t.Fatalf("OffAny should clear listeners")
	}
}
