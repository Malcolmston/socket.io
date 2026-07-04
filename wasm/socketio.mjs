// Idiomatic JS wrapper around the socket.io WebAssembly adapter.
//
//   import { loadSocketIO } from './socketio.mjs';
//   const sio = await loadSocketIO();          // browser (fetch) or Node
//
//   // Engine.IO framing:
//   sio.engineEncode(sio.engineTypes.message, '2["hi"]');   // '42["hi"]'
//   sio.engineDecode('42["hi"]');                            // { type, typeName, data }
//
//   // Socket.IO packets:
//   sio.sioEncode({ type: sio.sioTypes.EVENT, data: ['hello', 'world'] });
//   sio.sioDecode('2/admin,12["hello","world",42]');
//
// The same Go codec that powers the Go module runs here via wasm.

async function ensureGo() {
  if (typeof globalThis.Go === 'function') return;
  if (typeof window === 'undefined') {
    // Node: wasm_exec.js is a classic script that assigns globalThis.Go.
    const { readFileSync } = await import('node:fs');
    const { fileURLToPath } = await import('node:url');
    const path = fileURLToPath(new URL('./wasm_exec.js', import.meta.url));
    const { runInThisContext } = await import('node:vm');
    runInThisContext(readFileSync(path, 'utf8'));
  } else {
    await import('./wasm_exec.js');
  }
}

async function readWasm(wasmPath) {
  if (typeof window === 'undefined') {
    const { readFileSync } = await import('node:fs');
    const { fileURLToPath } = await import('node:url');
    const p = wasmPath ?? fileURLToPath(new URL('./socketio.wasm', import.meta.url));
    return readFileSync(p);
  }
  const res = await fetch(wasmPath ?? new URL('./socketio.wasm', import.meta.url));
  return new Uint8Array(await res.arrayBuffer());
}

// unwrap surfaces Go-side errors (returned as { __error }) as JS exceptions.
function unwrap(r) {
  if (r && typeof r === 'object' && r.__error !== undefined) {
    throw new Error(r.__error);
  }
  return r;
}

export async function loadSocketIO(wasmPath) {
  await ensureGo();
  const go = new globalThis.Go();
  const bytes = await readWasm(wasmPath);
  const { instance } = await WebAssembly.instantiate(bytes, go.importObject);
  go.run(instance); // long-running; resolves when the module exits (it won't)
  const g = globalThis.__mgo_socketio;
  if (!g) throw new Error('socketio wasm did not register __mgo_socketio');

  return {
    // Engine.IO codec.
    engineEncode: (type, data = '') => unwrap(g.engineEncode(Number(type), String(data))),
    engineDecode: (str) => unwrap(g.engineDecode(String(str))),
    engineEncodePayload: (packets = []) => unwrap(g.engineEncodePayload(packets)),
    engineDecodePayload: (str) => unwrap(g.engineDecodePayload(String(str))),

    // Socket.IO codec.
    sioEncode: (packet) => unwrap(g.sioEncode(packet ?? {})),
    sioDecode: (str) => unwrap(g.sioDecode(String(str))),

    // Constants.
    engineTypes: g.engineTypes,
    sioTypes: g.sioTypes,
    protocolVersion: g.protocolVersion,
    engineProtocol: g.engineProtocol,
  };
}
