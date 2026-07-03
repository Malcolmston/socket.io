// Package ws is a minimal, dependency-free RFC 6455 WebSocket server
// implementation, sufficient to carry Engine.IO/Socket.IO traffic. It supports
// the server-side opening handshake, fragmented data messages, and the control
// frames (ping/pong/close). Only what Socket.IO needs is implemented; it is not
// a general-purpose WebSocket library.
package ws

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
)

// guid is the RFC 6455 magic value appended to the client key.
const guid = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// Opcode values (RFC 6455 §5.2).
const (
	opContinuation = 0x0
	opText         = 0x1
	opBinary       = 0x2
	opClose        = 0x8
	opPing         = 0x9
	opPong         = 0xA
)

// MessageType distinguishes text and binary data messages.
type MessageType int

const (
	// TextMessage is a UTF-8 text message.
	TextMessage MessageType = iota
	// BinaryMessage is a binary message.
	BinaryMessage
)

// ErrClosed is returned once the connection has been closed.
var ErrClosed = errors.New("ws: connection closed")

// Conn is an accepted WebSocket connection.
type Conn struct {
	conn net.Conn
	br   *bufio.Reader
	bw   *bufio.Writer

	writeMu sync.Mutex
	closeMu sync.Mutex
	closed  bool
}

// IsWebSocketUpgrade reports whether r is a WebSocket upgrade request.
func IsWebSocketUpgrade(r *http.Request) bool {
	return tokenListContains(r.Header.Get("Connection"), "upgrade") &&
		strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}

// Upgrade completes the server handshake and hijacks the connection, returning
// a *Conn ready for ReadMessage/WriteMessage.
func Upgrade(w http.ResponseWriter, r *http.Request) (*Conn, error) {
	if !IsWebSocketUpgrade(r) {
		return nil, errors.New("ws: not a websocket upgrade request")
	}
	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		return nil, errors.New("ws: missing Sec-WebSocket-Key")
	}
	if v := r.Header.Get("Sec-WebSocket-Version"); v != "13" {
		return nil, fmt.Errorf("ws: unsupported version %q", v)
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		return nil, errors.New("ws: response writer does not support hijacking")
	}
	conn, rw, err := hj.Hijack()
	if err != nil {
		return nil, err
	}

	accept := acceptKey(key)
	resp := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + accept + "\r\n\r\n"
	if _, err := rw.WriteString(resp); err != nil {
		conn.Close()
		return nil, err
	}
	if err := rw.Flush(); err != nil {
		conn.Close()
		return nil, err
	}
	return &Conn{conn: conn, br: rw.Reader, bw: rw.Writer}, nil
}

func acceptKey(key string) string {
	h := sha1.New()
	h.Write([]byte(key + guid))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// ReadMessage reads the next data message, transparently answering ping and
// close control frames. It reassembles fragmented messages.
func (c *Conn) ReadMessage() (MessageType, []byte, error) {
	var (
		payload []byte
		msgType MessageType
		started bool
	)
	for {
		fin, opcode, data, err := c.readFrame()
		if err != nil {
			return 0, nil, err
		}
		switch opcode {
		case opPing:
			if err := c.writeFrame(opPong, data); err != nil {
				return 0, nil, err
			}
			continue
		case opPong:
			continue
		case opClose:
			_ = c.writeFrame(opClose, data)
			c.closeConn()
			return 0, nil, ErrClosed
		case opText, opBinary:
			if started {
				return 0, nil, errors.New("ws: unexpected non-continuation frame")
			}
			started = true
			if opcode == opText {
				msgType = TextMessage
			} else {
				msgType = BinaryMessage
			}
			payload = append(payload, data...)
		case opContinuation:
			if !started {
				return 0, nil, errors.New("ws: unexpected continuation frame")
			}
			payload = append(payload, data...)
		default:
			return 0, nil, fmt.Errorf("ws: unknown opcode 0x%x", opcode)
		}
		if fin {
			return msgType, payload, nil
		}
	}
}

// readFrame reads a single frame, unmasking the client payload.
func (c *Conn) readFrame() (fin bool, opcode byte, payload []byte, err error) {
	var header [2]byte
	if _, err = io.ReadFull(c.br, header[:]); err != nil {
		return
	}
	fin = header[0]&0x80 != 0
	opcode = header[0] & 0x0f
	masked := header[1]&0x80 != 0
	length := uint64(header[1] & 0x7f)

	switch length {
	case 126:
		var ext [2]byte
		if _, err = io.ReadFull(c.br, ext[:]); err != nil {
			return
		}
		length = uint64(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte
		if _, err = io.ReadFull(c.br, ext[:]); err != nil {
			return
		}
		length = binary.BigEndian.Uint64(ext[:])
	}

	// RFC 6455: client-to-server frames MUST be masked.
	var maskKey [4]byte
	if masked {
		if _, err = io.ReadFull(c.br, maskKey[:]); err != nil {
			return
		}
	}

	if length > 0 {
		payload = make([]byte, length)
		if _, err = io.ReadFull(c.br, payload); err != nil {
			return
		}
		if masked {
			for i := range payload {
				payload[i] ^= maskKey[i%4]
			}
		}
	}
	return fin, opcode, payload, nil
}

// WriteMessage writes a complete data message (single, unmasked frame).
func (c *Conn) WriteMessage(t MessageType, data []byte) error {
	op := byte(opText)
	if t == BinaryMessage {
		op = opBinary
	}
	return c.writeFrame(op, data)
}

// WriteText is a convenience for sending a text message.
func (c *Conn) WriteText(s string) error {
	return c.WriteMessage(TextMessage, []byte(s))
}

// writeFrame writes a single unmasked frame with FIN set.
func (c *Conn) writeFrame(opcode byte, data []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if c.closed {
		return ErrClosed
	}

	var header []byte
	b0 := byte(0x80) | opcode // FIN + opcode
	n := len(data)
	switch {
	case n < 126:
		header = []byte{b0, byte(n)}
	case n < 65536:
		header = []byte{b0, 126, 0, 0}
		binary.BigEndian.PutUint16(header[2:], uint16(n))
	default:
		header = make([]byte, 10)
		header[0] = b0
		header[1] = 127
		binary.BigEndian.PutUint64(header[2:], uint64(n))
	}
	if _, err := c.bw.Write(header); err != nil {
		return err
	}
	if n > 0 {
		if _, err := c.bw.Write(data); err != nil {
			return err
		}
	}
	return c.bw.Flush()
}

// Close sends a close frame and tears down the connection.
func (c *Conn) Close() error {
	_ = c.writeFrame(opClose, []byte{0x03, 0xe8}) // 1000 normal closure
	return c.closeConn()
}

func (c *Conn) closeConn() error {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	return c.conn.Close()
}

// RemoteAddr returns the underlying connection's remote address.
func (c *Conn) RemoteAddr() net.Addr { return c.conn.RemoteAddr() }

// tokenListContains reports whether a comma-separated header value contains
// token (case-insensitive).
func tokenListContains(header, token string) bool {
	for _, part := range strings.Split(header, ",") {
		if strings.EqualFold(strings.TrimSpace(part), token) {
			return true
		}
	}
	return false
}
