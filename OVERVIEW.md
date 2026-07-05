# Overview

`github.com/malcolmston/socketio` is a from-scratch Go implementation of a
[Socket.IO](https://socket.io) server (plus a matching client). It speaks the
same wire protocol as Node's Socket.IO — Engine.IO v4 underneath, Socket.IO v5
on top — so it interoperates with the official `socket.io-client@4`. Everything
is built on the Go standard library: no third-party dependencies, no
`node_modules`, one static binary.

This document covers three things: [how it works](#how-it-works),
[how to use it](#how-to-use-it), and [why it's better than its
predecessor](#why-its-better-than-its-predecessor) (honestly, with tradeoffs).

---

## How it works

The stack is two protocol layers riding on top of two interchangeable
transports, with an application layer (namespaces, rooms, acks) on top.

```
  application   Server / Namespace / Socket / rooms / acks / broadcasting
  ────────────────────────────────────────────────────────────────────────
  Socket.IO v5  CONNECT / EVENT / ACK / DISCONNECT / CONNECT_ERROR   (protocol.go)
  Engine.IO v4  OPEN / MESSAGE / PING / PONG / UPGRADE / CLOSE       (engineio/)
  ────────────────────────────────────────────────────────────────────────
  transport     HTTP long-polling            │   WebSocket (RFC 6455)
                (transport.go)                │   (internal/ws/)
```

### The two codecs

**Engine.IO v4** (`engineio/parser.go`) is the framing layer. A packet on the
wire is a single-digit type prefix followed by its data — `"4hello"` is a
`MESSAGE` carrying `"hello"`. `Encode`/`Decode` handle a single packet;
`EncodePayload`/`DecodePayload` handle the polling "payload" format, which
batches several packets separated by the ASCII record separator (`0x1e`). Binary
packets are base64-encoded with a `"b"` prefix when they travel inside a polling
payload.

**Socket.IO v5** (`protocol.go`) is the application-protocol layer carried inside
each Engine.IO `MESSAGE`. A `Packet` has a `Type` (`Connect`, `Event`, `Ack`,
`Disconnect`, `ConnectError`, and the `BinaryEvent`/`BinaryAck` variants), an
optional `Namespace`, an optional acknowledgement `ID`, and a JSON `Data`
payload. `Encode` renders the `"<type>[<attachments>-][<namespace>,][<id>]<json>"`
wire form; `DecodePacket` parses it back. Event helpers (`EventName`, `Args`)
pull the event name and arguments out of the decoded JSON array.

**Binary attachments** (`binary.go`) implement the Socket.IO binary sub-protocol.
`deconstruct` walks an outbound payload, replacing every `[]byte` with a
`{"_placeholder":true,"num":N}` marker and collecting the raw buffers; the packet
type is promoted to `BINARY_EVENT`/`BINARY_ACK` and the buffers follow as
separate Engine.IO binary frames. `Reconstruct` reverses this on the receiving
side.

### The two transports

**HTTP long-polling** (`transport.go`) is the baseline transport and the initial
handshake path. A `GET` with no `sid` mints a new session and returns the
Engine.IO `OPEN` packet (handshake JSON: `sid`, `upgrades`, `pingInterval`,
`pingTimeout`, `maxPayload`). Subsequent `GET`s hold the request open until the
server has packets to flush (or the ping window elapses); `POST`s carry packets
from the client. Each session (`conn` in `conn.go`) buffers outgoing packets and
parks at most one waiting poll request.

**WebSocket** (`internal/ws/ws.go`) is a minimal, dependency-free RFC 6455
implementation — just enough to carry Socket.IO traffic: the server opening
handshake (`Sec-WebSocket-Accept`), fragmented data messages, and the
ping/pong/close control frames. It supports both a fresh WebSocket connection
and an **upgrade** from an existing polling session via the Engine.IO probe
handshake (`2probe` → `3probe` → `5`, in `Server.probe`). Client-to-server frames
are unmasked on read and, for the bundled client, masked on write per the RFC.

A server-initiated **heartbeat** (`conn.pingLoop`) sends `PING` every
`PingInterval` and drops the session if a `PONG` doesn't arrive within
`PingInterval + PingTimeout`.

### Namespaces, rooms, and acks

A **`Namespace`** (`namespace.go`) partitions the server into independent
channels, each with its own set of sockets, rooms, connection handlers, and
middleware. The default namespace is `"/"`; `Server.Of("/admin")` creates or
returns others. `Namespace.Use` registers connection middleware — a middleware
that calls `next(err)` with a non-nil error rejects the connection with a
`CONNECT_ERROR`.

A **`Socket`** (`socket.go`) is one client's connection to one namespace. It is
the object applications interact with: `On`/`Off` register event handlers,
`Emit` sends events, `Join`/`Leave` manage room membership, and `Set`/`Get`
attach arbitrary per-connection state (the equivalent of `socket.data`). Every
socket implicitly joins a room named after its own id, which is how you address a
single client by id.

**Rooms** are just named sets of sockets, tracked by the adapter. Broadcasting is
expressed through a **`BroadcastOperator`** (`broadcast.go`) returned by
`.To(room)`: `io.To("room1").Except(id).Emit("evt", payload)` resolves the target
set (de-duplicated across rooms), applies exclusions, and emits.

**Acknowledgements** are two-way. The client can request an ack (the server's
handler returns a non-nil `[]any`, which is sent back as an `ACK`), and the
server can request one from the client: `Socket.EmitWithAck` (callback) or
`Socket.EmitAck` (blocking, with timeout). Dispatch of inbound events runs on its
own goroutine so a handler may block waiting on an ack without stalling the read
loop that must read the very `ACK` it's waiting for.

### The adapter interface

Room membership lives behind the **`Adapter`** interface (`adapter.go`):

```go
type Adapter interface {
    Add(socketID string, s *Socket)
    Remove(socketID string)
    Join(socketID, room string)
    Leave(socketID, room string)
    SocketsInRoom(room string) []*Socket
    AllSockets() []*Socket
    Get(socketID string) (*Socket, bool)
}
```

The default `memoryAdapter` keeps everything in process behind a `sync.RWMutex`.
`Namespace.SetAdapter` swaps in an alternative — this is the extension point for
custom membership storage.

### The Redis adapter (multi-node)

Fanning a broadcast out to sockets connected to *other* server instances is a
separate concern, handled by the **`Broadcaster`** interface (`broadcaster.go`):

```go
type Broadcaster interface {
    Publish(data []byte) error      // send a serialized broadcast to all nodes
    OnMessage(func(data []byte))    // register the received-broadcast handler
    Close() error
}
```

Install one with `Server.SetBroadcaster` and every room/namespace broadcast is
JSON-serialized and `Publish`ed once; each node (including the publisher, since
pub/sub echoes) receives it via `OnMessage` and delivers it to its own local
sockets. The reference implementation is the **`redis`** subpackage
(`redis/redis.go`): it speaks the Redis RESP protocol directly over a raw TCP
socket using only the standard library — one connection to `PUBLISH`, one
subscribed connection running a receive loop — so no third-party Redis client is
pulled in.

### The structural interfaces

`interfaces.go` collects the small shared shapes and wires each concrete type to
them with compile-time assertions:

- **`Emitter`** — anything that fans an event out to many sockets without a
  per-socket error: `Server`, `Namespace`, `BroadcastOperator`. (`Socket.Emit`
  targets one client and returns an `error`, so a `Socket` is deliberately *not*
  an `Emitter`.)
- **`RoomTargeter`** — anything exposing `To(room) *BroadcastOperator`: `Server`,
  `Namespace`, `Socket`, and `BroadcastOperator` itself.
- **`BroadcastTarget`** — the combination (`Emitter` + `RoomTargeter`) satisfied
  by the room-addressable emitters.

These let helpers be written against a contract instead of a concrete type.

---

## How to use it

Install:

```
go get github.com/malcolmston/socketio
```

### Server with rooms and acknowledgements

A complete, compiling program. Each connecting socket joins a room; a `chat`
event is broadcast to that room; a `ping` event demonstrates a client→server ack
(the handler's return value is sent back); and the server asks the client for an
ack when it receives `askme`.

```go
package main

import (
	"log"
	"net/http"
	"time"

	socketio "github.com/malcolmston/socketio"
)

func main() {
	io := socketio.New()

	io.OnConnection(func(s *socketio.Socket) {
		log.Printf("connected: %s", s.ID())
		s.Join("room1")

		// Broadcast to everyone else in the room (excludes the sender).
		s.On("chat", func(args []any) []any {
			s.To("room1").Emit("chat", args...)
			return nil
		})

		// Client → server ack: the returned slice is sent back to the caller.
		s.On("ping", func(args []any) []any {
			return []any{"pong", 42}
		})

		// Server → client ack: block up to 5s for the client's reply.
		s.On("askme", func(args []any) []any {
			reply, err := s.EmitAck("question", 5*time.Second, "what is 2+2?")
			if err != nil {
				log.Printf("no ack: %v", err)
				return nil
			}
			log.Printf("client answered: %v", reply)
			return nil
		})

		s.OnDisconnect(func(reason string) {
			log.Printf("disconnected: %s (%s)", s.ID(), reason)
		})
	})

	// A second namespace with its own handlers.
	io.Of("/admin").OnConnection(func(s *socketio.Socket) {
		s.On("whoami", func(args []any) []any { return []any{"admin"} })
	})

	http.Handle("/socket.io/", io)
	log.Println("listening on :3000")
	log.Fatal(http.ListenAndServe(":3000", nil))
}
```

Point a browser `socket.io-client@4` at `http://localhost:3000` and it connects,
upgrades to WebSocket, and exchanges events and acks with the code above.

### Mounting alongside your own `net/http` routes

`Server` implements `http.Handler`, so it can be registered directly on a
`ServeMux`. To serve Socket.IO *and* your own routes from one handler, use
`Server.Handler`, which intercepts requests under the Socket.IO path and
delegates everything else to the handler you pass:

```go
package main

import (
	"log"
	"net/http"

	socketio "github.com/malcolmston/socketio"
)

func main() {
	io := socketio.New()
	io.OnConnection(func(s *socketio.Socket) {
		s.On("hello", func(args []any) []any { return []any{"hi"} })
	})

	// Your ordinary HTTP application.
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	// io.Handler serves /socket.io/* itself and passes the rest to mux.
	log.Fatal(http.ListenAndServe(":3000", io.Handler(mux)))
}
```

`Handler` accepts any `http.Handler` (an `http.ServeMux`, a router, an Express-
style application, ...); pass `nil` to 404 everything that isn't Socket.IO.

### Scaling across nodes with Redis

Run several server instances behind a load balancer and relay broadcasts between
them over Redis pub/sub — a broadcast on any node reaches sockets on every node:

```go
import (
	socketio "github.com/malcolmston/socketio"
	"github.com/malcolmston/socketio/redis"
)

io := socketio.New()
bc, err := redis.New(redis.Options{Addr: "localhost:6379", Channel: "socket.io"})
if err != nil {
	log.Fatal(err)
}
io.SetBroadcaster(bc)
// io.To("room1").Emit(...) now reaches room1 members on all nodes.
```

### Connecting with the Go client

The `client` subpackage is a matching Go client (WebSocket transport):

```go
import "github.com/malcolmston/socketio/client"

c, err := client.Dial("http://localhost:3000")
if err != nil {
	log.Fatal(err)
}
c.On("chat", func(args []any) []any { log.Println(args); return nil })
_ = c.Emit("chat", "hello")

reply, err := c.EmitWithAck("ping", 5*time.Second)   // blocks for the ack
```

---

## Why it's better than its predecessor

The predecessor is Node.js [`socket.io`](https://github.com/socketio/socket.io).
This is an honest comparison — it wins on some axes and gives ground on others.

**A from-scratch standard-library implementation.** Both protocol codecs
(Engine.IO v4, Socket.IO v5), both transports (RFC 6455 WebSocket, HTTP
long-polling), and the Redis RESP client are written directly against the Go
standard library. There is no `ws`, no `engine.io-parser`, no `ioredis` — no
dependency tree to audit, update, or have a supply-chain incident in. `go.mod`
lists zero third-party requires.

**Verified interoperability, not just claimed.** The wire protocol is exercised
against the real `socket.io-client@4` (Node 22) over polling, WebSocket, and the
polling→WebSocket upgrade — connect handshakes, events, both ack directions, room
broadcasts, namespaces, and invalid-namespace rejection all pass. See
[`COMPATIBILITY.md`](COMPATIBILITY.md) and the harness in [`interop/`](interop/).
Existing browser and mobile Socket.IO clients keep working; only the server
changes.

**A single static binary.** Deployment is one Go executable — no Node runtime, no
`node_modules` to install, no `npm ci` in the container build. Smaller images,
faster cold starts, trivial cross-compilation.

**Multi-node out of the box.** The `redis` subpackage provides horizontal
scale-out over Redis pub/sub through the `Broadcaster` interface — the same role
`@socket.io/redis-adapter` plays for the Node server — implemented with no
external Redis client.

**Type safety and Go concurrency.** Handlers and the public API are statically
typed and checked at compile time. Concurrency uses goroutines and channels with
explicit locking rather than a single-threaded event loop, so CPU-bound handler
work doesn't block the whole process.

### Honest tradeoffs

- **Payloads are `[]any`, not statically typed events.** Event arguments arrive
  as decoded JSON (`[]any` / `map[string]any`); you assert types in the handler.
  There is no generated, per-event type safety — the safety is at the API
  boundary, not the message schema.
- **Smaller feature surface.** The Node project is mature and broad. Several of
  its advanced features are not surfaced through the high-level API here — an
  ergonomic binary-attachment API (the packet types decode, but the API targets
  JSON), connection-state recovery / packet buffering across reconnects, and
  per-message compression. See the "Not (yet) implemented" section of
  [`COMPATIBILITY.md`](COMPATIBILITY.md).
- **A younger, smaller ecosystem.** Node's Socket.IO has years of production
  hardening, a large community, and many third-party adapters and integrations.
  This implementation is deliberately focused on the core protocol and the
  verified interop surface above.

The pitch is not "strictly superior." It is: the same protocol your clients
already speak, delivered as a dependency-free, single-binary, statically typed Go
server that scales across nodes — with a smaller feature surface than the mature
Node original.
