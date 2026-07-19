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

// Add implements Adapter.Add, registering the socket under id in the namespace.
func (a *memoryAdapter) Add(id string, s *Socket) {
	a.mu.Lock()
	a.sockets[id] = s
	a.mu.Unlock()
}

// Remove implements Adapter.Remove, deleting the socket with the given id and
// dropping it from every room, discarding any room that becomes empty.
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

// Join implements Adapter.Join, adding the socket identified by id to room,
// creating the room if it does not yet exist. The socket is only added if it is
// currently registered in the namespace.
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

// Leave implements Adapter.Leave, removing the socket identified by id from
// room and discarding the room once it has no remaining members.
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

// SocketsInRoom implements Adapter.SocketsInRoom, returning the sockets that
// are members of room, or an empty slice if the room has no members.
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

// AllSockets implements Adapter.AllSockets, returning every socket currently
// registered in the namespace.
func (a *memoryAdapter) AllSockets() []*Socket {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]*Socket, 0, len(a.sockets))
	for _, s := range a.sockets {
		out = append(out, s)
	}
	return out
}

// Get implements Adapter.Get, returning the socket registered under id and
// whether it was present.
func (a *memoryAdapter) Get(id string) (*Socket, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	s, ok := a.sockets[id]
	return s, ok
}
