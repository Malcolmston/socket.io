// Command client connects to a Socket.IO server using the Go client and
// exchanges a couple of events.
package main

import (
	"log"
	"time"

	"github.com/malcolmston/socketio/client"
)

func main() {
	c, err := client.Dial("http://localhost:3000")
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()
	log.Printf("connected as %s", c.ID())

	// Listen for broadcasts.
	c.On("chat", func(args []any) []any {
		log.Printf("chat: %v", args)
		return nil
	})

	// Fire-and-forget emit.
	c.Emit("chat", "hello from Go")

	// Emit with acknowledgement.
	reply, err := c.EmitWithAck("ping", 5*time.Second)
	if err != nil {
		log.Printf("ping failed: %v", err)
	} else {
		log.Printf("ping ack: %v", reply)
	}

	time.Sleep(time.Second)
}
