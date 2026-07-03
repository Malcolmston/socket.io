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
	ws        *ws.Conn

	mu          sync.Mutex
	handlers    map[string][]Handler
	ackCounter  uint64
	pendingAcks map[uint64]chan []any
	sid         string
	closed      bool
	connErr     error

	connectedCh chan struct{}
	connectOnce sync.Once
}

// Options configures Dial.
type Options struct {
	// Namespace to connect to (default "/").
	Namespace string
	// Auth is an optional payload sent with the CONNECT packet.
	Auth any
	// DialTimeout bounds the connection handshake (default 10s).
	DialTimeout time.Duration
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

	wsURL, err := engineWSURL(rawURL)
	if err != nil {
		return nil, err
	}
	conn, err := ws.Dial(wsURL, nil)
	if err != nil {
		return nil, err
	}

	c := &Client{
		namespace:   o.Namespace,
		ws:          conn,
		handlers:    make(map[string][]Handler),
		pendingAcks: make(map[uint64]chan []any),
		connectedCh: make(chan struct{}),
	}

	// First frame must be the Engine.IO OPEN packet.
	mt, data, err := conn.ReadMessage()
	if err != nil || mt != ws.TextMessage {
		conn.Close()
		return nil, errors.New("client: expected OPEN packet")
	}
	openPkt, err := engineio.Decode(string(data))
	if err != nil || openPkt.Type != engineio.Open {
		conn.Close()
		return nil, errors.New("client: malformed OPEN packet")
	}

	go c.readLoop()

	// Send the Socket.IO CONNECT for the namespace.
	connect := socketio.Packet{Type: socketio.Connect, Namespace: o.Namespace}
	if o.Auth != nil {
		connect.Data = o.Auth
	}
	if err := c.sendPacket(connect); err != nil {
		conn.Close()
		return nil, err
	}

	select {
	case <-c.connectedCh:
		if c.connErr != nil {
			conn.Close()
			return nil, c.connErr
		}
		return c, nil
	case <-time.After(o.DialTimeout):
		conn.Close()
		return nil, errors.New("client: connect timeout")
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
	c.mu.Unlock()
	_ = c.sendPacket(socketio.Packet{Type: socketio.Disconnect, Namespace: c.namespace})
	return c.ws.Close()
}

func (c *Client) sendPacket(p socketio.Packet) error {
	s, err := p.Encode()
	if err != nil {
		return err
	}
	return c.ws.WriteText(engineio.NewMessage(s).Encode())
}

func (c *Client) readLoop() {
	for {
		mt, data, err := c.ws.ReadMessage()
		if err != nil {
			c.finishConnect(errors.New("client: connection closed before connect"))
			return
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
			_ = c.ws.WriteText(engineio.Packet{Type: engineio.Pong, Data: ep.Data}.Encode())
		case engineio.Message:
			c.handleSioPacket(ep.Data)
		case engineio.Close:
			return
		}
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
	case socketio.Event, socketio.BinaryEvent:
		c.dispatch(pkt)
	case socketio.Ack, socketio.BinaryAck:
		if pkt.ID != nil {
			c.mu.Lock()
			ch := c.pendingAcks[*pkt.ID]
			delete(c.pendingAcks, *pkt.ID)
			c.mu.Unlock()
			if ch != nil {
				ch <- pkt.Args()
			}
		}
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
	c.connectOnce.Do(func() {
		c.connErr = err
		close(c.connectedCh)
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
