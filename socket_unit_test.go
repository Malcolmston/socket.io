package socketio

import (
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/malcolmston/socketio/engineio"
)

// newBoundSocket creates a server, an engine.io session, and a socket bound to
// the given namespace, all without any real transport. Outgoing packets are
// buffered on the conn's outbuf where tests can inspect them.
func newBoundSocket(t *testing.T, s *Server, nsName string) (*Namespace, *conn, *Socket) {
	t.Helper()
	ns := s.Of(nsName)
	c := s.newConn()
	sock := ns.add(c, nsName, nil)
	c.mu.Lock()
	c.sockets[ns.name] = sock
	c.mu.Unlock()
	return ns, c, sock
}

// bufferedEvents decodes the Socket.IO EVENT packets buffered on a conn.
func bufferedEvents(t *testing.T, c *conn) []Packet {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []Packet
	for _, ep := range c.outbuf {
		if ep.Type != engineio.Message || ep.Data == "" {
			continue
		}
		pkt, err := DecodePacket(ep.Data)
		if err != nil {
			continue
		}
		out = append(out, pkt)
	}
	return out
}

func eventNames(pkts []Packet) []string {
	var names []string
	for _, p := range pkts {
		if p.Type == Event {
			names = append(names, p.EventName())
		}
	}
	return names
}

func TestSocketDataStore(t *testing.T) {
	s := &Socket{}

	if got := s.GetString("missing"); got != "" {
		t.Fatalf("GetString(missing) = %q, want empty", got)
	}
	if _, ok := s.Get("missing"); ok {
		t.Fatal("Get(missing) reported present")
	}

	// Set is chainable and persists values.
	if s.Set("user", "alice").Set("count", 42) != s {
		t.Fatal("Set should return the same socket for chaining")
	}
	if v, ok := s.Get("user"); !ok || v != "alice" {
		t.Fatalf("Get(user) = %v, %v", v, ok)
	}
	if got := s.GetString("user"); got != "alice" {
		t.Fatalf("GetString(user) = %q", got)
	}
	// GetString on a non-string value yields "".
	if got := s.GetString("count"); got != "" {
		t.Fatalf("GetString(count) = %q, want empty for non-string", got)
	}

	data := s.Data()
	if !reflect.DeepEqual(data, map[string]any{"user": "alice", "count": 42}) {
		t.Fatalf("Data() = %v", data)
	}
	// Data returns a copy: mutating it must not affect the socket.
	data["user"] = "mallory"
	if got := s.GetString("user"); got != "alice" {
		t.Fatalf("Data() should return a copy; socket now %q", got)
	}

	if s.Delete("user") != s {
		t.Fatal("Delete should return the same socket")
	}
	if _, ok := s.Get("user"); ok {
		t.Fatal("user should be gone after Delete")
	}
}

func TestSocketIDAndAuth(t *testing.T) {
	s := &Socket{id: "sock-1", auth: map[string]any{"token": "abc"}}
	if s.ID() != "sock-1" {
		t.Fatalf("ID() = %q", s.ID())
	}
	auth, ok := s.Auth().(map[string]any)
	if !ok || auth["token"] != "abc" {
		t.Fatalf("Auth() = %v", s.Auth())
	}
}

func TestSocketRoomsJoinLeave(t *testing.T) {
	s := New()
	ns, _, sock := newBoundSocket(t, s, "/")

	if sock.Namespace() != ns {
		t.Fatal("Namespace() mismatch")
	}

	// A freshly added socket is in its own id-room.
	rooms := sock.Rooms()
	if len(rooms) != 1 || rooms[0] != sock.id {
		t.Fatalf("initial rooms = %v, want [%s]", rooms, sock.id)
	}

	sock.Join("room1", "room2")
	got := sock.Rooms()
	sort.Strings(got)
	want := []string{"room1", "room2", sock.id}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rooms after join = %v, want %v", got, want)
	}
	if members := ns.SocketsInRoom("room1"); len(members) != 1 || members[0] != sock {
		t.Fatalf("room1 members = %v", members)
	}

	sock.Leave("room1")
	got = sock.Rooms()
	sort.Strings(got)
	want = []string{"room2", sock.id}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rooms after leave = %v, want %v", got, want)
	}
	if members := ns.SocketsInRoom("room1"); len(members) != 0 {
		t.Fatalf("room1 should be empty after leave, got %v", members)
	}
}

func TestSocketEmit(t *testing.T) {
	s := New()
	_, c, sock := newBoundSocket(t, s, "/")

	if err := sock.Emit("hello", "world", 42); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	pkts := bufferedEvents(t, c)
	if len(pkts) != 1 {
		t.Fatalf("expected 1 buffered packet, got %d", len(pkts))
	}
	if pkts[0].EventName() != "hello" {
		t.Fatalf("event = %q", pkts[0].EventName())
	}
	if args := pkts[0].Args(); len(args) != 2 || args[0] != "world" {
		t.Fatalf("args = %v", args)
	}
}

func TestSocketToAndBroadcast(t *testing.T) {
	s := New()
	_, _, sock := newBoundSocket(t, s, "/")

	op := sock.To("room1")
	if op.rooms[0] != "room1" {
		t.Fatalf("To rooms = %v", op.rooms)
	}
	if _, excluded := op.except[sock.id]; !excluded {
		t.Fatal("To should exclude the originating socket")
	}

	bop := sock.Broadcast()
	if len(bop.rooms) != 0 {
		t.Fatalf("Broadcast rooms = %v, want none", bop.rooms)
	}
	if _, excluded := bop.except[sock.id]; !excluded {
		t.Fatal("Broadcast should exclude the originating socket")
	}
}

func TestSocketEmitWithAckResolve(t *testing.T) {
	s := New()
	_, c, sock := newBoundSocket(t, s, "/")

	var got []any
	if err := sock.EmitWithAck("do", func(reply []any) { got = reply }, "arg"); err != nil {
		t.Fatalf("EmitWithAck: %v", err)
	}
	pkts := bufferedEvents(t, c)
	if len(pkts) != 1 || pkts[0].ID == nil {
		t.Fatalf("expected event with ack id, got %v", pkts)
	}
	id := *pkts[0].ID

	sock.resolveAck(id, []any{"done"})
	if len(got) != 1 || got[0] != "done" {
		t.Fatalf("ack callback got %v", got)
	}
	// Resolving an unknown id must be a no-op (no panic).
	sock.resolveAck(9999, []any{"ignored"})
}

func TestSocketEmitAckTimeout(t *testing.T) {
	s := New()
	_, _, sock := newBoundSocket(t, s, "/")

	_, err := sock.EmitAck("do", 20*time.Millisecond)
	if err != errAckTimeout {
		t.Fatalf("EmitAck err = %v, want %v", err, errAckTimeout)
	}
}

func TestSocketEmitAckSuccess(t *testing.T) {
	s := New()
	_, c, sock := newBoundSocket(t, s, "/")

	done := make(chan []any, 1)
	go func() {
		reply, err := sock.EmitAck("do", time.Second)
		if err != nil {
			t.Errorf("EmitAck err = %v", err)
		}
		done <- reply
	}()

	// Wait for the pending ack to register, then resolve it.
	var id uint64
	deadline := time.Now().Add(time.Second)
	for {
		pkts := bufferedEvents(t, c)
		if len(pkts) > 0 && pkts[0].ID != nil {
			id = *pkts[0].ID
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("event never buffered")
		}
		time.Sleep(time.Millisecond)
	}
	sock.resolveAck(id, []any{"ok"})

	select {
	case reply := <-done:
		if len(reply) != 1 || reply[0] != "ok" {
			t.Fatalf("reply = %v", reply)
		}
	case <-time.After(time.Second):
		t.Fatal("EmitAck did not return")
	}
}

func TestSocketOnOffDispatch(t *testing.T) {
	s := New()
	_, _, sock := newBoundSocket(t, s, "/")

	calls := 0
	sock.On("ev", func(args []any) []any { calls++; return nil })
	sock.dispatch(Packet{Type: Event, Namespace: "/", Data: []any{"ev", "x"}})
	if calls != 1 {
		t.Fatalf("handler calls = %d, want 1", calls)
	}
	sock.Off("ev")
	sock.dispatch(Packet{Type: Event, Namespace: "/", Data: []any{"ev", "x"}})
	if calls != 1 {
		t.Fatalf("handler still fired after Off: calls = %d", calls)
	}
}

func TestSocketDispatchAck(t *testing.T) {
	s := New()
	_, c, sock := newBoundSocket(t, s, "/")

	sock.On("q", func(args []any) []any { return []any{"a"} })
	id := uint64(3)
	sock.dispatch(Packet{Type: Event, Namespace: "/", ID: &id, Data: []any{"q"}})

	pkts := bufferedEvents(t, c)
	if len(pkts) != 1 || pkts[0].Type != Ack {
		t.Fatalf("expected an ACK packet, got %v", pkts)
	}
	if pkts[0].ID == nil || *pkts[0].ID != 3 {
		t.Fatalf("ack id = %v", pkts[0].ID)
	}
}

func TestSocketDisconnectFiresHandlerOnce(t *testing.T) {
	s := New()
	ns, _, sock := newBoundSocket(t, s, "/")

	reasons := 0
	sock.OnDisconnect(func(reason string) { reasons++ })

	sock.Disconnect(false)
	if reasons != 1 {
		t.Fatalf("disconnect handler fired %d times, want 1", reasons)
	}
	if len(ns.Sockets()) != 0 {
		t.Fatalf("socket should be removed from namespace, have %d", len(ns.Sockets()))
	}
	// A second fireDisconnect must not re-invoke handlers.
	sock.fireDisconnect("again")
	if reasons != 1 {
		t.Fatalf("handler fired again: %d", reasons)
	}
}
