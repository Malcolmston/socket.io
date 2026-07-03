package main

import (
	"fmt"
	"net/http"
	"os"

	socketio "github.com/malcolmston/socketio"
)

func main() {
	io := socketio.New()
	io.OnConnection(func(s *socketio.Socket) {
		s.Join("room1")
		s.On("echo", func(args []any) []any { s.Emit("echo", args...); return nil })
		s.On("ping", func(args []any) []any { return []any{"pong", 42} })
		s.On("shout", func(args []any) []any { io.To("room1").Emit("news", args...); return nil })
		// server asks client for an ack
		s.On("askme", func(args []any) []any {
			s.EmitWithAck("question", func(reply []any) {
				fmt.Fprintf(os.Stderr, "SERVER_GOT_ACK:%v\n", reply)
			}, "what is 2+2?")
			return nil
		})
	})
	io.Of("/admin").OnConnection(func(s *socketio.Socket) {
		s.On("whoami", func(args []any) []any { return []any{"admin"} })
	})
	http.Handle("/socket.io/", io)
	fmt.Fprintln(os.Stderr, "LISTENING")
	http.ListenAndServe("127.0.0.1:9731", nil)
}
