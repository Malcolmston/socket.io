# Backlog — missing features & gaps

Curated real work for this Socket.IO port. (See the note at the bottom about the
"10,000" target.)

## Protocol / engine

- [ ] Engine.IO: `maxHttpBufferSize` enforcement, per-message-deflate
      (permessage-deflate) compression, WebSocket close-code handling.
- [ ] Binary attachments over the polling transport (base64) end-to-end tests.
- [ ] Connection State Recovery (resume missed packets after a brief disconnect).
- [ ] Packet buffering while disconnected + `volatile` real drop semantics.
- [ ] Heartbeat tuning per-connection; idle-timeout eviction on polling.
- [ ] EIO v3 compatibility mode (Socket.IO v2 clients).
- [ ] `wsEngine` options, `perMessageDeflate` thresholds.
- [ ] CORS full option set (methods, headers, credentials, maxAge, dynamic).

## Server API

- [ ] Dynamic namespaces: `io.Of(regexp)` + parent-namespace hooks.
- [ ] Namespace middleware error types + `connect_error` data payloads.
- [ ] `io.timeout(ms).emit(...)` broadcast with acks + per-socket ack aggregation.
- [ ] `socket.broadcast`, `socket.local`, `socket.compress`, `socket.volatile`
      as first-class chainable flags (partial today).
- [ ] `io.except(room)`, `io.in(room)` (alias done), room-based `fetchSockets`
      with cross-node support.
- [ ] `socket.handshake` object (headers, query, auth, address, issued, url).
- [ ] `socket.onAny` / `socket.offAny` / `socket.prependAny` catch-all listeners.
- [ ] `socket.use()` per-socket packet middleware.
- [ ] `socket.timeout()` per-emit ack timeout chaining.
- [ ] Disconnecting vs disconnected lifecycle events with reasons enum.
- [ ] `io.serverSideEmit` with acks across the cluster.

## Adapters (scale-out)

- [ ] Redis adapter (pub/sub) implementing the `Adapter` interface.
- [ ] Redis Streams / sharded adapter, cluster adapter, MongoDB adapter.
- [ ] Postgres adapter, NATS adapter, AMQP adapter.
- [ ] Sticky-session helpers for multi-node polling.
- [ ] `@socket.io/admin-ui` instrumentation endpoint.

## Client

- [ ] Polling transport in the Go client (currently WebSocket-only) + upgrade.
- [ ] Multiplexing multiple namespaces over one client connection.
- [ ] Client-side `volatile`, `timeout`, and offline emit buffering.
- [ ] Automatic reconnection backoff jitter + `reconnectionAttempts` events
      (`reconnect_attempt`, `reconnect_error`, `reconnect_failed`).
- [ ] Manager/Socket split mirroring the JS client.
- [ ] Auth refresh on reconnect.

## Testing / tooling

- [ ] Interop tests against the Node **server** (Go client ↔ Node server).
- [ ] Binary interop with `socket.io-client` over polling.
- [ ] Load/soak tests; fuzz the Engine.IO and Socket.IO parsers.
- [ ] `golangci-lint` + race CI on the full matrix.

---

### On the "10,000 items" request

This lists real, actionable gaps rather than padding to 10,000 synthetic
entries. The adapters and client-parity items are where most genuine volume
lives; ask and I'll break any of them into a detailed sub-checklist.
