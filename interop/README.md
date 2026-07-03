# Interop tests

These tests run the **real Node.js `socket.io-client`** against the Go server in
this repository, proving wire-protocol compatibility with the reference
implementation.

## Run

```sh
# 1. Start the Go server (uses the local module via a relative replace).
go run ./server.go        # listens on 127.0.0.1:9731

# 2. In another shell, install the client and run the tests.
npm install
node client.mjs           # connect / echo / acks / rooms / namespaces
node upgrade.mjs          # polling -> websocket upgrade
```

## What is covered

`client.mjs` runs the full suite over **both** transports (`polling` and
`websocket`):

- connection handshake and `socket.id`
- event emit + receive (`echo`)
- client→server acknowledgement (`emit("ping", cb)`)
- room broadcast (`io.to(room).emit`)
- server→client acknowledgement (`socket.emitWithAck`)
- namespace connection (`/admin`) and events

`upgrade.mjs` verifies the default client transparently upgrades from HTTP
long-polling to WebSocket (the Engine.IO `2probe`/`3probe`/`5` handshake).

## Last verified result

Against `socket.io-client@4` on Node 22:

```
PASS  [polling]   connect / echo / client-ack / room-broadcast / server-ack / namespace
PASS  [websocket] connect / echo / client-ack / room-broadcast / server-ack / namespace
UPGRADE PASS  (polling -> websocket)
```
