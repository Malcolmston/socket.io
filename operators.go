package socketio

import "time"

// This file completes the chainable broadcast-flag surface (io.local,
// io.timeout, io.except) and the in/except room entry points on Namespace,
// Server, and Socket, bringing the broadcast operator closer to parity with the
// JavaScript BroadcastOperator.

// Local restricts the broadcast to sockets connected to this node, suppressing
// cluster fan-out even when a Broadcaster is installed — the equivalent of
// io.local.emit(...). It is a no-op on a single-node deployment.
func (b *BroadcastOperator) Local() *BroadcastOperator {
	b.local = true
	return b
}

// Timeout records an acknowledgement timeout for the broadcast, matching
// io.timeout(ms).emit(...). It is stored for callers that aggregate acks; the
// convenience Emit itself is fire-and-forget.
func (b *BroadcastOperator) Timeout(d time.Duration) *BroadcastOperator {
	b.timeout = d
	return b
}

// ExceptRoom excludes every socket that is a member of any of the given rooms
// from the broadcast — the equivalent of io.except(room). Unlike Except, which
// excludes an individual socket id, ExceptRoom excludes whole rooms.
func (b *BroadcastOperator) ExceptRoom(rooms ...string) *BroadcastOperator {
	b.exceptRooms = append(b.exceptRooms, rooms...)
	return b
}

// In is an alias for To, scoping a broadcast to a room within this namespace —
// the equivalent of io.in(room).
func (ns *Namespace) In(room string) *BroadcastOperator {
	return &BroadcastOperator{ns: ns, rooms: []string{room}}
}

// Except returns a broadcast operator over the namespace that excludes every
// socket in the given room — the equivalent of io.except(room).
func (ns *Namespace) Except(room string) *BroadcastOperator {
	return &BroadcastOperator{ns: ns, exceptRooms: []string{room}}
}

// Local returns a broadcast operator over the namespace restricted to
// this node — the equivalent of io.local.
func (ns *Namespace) Local() *BroadcastOperator {
	return &BroadcastOperator{ns: ns, local: true}
}

// In scopes a server-wide broadcast to a room in the default namespace — the
// equivalent of io.in(room).
func (s *Server) In(room string) *BroadcastOperator { return s.Of("/").In(room) }

// Except returns a broadcast operator over the default namespace excluding every
// socket in the given room — the equivalent of io.except(room).
func (s *Server) Except(room string) *BroadcastOperator { return s.Of("/").Except(room) }

// Local returns a broadcast operator over the default namespace restricted to
// this node — the equivalent of io.local.
func (s *Server) Local() *BroadcastOperator { return s.Of("/").Local() }

// In returns a broadcast operator that targets a room while excluding this
// socket — the equivalent of socket.in(room). It behaves like To.
func (s *Socket) In(room string) *BroadcastOperator {
	return &BroadcastOperator{
		ns:     s.namespace,
		rooms:  []string{room},
		except: map[string]struct{}{s.id: {}},
	}
}

// Except returns a broadcast operator over the namespace that excludes this
// socket and every member of the given room — the equivalent of
// socket.except(room).
func (s *Socket) Except(room string) *BroadcastOperator {
	return &BroadcastOperator{
		ns:          s.namespace,
		except:      map[string]struct{}{s.id: {}},
		exceptRooms: []string{room},
	}
}
