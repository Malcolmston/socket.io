#!/usr/bin/env bash
# Build the socket.io WebAssembly adapter.
set -euo pipefail
cd "$(dirname "$0")"
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" ./wasm_exec.js
GOOS=js GOARCH=wasm go build -ldflags="-s -w" -o socketio.wasm .
echo "built socketio.wasm ($(du -h socketio.wasm | cut -f1))"
