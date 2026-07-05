package client_test

import (
	"testing"
	"time"

	socketio "github.com/malcolmston/socketio"
	"github.com/malcolmston/socketio/client"
)

func TestClientIDAssignedAfterDial(t *testing.T) {
	url, closeFn := serve(t, func(io *socketio.Server) {})
	defer closeFn()

	c, err := client.Dial(url)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	if c.ID() == "" {
		t.Fatal("ID() should be non-empty once the server has acknowledged CONNECT")
	}
}

func TestClientEmitWithAckTimeout(t *testing.T) {
	release := make(chan struct{})
	url, closeFn := serve(t, func(io *socketio.Server) {
		io.OnConnection(func(s *socketio.Socket) {
			// The handler blocks until released, so the server's acknowledgement
			// arrives well after the client's short timeout has elapsed.
			s.On("slow", func(args []any) []any {
				<-release
				return []any{"late"}
			})
		})
	})
	defer closeFn()
	defer close(release)

	c, err := client.Dial(url)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	_, err = c.EmitWithAck("slow", 30*time.Millisecond)
	if err == nil {
		t.Fatal("expected an ack timeout error")
	}
}

func TestClientEmitAfterClose(t *testing.T) {
	url, closeFn := serve(t, func(io *socketio.Server) {})
	defer closeFn()

	c, err := client.Dial(url)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// A second Close is a no-op.
	if err := c.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}
