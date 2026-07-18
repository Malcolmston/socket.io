package engineio

// This file rounds out the convenience constructors for the Engine.IO packet
// types beyond NewMessage and NewOpen, so callers building the transport
// heartbeat and lifecycle packets do not have to spell out struct literals, and
// adds a small predicate for distinguishing binary message packets.

// NewPing builds a PING packet. Engine.IO v4 servers send PING with an empty
// payload for the heartbeat; the "probe" data ("2probe") is used during a
// transport upgrade.
func NewPing(data string) Packet { return Packet{Type: Ping, Data: data} }

// NewPong builds a PONG packet answering a PING, echoing its data (an empty
// string for a heartbeat pong, "probe" for the upgrade handshake).
func NewPong(data string) Packet { return Packet{Type: Pong, Data: data} }

// NewClose builds a CLOSE packet, which requests that the transport be closed.
func NewClose() Packet { return Packet{Type: Close} }

// NewUpgrade builds an UPGRADE packet, sent by the client to complete a
// polling-to-WebSocket transport upgrade.
func NewUpgrade() Packet { return Packet{Type: Upgrade} }

// NewNoop builds a NOOP packet, which does nothing and is used to cleanly
// terminate a pending long-polling request when the transport upgrades.
func NewNoop() Packet { return Packet{Type: Noop} }

// NewBinaryMessage builds a MESSAGE packet carrying raw binary data. In a
// polling payload it is serialized as "b" + base64; over WebSocket it rides in
// a native binary frame.
func NewBinaryMessage(data []byte) Packet { return Packet{Type: Message, Binary: data} }

// IsBinary reports whether the packet is a binary MESSAGE (its Binary field is
// set), as opposed to a text packet.
func (p Packet) IsBinary() bool { return p.Binary != nil }
