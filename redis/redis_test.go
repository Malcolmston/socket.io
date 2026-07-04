package redis_test

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	socketio "github.com/malcolmston/socketio"
	"github.com/malcolmston/socketio/client"
	"github.com/malcolmston/socketio/redis"
)

// ---- a minimal in-process Redis pub/sub server for testing -------------------

type fakeRedis struct {
	ln   net.Listener
	mu   sync.Mutex
	subs map[string][]net.Conn
	wmu  sync.Mutex // serializes writes across conns
}

func newFakeRedis(t *testing.T) *fakeRedis {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeRedis{ln: ln, subs: map[string][]net.Conn{}}
	go f.accept()
	return f
}

func (f *fakeRedis) addr() string { return f.ln.Addr().String() }
func (f *fakeRedis) close()       { f.ln.Close() }

func (f *fakeRedis) accept() {
	for {
		c, err := f.ln.Accept()
		if err != nil {
			return
		}
		go f.handle(c)
	}
}

func (f *fakeRedis) write(c net.Conn, b []byte) {
	f.wmu.Lock()
	c.Write(b)
	f.wmu.Unlock()
}

func (f *fakeRedis) handle(c net.Conn) {
	r := bufio.NewReader(c)
	for {
		args, err := readCommand(r)
		if err != nil {
			return
		}
		if len(args) == 0 {
			continue
		}
		switch string(args[0]) {
		case "SUBSCRIBE", "subscribe":
			ch := string(args[1])
			f.mu.Lock()
			f.subs[ch] = append(f.subs[ch], c)
			f.mu.Unlock()
			f.write(c, encodeArray([][]byte{[]byte("subscribe"), args[1]}, 1))
		case "PUBLISH", "publish":
			ch, msg := string(args[1]), args[2]
			f.mu.Lock()
			subs := append([]net.Conn{}, f.subs[ch]...)
			f.mu.Unlock()
			for _, s := range subs {
				f.write(s, encodeMessage(ch, msg))
			}
			f.write(c, []byte(fmt.Sprintf(":%d\r\n", len(subs))))
		default:
			f.write(c, []byte("+OK\r\n"))
		}
	}
}

// readCommand parses a RESP array of bulk strings (a client command).
func readCommand(r *bufio.Reader) ([][]byte, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	if len(line) == 0 || line[0] != '*' {
		return nil, fmt.Errorf("bad command")
	}
	n, _ := strconv.Atoi(line[1 : len(line)-2])
	args := make([][]byte, n)
	for i := 0; i < n; i++ {
		hdr, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		l, _ := strconv.Atoi(hdr[1 : len(hdr)-2])
		buf := make([]byte, l+2)
		if _, err := readFull(r, buf); err != nil {
			return nil, err
		}
		args[i] = buf[:l]
	}
	return args, nil
}

func readFull(r *bufio.Reader, buf []byte) (int, error) {
	t := 0
	for t < len(buf) {
		n, err := r.Read(buf[t:])
		t += n
		if err != nil {
			return t, err
		}
	}
	return t, nil
}

// encodeMessage builds a RESP "message" pub/sub frame.
func encodeMessage(channel string, payload []byte) []byte {
	return []byte(fmt.Sprintf("*3\r\n$7\r\nmessage\r\n$%d\r\n%s\r\n$%d\r\n%s\r\n",
		len(channel), channel, len(payload), payload))
}

// encodeArray builds a RESP array of bulk strings followed by an integer.
func encodeArray(parts [][]byte, trailing int) []byte {
	out := []byte(fmt.Sprintf("*%d\r\n", len(parts)+1))
	for _, p := range parts {
		out = append(out, []byte(fmt.Sprintf("$%d\r\n%s\r\n", len(p), p))...)
	}
	out = append(out, []byte(fmt.Sprintf(":%d\r\n", trailing))...)
	return out
}

// ---- tests -------------------------------------------------------------------

func TestBroadcasterPubSub(t *testing.T) {
	fr := newFakeRedis(t)
	defer fr.close()

	a, err := redis.New(redis.Options{Addr: fr.addr(), Channel: "t"})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := redis.New(redis.Options{Addr: fr.addr(), Channel: "t"})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	got := make(chan []byte, 2)
	a.OnMessage(func(d []byte) { got <- d })
	b.OnMessage(func(d []byte) { got <- d })
	time.Sleep(50 * time.Millisecond) // let subscriptions register

	if err := a.Publish([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	// Both subscribers (a and b) should receive the published payload.
	for i := 0; i < 2; i++ {
		select {
		case d := <-got:
			if string(d) != "hello" {
				t.Fatalf("payload = %q", d)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("did not receive published message")
		}
	}
}

func TestTwoNodeBroadcast(t *testing.T) {
	fr := newFakeRedis(t)
	defer fr.close()

	// Two independent socket.io servers ("nodes"), each with its own Redis
	// broadcaster pointed at the same bus.
	newNode := func() (*socketio.Server, string, func()) {
		io := socketio.New()
		io.OnConnection(func(s *socketio.Socket) { s.Join("room1") })
		bc, err := redis.New(redis.Options{Addr: fr.addr(), Channel: "sio"})
		if err != nil {
			t.Fatal(err)
		}
		io.SetBroadcaster(bc)
		srv := httptest.NewServer(http.HandlerFunc(io.ServeHTTP))
		return io, srv.URL, func() { srv.Close(); bc.Close() }
	}

	ioA, _, closeA := newNode()
	defer closeA()
	_, urlB, closeB := newNode()
	defer closeB()
	time.Sleep(50 * time.Millisecond)

	// A client connects to node B and listens for the broadcast.
	cb, err := client.Dial(urlB)
	if err != nil {
		t.Fatal(err)
	}
	defer cb.Close()
	got := make(chan []any, 1)
	cb.On("news", func(args []any) []any { got <- args; return nil })
	time.Sleep(50 * time.Millisecond) // let node B register the socket in room1

	// Broadcast from node A's server to room1 — the client is on node B, so this
	// only works if the broadcast crosses the Redis bus.
	ioA.To("room1").Emit("news", "hi cluster")

	select {
	case args := <-got:
		if len(args) == 0 || args[0] != "hi cluster" {
			t.Fatalf("cross-node broadcast args = %v", args)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cross-node broadcast did not reach the other node's client")
	}
}
