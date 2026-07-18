package socketio

import (
	"testing"
	"time"
)

func TestBroadcastOperatorFlags(t *testing.T) {
	b := (&BroadcastOperator{}).Local().Timeout(2*time.Second).ExceptRoom("r1", "r2")
	if !b.local {
		t.Error("Local did not set the flag")
	}
	if b.timeout != 2*time.Second {
		t.Errorf("timeout = %v", b.timeout)
	}
	if len(b.exceptRooms) != 2 || b.exceptRooms[0] != "r1" || b.exceptRooms[1] != "r2" {
		t.Errorf("exceptRooms = %v", b.exceptRooms)
	}
}

func TestNamespaceExceptRoom(t *testing.T) {
	s := New()
	ns := s.Of("/")

	_, ca, a := newBoundSocket(t, s, "/")
	_, cb, b := newBoundSocket(t, s, "/")

	a.Join("skip")
	// b is not in "skip".

	ns.Except("skip").Emit("news", "hi")

	if names := eventNames(bufferedEvents(t, ca)); len(names) != 0 {
		t.Fatalf("socket in excepted room should not receive, got %v", names)
	}
	if names := eventNames(bufferedEvents(t, cb)); len(names) != 1 || names[0] != "news" {
		t.Fatalf("socket outside excepted room should receive, got %v", names)
	}
	_ = b
}

func TestServerInAlias(t *testing.T) {
	s := New()
	_, ca, a := newBoundSocket(t, s, "/")
	a.Join("room")

	s.In("room").Emit("ping")

	if names := eventNames(bufferedEvents(t, ca)); len(names) != 1 {
		t.Fatalf("In(room) should deliver, got %v", names)
	}
}

func TestSocketExceptExcludesSelfAndRoom(t *testing.T) {
	s := New()
	_, ca, a := newBoundSocket(t, s, "/")
	_, cb, b := newBoundSocket(t, s, "/")
	_, cc, c := newBoundSocket(t, s, "/")

	b.Join("muted")
	// a broadcasts to everyone except itself and the "muted" room (b).
	a.Except("muted").Emit("hello")

	if names := eventNames(bufferedEvents(t, ca)); len(names) != 0 {
		t.Fatalf("originating socket excluded, got %v", names)
	}
	if names := eventNames(bufferedEvents(t, cb)); len(names) != 0 {
		t.Fatalf("muted-room socket excluded, got %v", names)
	}
	if names := eventNames(bufferedEvents(t, cc)); len(names) != 1 {
		t.Fatalf("uninvolved socket should receive, got %v", names)
	}
	_, _ = b, c
}

func TestSocketInExcludesSelf(t *testing.T) {
	s := New()
	_, ca, a := newBoundSocket(t, s, "/")
	_, cb, b := newBoundSocket(t, s, "/")
	a.Join("room")
	b.Join("room")

	a.In("room").Emit("x")

	if names := eventNames(bufferedEvents(t, ca)); len(names) != 0 {
		t.Fatalf("self should be excluded from socket.In, got %v", names)
	}
	if names := eventNames(bufferedEvents(t, cb)); len(names) != 1 {
		t.Fatalf("other room member should receive, got %v", names)
	}
}
