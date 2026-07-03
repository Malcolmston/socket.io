package socketio

import "sync"

// Namespace is a communication channel that partitions a Socket.IO server. Each
// namespace has its own set of connected sockets, rooms, and connection
// handlers. The default namespace is "/".
type Namespace struct {
	name   string
	server *Server

	mu           sync.RWMutex
	sockets      map[string]*Socket            // by socket id
	rooms        map[string]map[string]*Socket // room -> socket id -> socket
	connHandlers []func(*Socket)
}

func newNamespace(server *Server, name string) *Namespace {
	return &Namespace{
		name:    name,
		server:  server,
		sockets: make(map[string]*Socket),
		rooms:   make(map[string]map[string]*Socket),
	}
}

// Name returns the namespace name (e.g. "/" or "/admin").
func (ns *Namespace) Name() string { return ns.name }

// OnConnection registers a handler invoked for each new socket that connects to
// this namespace.
func (ns *Namespace) OnConnection(fn func(*Socket)) *Namespace {
	ns.mu.Lock()
	ns.connHandlers = append(ns.connHandlers, fn)
	ns.mu.Unlock()
	return ns
}

// add creates and registers a socket for a new namespace connection.
func (ns *Namespace) add(c *conn, _ string, auth any) *Socket {
	s := newSocket(c, ns, auth)
	ns.mu.Lock()
	ns.sockets[s.id] = s
	ns.mu.Unlock()
	return s
}

// fireConnection runs the namespace connection handlers for a socket.
func (ns *Namespace) fireConnection(s *Socket) {
	ns.mu.RLock()
	handlers := append([]func(*Socket){}, ns.connHandlers...)
	ns.mu.RUnlock()
	for _, h := range handlers {
		h(s)
	}
}

// remove deletes a socket from the namespace and all rooms.
func (ns *Namespace) remove(s *Socket) {
	ns.mu.Lock()
	delete(ns.sockets, s.id)
	for room, members := range ns.rooms {
		if _, ok := members[s.id]; ok {
			delete(members, s.id)
			if len(members) == 0 {
				delete(ns.rooms, room)
			}
		}
	}
	ns.mu.Unlock()
}

// join adds a socket to a room.
func (ns *Namespace) join(s *Socket, room string) {
	ns.mu.Lock()
	members := ns.rooms[room]
	if members == nil {
		members = make(map[string]*Socket)
		ns.rooms[room] = members
	}
	members[s.id] = s
	ns.mu.Unlock()
	s.addRoom(room)
}

// leave removes a socket from a room.
func (ns *Namespace) leave(s *Socket, room string) {
	ns.mu.Lock()
	if members := ns.rooms[room]; members != nil {
		delete(members, s.id)
		if len(members) == 0 {
			delete(ns.rooms, room)
		}
	}
	ns.mu.Unlock()
	s.removeRoom(room)
}

// Sockets returns all sockets currently connected to the namespace.
func (ns *Namespace) Sockets() []*Socket {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	out := make([]*Socket, 0, len(ns.sockets))
	for _, s := range ns.sockets {
		out = append(out, s)
	}
	return out
}

// SocketsInRoom returns the sockets that are members of a room.
func (ns *Namespace) SocketsInRoom(room string) []*Socket {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	members := ns.rooms[room]
	out := make([]*Socket, 0, len(members))
	for _, s := range members {
		out = append(out, s)
	}
	return out
}

// Emit broadcasts an event to every socket in the namespace.
func (ns *Namespace) Emit(event string, args ...any) {
	(&BroadcastOperator{ns: ns}).Emit(event, args...)
}

// To returns a broadcast operator scoped to a room within this namespace.
func (ns *Namespace) To(room string) *BroadcastOperator {
	return &BroadcastOperator{ns: ns, rooms: []string{room}}
}
