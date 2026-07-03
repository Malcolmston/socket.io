package client_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	socketio "github.com/malcolmston/socketio"
	"github.com/malcolmston/socketio/client"
)

func serve(t *testing.T, configure func(*socketio.Server)) (string, func()) {
	io := socketio.New()
	configure(io)
	srv := httptest.NewServer(http.HandlerFunc(io.ServeHTTP))
	return srv.URL, srv.Close
}

func TestClientServerEcho(t *testing.T) {
	url, closeFn := serve(t, func(io *socketio.Server) {
		io.OnConnection(func(s *socketio.Socket) {
			s.On("echo", func(args []any) []any {
				s.Emit("echo", args...)
				return nil
			})
		})
	})
	defer closeFn()

	c, err := client.Dial(url)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	got := make(chan []any, 1)
	c.On("echo", func(args []any) []any { got <- args; return nil })
	c.Emit("echo", "hello", float64(7))

	select {
	case args := <-got:
		if len(args) != 2 || args[0] != "hello" || args[1] != float64(7) {
			t.Fatalf("echo args = %v", args)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("did not receive echo")
	}
}

func TestClientAck(t *testing.T) {
	url, closeFn := serve(t, func(io *socketio.Server) {
		io.OnConnection(func(s *socketio.Socket) {
			s.On("add", func(args []any) []any {
				return []any{args[0].(float64) + args[1].(float64)}
			})
		})
	})
	defer closeFn()

	c, err := client.Dial(url)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	reply, err := c.EmitWithAck("add", 2*time.Second, float64(2), float64(3))
	if err != nil {
		t.Fatal(err)
	}
	if len(reply) != 1 || reply[0] != float64(5) {
		t.Fatalf("ack reply = %v, want [5]", reply)
	}
}

func TestServerEmitAckToClient(t *testing.T) {
	url, closeFn := serve(t, func(io *socketio.Server) {
		io.OnConnection(func(s *socketio.Socket) {
			s.On("trigger", func(args []any) []any {
				reply, err := s.EmitAck("question", 2*time.Second, "2+2?")
				if err != nil {
					t.Errorf("server EmitAck: %v", err)
					return nil
				}
				return reply // echo the client's answer back as this event's ack
			})
		})
	})
	defer closeFn()

	c, err := client.Dial(url)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	// The client answers the server's question.
	c.On("question", func(args []any) []any { return []any{"4"} })

	reply, err := c.EmitWithAck("trigger", 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(reply) != 1 || reply[0] != "4" {
		t.Fatalf("round-trip ack = %v, want [4]", reply)
	}
}

func TestNamespaceMiddlewareRejects(t *testing.T) {
	url, closeFn := serve(t, func(io *socketio.Server) {
		io.Use(func(s *socketio.Socket, next func(error)) {
			next(errors.New("unauthorized"))
		})
		io.OnConnection(func(s *socketio.Socket) {})
	})
	defer closeFn()

	_, err := client.Dial(url)
	if err == nil {
		t.Fatal("expected connect to be rejected by middleware")
	}
}

func TestNamespaceMiddlewareAllows(t *testing.T) {
	url, closeFn := serve(t, func(io *socketio.Server) {
		io.Use(func(s *socketio.Socket, next func(error)) {
			next(nil) // allow
		})
		io.OnConnection(func(s *socketio.Socket) {})
	})
	defer closeFn()

	c, err := client.Dial(url)
	if err != nil {
		t.Fatalf("connect should succeed: %v", err)
	}
	c.Close()
}
