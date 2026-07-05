package socketio

import (
	"reflect"
	"sort"
	"testing"
)

func TestBroadcastOperatorChaining(t *testing.T) {
	b := &BroadcastOperator{}
	// Every builder method returns the operator for fluent chaining.
	got := b.Volatile().Compress(true).In("r1").To("r2").Except("x").Except("y")
	if got != b {
		t.Fatal("chaining should return the same operator")
	}
	if !b.volatile {
		t.Fatal("Volatile did not set the flag")
	}
	if !b.compress {
		t.Fatal("Compress(true) did not set the flag")
	}
	if !reflect.DeepEqual(b.rooms, []string{"r1", "r2"}) {
		t.Fatalf("rooms = %v, want [r1 r2]", b.rooms)
	}
	if _, ok := b.except["x"]; !ok {
		t.Fatal("Except(x) missing")
	}
	if _, ok := b.except["y"]; !ok {
		t.Fatal("Except(y) missing")
	}

	b.Compress(false)
	if b.compress {
		t.Fatal("Compress(false) should clear the flag")
	}
}

func TestExceptKeys(t *testing.T) {
	keys := exceptKeys(map[string]struct{}{"a": {}, "b": {}})
	sort.Strings(keys)
	if !reflect.DeepEqual(keys, []string{"a", "b"}) {
		t.Fatalf("exceptKeys = %v", keys)
	}
	if got := exceptKeys(nil); len(got) != 0 {
		t.Fatalf("exceptKeys(nil) = %v, want empty", got)
	}
}

func TestBroadcastEmitToRoom(t *testing.T) {
	s := New()
	ns := s.Of("/")

	_, ca, a := newBoundSocket(t, s, "/")
	_, cb, b := newBoundSocket(t, s, "/")
	_, cc, c := newBoundSocket(t, s, "/")

	a.Join("news")
	b.Join("news")
	// c is not in the room.

	ns.To("news").Emit("update", "hello")

	if names := eventNames(bufferedEvents(t, ca)); len(names) != 1 || names[0] != "update" {
		t.Fatalf("a events = %v", names)
	}
	if names := eventNames(bufferedEvents(t, cb)); len(names) != 1 || names[0] != "update" {
		t.Fatalf("b events = %v", names)
	}
	if names := eventNames(bufferedEvents(t, cc)); len(names) != 0 {
		t.Fatalf("c should not receive room broadcast, got %v", names)
	}
	_ = c
}

func TestBroadcastEmitExcludesSocket(t *testing.T) {
	s := New()
	_, ca, a := newBoundSocket(t, s, "/")
	_, cb, b := newBoundSocket(t, s, "/")

	// a.Broadcast() targets everyone in the namespace except a.
	a.Broadcast().Emit("ping")

	if names := eventNames(bufferedEvents(t, ca)); len(names) != 0 {
		t.Fatalf("originating socket should be excluded, got %v", names)
	}
	if names := eventNames(bufferedEvents(t, cb)); len(names) != 1 {
		t.Fatalf("b should receive broadcast, got %v", names)
	}
	_ = b
}

func TestBroadcastToDeduplicatesAcrossRooms(t *testing.T) {
	s := New()
	ns := s.Of("/")
	_, ca, a := newBoundSocket(t, s, "/")

	a.Join("r1", "r2")
	// Socket is in both rooms; a broadcast to both must deliver only once.
	ns.To("r1").To("r2").Emit("x")

	if names := eventNames(bufferedEvents(t, ca)); len(names) != 1 {
		t.Fatalf("expected exactly one delivery, got %v", names)
	}
}
