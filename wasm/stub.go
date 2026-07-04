//go:build !(js && wasm)

// This file builds on non-wasm platforms so that `go build ./...` and CI stay
// green. The real adapter lives in main.go behind the `js && wasm` tag.
package main

import "fmt"

func main() {
	fmt.Println("socketio wasm adapter — build with: GOOS=js GOARCH=wasm go build -o socketio.wasm ./wasm")
}
