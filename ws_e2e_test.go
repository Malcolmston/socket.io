package socketio_test

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	socketio "github.com/malcolmston/socketio"
)

// wsClient is a minimal WebSocket + Engine.IO client for exercising the
// websocket transport end-to-end.
type wsClient struct {
	conn net.Conn
	br   *bufio.Reader
}

func dialSocketWS(t *testing.T, base string) *wsClient {
	t.Helper()
	u := strings.TrimPrefix(base, "http://")
	host := u[:strings.IndexByte(u, '/')]
	path := u[strings.IndexByte(u, '/'):] + "?EIO=4&transport=websocket"

	conn, err := net.Dial("tcp", host)
	if err != nil {
		t.Fatal(err)
	}
	key := make([]byte, 16)
	rand.Read(key)
	req := "GET " + path + " HTTP/1.1\r\n" +
		"Host: " + host + "\r\n" +
		"Upgrade: websocket\r\nConnection: Upgrade\r\n" +
		"Sec-WebSocket-Key: " + base64.StdEncoding.EncodeToString(key) + "\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"
	conn.Write([]byte(req))

	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("ws status = %d", resp.StatusCode)
	}
	return &wsClient{conn: conn, br: br}
}

func (c *wsClient) writeText(s string) {
	data := []byte(s)
	var mask [4]byte
	rand.Read(mask[:])
	n := len(data)
	var header []byte
	if n < 126 {
		header = []byte{0x81, byte(0x80 | n)}
	} else {
		header = []byte{0x81, byte(0x80 | 126), 0, 0}
		binary.BigEndian.PutUint16(header[2:], uint16(n))
	}
	masked := make([]byte, n)
	for i := range data {
		masked[i] = data[i] ^ mask[i%4]
	}
	buf := append(header, mask[:]...)
	buf = append(buf, masked...)
	c.conn.Write(buf)
}

func (c *wsClient) readText(t *testing.T) string {
	t.Helper()
	var header [2]byte
	if _, err := io.ReadFull(c.br, header[:]); err != nil {
		t.Fatal(err)
	}
	length := uint64(header[1] & 0x7f)
	switch length {
	case 126:
		var ext [2]byte
		io.ReadFull(c.br, ext[:])
		length = uint64(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte
		io.ReadFull(c.br, ext[:])
		length = binary.BigEndian.Uint64(ext[:])
	}
	payload := make([]byte, length)
	io.ReadFull(c.br, payload)
	return string(payload)
}

func TestWebSocketTransportEndToEnd(t *testing.T) {
	io := socketio.New()
	io.OnConnection(func(s *socketio.Socket) {
		s.On("echo", func(args []any) []any {
			s.Emit("echo", args...)
			return nil
		})
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.ServeHTTP(w, r)
	}))
	defer srv.Close()

	c := dialSocketWS(t, srv.URL+"/socket.io/")
	defer c.conn.Close()

	// First frame is the Engine.IO OPEN packet.
	if open := c.readText(t); !strings.HasPrefix(open, "0{") {
		t.Fatalf("expected OPEN packet, got %q", open)
	}

	// Socket.IO CONNECT to the default namespace.
	c.writeText("40")
	if ack := c.readText(t); !strings.HasPrefix(ack, "40") {
		t.Fatalf("expected CONNECT ack, got %q", ack)
	}

	// Emit an event; expect it echoed back.
	c.writeText(`42["echo","over websocket"]`)
	if echo := c.readText(t); !strings.Contains(echo, `["echo","over websocket"]`) {
		t.Fatalf("expected echo, got %q", echo)
	}
}
