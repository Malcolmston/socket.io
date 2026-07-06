package socketio_test

import (
	"fmt"
	"net/http/httptest"

	socketio "github.com/malcolmston/socketio"
)

// ExampleServer wires up a complete Socket.IO server the way a chat backend
// would. It creates a server with New, registers a connection handler that runs
// for every client that connects to the default namespace, and inside that
// handler joins the socket to a room, registers a "chat" event handler that
// re-broadcasts each message to everyone in that room via To(...).Emit, and
// installs a disconnect callback. The server is an http.Handler, so it is
// mounted here on an httptest.Server (a real net/http server would use
// http.Handle plus http.ListenAndServe at the same DefaultPath). The example
// then shuts everything down cleanly with Close on both the HTTP server and the
// Socket.IO server. The reader should take away the end-to-end shape of a
// server — New, OnConnection, Join, On, room broadcast, and mounting on HTTP —
// which mirrors the JavaScript io.on("connection")/socket.join/io.to().emit API.
func ExampleServer() {
	io := socketio.New()
	io.OnConnection(func(s *socketio.Socket) {
		// Every socket joins a shared room on connect.
		s.Join("room1")

		// Re-broadcast each chat message to the whole room.
		s.On("chat", func(args []any) []any {
			io.To("room1").Emit("chat", args...)
			return nil
		})

		s.OnDisconnect(func(reason string) {
			// Clean up per-socket state here.
			_ = reason
		})
	})

	// A *Server is an http.Handler. In production:
	//
	//	http.Handle(socketio.DefaultPath, io)
	//	http.ListenAndServe(":3000", nil)
	//
	// Here httptest hosts it so the example is self-contained.
	ts := httptest.NewServer(io)
	defer ts.Close()
	defer io.Close()

	fmt.Println("serving Socket.IO at", socketio.DefaultPath)
	// Output: serving Socket.IO at /socket.io/
}
