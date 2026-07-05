package socketio

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/malcolmston/socketio/engineio"
)

// fakeBroadcaster is an in-memory Broadcaster that echoes published messages
// straight back to the registered handler, emulating pub/sub loopback.
type fakeBroadcaster struct {
	mu        sync.Mutex
	handler   func([]byte)
	published [][]byte
	closed    bool
}

func (f *fakeBroadcaster) Publish(data []byte) error {
	f.mu.Lock()
	f.published = append(f.published, data)
	h := f.handler
	f.mu.Unlock()
	if h != nil {
		h(data)
	}
	return nil
}

func (f *fakeBroadcaster) OnMessage(h func([]byte)) {
	f.mu.Lock()
	f.handler = h
	f.mu.Unlock()
}

func (f *fakeBroadcaster) Close() error {
	f.mu.Lock()
	f.closed = true
	f.mu.Unlock()
	return nil
}

func TestSetBroadcasterAndClusterEmit(t *testing.T) {
	s := New()
	fb := &fakeBroadcaster{}
	if s.SetBroadcaster(fb) != s {
		t.Fatal("SetBroadcaster should return the server")
	}

	_, ca, a := newBoundSocket(t, s, "/")
	a.Join("news")

	// With a broadcaster installed, Emit publishes to the cluster, and the
	// loopback echo delivers it to local sockets.
	s.To("news").Emit("update", "hi")

	fb.mu.Lock()
	published := len(fb.published)
	fb.mu.Unlock()
	if published != 1 {
		t.Fatalf("expected 1 published message, got %d", published)
	}
	if names := eventNames(bufferedEvents(t, ca)); len(names) != 1 || names[0] != "update" {
		t.Fatalf("local socket events = %v", names)
	}
}

func TestHandleRemoteBroadcastUnknownNamespace(t *testing.T) {
	s := New()
	// Unmarshalling failure and unknown namespace are both no-ops (no panic).
	s.handleRemoteBroadcast([]byte("not json"))
	s.handleRemoteBroadcast([]byte(`{"nsp":"/missing","event":"x"}`))
}

func TestConnHandleConnectInvalidNamespace(t *testing.T) {
	s := New()
	c := s.newConn()
	c.handleMessage("0/nope,")
	pkts := bufferedEvents(t, c)
	if len(pkts) != 1 || pkts[0].Type != ConnectError {
		t.Fatalf("expected a CONNECT_ERROR, got %v", pkts)
	}
}

func TestConnHandleConnectSuccess(t *testing.T) {
	s := New()
	connected := make(chan *Socket, 1)
	s.OnConnection(func(sock *Socket) { connected <- sock })

	c := s.newConn()
	c.handleMessage("0")

	select {
	case <-connected:
	case <-time.After(time.Second):
		t.Fatal("connection handler not fired")
	}
	pkts := bufferedEvents(t, c)
	if len(pkts) != 1 || pkts[0].Type != Connect {
		t.Fatalf("expected a CONNECT ack, got %v", pkts)
	}
}

func TestConnHandleConnectMiddlewareRejects(t *testing.T) {
	s := New()
	s.Use(func(_ *Socket, next func(error)) { next(errAckTimeout) })

	c := s.newConn()
	c.handleMessage("0")
	pkts := bufferedEvents(t, c)
	if len(pkts) != 1 || pkts[0].Type != ConnectError {
		t.Fatalf("expected CONNECT_ERROR when middleware rejects, got %v", pkts)
	}
	if len(s.Sockets()) != 0 {
		t.Fatal("rejected socket should not remain in namespace")
	}
}

func TestConnHandleEventDispatch(t *testing.T) {
	s := New()
	got := make(chan []any, 1)
	s.OnConnection(func(sock *Socket) {
		sock.On("hi", func(args []any) []any { got <- args; return nil })
	})

	c := s.newConn()
	c.handleMessage("0")
	// Drain the connect ack.
	c.mu.Lock()
	c.outbuf = nil
	c.mu.Unlock()

	c.handleMessage(`2["hi","there"]`)
	select {
	case args := <-got:
		if len(args) != 1 || args[0] != "there" {
			t.Fatalf("args = %v", args)
		}
	case <-time.After(time.Second):
		t.Fatal("event handler not invoked")
	}
}

func TestConnHandleAck(t *testing.T) {
	s := New()
	_, c, sock := newBoundSocket(t, s, "/")

	got := make(chan []any, 1)
	if err := sock.EmitWithAck("do", func(reply []any) { got <- reply }); err != nil {
		t.Fatal(err)
	}
	pkts := bufferedEvents(t, c)
	if len(pkts) == 0 || pkts[0].ID == nil {
		t.Fatalf("no ack id emitted: %v", pkts)
	}
	// Client answers the ack.
	c.handleMessage("3" + strconv.FormatUint(*pkts[0].ID, 10) + `["ok"]`)

	select {
	case reply := <-got:
		if len(reply) != 1 || reply[0] != "ok" {
			t.Fatalf("reply = %v", reply)
		}
	case <-time.After(time.Second):
		t.Fatal("ack callback not invoked")
	}
}

func TestConnHandleDisconnect(t *testing.T) {
	s := New()
	ns, c, sock := newBoundSocket(t, s, "/")

	fired := make(chan string, 1)
	sock.OnDisconnect(func(reason string) { fired <- reason })

	c.handleMessage("1")
	select {
	case reason := <-fired:
		if reason != "client namespace disconnect" {
			t.Fatalf("reason = %q", reason)
		}
	case <-time.After(time.Second):
		t.Fatal("disconnect handler not fired")
	}
	if len(ns.Sockets()) != 0 {
		t.Fatal("socket should be removed on disconnect")
	}
}

func TestConnHandleEnginePacketPing(t *testing.T) {
	s := New()
	c := s.newConn()
	c.handleEnginePacket(engineio.Packet{Type: engineio.Ping})
	c.mu.Lock()
	n := len(c.outbuf)
	c.mu.Unlock()
	if n == 0 {
		t.Fatal("a ping should be answered with a pong")
	}
}

func TestConnSendAfterClose(t *testing.T) {
	s := New()
	c := s.newConn()
	c.cleanup()
	// send on a closed conn is a no-op and must not panic.
	c.send(engineio.Packet{Type: engineio.Ping})
	c.mu.Lock()
	n := len(c.outbuf)
	c.mu.Unlock()
	if n != 0 {
		t.Fatalf("closed conn buffered %d packets, want 0", n)
	}
}
