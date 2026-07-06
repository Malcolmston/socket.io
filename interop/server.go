// Command interop is a small, fixed Socket.IO server used to prove wire-level
// interoperability between this Go implementation and the Node.js reference
// Socket.IO client (and the official server). It is a test harness, not a
// reusable library: it exists so cross-implementation conformance tests can dial
// a known endpoint, exercise a known set of events, and assert that a Go server
// and a JavaScript client agree on the Engine.IO v4 / Socket.IO v5 protocols.
//
// Run it and it constructs a socketio.Server, registers a handful of
// deterministic handlers on the default namespace, and listens on a fixed
// loopback address. The handlers cover the protocol features most worth
// checking against another implementation: "echo" bounces its arguments back to
// the sender, "ping" returns a fixed acknowledgement payload, "shout"
// broadcasts to a room every connection auto-joins, and "askme" exercises the
// less common server-initiated acknowledgement by calling EmitWithAck and
// logging the client's reply. A second namespace, "/admin", answers "whoami",
// verifying that namespace multiplexing works across implementations.
//
// The process is intentionally chatty on standard error rather than standard
// output: it prints "LISTENING" once the handlers are wired (so a supervising
// test can wait for readiness before dialing) and "SERVER_GOT_ACK:<reply>" when
// the "askme" acknowledgement arrives. Those markers are the contract a driving
// test scripts against; the program itself takes no flags and has no exported
// API. The server binds 127.0.0.1:9731 and serves Socket.IO at the default
// /socket.io/ path.
//
// This command is not meant to be imported or embedded in another program. For
// application use, depend on the socketio package (server) or the client package
// (client) directly; interop simply demonstrates and validates that both speak
// the same protocol as the Node originals. It mirrors, in miniature, the kind of
// end-to-end setup a real deployment would build with socketio.New,
// OnConnection, Join, On, To(...).Emit, and EmitWithAck.
//
// It lives in its own nested Go module (built with GOWORK=off) so the conformance
// harness can pin its own dependencies without perturbing the parent workspace,
// and so the main library's public API surface stays free of test-only command
// code. Keeping the fixture deterministic and dependency-light is deliberate: a
// conformance check is only trustworthy if the endpoint it probes never drifts,
// so every handler, address, and readiness marker here is fixed rather than
// configurable.
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
