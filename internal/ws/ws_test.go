package ws

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
)

// echoHandler upgrades and echoes each text message back.
func echoServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := Upgrade(w, r)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer c.Close()
		for {
			mt, data, err := c.ReadMessage()
			if err != nil {
				return
			}
			if err := c.WriteMessage(mt, data); err != nil {
				return
			}
		}
	}))
}

// dialWS performs a client handshake and returns a framed client connection.
func dialWS(t *testing.T, url string) *clientConn {
	t.Helper()
	host := strings.TrimPrefix(url, "http://")
	conn, err := net.Dial("tcp", host)
	if err != nil {
		t.Fatal(err)
	}
	keyBytes := make([]byte, 16)
	rand.Read(keyBytes)
	key := base64.StdEncoding.EncodeToString(keyBytes)

	req := "GET / HTTP/1.1\r\n" +
		"Host: " + host + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: " + key + "\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		t.Fatal(err)
	}

	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("status = %d, want 101", resp.StatusCode)
	}
	if got, want := resp.Header.Get("Sec-WebSocket-Accept"), acceptKey(key); got != want {
		t.Fatalf("accept = %q, want %q", got, want)
	}
	return &clientConn{conn: conn, br: br}
}

type clientConn struct {
	conn net.Conn
	br   *bufio.Reader
}

// writeText sends a masked client text frame.
func (c *clientConn) writeText(s string) error {
	data := []byte(s)
	var mask [4]byte
	rand.Read(mask[:])

	var header []byte
	b0 := byte(0x80 | opText)
	n := len(data)
	switch {
	case n < 126:
		header = []byte{b0, byte(0x80 | n)}
	default:
		header = []byte{b0, byte(0x80 | 126), 0, 0}
		binary.BigEndian.PutUint16(header[2:], uint16(n))
	}
	masked := make([]byte, n)
	for i := range data {
		masked[i] = data[i] ^ mask[i%4]
	}
	buf := append(header, mask[:]...)
	buf = append(buf, masked...)
	_, err := c.conn.Write(buf)
	return err
}

// readText reads a single unmasked server text frame.
func (c *clientConn) readText(t *testing.T) string {
	t.Helper()
	var header [2]byte
	if _, err := io.ReadFull(c.br, header[:]); err != nil {
		t.Fatal(err)
	}
	length := uint64(header[1] & 0x7f)
	if length == 126 {
		var ext [2]byte
		io.ReadFull(c.br, ext[:])
		length = uint64(binary.BigEndian.Uint16(ext[:]))
	}
	payload := make([]byte, length)
	io.ReadFull(c.br, payload)
	return string(payload)
}

func TestWebSocketEcho(t *testing.T) {
	srv := echoServer(t)
	defer srv.Close()

	c := dialWS(t, srv.URL)
	defer c.conn.Close()

	for _, msg := range []string{"hello", "world", strings.Repeat("x", 200)} {
		if err := c.writeText(msg); err != nil {
			t.Fatal(err)
		}
		if got := c.readText(t); got != msg {
			t.Fatalf("echo = %q, want %q", got, msg)
		}
	}
}

func TestRejectsNonUpgrade(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	if IsWebSocketUpgrade(req) {
		t.Fatal("plain request should not be a websocket upgrade")
	}
}
