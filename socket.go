package socketio

import (
	"errors"
	"sync"
	"time"
)

// EventHandler handles an inbound event. It receives the event arguments and
// may return a non-nil slice to acknowledge the event (sent back to the client
// when the event requested an ack).
type EventHandler func(args []any) []any

// Socket is a single client connection to a namespace. It is the primary object
// applications interact with — registering event handlers, emitting events, and
// joining rooms.
type Socket struct {
	id        string
	namespace *Namespace
	conn      *conn
	auth      any

	mu                 sync.Mutex
	handlers           map[string][]EventHandler
	rooms              map[string]struct{}
	disconnectHandlers []func(reason string)
	ackCounter         uint64
	pendingAcks        map[uint64]func([]any)
	disconnected       bool
	data               map[string]any
}

// ID returns the socket's unique identifier.
func (s *Socket) ID() string { return s.id }

// Auth returns the authentication payload the client sent with CONNECT.
func (s *Socket) Auth() any { return s.auth }

// Set stores an arbitrary value on the socket, persisting for the lifetime of
// the connection — the equivalent of Socket.IO's socket.data. Use it to attach
// session-like state (the authenticated user, a tenant id, ...).
func (s *Socket) Set(key string, value any) *Socket {
	s.mu.Lock()
	if s.data == nil {
		s.data = make(map[string]any)
	}
	s.data[key] = value
	s.mu.Unlock()
	return s
}

// Get returns a value previously stored with Set, and whether it was present.
func (s *Socket) Get(key string) (any, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.data[key]
	return v, ok
}

// GetString returns a stored string value, or "" if missing / not a string.
func (s *Socket) GetString(key string) string {
	if v, ok := s.Get(key); ok {
		if str, ok := v.(string); ok {
			return str
		}
	}
	return ""
}

// Delete removes a stored value.
func (s *Socket) Delete(key string) *Socket {
	s.mu.Lock()
	delete(s.data, key)
	s.mu.Unlock()
	return s
}

// Data returns a copy of all values stored on the socket.
func (s *Socket) Data() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]any, len(s.data))
	for k, v := range s.data {
		out[k] = v
	}
	return out
}

// Namespace returns the namespace this socket belongs to.
func (s *Socket) Namespace() *Namespace { return s.namespace }

// On registers a handler for an event.
func (s *Socket) On(event string, handler EventHandler) *Socket {
	s.mu.Lock()
	s.handlers[event] = append(s.handlers[event], handler)
	s.mu.Unlock()
	return s
}

// Off removes all handlers for an event.
func (s *Socket) Off(event string) *Socket {
	s.mu.Lock()
	delete(s.handlers, event)
	s.mu.Unlock()
	return s
}

// OnDisconnect registers a callback invoked when the socket disconnects.
func (s *Socket) OnDisconnect(fn func(reason string)) *Socket {
	s.mu.Lock()
	s.disconnectHandlers = append(s.disconnectHandlers, fn)
	s.mu.Unlock()
	return s
}

// Emit sends an event to this socket.
func (s *Socket) Emit(event string, args ...any) error {
	return s.conn.sendPacket(newEvent(s.namespace.name, event, args, nil))
}

// EmitWithAck sends an event and invokes ackFn with the client's acknowledgement
// arguments.
func (s *Socket) EmitWithAck(event string, ackFn func(args []any), args ...any) error {
	s.mu.Lock()
	id := s.ackCounter
	s.ackCounter++
	s.pendingAcks[id] = ackFn
	s.mu.Unlock()
	return s.conn.sendPacket(newEvent(s.namespace.name, event, args, &id))
}

// EmitAck sends an event and blocks until the client acknowledges it or timeout
// elapses, returning the acknowledgement arguments.
func (s *Socket) EmitAck(event string, timeout time.Duration, args ...any) ([]any, error) {
	ch := make(chan []any, 1)
	if err := s.EmitWithAck(event, func(reply []any) { ch <- reply }, args...); err != nil {
		return nil, err
	}
	select {
	case reply := <-ch:
		return reply, nil
	case <-time.After(timeout):
		return nil, errAckTimeout
	}
}

var errAckTimeout = errors.New("socketio: ack timeout")

// dispatch delivers an inbound event to the registered handlers, replying with
// an ACK when the packet requested one.
func (s *Socket) dispatch(pkt Packet) {
	s.mu.Lock()
	handlers := append([]EventHandler(nil), s.handlers[pkt.EventName()]...)
	s.mu.Unlock()

	var ackData []any
	for _, h := range handlers {
		if reply := h(pkt.Args()); reply != nil && ackData == nil {
			ackData = reply
		}
	}
	if pkt.ID != nil {
		reply := ackData
		if reply == nil {
			reply = []any{}
		}
		_ = s.conn.sendPacket(Packet{
			Type:      Ack,
			Namespace: s.namespace.name,
			ID:        pkt.ID,
			Data:      reply,
		})
	}
}

// resolveAck invokes and clears a pending ack callback.
func (s *Socket) resolveAck(id uint64, args []any) {
	s.mu.Lock()
	fn := s.pendingAcks[id]
	delete(s.pendingAcks, id)
	s.mu.Unlock()
	if fn != nil {
		fn(args)
	}
}

// Join adds the socket to a room.
func (s *Socket) Join(rooms ...string) *Socket {
	for _, room := range rooms {
		s.namespace.join(s, room)
	}
	return s
}

// Leave removes the socket from a room.
func (s *Socket) Leave(rooms ...string) *Socket {
	for _, room := range rooms {
		s.namespace.leave(s, room)
	}
	return s
}

// Rooms returns the set of rooms the socket is currently in.
func (s *Socket) Rooms() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.rooms))
	for r := range s.rooms {
		out = append(out, r)
	}
	return out
}

// To returns a broadcast operator that targets a room, excluding this socket —
// the equivalent of socket.to(room).
func (s *Socket) To(room string) *BroadcastOperator {
	return &BroadcastOperator{
		ns:     s.namespace,
		rooms:  []string{room},
		except: map[string]struct{}{s.id: {}},
	}
}

// Broadcast returns an operator targeting every other socket in the namespace.
func (s *Socket) Broadcast() *BroadcastOperator {
	return &BroadcastOperator{
		ns:     s.namespace,
		except: map[string]struct{}{s.id: {}},
	}
}

// Disconnect closes the socket, optionally closing the underlying transport.
func (s *Socket) Disconnect(closeTransport bool) {
	_ = s.conn.sendPacket(Packet{Type: Disconnect, Namespace: s.namespace.name})
	s.conn.mu.Lock()
	delete(s.conn.sockets, s.namespace.name)
	s.conn.mu.Unlock()
	s.namespace.remove(s)
	s.fireDisconnect("server namespace disconnect")
	if closeTransport {
		s.conn.server.removeConn(s.conn)
	}
}

// fireDisconnect notifies disconnect handlers exactly once.
func (s *Socket) fireDisconnect(reason string) {
	s.mu.Lock()
	if s.disconnected {
		s.mu.Unlock()
		return
	}
	s.disconnected = true
	handlers := append([]func(reason string){}, s.disconnectHandlers...)
	s.mu.Unlock()
	for _, h := range handlers {
		h(reason)
	}
}

// addRoom / removeRoom maintain the socket's own room set (called by the
// namespace adapter under its lock).
func (s *Socket) addRoom(room string) {
	s.mu.Lock()
	s.rooms[room] = struct{}{}
	s.mu.Unlock()
}

func (s *Socket) removeRoom(room string) {
	s.mu.Lock()
	delete(s.rooms, room)
	s.mu.Unlock()
}
