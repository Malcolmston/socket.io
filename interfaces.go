package socketio

// This file collects the small, structural interfaces that several of the
// package's concrete types already satisfy, together with compile-time
// assertions wiring each type to the contract it fulfills. Nothing here changes
// behavior; it documents the shared shapes and lets callers write helpers
// against them instead of a specific concrete type.

// Emitter is anything that can broadcast an event to a set of sockets without a
// per-socket error — the server, a namespace, and a broadcast operator all fan
// an event out to many recipients, so a delivery error to any single socket is
// not surfaced. (Socket.Emit, which targets one client, deliberately returns an
// error and is therefore not an Emitter.)
type Emitter interface {
	Emit(event string, args ...any)
}

// RoomTargeter is anything that can scope a broadcast to a room, returning a
// *BroadcastOperator for further chaining (.To/.Except/.Emit). The server, a
// namespace, an individual socket, and an existing operator all expose this,
// mirroring socket.io's io.to(room) / socket.to(room) API.
type RoomTargeter interface {
	To(room string) *BroadcastOperator
}

// BroadcastTarget is the combination satisfied by the room-addressable emitters
// (server, namespace, and broadcast operator): they can both narrow to a room
// and emit to the resulting set.
type BroadcastTarget interface {
	Emitter
	RoomTargeter
}

// Compile-time assertions: each concrete type is wired to the contracts it
// already satisfies.
var (
	_ BroadcastTarget = (*Server)(nil)
	_ BroadcastTarget = (*Namespace)(nil)
	_ BroadcastTarget = (*BroadcastOperator)(nil)

	// Socket can target a room but reports per-socket emit errors, so it is a
	// RoomTargeter but not an Emitter.
	_ RoomTargeter = (*Socket)(nil)

	// The in-process adapter is the reference implementation of Adapter.
	_ Adapter = (*memoryAdapter)(nil)
)
