package engineio_test

import (
	"fmt"

	"github.com/malcolmston/socketio/engineio"
)

// Example demonstrates the Engine.IO codec end to end. It first builds a MESSAGE
// packet with NewMessage and renders it to the wire form with Encode, showing
// the single-digit type prefix ("4") in front of the data. It then parses a
// wire string back into a Packet with Decode and prints the decoded type (via
// PacketType.String) and data. Finally it batches an OPEN handshake packet and a
// MESSAGE packet into a single HTTP long-polling body with EncodePayload, whose
// parts are joined by the Engine.IO v4 record separator (shown here as the \x1e
// escape). The reader should take away that Encode/Decode handle one packet and
// EncodePayload/DecodePayload handle the batched polling form, and that the wire
// format is a compact, human-readable string.
func Example() {
	// Encode a single MESSAGE packet to its wire form.
	fmt.Println(engineio.NewMessage("hello").Encode())

	// Decode a wire string back into a Packet.
	p, _ := engineio.Decode("4hello")
	fmt.Printf("%s %q\n", p.Type, p.Data)

	// Batch several packets into one polling payload.
	payload := engineio.EncodePayload([]engineio.Packet{
		engineio.NewOpen(`{"sid":"abc"}`),
		engineio.NewMessage("hi"),
	})
	fmt.Printf("%q\n", payload)

	// Output:
	// 4hello
	// message "hello"
	// "0{\"sid\":\"abc\"}\x1e4hi"
}
