# socket.io

**Node's Socket.IO, for Go.**

A dependency-free Go port of the [Socket.IO](https://socket.io/) server. It
implements the real wire protocol — **Engine.IO v4** (HTTP long-polling and
WebSocket, including the polling→WebSocket upgrade) and **Socket.IO v5**
(namespaces, rooms, events, and acknowledgements) — so it interoperates with
standard Socket.IO clients. The WebSocket transport is implemented from scratch
on `net/http`; there are **no third-party dependencies**.

```go
package main

import (
	"net/http"

	socketio "github.com/malcolmston/socketio"
)

func main() {
	io := socketio.New()

	io.OnConnection(func(s *socketio.Socket) {
		s.Join("general")

		s.On("chat", func(args []any) []any {
			io.To("general").Emit("chat", args...) // broadcast to the room
			return nil
		})

		s.On("ping", func(args []any) []any {
			return []any{"pong"} // acknowledgement reply
		})
	})

	http.Handle(socketio.DefaultPath, io)
	http.ListenAndServe(":3000", nil)
}
```

## Install

```sh
go get github.com/malcolmston/socketio
```

## Design

The implementation is layered exactly like the Node original:

| Layer | Package | Responsibility |
| ----- | ------- | -------------- |
| Engine.IO codec | `engineio` | packet + polling-payload encode/decode |
| WebSocket | `internal/ws` | RFC 6455 server (handshake, framing, control frames) |
| Socket.IO codec | `socketio` (`protocol.go`) | CONNECT/EVENT/ACK/... packet encode/decode |
| Server | `socketio` | transports, sessions, namespaces, rooms, sockets |

Both codecs are pure and independently unit-tested; the server is exercised
end-to-end over **both** transports.

## The API

### Server

```go
io := socketio.New()                 // default options
io := socketio.New(socketio.Options{ // or configure
	PingInterval: 25 * time.Second,
	PingTimeout:  20 * time.Second,
})

io.OnConnection(func(s *socketio.Socket) { ... }) // default namespace
io.Of("/admin").OnConnection(func(s *socketio.Socket) { ... })
io.Emit("event", data)               // broadcast to everyone
io.To("room").Emit("event", data)    // broadcast to a room
```

An `*socketio.Server` is an `http.Handler`; mount it at `socketio.DefaultPath`
(`/socket.io/`).

### Socket

```go
s.ID()                               // unique socket id
s.Auth()                             // CONNECT auth payload
s.On("event", handler)               // register a handler
s.Emit("event", args...)             // send to this client
s.EmitWithAck("event", cb, args...)  // send and await the client's ack
s.Join("room"); s.Leave("room")      // room membership
s.To("room").Emit("event", data)     // broadcast to a room, excluding self
s.Broadcast().Emit("event", data)    // everyone except this socket
s.OnDisconnect(func(reason string){})
s.Disconnect(true)                   // force-disconnect
```

### Handlers & acknowledgements

An event handler receives the event arguments and returns an optional
acknowledgement payload:

```go
s.On("ping", func(args []any) []any {
	return []any{"pong"} // sent back only if the client requested an ack
})
```

Return `nil` for no acknowledgement.

### Rooms & broadcasting

```go
s.Join("room42")
io.To("room42").Emit("news", payload)          // all members
s.To("room42").Emit("news", payload)           // members except the sender
io.Of("/admin").To("ops").Emit("alert", data)  // per-namespace
```

## Transports

- **HTTP long-polling** — the default the JS client opens with; fully supported
  (handshake, GET poll, POST).
- **WebSocket** — implemented from scratch (RFC 6455). Clients may connect
  directly (`transports: ['websocket']`) or start on polling and upgrade; the
  Engine.IO `2probe`/`3probe`/`5` upgrade handshake is handled.

## Status & scope

Supported: Engine.IO v4 framing, both transports + upgrade, heartbeat, the
Socket.IO v5 text protocol (CONNECT, DISCONNECT, EVENT, ACK, CONNECT_ERROR),
multiple namespaces, rooms, broadcasting, and acknowledgements in both
directions. Binary attachments (BINARY_EVENT/BINARY_ACK) are parsed but the
convenience API focuses on JSON payloads.

## Example

```sh
go run ./examples/chat
```

## License

[MIT](LICENSE)
