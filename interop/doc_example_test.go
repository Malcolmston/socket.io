package main

import (
	"fmt"

	socketio "github.com/malcolmston/socketio"
)

// Example mirrors the wiring the interop command performs, without binding a
// network port, so it can run deterministically. It constructs a socketio.Server
// with New, registers a default-namespace connection handler that joins every
// socket to a shared room and installs the harness's fixed event handlers
// ("echo" bounces its arguments back, "ping" returns a constant ack payload),
// and then registers a second "/admin" namespace with its own "whoami" handler.
// This is exactly the deterministic surface the cross-implementation
// conformance tests exercise against the Node.js reference client. The reader
// should take away how the interop harness is assembled from the ordinary
// socketio building blocks (New, OnConnection, Join, On, Of) and that the real
// command differs only in calling http.ListenAndServe to actually serve.
func Example() {
	io := socketio.New()
	io.OnConnection(func(s *socketio.Socket) {
		s.Join("room1")
		s.On("echo", func(args []any) []any { s.Emit("echo", args...); return nil })
		s.On("ping", func(args []any) []any { return []any{"pong", 42} })
	})
	io.Of("/admin").OnConnection(func(s *socketio.Socket) {
		s.On("whoami", func(args []any) []any { return []any{"admin"} })
	})

	fmt.Println("configured namespaces: / and /admin")
	// Output: configured namespaces: / and /admin
}
