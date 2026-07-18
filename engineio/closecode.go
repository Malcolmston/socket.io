package engineio

import (
	"encoding/binary"
	"errors"
)

// CloseCode is a WebSocket close status code (RFC 6455 §7.4). Engine.IO runs
// its WebSocket transport over RFC 6455 frames, so a close frame's two-byte
// status code — and any UTF-8 reason that follows it — is the signal that
// carries why a transport shut down. These helpers encode and decode that
// payload without pulling in a third-party WebSocket library.
type CloseCode uint16

// RFC 6455 §7.4.1 registered close codes, plus the pseudo-codes reserved for
// applications that never appear on the wire.
const (
	// CloseNormalClosure (1000) indicates a normal closure: the purpose for
	// which the connection was established has been fulfilled.
	CloseNormalClosure CloseCode = 1000
	// CloseGoingAway (1001) indicates an endpoint is going away, e.g. a server
	// shutting down or a browser navigating away from a page.
	CloseGoingAway CloseCode = 1001
	// CloseProtocolError (1002) indicates termination due to a protocol error.
	CloseProtocolError CloseCode = 1002
	// CloseUnsupportedData (1003) indicates receipt of a data type the endpoint
	// cannot accept (e.g. binary where only text is understood).
	CloseUnsupportedData CloseCode = 1003
	// CloseNoStatusReceived (1005) is a reserved pseudo-code meaning no status
	// code was present in the close frame. It must not be sent on the wire.
	CloseNoStatusReceived CloseCode = 1005
	// CloseAbnormalClosure (1006) is a reserved pseudo-code meaning the
	// connection closed without a close frame. It must not be sent on the wire.
	CloseAbnormalClosure CloseCode = 1006
	// CloseInvalidFramePayloadData (1007) indicates a message contained data
	// inconsistent with its type (e.g. non-UTF-8 in a text message).
	CloseInvalidFramePayloadData CloseCode = 1007
	// ClosePolicyViolation (1008) indicates a message violated the endpoint's
	// policy.
	ClosePolicyViolation CloseCode = 1008
	// CloseMessageTooBig (1009) indicates a message was too big to process —
	// the code Engine.IO uses when maxHttpBufferSize is exceeded.
	CloseMessageTooBig CloseCode = 1009
	// CloseMandatoryExtension (1010) indicates the client expected the server
	// to negotiate an extension that it did not.
	CloseMandatoryExtension CloseCode = 1010
	// CloseInternalServerErr (1011) indicates the server hit an unexpected
	// condition that prevented it from fulfilling the request.
	CloseInternalServerErr CloseCode = 1011
	// CloseServiceRestart (1012) indicates the server is restarting.
	CloseServiceRestart CloseCode = 1012
	// CloseTryAgainLater (1013) indicates the server is overloaded and the
	// client should retry after a delay.
	CloseTryAgainLater CloseCode = 1013
	// CloseTLSHandshake (1015) is a reserved pseudo-code meaning the TLS
	// handshake failed. It must not be sent on the wire.
	CloseTLSHandshake CloseCode = 1015
)

// ErrInvalidCloseFrame indicates a close-frame payload that is neither empty nor
// a valid two-byte-code (optionally UTF-8 reason) body.
var ErrInvalidCloseFrame = errors.New("engineio: invalid close frame payload")

// String returns a short human-readable description of the close code, e.g.
// "normal closure" for 1000, or "close code <n>" for an unregistered value.
func (c CloseCode) String() string {
	switch c {
	case CloseNormalClosure:
		return "normal closure"
	case CloseGoingAway:
		return "going away"
	case CloseProtocolError:
		return "protocol error"
	case CloseUnsupportedData:
		return "unsupported data"
	case CloseNoStatusReceived:
		return "no status received"
	case CloseAbnormalClosure:
		return "abnormal closure"
	case CloseInvalidFramePayloadData:
		return "invalid frame payload data"
	case ClosePolicyViolation:
		return "policy violation"
	case CloseMessageTooBig:
		return "message too big"
	case CloseMandatoryExtension:
		return "mandatory extension"
	case CloseInternalServerErr:
		return "internal server error"
	case CloseServiceRestart:
		return "service restart"
	case CloseTryAgainLater:
		return "try again later"
	case CloseTLSHandshake:
		return "TLS handshake"
	default:
		return "close code " + itoa(uint16(c))
	}
}

// IsValid reports whether the code may legally be sent in a close frame. The
// reserved pseudo-codes (1005, 1006, 1015) and codes below 1000 or in the
// unassigned 1016–2999 range are not valid to send; 3000–4999 (library and
// application use) are accepted.
func (c CloseCode) IsValid() bool {
	switch c {
	case CloseNoStatusReceived, CloseAbnormalClosure, CloseTLSHandshake:
		return false
	}
	if c >= 1000 && c <= 1013 {
		return true
	}
	if c >= 3000 && c <= 4999 {
		return true
	}
	return false
}

// EncodeCloseFrame builds the payload of a WebSocket close frame: the two-byte
// big-endian status code followed by the UTF-8 reason. Pass an empty reason for
// a code-only frame.
func EncodeCloseFrame(code CloseCode, reason string) []byte {
	out := make([]byte, 2+len(reason))
	binary.BigEndian.PutUint16(out[:2], uint16(code))
	copy(out[2:], reason)
	return out
}

// DecodeCloseFrame parses a WebSocket close-frame payload into its status code
// and reason. An empty payload decodes to CloseNoStatusReceived with an empty
// reason (per RFC 6455, a close frame may omit the status code). A one-byte
// payload is invalid and returns ErrInvalidCloseFrame.
func DecodeCloseFrame(payload []byte) (CloseCode, string, error) {
	if len(payload) == 0 {
		return CloseNoStatusReceived, "", nil
	}
	if len(payload) == 1 {
		return 0, "", ErrInvalidCloseFrame
	}
	code := CloseCode(binary.BigEndian.Uint16(payload[:2]))
	return code, string(payload[2:]), nil
}

// itoa renders a uint16 without importing strconv, keeping this file's
// dependency footprint minimal.
func itoa(v uint16) string {
	if v == 0 {
		return "0"
	}
	var buf [5]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
