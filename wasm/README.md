# socket.io — JavaScript adapter (WebAssembly)

Run the **same Go implementation** of socket.io's wire-protocol codecs from
JavaScript — in the browser or Node — via WebAssembly. No reimplementation:
`main.go` exposes the module's pure, portable Engine.IO and Socket.IO packet
encode/decode functions to JS and `socketio.mjs` wraps them in an idiomatic API.

Only the **codecs** are exposed — the parts that are genuinely useful in JS. The
`Server`/`Socket` types need `net/http` and are intentionally left out, so a JS
program can read and write the exact wire format the Go server speaks.

## Build

```sh
./build.sh          # produces socketio.wasm (+ copies the Go wasm_exec.js runtime)
```

## Use (Node or browser)

```js
import { loadSocketIO } from './socketio.mjs';
const sio = await loadSocketIO();

// Engine.IO framing (the transport layer):
sio.engineEncode(sio.engineTypes.message, '2["hi"]');   // '42["hi"]'
sio.engineDecode('42["hi"]');                            // { type, typeName, data }
sio.engineEncodePayload([{ type: sio.engineTypes.message, data: '0' }]);
sio.engineDecodePayload('40\x1e2');                      // [ {…}, {…} ]

// Socket.IO packets (the application layer inside a MESSAGE):
sio.sioEncode({ type: sio.sioTypes.EVENT, data: ['hello', 'world'] }); // '2["hello","world"]'
sio.sioDecode('2/admin,12["hello","world",42]');
// -> { type, typeName:'EVENT', namespace:'/admin', id:12,
//      eventName:'hello', args:['world',42], data:[…], attachments:0 }
```

### API

| Function | Signature |
| --- | --- |
| `engineEncode` | `(typeNum, data?) -> string` |
| `engineDecode` | `(str) -> { type, typeName, data }` (binary: `{ type, typeName, binary, base64 }`) |
| `engineEncodePayload` | `(Array<{type, data?}>) -> string` |
| `engineDecodePayload` | `(str) -> Array<{type, typeName, data}>` |
| `sioEncode` | `({type, namespace?, id?, data?}) -> string` |
| `sioDecode` | `(str) -> {type, typeName, namespace, id, data, attachments, eventName?, args?}` |

Plus the `engineTypes` / `sioTypes` name→number maps and `protocolVersion` /
`engineProtocol` constants. Codec errors surface as thrown `Error`s.

## Verify

```sh
./build.sh && node test.mjs
```

The adapter is compiled with `GOOS=js GOARCH=wasm`; on normal platforms
`stub.go` keeps `go build ./...` and CI green. Build artifacts (`*.wasm`) are
gitignored.
