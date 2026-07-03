package socketio_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	socketio "github.com/malcolmston/socketio"
	"github.com/malcolmston/socketio/engineio"
)

// pollClient is a minimal Socket.IO client speaking the HTTP long-polling
// transport, used to drive the server end-to-end in tests.
type pollClient struct {
	t    *testing.T
	base string
	sid  string
	http *http.Client
}

func newPollClient(t *testing.T, base string) *pollClient {
	t.Helper()
	c := &pollClient{t: t, base: base, http: &http.Client{Timeout: 5 * time.Second}}
	// Engine.IO handshake.
	resp, err := c.http.Get(base + "?EIO=4&transport=polling")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	packets, err := engineio.DecodePayload(string(body))
	if err != nil || len(packets) == 0 || packets[0].Type != engineio.Open {
		t.Fatalf("bad handshake: body=%q err=%v", body, err)
	}
	var hs struct {
		Sid string `json:"sid"`
	}
	if err := json.Unmarshal([]byte(packets[0].Data), &hs); err != nil {
		t.Fatal(err)
	}
	c.sid = hs.Sid
	return c
}

// send posts raw Engine.IO packet strings to the server.
func (c *pollClient) send(packets ...string) {
	c.t.Helper()
	body := strings.Join(packets, "\x1e")
	resp, err := c.http.Post(c.url(), "text/plain", strings.NewReader(body))
	if err != nil {
		c.t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
}

// poll performs a single long-poll GET and returns the Engine.IO packet strings.
func (c *pollClient) poll() []string {
	c.t.Helper()
	resp, err := c.http.Get(c.url())
	if err != nil {
		c.t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if len(body) == 0 {
		return nil
	}
	return strings.Split(string(body), "\x1e")
}

func (c *pollClient) url() string {
	return c.base + "?EIO=4&transport=polling&sid=" + c.sid
}

// connect sends the Socket.IO CONNECT for the default namespace and waits for
// the CONNECT acknowledgement.
func (c *pollClient) connect() {
	c.t.Helper()
	c.send("40")
	for _, p := range c.poll() {
		if strings.HasPrefix(p, "40") {
			return
		}
	}
	c.t.Fatal("did not receive CONNECT ack")
}

func newTestServer(t *testing.T, configure func(*socketio.Server)) (*socketio.Server, string, func()) {
	io := socketio.New()
	configure(io)
	srv := httptest.NewServer(http.StripPrefix("/socket.io/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// httptest serves at root; the client hits base + query, so route all here.
		io.ServeHTTP(w, r)
	})))
	return io, srv.URL + "/socket.io/", srv.Close
}

func TestPollingConnectAndEcho(t *testing.T) {
	_, base, closeFn := newTestServer(t, func(io *socketio.Server) {
		io.OnConnection(func(s *socketio.Socket) {
			s.On("echo", func(args []any) []any {
				s.Emit("echo", args...)
				return nil
			})
		})
	})
	defer closeFn()

	c := newPollClient(t, base)
	c.connect()

	c.send(`42["echo","hello"]`)
	got := c.poll()
	if len(got) == 0 || !strings.Contains(got[0], `["echo","hello"]`) {
		t.Fatalf("echo not received: %v", got)
	}
}

func TestPollingAck(t *testing.T) {
	_, base, closeFn := newTestServer(t, func(io *socketio.Server) {
		io.OnConnection(func(s *socketio.Socket) {
			s.On("ping", func(args []any) []any { return []any{"pong"} })
		})
	})
	defer closeFn()

	c := newPollClient(t, base)
	c.connect()

	// Event with ack id 7.
	c.send(`427["ping"]`)
	got := c.poll()
	if len(got) == 0 || !strings.HasPrefix(got[0], "437") || !strings.Contains(got[0], `["pong"]`) {
		t.Fatalf("ack not received: %v", got)
	}
}

func TestPollingBroadcastToRoom(t *testing.T) {
	_, base, closeFn := newTestServer(t, func(io *socketio.Server) {
		io.OnConnection(func(s *socketio.Socket) {
			s.Join("room1")
			s.On("shout", func(args []any) []any {
				io.To("room1").Emit("news", args...)
				return nil
			})
		})
	})
	defer closeFn()

	a := newPollClient(t, base)
	a.connect()
	b := newPollClient(t, base)
	b.connect()

	a.send(`42["shout","hi all"]`)

	// Both A and B are in room1 and should receive "news".
	for name, c := range map[string]*pollClient{"a": a, "b": b} {
		got := c.poll()
		found := false
		for _, p := range got {
			if strings.Contains(p, `["news","hi all"]`) {
				found = true
			}
		}
		if !found {
			t.Fatalf("client %s did not receive broadcast: %v", name, got)
		}
	}
}

func TestNamespaces(t *testing.T) {
	_, base, closeFn := newTestServer(t, func(io *socketio.Server) {
		io.Of("/admin").OnConnection(func(s *socketio.Socket) {
			s.On("whoami", func(args []any) []any { return []any{"admin"} })
		})
	})
	defer closeFn()

	c := newPollClient(t, base)
	// Connect to the /admin namespace.
	c.send("40/admin,")
	connected := false
	for _, p := range c.poll() {
		if strings.HasPrefix(p, "40/admin,") {
			connected = true
		}
	}
	if !connected {
		t.Fatal("did not connect to /admin namespace")
	}

	c.send(`42/admin,3["whoami"]`)
	got := c.poll()
	if len(got) == 0 || !strings.Contains(got[0], `["admin"]`) || !strings.HasPrefix(got[0], "43/admin,3") {
		t.Fatalf("namespace ack not received: %v", got)
	}
}

func TestInvalidNamespace(t *testing.T) {
	_, base, closeFn := newTestServer(t, func(io *socketio.Server) {})
	defer closeFn()

	c := newPollClient(t, base)
	c.send("40/nope,")
	got := c.poll()
	if len(got) == 0 || !strings.HasPrefix(got[0], "44/nope,") {
		t.Fatalf("expected CONNECT_ERROR for invalid namespace, got: %v", got)
	}
}
