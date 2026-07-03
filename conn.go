package socketio

import (
	"sync"
	"time"

	"github.com/malcolmston/socketio/engineio"
	"github.com/malcolmston/socketio/internal/ws"
)

// conn is a single Engine.IO session (one client). It owns the active transport
// (polling or websocket), buffers outgoing packets for polling, and dispatches
// incoming Socket.IO packets to the sockets bound on this session.
type conn struct {
	sid    string
	server *Server

	mu         sync.Mutex
	wsConn     *ws.Conn
	outbuf     []engineio.Packet
	pollWaiter chan []engineio.Packet
	sockets    map[string]*Socket // socket.io sockets by namespace name
	closed     bool

	lastPong time.Time
	stopPing chan struct{}
	pingOnce sync.Once

	// pendingBin buffers a binary packet awaiting its attachment frames.
	pendingBin *pendingBinary
}

// pendingBinary accumulates the binary attachments for a BINARY_EVENT/ACK until
// all declared buffers have arrived.
type pendingBinary struct {
	packet  Packet
	buffers [][]byte
	need    int
}

// send queues or writes an Engine.IO packet, depending on the active transport.
func (c *conn) send(p engineio.Packet) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	if c.wsConn != nil {
		w := c.wsConn
		c.mu.Unlock()
		if p.Binary != nil {
			// Binary attachments travel as native websocket binary frames.
			_ = w.WriteMessage(ws.BinaryMessage, p.Binary)
		} else {
			_ = w.WriteText(p.Encode())
		}
		return
	}
	c.outbuf = append(c.outbuf, p)
	waiter := c.pollWaiter
	if waiter != nil {
		c.pollWaiter = nil
		buf := c.outbuf
		c.outbuf = nil
		c.mu.Unlock()
		select {
		case waiter <- buf:
		default:
		}
		return
	}
	c.mu.Unlock()
}

// sendPacket encodes and sends a Socket.IO packet inside an Engine.IO MESSAGE.
// When the payload contains binary ([]byte) data it is sent as a BINARY_EVENT/
// BINARY_ACK followed by the raw attachment frames.
func (c *conn) sendPacket(p Packet) error {
	text, buffers, err := p.EncodeBinary()
	if err != nil {
		return err
	}
	c.send(engineio.NewMessage(text))
	for _, buf := range buffers {
		c.send(engineio.Packet{Type: engineio.Message, Binary: buf})
	}
	return nil
}

// handleEnginePacket processes one decoded Engine.IO packet.
func (c *conn) handleEnginePacket(p engineio.Packet) {
	switch p.Type {
	case engineio.Message:
		if p.Binary != nil {
			c.handleBinaryAttachment(p.Binary)
			return
		}
		c.handleMessage(p.Data)
	case engineio.Ping:
		// A client-initiated ping (rare in EIO4) is answered with a pong.
		c.send(engineio.Packet{Type: engineio.Pong, Data: p.Data})
	case engineio.Pong:
		c.mu.Lock()
		c.lastPong = time.Now()
		c.mu.Unlock()
	case engineio.Close:
		c.server.removeConn(c)
	}
}

// handleMessage decodes and dispatches a Socket.IO packet.
func (c *conn) handleMessage(data string) {
	pkt, err := DecodePacket(data)
	if err != nil {
		return
	}
	switch pkt.Type {
	case Connect:
		c.handleConnect(pkt)
	case Event:
		c.handleEvent(pkt)
	case Ack:
		c.handleAck(pkt)
	case BinaryEvent, BinaryAck:
		if pkt.attachments > 0 {
			// Wait for the attachment frames before dispatching.
			c.mu.Lock()
			c.pendingBin = &pendingBinary{packet: pkt, need: pkt.attachments}
			c.mu.Unlock()
			return
		}
		if pkt.Type == BinaryEvent {
			c.handleEvent(pkt)
		} else {
			c.handleAck(pkt)
		}
	case Disconnect:
		c.handleDisconnect(pkt)
	}
}

// handleBinaryAttachment collects a binary buffer for a pending binary packet
// and dispatches the packet once all attachments have arrived.
func (c *conn) handleBinaryAttachment(buf []byte) {
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
	pkt.Data = reconstruct(pkt.Data, pb.buffers)
	if pkt.Type == BinaryEvent {
		pkt.Type = Event
		c.handleEvent(pkt)
	} else {
		pkt.Type = Ack
		c.handleAck(pkt)
	}
}

func (c *conn) handleConnect(pkt Packet) {
	ns := c.server.namespace(pkt.Namespace)
	if ns == nil {
		_ = c.sendPacket(Packet{
			Type:      ConnectError,
			Namespace: pkt.Namespace,
			Data:      map[string]any{"message": "Invalid namespace"},
		})
		return
	}
	socket := ns.add(c, pkt.Namespace, pkt.Data)

	// Run connection middleware; a raised error rejects the connection.
	if err := ns.runMiddleware(socket); err != nil {
		ns.remove(socket)
		_ = c.sendPacket(Packet{
			Type:      ConnectError,
			Namespace: pkt.Namespace,
			Data:      map[string]any{"message": err.Error()},
		})
		return
	}

	c.mu.Lock()
	c.sockets[pkt.Namespace] = socket
	c.mu.Unlock()

	// The client considers itself connected on receiving this CONNECT packet.
	_ = c.sendPacket(Packet{
		Type:      Connect,
		Namespace: pkt.Namespace,
		Data:      map[string]any{"sid": socket.id},
	})
	ns.fireConnection(socket)
}

func (c *conn) handleEvent(pkt Packet) {
	c.mu.Lock()
	socket := c.sockets[pkt.Namespace]
	c.mu.Unlock()
	if socket != nil {
		// Dispatch off the transport read loop so a handler may block on an
		// acknowledgement (EmitAck) without preventing the loop from reading
		// the very ACK it is waiting for.
		go socket.dispatch(pkt)
	}
}

func (c *conn) handleAck(pkt Packet) {
	c.mu.Lock()
	socket := c.sockets[pkt.Namespace]
	c.mu.Unlock()
	if socket != nil && pkt.ID != nil {
		socket.resolveAck(*pkt.ID, pkt.Args())
	}
}

func (c *conn) handleDisconnect(pkt Packet) {
	c.mu.Lock()
	socket := c.sockets[pkt.Namespace]
	delete(c.sockets, pkt.Namespace)
	c.mu.Unlock()
	if socket != nil {
		socket.namespace.remove(socket)
		socket.fireDisconnect("client namespace disconnect")
	}
}

// socketFor returns the socket bound to a namespace on this session.
func (c *conn) socketFor(nsp string) *Socket {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sockets[nsp]
}

// startPing launches the server heartbeat for this session.
func (c *conn) startPing() {
	c.pingOnce.Do(func() {
		c.stopPing = make(chan struct{})
		go c.pingLoop()
	})
}

func (c *conn) pingLoop() {
	interval := c.server.opts.PingInterval
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-c.stopPing:
			return
		case <-ticker.C:
			c.mu.Lock()
			last := c.lastPong
			c.mu.Unlock()
			if time.Since(last) > c.server.opts.PingInterval+c.server.opts.PingTimeout {
				c.server.removeConn(c)
				return
			}
			c.send(engineio.Packet{Type: engineio.Ping})
		}
	}
}

// cleanup disconnects all sockets and closes the transport.
func (c *conn) cleanup() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	sockets := make([]*Socket, 0, len(c.sockets))
	for _, s := range c.sockets {
		sockets = append(sockets, s)
	}
	c.sockets = map[string]*Socket{}
	wsc := c.wsConn
	waiter := c.pollWaiter
	c.pollWaiter = nil
	stop := c.stopPing
	c.mu.Unlock()

	for _, s := range sockets {
		s.namespace.remove(s)
		s.fireDisconnect("transport close")
	}
	if wsc != nil {
		_ = wsc.Close()
	}
	if waiter != nil {
		select {
		case waiter <- nil:
		default:
		}
	}
	if stop != nil {
		close(stop)
	}
}

// attachWebSocket switches this session to the websocket transport, releasing
// any pending polling request and flushing buffered packets.
func (c *conn) attachWebSocket(wsc *ws.Conn) {
	c.mu.Lock()
	c.wsConn = wsc
	waiter := c.pollWaiter
	c.pollWaiter = nil
	buf := c.outbuf
	c.outbuf = nil
	c.mu.Unlock()

	if waiter != nil {
		// Release the outstanding poll with a noop so it returns promptly.
		select {
		case waiter <- []engineio.Packet{{Type: engineio.Noop}}:
		default:
		}
	}
	for _, p := range buf {
		_ = wsc.WriteText(p.Encode())
	}
}

// readLoop pumps websocket frames into the engine.io dispatcher until close.
func (c *conn) readLoop(wsc *ws.Conn) {
	for {
		mt, data, err := wsc.ReadMessage()
		if err != nil {
			break
		}
		if mt == ws.BinaryMessage {
			c.handleBinaryAttachment(data)
			continue
		}
		if mt != ws.TextMessage {
			continue
		}
		p, err := engineio.Decode(string(data))
		if err != nil {
			continue
		}
		c.handleEnginePacket(p)
	}
	c.server.removeConn(c)
}
