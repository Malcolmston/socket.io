package socketio

// BroadcastOperator emits events to a filtered set of sockets in a namespace —
// optionally scoped to one or more rooms and excluding specific sockets. It is
// returned by Namespace.To, Server.To, and Socket.To, and is the equivalent of
// io.to(room).emit(...).
type BroadcastOperator struct {
	ns     *Namespace
	rooms  []string
	except map[string]struct{}
}

// To narrows the broadcast to an additional room.
func (b *BroadcastOperator) To(room string) *BroadcastOperator {
	b.rooms = append(b.rooms, room)
	return b
}

// Except excludes a socket id from the broadcast.
func (b *BroadcastOperator) Except(socketID string) *BroadcastOperator {
	if b.except == nil {
		b.except = make(map[string]struct{})
	}
	b.except[socketID] = struct{}{}
	return b
}

// Emit sends an event to every socket matched by the operator.
func (b *BroadcastOperator) Emit(event string, args ...any) {
	for _, s := range b.targets() {
		if _, skip := b.except[s.id]; skip {
			continue
		}
		_ = s.Emit(event, args...)
	}
}

// targets resolves the set of sockets the operator addresses, de-duplicated by
// socket id when several rooms overlap.
func (b *BroadcastOperator) targets() []*Socket {
	if len(b.rooms) == 0 {
		return b.ns.Sockets()
	}
	seen := make(map[string]*Socket)
	for _, room := range b.rooms {
		for _, s := range b.ns.SocketsInRoom(room) {
			seen[s.id] = s
		}
	}
	out := make([]*Socket, 0, len(seen))
	for _, s := range seen {
		out = append(out, s)
	}
	return out
}
