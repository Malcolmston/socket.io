// Package redis provides a Redis-backed Broadcaster for socketio, enabling
// multi-node scale-out: broadcasts are relayed between server instances over
// Redis pub/sub so a message emitted on one node reaches sockets connected to
// any node. It speaks the Redis RESP protocol directly using only the standard
// library — no third-party client.
//
//	bc, _ := redis.New(redis.Options{Addr: "localhost:6379", Channel: "socket.io"})
//	io.SetBroadcaster(bc) // io is a *socketio.Server
package redis

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"
)

// Options configures the Redis broadcaster.
type Options struct {
	// Addr is the Redis server address (host:port).
	Addr string
	// Channel is the pub/sub channel used for broadcasts (default "socket.io").
	Channel string
	// Password, if set, authenticates via AUTH.
	Password string
	// DB selects a Redis database via SELECT (default 0).
	DB int
	// Dial overrides the network dialer (used in tests).
	Dial func(addr string) (net.Conn, error)
}

// Broadcaster relays socketio broadcasts over Redis pub/sub. It satisfies
// socketio.Broadcaster.
type Broadcaster struct {
	channel string

	pubMu   sync.Mutex
	pubConn net.Conn
	pubRW   *bufio.ReadWriter

	subConn net.Conn

	mu      sync.Mutex
	handler func([]byte)
	closed  bool
}

// New connects to Redis, subscribes to the broadcast channel, and returns a
// Broadcaster ready to install with server.SetBroadcaster.
func New(opts Options) (*Broadcaster, error) {
	if opts.Channel == "" {
		opts.Channel = "socket.io"
	}
	if opts.Addr == "" {
		opts.Addr = "127.0.0.1:6379"
	}
	dial := opts.Dial
	if dial == nil {
		dial = func(addr string) (net.Conn, error) { return net.Dial("tcp", addr) }
	}

	pub, err := connect(dial, opts)
	if err != nil {
		return nil, err
	}
	sub, err := connect(dial, opts)
	if err != nil {
		pub.Close()
		return nil, err
	}

	b := &Broadcaster{
		channel: opts.Channel,
		pubConn: pub,
		pubRW:   bufio.NewReadWriter(bufio.NewReader(pub), bufio.NewWriter(pub)),
		subConn: sub,
	}

	// Subscribe on the sub connection and start the receive loop.
	subRW := bufio.NewReadWriter(bufio.NewReader(sub), bufio.NewWriter(sub))
	if err := writeCommand(subRW.Writer, "SUBSCRIBE", opts.Channel); err != nil {
		_ = b.Close()
		return nil, err
	}
	go b.readLoop(subRW.Reader)
	return b, nil
}

// connect dials Redis and performs optional AUTH/SELECT.
func connect(dial func(string) (net.Conn, error), opts Options) (net.Conn, error) {
	conn, err := dial(opts.Addr)
	if err != nil {
		return nil, err
	}
	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	if opts.Password != "" {
		if err := command(rw, "AUTH", opts.Password); err != nil {
			conn.Close()
			return nil, err
		}
	}
	if opts.DB != 0 {
		if err := command(rw, "SELECT", strconv.Itoa(opts.DB)); err != nil {
			conn.Close()
			return nil, err
		}
	}
	return conn, nil
}

// Publish implements socketio.Broadcaster: it PUBLISHes data to the channel.
func (b *Broadcaster) Publish(data []byte) error {
	b.pubMu.Lock()
	defer b.pubMu.Unlock()
	if err := writeCommandBytes(b.pubRW.Writer, [][]byte{[]byte("PUBLISH"), []byte(b.channel), data}); err != nil {
		return err
	}
	_, err := readReply(b.pubRW.Reader) // integer: number of receivers
	return err
}

// OnMessage implements socketio.Broadcaster.
func (b *Broadcaster) OnMessage(fn func([]byte)) {
	b.mu.Lock()
	b.handler = fn
	b.mu.Unlock()
}

// Close implements socketio.Broadcaster.
func (b *Broadcaster) Close() error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	b.mu.Unlock()
	var err error
	if b.subConn != nil {
		err = b.subConn.Close()
	}
	if b.pubConn != nil {
		if e := b.pubConn.Close(); err == nil {
			err = e
		}
	}
	return err
}

// readLoop reads pub/sub frames and dispatches "message" payloads to the
// handler.
func (b *Broadcaster) readLoop(r *bufio.Reader) {
	for {
		v, err := readReply(r)
		if err != nil {
			return
		}
		arr, ok := v.([]any)
		if !ok || len(arr) < 3 {
			continue
		}
		kind, _ := arr[0].([]byte)
		if string(kind) != "message" {
			continue // subscribe confirmation, etc.
		}
		payload, _ := arr[2].([]byte)
		b.mu.Lock()
		h := b.handler
		b.mu.Unlock()
		if h != nil {
			cp := make([]byte, len(payload))
			copy(cp, payload)
			h(cp)
		}
	}
}

// ---- RESP protocol ----------------------------------------------------------

func command(rw *bufio.ReadWriter, args ...string) error {
	if err := writeCommand(rw.Writer, args...); err != nil {
		return err
	}
	_, err := readReply(rw.Reader)
	return err
}

func writeCommand(w *bufio.Writer, args ...string) error {
	parts := make([][]byte, len(args))
	for i, a := range args {
		parts[i] = []byte(a)
	}
	return writeCommandBytes(w, parts)
}

func writeCommandBytes(w *bufio.Writer, args [][]byte) error {
	if _, err := fmt.Fprintf(w, "*%d\r\n", len(args)); err != nil {
		return err
	}
	for _, a := range args {
		if _, err := fmt.Fprintf(w, "$%d\r\n", len(a)); err != nil {
			return err
		}
		if _, err := w.Write(a); err != nil {
			return err
		}
		if _, err := w.WriteString("\r\n"); err != nil {
			return err
		}
	}
	return w.Flush()
}

// readReply reads one RESP value: simple string/int → string/int64, bulk →
// []byte, array → []any, error → error.
func readReply(r *bufio.Reader) (any, error) {
	prefix, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	line, err := readLine(r)
	if err != nil {
		return nil, err
	}
	switch prefix {
	case '+':
		return string(line), nil
	case '-':
		return nil, errors.New("redis: " + string(line))
	case ':':
		n, _ := strconv.ParseInt(string(line), 10, 64)
		return n, nil
	case '$':
		n, _ := strconv.Atoi(string(line))
		if n < 0 {
			return nil, nil
		}
		buf := make([]byte, n)
		if _, err := readFull(r, buf); err != nil {
			return nil, err
		}
		if _, err := readLine(r); err != nil { // trailing CRLF
			return nil, err
		}
		return buf, nil
	case '*':
		n, _ := strconv.Atoi(string(line))
		if n < 0 {
			return nil, nil
		}
		arr := make([]any, n)
		for i := 0; i < n; i++ {
			v, err := readReply(r)
			if err != nil {
				return nil, err
			}
			arr[i] = v
		}
		return arr, nil
	default:
		return nil, fmt.Errorf("redis: unexpected reply type %q", prefix)
	}
}

func readLine(r *bufio.Reader) ([]byte, error) {
	line, err := r.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	// strip trailing \r\n
	n := len(line)
	if n >= 2 && line[n-2] == '\r' {
		return line[:n-2], nil
	}
	return line[:n-1], nil
}

func readFull(r *bufio.Reader, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := r.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}
