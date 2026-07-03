package socketio

import "sync"

// Adapter stores which sockets belong to a namespace and its rooms. The default
// implementation keeps everything in process; supplying a custom Adapter (e.g.
// backed by Redis) is the extension point for scaling a namespace across
// multiple server instances.
type Adapter interface {
	// Add registers a socket in the namespace.
	Add(socketID string, s *Socket)
	// Remove deletes a socket and drops it from every room.
	Remove(socketID string)
	// Join adds a socket to a room.
	Join(socketID, room string)
	// Leave removes a socket from a room.
	Leave(socketID, room string)
	// SocketsInRoom returns the sockets that are members of a room.
	SocketsInRoom(room string) []*Socket
	// AllSockets returns every socket in the namespace.
	AllSockets() []*Socket
	// Get returns a socket by id.
	Get(socketID string) (*Socket, bool)
}

// memoryAdapter is the default in-process Adapter.
type memoryAdapter struct {
	mu      sync.RWMutex
	sockets map[string]*Socket
	rooms   map[string]map[string]*Socket
}

func newMemoryAdapter() *memoryAdapter {
	return &memoryAdapter{
		sockets: make(map[string]*Socket),
		rooms:   make(map[string]map[string]*Socket),
	}
}

func (a *memoryAdapter) Add(id string, s *Socket) {
	a.mu.Lock()
	a.sockets[id] = s
	a.mu.Unlock()
}

func (a *memoryAdapter) Remove(id string) {
	a.mu.Lock()
	delete(a.sockets, id)
	for room, members := range a.rooms {
		if _, ok := members[id]; ok {
			delete(members, id)
			if len(members) == 0 {
				delete(a.rooms, room)
			}
		}
	}
	a.mu.Unlock()
}

func (a *memoryAdapter) Join(id, room string) {
	a.mu.Lock()
	members := a.rooms[room]
	if members == nil {
		members = make(map[string]*Socket)
		a.rooms[room] = members
	}
	if s, ok := a.sockets[id]; ok {
		members[id] = s
	}
	a.mu.Unlock()
}

func (a *memoryAdapter) Leave(id, room string) {
	a.mu.Lock()
	if members := a.rooms[room]; members != nil {
		delete(members, id)
		if len(members) == 0 {
			delete(a.rooms, room)
		}
	}
	a.mu.Unlock()
}

func (a *memoryAdapter) SocketsInRoom(room string) []*Socket {
	a.mu.RLock()
	defer a.mu.RUnlock()
	members := a.rooms[room]
	out := make([]*Socket, 0, len(members))
	for _, s := range members {
		out = append(out, s)
	}
	return out
}

func (a *memoryAdapter) AllSockets() []*Socket {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]*Socket, 0, len(a.sockets))
	for _, s := range a.sockets {
		out = append(out, s)
	}
	return out
}

func (a *memoryAdapter) Get(id string) (*Socket, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	s, ok := a.sockets[id]
	return s, ok
}
