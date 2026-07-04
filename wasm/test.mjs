// Node smoke test: build must be run first (see build.sh). Verifies the Go
// wire-protocol codec is reachable from JS through wasm and round-trips
// packets, checking against known wire strings from the Go tests.
import assert from 'node:assert';
import { loadSocketIO } from './socketio.mjs';

const sio = await loadSocketIO();

// --- Engine.IO codec ------------------------------------------------------
// Known wire string from engineio/parser_test.go: Message + `2["ev"]`.
const eioWire = sio.engineEncode(sio.engineTypes.message, '2["ev"]');
assert.strictEqual(eioWire, '42["ev"]', 'engine encode matches Go test');

const eio = sio.engineDecode(eioWire);
assert.strictEqual(eio.type, sio.engineTypes.message);
assert.strictEqual(eio.typeName, 'message');
assert.strictEqual(eio.data, '2["ev"]', 'engine decode round-trips data');

// Engine.IO payload batching (0x1e separator), also from the Go tests.
const payload = sio.engineEncodePayload([
  { type: sio.engineTypes.message, data: '0' },
  { type: sio.engineTypes.message, data: '2["hello","world"]' },
  { type: sio.engineTypes.ping },
]);
assert.strictEqual(payload, '40\x1e42["hello","world"]\x1e2', 'payload matches Go test');
const decoded = sio.engineDecodePayload(payload);
assert.strictEqual(decoded.length, 3);
assert.strictEqual(decoded[2].typeName, 'ping');

// --- Socket.IO codec ------------------------------------------------------
// Known wire string from protocol_test.go: EVENT ["hello","world"].
const sioWire = sio.sioEncode({ type: sio.sioTypes.EVENT, data: ['hello', 'world'] });
assert.strictEqual(sioWire, '2["hello","world"]', 'sio encode matches Go test');

// Full round-trip with namespace + ack id, asserting fields survive.
const pkt = { type: sio.sioTypes.EVENT, namespace: '/admin', id: 12, data: ['hello', 'world', 42] };
const wire = sio.sioEncode(pkt);
assert.strictEqual(wire, '2/admin,12["hello","world",42]', 'namespaced event wire string');

const back = sio.sioDecode(wire);
assert.strictEqual(back.type, sio.sioTypes.EVENT);
assert.strictEqual(back.typeName, 'EVENT');
assert.strictEqual(back.namespace, '/admin');
assert.strictEqual(back.id, 12);
assert.strictEqual(back.eventName, 'hello');
assert.deepStrictEqual(back.args, ['world', 42], 'decoded args survive round-trip');
assert.deepStrictEqual(back.data, ['hello', 'world', 42]);

// CONNECT packet with an object payload.
const conn = sio.sioDecode('0/chat,{"token":"xyz"}');
assert.strictEqual(conn.typeName, 'CONNECT');
assert.strictEqual(conn.namespace, '/chat');
assert.deepStrictEqual(conn.data, { token: 'xyz' });

// Errors surface as JS exceptions.
assert.throws(() => sio.sioDecode('9[]'), /invalid packet/, 'invalid type throws');

console.log('socketio wasm adapter: all JS-side assertions passed');
console.log(`  Socket.IO protocol v${sio.protocolVersion} on Engine.IO v${sio.engineProtocol}`);
console.log(`  round-trip: ${JSON.stringify(pkt)}  <->  ${wire}`);
process.exit(0);
