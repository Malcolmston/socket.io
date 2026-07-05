# socket.io

[![CI](https://github.com/Malcolmston/socket.io/actions/workflows/ci.yml/badge.svg)](https://github.com/Malcolmston/socket.io/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/malcolmston/socketio.svg)](https://pkg.go.dev/github.com/malcolmston/socketio)
[![Go Report Card](https://goreportcard.com/badge/github.com/malcolmston/socketio)](https://goreportcard.com/report/github.com/malcolmston/socketio)
[![Release](https://img.shields.io/github/v/release/Malcolmston/socket.io?sort=semver)](https://github.com/Malcolmston/socket.io/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Docs](https://img.shields.io/badge/docs-pages-2f9bff)](https://malcolmston.github.io/socket.io/)

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

### Per-socket data (session-like state)

Attach arbitrary state to a socket for the lifetime of the connection — the
equivalent of `socket.data`:

```go
io.OnConnection(func(s *socketio.Socket) {
	s.Set("user", currentUser).Set("tenant", "acme")

	s.On("action", func(args []any) []any {
		user := s.GetString("user")
		v, _ := s.Get("tenant")
		_ = v
		return nil
	})
})
```

`s.Set`, `s.Get`, `s.GetString`, `s.Delete`, and `s.Data()` manage the store;
it is concurrency-safe.

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

## Connection middleware

Gate connections with `Use` (the equivalent of `io.use`). Calling `next(err)`
rejects the connection with a `connect_error`:

```go
io.Use(func(s *socketio.Socket, next func(error)) {
	if s.Auth() == nil {
		next(errors.New("unauthorized"))
		return
	}
	next(nil)
})
io.Of("/admin").Use(adminOnly) // per-namespace middleware
```

## Go client

A Go client ships in [`client`](client/), built on the same transport stack:

```go
import "github.com/malcolmston/socketio/client"

c, _ := client.Dial("http://localhost:3000")
defer c.Close()

c.On("news", func(args []any) []any { fmt.Println(args); return nil })
c.Emit("hello", "world")

reply, _ := c.EmitWithAck("ping", 5*time.Second) // blocks for the ack
```

The client supports binary payloads and automatic reconnection:

```go
c, _ := client.Dial(url, client.Options{Reconnection: true})
c.On("reconnect", func(args []any) []any { log.Println("reconnected"); return nil })
```

The server side can likewise request an acknowledgement and block for it with
`socket.EmitAck(event, timeout, args...)`. `Server.Close()` disconnects all
sessions.

## Using with express (or any net/http handler)

Attach the Socket.IO server to an existing HTTP server, letting
[`express`](https://github.com/malcolmston/express) (or an `http.ServeMux`)
handle everything else — exactly like `new Server(httpServer)` in Node:

```go
app := express.New()                       // your routes
app.Get("/api/hello", helloHandler)

io := socketio.New()                       // your socket handlers
io.OnConnection(func(s *socketio.Socket) { ... })

// io.Handler intercepts /socket.io/ and delegates the rest to app.
http.ListenAndServe(":3000", io.Handler(app))
```

`io.Handler(next)` takes any `http.Handler`; pass `nil` to 404 non-Socket.IO
requests. You can also mount it the other way — as express middleware — with
`app.Use("/socket.io", express.WrapHandler(io))`, or register it on a mux with
`io.Attach(mux)`.

## Binary events

Emit and receive `[]byte` payloads; they are carried as Socket.IO binary
attachments (native WebSocket binary frames, or base64 over polling):

```go
s.Emit("blob", []byte{0x00, 0x01, 0x02})
s.On("upload", func(args []any) []any {
	data := args[0].([]byte)
	return []any{len(data)}
})
```

## Server-wide operations

```go
io.FetchSockets()                 // all connected sockets
io.SocketsJoin("room")            // make every socket join a room
io.SocketsLeave("room")
io.DisconnectSockets(true)        // disconnect everyone
io.ServerSideEmit("event", data)  // server-to-server event (single-node: local)
```

Rooms are stored behind a pluggable `Adapter` (`ns.SetAdapter`); the default is
in-process. Broadcast flags `io.To("r").Volatile().Compress(false).Emit(...)`
are supported (advisory on this single-node implementation).

## Scaling out with Redis

For multiple server instances, install a `Broadcaster` so broadcasts fan out
across nodes. The [`redis`](redis/) subpackage provides one, speaking the Redis
pub/sub protocol directly (no third-party client):

```go
import "github.com/malcolmston/socketio/redis"

bc, _ := redis.New(redis.Options{Addr: "localhost:6379", Channel: "socket.io"})
io.SetBroadcaster(bc)
```

With a broadcaster installed, `io.To(room).Emit(...)` on any node is delivered
to matching sockets on **every** node. `Broadcaster` is a small interface
(`Publish` / `OnMessage` / `Close`), so any pub/sub transport can back it.

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
