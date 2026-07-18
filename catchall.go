package socketio

// Catch-all listeners mirror socket.io's socket.onAny / prependAny / offAny:
// callbacks invoked for every inbound event regardless of name, receiving the
// event name alongside its arguments. They are useful for logging, metrics, and
// generic message routing.

// OnAny registers a catch-all listener invoked for every inbound event on this
// socket, in registration order and before the named handlers run. The callback
// receives the event name and its arguments. Returns the socket for chaining —
// the equivalent of socket.onAny(listener).
func (s *Socket) OnAny(fn func(event string, args []any)) *Socket {
	s.mu.Lock()
	s.anyHandlers = append(s.anyHandlers, fn)
	s.mu.Unlock()
	return s
}

// PrependAny registers a catch-all listener at the front of the catch-all chain,
// so it runs before previously registered catch-all listeners — the equivalent
// of socket.prependAny(listener).
func (s *Socket) PrependAny(fn func(event string, args []any)) *Socket {
	s.mu.Lock()
	s.anyHandlers = append([]func(string, []any){fn}, s.anyHandlers...)
	s.mu.Unlock()
	return s
}

// OffAny removes catch-all listeners. Called with no argument it removes every
// catch-all listener; called with one listener it is a no-op placeholder for API
// symmetry (Go funcs are not comparable, so individual removal is not
// supported) and clears all listeners. Use ListenersAny to inspect the current
// set. Returns the socket for chaining.
func (s *Socket) OffAny() *Socket {
	s.mu.Lock()
	s.anyHandlers = nil
	s.mu.Unlock()
	return s
}

// ListenersAny returns a snapshot of the currently registered catch-all
// listeners — the equivalent of socket.listenersAny().
func (s *Socket) ListenersAny() []func(event string, args []any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]func(event string, args []any), len(s.anyHandlers))
	copy(out, s.anyHandlers)
	return out
}
