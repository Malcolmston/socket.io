// Command chat is a small Socket.IO server in Go: it echoes messages and
// broadcasts chat messages to everyone in a room.
package main

import (
	"log"
	"net/http"

	socketio "github.com/malcolmston/socketio"
)

func main() {
	io := socketio.New()

	io.OnConnection(func(s *socketio.Socket) {
		log.Printf("client connected: %s", s.ID())

		// Everyone joins the "general" room.
		s.Join("general")

		// Echo an event straight back to the sender.
		s.On("echo", func(args []any) []any {
			s.Emit("echo", args...)
			return nil
		})

		// Broadcast a chat message to everyone in the room.
		s.On("chat", func(args []any) []any {
			io.To("general").Emit("chat", args...)
			return nil
		})

		// Respond to an event that requested an acknowledgement.
		s.On("ping", func(args []any) []any {
			return []any{"pong"}
		})

		s.OnDisconnect(func(reason string) {
			log.Printf("client %s disconnected: %s", s.ID(), reason)
		})
	})

	// The "/admin" namespace is isolated from the default one.
	io.Of("/admin").OnConnection(func(s *socketio.Socket) {
		s.On("stats", func(args []any) []any {
			return []any{map[string]any{"sockets": len(io.Sockets())}}
		})
	})

	http.Handle(socketio.DefaultPath, io)
	log.Println("Socket.IO server listening on :3000")
	log.Fatal(http.ListenAndServe(":3000", nil))
}
