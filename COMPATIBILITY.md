# Compatibility

This Go server speaks the same wire protocol as Node's Socket.IO, so it
interoperates with the official clients. Compatibility is **verified against the
real `socket.io-client`**, not just asserted — see [`interop/`](interop/).

## Protocol versions

| Protocol | Version | Notes |
| -------- | ------- | ----- |
| Engine.IO | 4 (`EIO=4`) | packet types, `\x1e`-separated polling payloads, base64 binary |
| Socket.IO | 5 | CONNECT/DISCONNECT/EVENT/ACK/CONNECT_ERROR |

Matches `socket.io@4.x` / `socket.io-client@4.x`.

## Verified interop (socket.io-client@4, Node 22)

Run over **both** transports and the upgrade path:

| Feature | polling | websocket |
| ------- | :-----: | :-------: |
| Connect handshake + `socket.id` | ✅ | ✅ |
| Event emit / receive | ✅ | ✅ |
| Client → server ack (`emit(ev, cb)`) | ✅ | ✅ |
| Server → client ack (`emitWithAck`) | ✅ | ✅ |
| Room broadcast (`io.to(room).emit`) | ✅ | ✅ |
| Namespaces (`/admin`) | ✅ | ✅ |
| Invalid namespace → `connect_error` | ✅ | ✅ |
| Polling → WebSocket upgrade | ✅ (probe handshake) | — |

Reproduce with the harness in [`interop/`](interop/README.md).

## Supported

- Engine.IO v4 framing and both transports (long-polling + WebSocket) with the
  `2probe`/`3probe`/`5` upgrade handshake.
- Server-initiated heartbeat (`pingInterval` / `pingTimeout`, advertised in the
  handshake and honored by clients).
- Socket.IO v5 text protocol: CONNECT (with auth payload), DISCONNECT, EVENT,
  ACK, CONNECT_ERROR.
- Multiple namespaces, rooms, broadcasting, and two-way acknowledgements.
- CORS preflight + `Access-Control-Allow-Origin` reflection for browser clients.

## Not (yet) implemented

These are parsed or stubbed but not surfaced through the high-level API:

- Binary attachments (`BINARY_EVENT` / `BINARY_ACK`): the packet types decode,
  but the ergonomic API targets JSON payloads. Binary is carried as base64 in
  polling.
- Connection state recovery / packet buffering across reconnects.
- Built-in adapters for multi-node scale-out (Redis adapter). The room adapter
  is in-process; the `Namespace` room API is the extension point.
- Per-message compression (`permessage-deflate`).

None of these affect the verified single-node interop above; they are advanced
features of the Node server that a future iteration can add.
