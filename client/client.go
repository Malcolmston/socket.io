// Package client is a Go Socket.IO client. It connects to a Socket.IO server
// (this one or the Node reference server) over the WebSocket transport and
// provides the familiar On/Emit/EmitWithAck API.
//
//	c, err := client.Dial("http://localhost:3000")
//	c.On("news", func(args []any) []any { fmt.Println(args); return nil })
//	c.Emit("hello", "world")
package client

import (
	"errors"
	"net/url"
	"strings"
	"sync"
	"time"

	socketio "github.com/malcolmston/socketio"
	"github.com/malcolmston/socketio/engineio"
	"github.com/malcolmston/socketio/internal/ws"
)

// Handler handles an inbound event. Returning a non-nil slice acknowledges an
// event the server sent with an ack id.
type Handler func(args []any) []any

// Client is a connected Socket.IO client.
type Client struct {
	namespace string
	wsURL     string
	opts      Options

	mu          sync.Mutex
	ws          *ws.Conn
	handlers    map[string][]Handler
	ackCounter  uint64
	pendingAcks map[uint64]chan []any
	sid         string
	closed      bool
	connErr     error
	pendingBin  *pendingBinary

	connectedCh chan struct{}
	connectOnce *sync.Once
}

// Options configures Dial.
type Options struct {
	// Namespace to connect to (default "/").
	Namespace string
	// Auth is an optional payload sent with the CONNECT packet.
	Auth any
	// DialTimeout bounds the connection handshake (default 10s).
	DialTimeout time.Duration
	// Reconnection enables automatic reconnection after an unexpected
	// disconnect.
	Reconnection bool
	// ReconnectionAttempts bounds reconnection tries (0 = unlimited).
	ReconnectionAttempts int
	// ReconnectionDelay is the initial backoff between attempts (default 1s,
	// doubling up to 5s).
	ReconnectionDelay time.Duration
}

// Dial connects to a Socket.IO server at rawURL (http:// or ws://) and returns
// once the Socket.IO CONNECT handshake completes.
func Dial(rawURL string, opts ...Options) (*Client, error) {
	var o Options
	if len(opts) > 0 {
		o = opts[0]
	}
	if o.Namespace == "" {
		o.Namespace = "/"
	}
	if o.DialTimeout == 0 {
		o.DialTimeout = 10 * time.Second
	}
	if o.ReconnectionDelay == 0 {
		o.ReconnectionDelay = time.Second
	}

	wsURL, err := engineWSURL(rawURL)
	if err != nil {
		return nil, err
	}

	c := &Client{
		namespace:   o.Namespace,
		wsURL:       wsURL,
		opts:        o,
		handlers:    make(map[string][]Handler),
		pendingAcks: make(map[uint64]chan []any),
	}
	if err := c.establish(); err != nil {
		return nil, err
	}
	return c, nil
}

// establish opens the transport, performs the Engine.IO + Socket.IO handshakes,
// and starts the read loop. It is used for both the initial connection and
// reconnections.
func (c *Client) establish() error {
	conn, err := ws.Dial(c.wsURL, nil)
	if err != nil {
		return err
	}

	// First frame must be the Engine.IO OPEN packet.
	mt, data, err := conn.ReadMessage()
	if err != nil || mt != ws.TextMessage {
		conn.Close()
		return errors.New("client: expected OPEN packet")
	}
	openPkt, err := engineio.Decode(string(data))
	if err != nil || openPkt.Type != engineio.Open {
		conn.Close()
		return errors.New("client: malformed OPEN packet")
	}

	c.mu.Lock()
	c.ws = conn
	c.connectedCh = make(chan struct{})
	c.connectOnce = &sync.Once{}
	c.connErr = nil
	c.mu.Unlock()

	go c.readLoop(conn)

	connect := socketio.Packet{Type: socketio.Connect, Namespace: c.namespace}
	if c.opts.Auth != nil {
		connect.Data = c.opts.Auth
	}
	if err := c.sendPacket(connect); err != nil {
		conn.Close()
		return err
	}

	select {
	case <-c.connectedCh:
		c.mu.Lock()
		e := c.connErr
		c.mu.Unlock()
		if e != nil {
			conn.Close()
			return e
		}
		return nil
	case <-time.After(c.opts.DialTimeout):
		conn.Close()
		return errors.New("client: connect timeout")
	}
}

// reconnectLoop attempts to re-establish the connection with exponential
// backoff after an unexpected disconnect.
func (c *Client) reconnectLoop() {
	delay := c.opts.ReconnectionDelay
	for attempt := 1; c.opts.ReconnectionAttempts == 0 || attempt <= c.opts.ReconnectionAttempts; attempt++ {
		time.Sleep(delay)
		c.mu.Lock()
		closed := c.closed
		c.mu.Unlock()
		if closed {
			return
		}
		if err := c.establish(); err == nil {
			c.fireLocal("reconnect")
			return
		}
		if delay < 5*time.Second {
			delay *= 2
		}
	}
}

// fireLocal dispatches a client-lifecycle event ("reconnect", "disconnect") to
// any handlers registered for it.
func (c *Client) fireLocal(event string, args ...any) {
	c.mu.Lock()
	handlers := append([]Handler(nil), c.handlers[event]...)
	c.mu.Unlock()
	for _, h := range handlers {
		h(args)
	}
}

// ID returns the socket id assigned by the server.
func (c *Client) ID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sid
}

// On registers a handler for an event.
func (c *Client) On(event string, h Handler) {
	c.mu.Lock()
	c.handlers[event] = append(c.handlers[event], h)
	c.mu.Unlock()
}

// Emit sends an event to the server.
func (c *Client) Emit(event string, args ...any) error {
	data := append([]any{event}, args...)
	return c.sendPacket(socketio.Packet{Type: socketio.Event, Namespace: c.namespace, Data: data})
}

// EmitWithAck sends an event and waits for the server's acknowledgement, up to
// timeout.
func (c *Client) EmitWithAck(event string, timeout time.Duration, args ...any) ([]any, error) {
	c.mu.Lock()
	id := c.ackCounter
	c.ackCounter++
	ch := make(chan []any, 1)
	c.pendingAcks[id] = ch
	c.mu.Unlock()

	data := append([]any{event}, args...)
	if err := c.sendPacket(socketio.Packet{Type: socketio.Event, Namespace: c.namespace, ID: &id, Data: data}); err != nil {
		return nil, err
	}
	select {
	case reply := <-ch:
		return reply, nil
	case <-time.After(timeout):
		c.mu.Lock()
		delete(c.pendingAcks, id)
		c.mu.Unlock()
		return nil, errors.New("client: ack timeout")
	}
}

// Close disconnects from the server.
func (c *Client) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	conn := c.ws
	c.mu.Unlock()
	_ = c.sendPacket(socketio.Packet{Type: socketio.Disconnect, Namespace: c.namespace})
	if conn != nil {
		return conn.Close()
	}
	return nil
}

// pendingBinary accumulates binary attachments for an inbound binary packet.
type pendingBinary struct {
	packet  socketio.Packet
	buffers [][]byte
	need    int
}

func (c *Client) sendPacket(p socketio.Packet) error {
	c.mu.Lock()
	conn := c.ws
	c.mu.Unlock()
	if conn == nil {
		return errors.New("client: not connected")
	}
	text, buffers, err := p.EncodeBinary()
	if err != nil {
		return err
	}
	if err := conn.WriteText(engineio.NewMessage(text).Encode()); err != nil {
		return err
	}
	for _, buf := range buffers {
		if err := conn.WriteMessage(ws.BinaryMessage, buf); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) readLoop(conn *ws.Conn) {
	for {
		mt, data, err := conn.ReadMessage()
		if err != nil {
			c.finishConnect(errors.New("client: connection closed before connect"))
			c.handleDisconnect()
			return
		}
		if mt == ws.BinaryMessage {
			c.handleBinaryAttachment(data)
			continue
		}
		if mt != ws.TextMessage {
			continue
		}
		ep, err := engineio.Decode(string(data))
		if err != nil {
			continue
		}
		switch ep.Type {
		case engineio.Ping:
			_ = conn.WriteText(engineio.Packet{Type: engineio.Pong, Data: ep.Data}.Encode())
		case engineio.Message:
			c.handleSioPacket(ep.Data)
		case engineio.Close:
			c.handleDisconnect()
			return
		}
	}
}

// handleDisconnect fails any pending acks and, when reconnection is enabled and
// the client was not closed by the user, starts the reconnect loop.
func (c *Client) handleDisconnect() {
	c.mu.Lock()
	closed := c.closed
	reconnect := c.opts.Reconnection && !closed
	c.mu.Unlock()
	// In-flight EmitWithAck calls unblock via their own timeout.

	c.fireLocal("disconnect")
	if reconnect {
		go c.reconnectLoop()
	}
}

func (c *Client) handleSioPacket(raw string) {
	pkt, err := socketio.DecodePacket(raw)
	if err != nil || pkt.Namespace != c.namespace {
		return
	}
	switch pkt.Type {
	case socketio.Connect:
		if obj, ok := pkt.Data.(map[string]any); ok {
			if id, ok := obj["sid"].(string); ok {
				c.mu.Lock()
				c.sid = id
				c.mu.Unlock()
			}
		}
		c.finishConnect(nil)
	case socketio.ConnectError:
		msg := "connect error"
		if obj, ok := pkt.Data.(map[string]any); ok {
			if m, ok := obj["message"].(string); ok {
				msg = m
			}
		}
		c.finishConnect(errors.New("client: " + msg))
	case socketio.Event:
		c.dispatch(pkt)
	case socketio.Ack:
		c.resolveAck(pkt)
	case socketio.BinaryEvent, socketio.BinaryAck:
		if pkt.Attachments() > 0 {
			c.mu.Lock()
			c.pendingBin = &pendingBinary{packet: pkt, need: pkt.Attachments()}
			c.mu.Unlock()
			return
		}
		c.dispatchBinary(pkt)
	}
}

// handleBinaryAttachment collects an inbound binary buffer and dispatches once
// all attachments for the pending packet have arrived.
func (c *Client) handleBinaryAttachment(buf []byte) {
	c.mu.Lock()
	pb := c.pendingBin
	if pb == nil {
		c.mu.Unlock()
		return
	}
	pb.buffers = append(pb.buffers, buf)
	if len(pb.buffers) < pb.need {
		c.mu.Unlock()
		return
	}
	c.pendingBin = nil
	c.mu.Unlock()

	pkt := pb.packet
	pkt.Data = socketio.Reconstruct(pkt.Data, pb.buffers)
	c.dispatchBinary(pkt)
}

// dispatchBinary routes a reassembled binary packet as an event or ack.
func (c *Client) dispatchBinary(pkt socketio.Packet) {
	if pkt.Type == socketio.BinaryEvent {
		pkt.Type = socketio.Event
		c.dispatch(pkt)
		return
	}
	pkt.Type = socketio.Ack
	c.resolveAck(pkt)
}

func (c *Client) resolveAck(pkt socketio.Packet) {
	if pkt.ID == nil {
		return
	}
	c.mu.Lock()
	ch := c.pendingAcks[*pkt.ID]
	delete(c.pendingAcks, *pkt.ID)
	c.mu.Unlock()
	if ch != nil {
		ch <- pkt.Args()
	}
}

func (c *Client) dispatch(pkt socketio.Packet) {
	c.mu.Lock()
	handlers := append([]Handler(nil), c.handlers[pkt.EventName()]...)
	c.mu.Unlock()

	var ack []any
	for _, h := range handlers {
		if reply := h(pkt.Args()); reply != nil && ack == nil {
			ack = reply
		}
	}
	if pkt.ID != nil {
		if ack == nil {
			ack = []any{}
		}
		_ = c.sendPacket(socketio.Packet{Type: socketio.Ack, Namespace: c.namespace, ID: pkt.ID, Data: ack})
	}
}

func (c *Client) finishConnect(err error) {
	// Snapshot the once/channel under the lock so a concurrent reconnect (which
	// swaps them in establish) can't race with this signal.
	c.mu.Lock()
	once := c.connectOnce
	ch := c.connectedCh
	c.mu.Unlock()
	if once == nil {
		return
	}
	once.Do(func() {
		c.mu.Lock()
		c.connErr = err
		c.mu.Unlock()
		close(ch)
	})
}

// engineWSURL turns an http(s)/ws(s) base URL into the Engine.IO websocket URL.
func engineWSURL(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "http", "ws":
		u.Scheme = "ws"
	case "https", "wss":
		u.Scheme = "wss"
	default:
		u.Scheme = "ws"
	}
	if u.Path == "" || u.Path == "/" {
		u.Path = "/socket.io/"
	} else if !strings.HasSuffix(u.Path, "/") {
		u.Path += "/"
	}
	q := u.Query()
	q.Set("EIO", "4")
	q.Set("transport", "websocket")
	u.RawQuery = q.Encode()
	return u.String(), nil
}
