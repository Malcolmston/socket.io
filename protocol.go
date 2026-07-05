package socketio

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

// ProtocolVersion is the Socket.IO protocol revision implemented here (v5,
// which rides on Engine.IO v4).
const ProtocolVersion = 5

// PacketType identifies a Socket.IO packet.
type PacketType byte

const (
	// Connect initiates a namespace connection.
	Connect PacketType = iota
	// Disconnect leaves a namespace.
	Disconnect
	// Event carries an application event and its arguments.
	Event
	// Ack answers an Event that requested acknowledgement.
	Ack
	// ConnectError reports a failed namespace connection.
	ConnectError
	// BinaryEvent is an Event with binary attachments (decoded as text here).
	BinaryEvent
	// BinaryAck is an Ack with binary attachments (decoded as text here).
	BinaryAck
)

// String returns the Socket.IO name of the packet type (e.g. "EVENT", "ACK"),
// or "UNKNOWN" for an unrecognized value.
func (t PacketType) String() string {
	switch t {
	case Connect:
		return "CONNECT"
	case Disconnect:
		return "DISCONNECT"
	case Event:
		return "EVENT"
	case Ack:
		return "ACK"
	case ConnectError:
		return "CONNECT_ERROR"
	case BinaryEvent:
		return "BINARY_EVENT"
	case BinaryAck:
		return "BINARY_ACK"
	default:
		return "UNKNOWN"
	}
}

// Packet is a decoded Socket.IO protocol packet.
type Packet struct {
	// Type is the packet's Socket.IO type (Connect, Event, Ack, ...).
	Type PacketType
	// Namespace the packet targets; defaults to "/".
	Namespace string
	// ID is the acknowledgement id when the packet requests or answers an ack;
	// nil otherwise.
	ID *uint64
	// Data is the decoded JSON payload: an array for Event/Ack ([name, args...]
	// or [args...]) and an object for Connect/ConnectError.
	Data any
	// attachments is the number of binary buffers a BINARY_EVENT/BINARY_ACK
	// packet carries (parsed from the "<n>-" wire prefix).
	attachments int
}

// Attachments returns the declared number of binary attachments for a
// BINARY_EVENT/BINARY_ACK packet.
func (p Packet) Attachments() int { return p.attachments }

// ErrInvalidPacket indicates a malformed Socket.IO packet.
var ErrInvalidPacket = errors.New("socketio: invalid packet")

// Encode renders a packet to its Socket.IO wire form (the string carried inside
// an Engine.IO MESSAGE packet).
func (p Packet) Encode() (string, error) {
	if p.Type > BinaryAck {
		return "", ErrInvalidPacket
	}
	var b strings.Builder
	b.WriteByte('0' + byte(p.Type))

	// Binary attachment count prefix for binary packet types.
	if (p.Type == BinaryEvent || p.Type == BinaryAck) && p.attachments > 0 {
		b.WriteString(strconv.Itoa(p.attachments))
		b.WriteByte('-')
	}

	if p.Namespace != "" && p.Namespace != "/" {
		b.WriteString(p.Namespace)
		b.WriteByte(',')
	}
	if p.ID != nil {
		b.WriteString(strconv.FormatUint(*p.ID, 10))
	}
	if p.Data != nil {
		j, err := json.Marshal(p.Data)
		if err != nil {
			return "", err
		}
		b.Write(j)
	}
	return b.String(), nil
}

// DecodePacket parses a Socket.IO packet from its wire form.
func DecodePacket(s string) (Packet, error) {
	if s == "" {
		return Packet{}, ErrInvalidPacket
	}
	var p Packet
	if s[0] < '0' || s[0] > '6' {
		return Packet{}, ErrInvalidPacket
	}
	p.Type = PacketType(s[0] - '0')
	i := 1

	// Binary attachment count ("<n>-") for binary packet types.
	if p.Type == BinaryEvent || p.Type == BinaryAck {
		j := i
		for j < len(s) && isDigit(s[j]) {
			j++
		}
		if j < len(s) && s[j] == '-' {
			if n, err := strconv.Atoi(s[i:j]); err == nil {
				p.attachments = n
			}
			i = j + 1
		}
	}

	// Optional namespace, terminated by a comma.
	if i < len(s) && s[i] == '/' {
		if comma := strings.IndexByte(s[i:], ','); comma >= 0 {
			p.Namespace = s[i : i+comma]
			i += comma + 1
		} else {
			p.Namespace = s[i:]
			i = len(s)
		}
	} else {
		p.Namespace = "/"
	}

	// Optional ack id (a run of digits).
	j := i
	for j < len(s) && isDigit(s[j]) {
		j++
	}
	if j > i {
		id, err := strconv.ParseUint(s[i:j], 10, 64)
		if err != nil {
			return Packet{}, ErrInvalidPacket
		}
		p.ID = &id
		i = j
	}

	// Remaining bytes are the JSON payload.
	if i < len(s) {
		if err := json.Unmarshal([]byte(s[i:]), &p.Data); err != nil {
			return Packet{}, err
		}
	}
	return p, nil
}

// EventName returns the event name for an Event/BinaryEvent packet.
func (p Packet) EventName() string {
	if arr, ok := p.Data.([]any); ok && len(arr) > 0 {
		if name, ok := arr[0].(string); ok {
			return name
		}
	}
	return ""
}

// Args returns the event arguments (everything after the event name) for an
// Event packet, or the full array for an Ack packet.
func (p Packet) Args() []any {
	arr, ok := p.Data.([]any)
	if !ok {
		return nil
	}
	if p.Type == Event || p.Type == BinaryEvent {
		if len(arr) > 0 {
			return arr[1:]
		}
		return nil
	}
	return arr
}

// newEvent builds an EVENT packet for a namespace.
func newEvent(nsp, name string, args []any, id *uint64) Packet {
	data := make([]any, 0, len(args)+1)
	data = append(data, name)
	data = append(data, args...)
	return Packet{Type: Event, Namespace: nsp, ID: id, Data: data}
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }
