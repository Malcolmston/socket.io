package ws

import (
	"net/http"
)

// ExampleUpgrade shows how to accept a WebSocket connection inside an ordinary
// net/http handler. The handler first calls IsWebSocketUpgrade to confirm the
// request is a genuine upgrade (rejecting anything else with a 400), then calls
// Upgrade to complete the RFC 6455 handshake and hijack the connection, yielding
// a *Conn. It then loops with ReadMessage — which transparently answers ping
// control frames and reassembles fragmented messages — and echoes every text
// message back to the client with WriteText, breaking out of the loop (and
// closing the connection via the deferred Close) as soon as the peer goes away.
// The reader should take away the minimal server recipe: check, Upgrade, then
// read/write whole messages. (This example is compiled to verify the API surface
// but is not executed here, because it needs a real WebSocket peer to drive it.)
func ExampleUpgrade() {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !IsWebSocketUpgrade(r) {
			http.Error(w, "expected websocket upgrade", http.StatusBadRequest)
			return
		}
		conn, err := Upgrade(w, r)
		if err != nil {
			return
		}
		defer conn.Close()

		for {
			mt, data, err := conn.ReadMessage()
			if err != nil {
				return // peer closed or read failed
			}
			if mt == TextMessage {
				if err := conn.WriteText("echo: " + string(data)); err != nil {
					return
				}
			}
		}
	})

	// Mount the handler; ListenAndServe is omitted so the example stays
	// self-contained.
	http.Handle("/ws", handler)
}
