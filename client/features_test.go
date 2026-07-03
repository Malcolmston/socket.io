package client_test

import (
	"bytes"
	"net"
	"net/http"
	"testing"
	"time"

	socketio "github.com/malcolmston/socketio"
	"github.com/malcolmston/socketio/client"
)

func TestBinaryEventRoundTrip(t *testing.T) {
	url, closeFn := serve(t, func(io *socketio.Server) {
		io.OnConnection(func(s *socketio.Socket) {
			// Echo back a binary payload plus a derived value.
			s.On("upload", func(args []any) []any {
				buf, _ := args[0].([]byte)
				return []any{[]byte("ack:" + string(buf)), len(buf)}
			})
		})
	})
	defer closeFn()

	c, err := client.Dial(url)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	reply, err := c.EmitWithAck("upload", 3*time.Second, []byte("hello-bytes"))
	if err != nil {
		t.Fatal(err)
	}
	got, ok := reply[0].([]byte)
	if !ok || !bytes.Equal(got, []byte("ack:hello-bytes")) {
		t.Fatalf("binary ack = %v (%T), want ack:hello-bytes", reply[0], reply[0])
	}
	if reply[1] != float64(11) {
		t.Fatalf("length arg = %v, want 11", reply[1])
	}
}

func TestServerBinaryEmit(t *testing.T) {
	url, closeFn := serve(t, func(io *socketio.Server) {
		io.OnConnection(func(s *socketio.Socket) {
			s.On("get", func(args []any) []any {
				s.Emit("blob", []byte{0x00, 0x01, 0x02, 0xff})
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

	got := make(chan []byte, 1)
	c.On("blob", func(args []any) []any {
		if b, ok := args[0].([]byte); ok {
			got <- b
		}
		return nil
	})
	c.Emit("get")

	select {
	case b := <-got:
		if !bytes.Equal(b, []byte{0x00, 0x01, 0x02, 0xff}) {
			t.Fatalf("blob = %v", b)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no binary blob received")
	}
}

func TestServerSocketsJoinAndBroadcast(t *testing.T) {
	var io *socketio.Server
	url, closeFn := serve(t, func(s *socketio.Server) {
		io = s
		s.OnConnection(func(sock *socketio.Socket) {})
	})
	defer closeFn()

	c, err := client.Dial(url)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	got := make(chan []any, 1)
	c.On("announce", func(args []any) []any { got <- args; return nil })

	// Wait for the server to see the socket, then make everyone join "vip".
	deadline := time.After(2 * time.Second)
	for len(io.FetchSockets()) == 0 {
		select {
		case <-deadline:
			t.Fatal("server never registered the socket")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	io.SocketsJoin("vip")
	io.To("vip").Emit("announce", "hello vip")

	select {
	case args := <-got:
		if args[0] != "hello vip" {
			t.Fatalf("announce = %v", args)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("did not receive room broadcast after SocketsJoin")
	}
}

func TestServerSideEmit(t *testing.T) {
	io := socketio.New()
	got := make(chan []any, 1)
	io.OnServerEvent("cluster-ping", func(args []any) { got <- args })
	io.ServerSideEmit("cluster-ping", "hi")
	select {
	case args := <-got:
		if args[0] != "hi" {
			t.Fatalf("server event = %v", args)
		}
	case <-time.After(time.Second):
		t.Fatal("server-side event not delivered")
	}
}

func TestClientReconnection(t *testing.T) {
	// A fixed listener so the client can reconnect to the same address after the
	// server drops its socket.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	io := socketio.New()
	io.OnConnection(func(s *socketio.Socket) {
		s.On("ping", func(args []any) []any { return []any{"pong"} })
	})
	srv := &http.Server{Handler: io}
	go srv.Serve(ln)
	defer srv.Close()

	url := "http://" + ln.Addr().String()
	c, err := client.Dial(url, client.Options{Reconnection: true, ReconnectionDelay: 50 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	reconnected := make(chan struct{}, 1)
	c.On("reconnect", func(args []any) []any { reconnected <- struct{}{}; return nil })

	// Force a server-side disconnect of all sockets.
	deadline := time.After(2 * time.Second)
	for len(io.FetchSockets()) == 0 {
		select {
		case <-deadline:
			t.Fatal("no socket registered")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	io.DisconnectSockets(true)

	select {
	case <-reconnected:
	case <-time.After(3 * time.Second):
		t.Fatal("client did not reconnect")
	}

	// The reconnected client can still round-trip.
	reply, err := c.EmitWithAck("ping", 2*time.Second)
	if err != nil || len(reply) == 0 || reply[0] != "pong" {
		t.Fatalf("post-reconnect ack = %v err=%v", reply, err)
	}
}
