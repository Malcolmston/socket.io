package client_test

import (
	"fmt"
	"log"
	"time"

	"github.com/malcolmston/socketio/client"
)

// ExampleDial demonstrates the full lifecycle of a Go Socket.IO client. It calls
// Dial to connect to a running server, passing Options to select a namespace and
// to enable automatic reconnection; Dial blocks until the CONNECT handshake
// completes, so a nil error means the client is ready to use. The example then
// registers a handler for the server-pushed "news" event with On, sends a
// fire-and-forget event with Emit, and issues a request/response round-trip with
// EmitWithAck that waits up to five seconds for the server's acknowledgement.
// Finally it closes the connection with a deferred Close, which also suppresses
// any pending reconnection. The reader should take away how a Go program becomes
// a first-class Socket.IO peer using the same On/Emit/EmitWithAck vocabulary as
// the browser client. (This example is compiled to verify the API but is not run
// here, since it needs a live server to connect to.)
func ExampleDial() {
	c, err := client.Dial("http://localhost:3000", client.Options{
		Namespace:    "/",
		Auth:         map[string]any{"token": "s3cret"},
		Reconnection: true,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	// Handle events the server pushes to us.
	c.On("news", func(args []any) []any {
		fmt.Println("news:", args)
		return nil
	})

	// Fire-and-forget emit.
	if err := c.Emit("hello", "world"); err != nil {
		log.Fatal(err)
	}

	// Request/response with an acknowledgement.
	reply, err := c.EmitWithAck("question", 5*time.Second, "what is 2+2?")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("answer:", reply)
}
