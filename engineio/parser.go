// Package engineio implements the Engine.IO v4 protocol codec — the transport
// framing layer that Socket.IO is built on. It encodes and decodes the small
// set of Engine.IO packets and the polling "payload" that batches several
// packets into a single HTTP body.
//
// Engine.IO packet wire format (string): a single-digit type prefix followed by
// the packet data, e.g. "4hello" is a MESSAGE packet carrying "hello".
package engineio

import (
	"encoding/base64"
	"errors"
	"strings"
)

// Protocol is the Engine.IO protocol revision implemented here.
const Protocol = 4

// PacketType identifies an Engine.IO packet.
type PacketType byte

const (
	// Open is sent by the server on connection with the handshake data.
	Open PacketType = iota
	// Close requests the transport be closed.
	Close
	// Ping is part of the heartbeat (server -> client in EIO4).
	Ping
	// Pong answers a Ping.
	Pong
	// Message carries an application payload (a Socket.IO packet).
	Message
	// Upgrade signals a transport upgrade (polling -> websocket).
	Upgrade
	// Noop does nothing; used to cleanly terminate a polling request.
	Noop
)

// String returns the lowercase Engine.IO name of the packet type (e.g.
// "message", "ping"), or "unknown" for an unrecognized value.
func (t PacketType) String() string {
	switch t {
	case Open:
		return "open"
	case Close:
		return "close"
	case Ping:
		return "ping"
	case Pong:
		return "pong"
	case Message:
		return "message"
	case Upgrade:
		return "upgrade"
	case Noop:
		return "noop"
	default:
		return "unknown"
	}
}

// separator batches multiple packets in a single polling payload (EIO4 uses the
// ASCII record separator, 0x1e).
const separator = "\x1e"

// Packet is a single Engine.IO packet.
type Packet struct {
	// Type is the Engine.IO packet type (Open, Message, Ping, ...).
	Type PacketType
	// Data is the textual payload for string packets.
	Data string
	// Binary holds raw bytes for binary packets; when non-nil the packet is
	// encoded with a "b" prefix + base64 in polling payloads.
	Binary []byte
}

// ErrEmptyPacket is returned when decoding an empty packet string.
var ErrEmptyPacket = errors.New("engineio: empty packet")

// Encode renders a packet to its string wire form (used by the websocket
// transport for text frames). Binary packets return their base64 form here.
func (p Packet) Encode() string {
	if p.Binary != nil {
		return "b" + base64.StdEncoding.EncodeToString(p.Binary)
	}
	return string('0'+byte(p.Type)) + p.Data
}

// Decode parses a single packet from its string wire form.
func Decode(s string) (Packet, error) {
	if s == "" {
		return Packet{}, ErrEmptyPacket
	}
	if s[0] == 'b' {
		raw, err := base64.StdEncoding.DecodeString(s[1:])
		if err != nil {
			return Packet{}, err
		}
		return Packet{Type: Message, Binary: raw}, nil
	}
	t := PacketType(s[0] - '0')
	if t > Noop {
		return Packet{}, errors.New("engineio: invalid packet type")
	}
	return Packet{Type: t, Data: s[1:]}, nil
}

// EncodePayload joins packets into a single polling payload, separated by the
// record separator.
func EncodePayload(packets []Packet) string {
	parts := make([]string, len(packets))
	for i, p := range packets {
		parts[i] = p.Encode()
	}
	return strings.Join(parts, separator)
}

// DecodePayload splits a polling payload into its constituent packets.
func DecodePayload(payload string) ([]Packet, error) {
	if payload == "" {
		return nil, nil
	}
	parts := strings.Split(payload, separator)
	packets := make([]Packet, 0, len(parts))
	for _, part := range parts {
		p, err := Decode(part)
		if err != nil {
			return nil, err
		}
		packets = append(packets, p)
	}
	return packets, nil
}

// Convenience constructors for common packets.

// NewMessage builds a MESSAGE packet with string data.
func NewMessage(data string) Packet { return Packet{Type: Message, Data: data} }

// NewOpen builds an OPEN packet carrying handshake JSON.
func NewOpen(handshakeJSON string) Packet { return Packet{Type: Open, Data: handshakeJSON} }
