package socketio

import (
	"sort"
	"testing"
)

func socketIDs(sockets []*Socket) []string {
	ids := make([]string, len(sockets))
	for i, s := range sockets {
		ids[i] = s.id
	}
	sort.Strings(ids)
	return ids
}

func TestMemoryAdapterRooms(t *testing.T) {
	a := newMemoryAdapter()
	s1 := &Socket{id: "a"}
	s2 := &Socket{id: "b"}
	a.Add("a", s1)
	a.Add("b", s2)

	if got, ok := a.Get("a"); !ok || got != s1 {
		t.Fatalf("Get(a) = %v, %v", got, ok)
	}
	if _, ok := a.Get("missing"); ok {
		t.Fatal("Get(missing) should report not found")
	}

	all := socketIDs(a.AllSockets())
	if len(all) != 2 || all[0] != "a" || all[1] != "b" {
		t.Fatalf("AllSockets = %v, want [a b]", all)
	}

	a.Join("a", "room1")
	a.Join("b", "room1")
	a.Join("a", "room2")

	if got := socketIDs(a.SocketsInRoom("room1")); len(got) != 2 {
		t.Fatalf("room1 = %v, want 2 members", got)
	}
	if got := socketIDs(a.SocketsInRoom("room2")); len(got) != 1 || got[0] != "a" {
		t.Fatalf("room2 = %v, want [a]", got)
	}

	// Leaving a room drops the member; emptying a room removes it.
	a.Leave("a", "room2")
	if got := a.SocketsInRoom("room2"); len(got) != 0 {
		t.Fatalf("room2 after leave = %v, want empty", got)
	}

	// Removing a socket drops it from every room and the socket table.
	a.Remove("a")
	if _, ok := a.Get("a"); ok {
		t.Fatal("socket a should be gone after Remove")
	}
	if got := socketIDs(a.SocketsInRoom("room1")); len(got) != 1 || got[0] != "b" {
		t.Fatalf("room1 after remove = %v, want [b]", got)
	}
}

func TestMemoryAdapterJoinUnknownSocket(t *testing.T) {
	a := newMemoryAdapter()
	// Joining a room with an unregistered socket creates the room but adds no
	// member.
	a.Join("ghost", "room")
	if got := a.SocketsInRoom("room"); len(got) != 0 {
		t.Fatalf("room = %v, want empty", got)
	}
}

func TestMemoryAdapterLeaveUnknownRoom(t *testing.T) {
	a := newMemoryAdapter()
	// Must not panic on a non-existent room.
	a.Leave("x", "nope")
}
