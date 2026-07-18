package socketio

import (
	"net"
	"net/http"
	"net/url"
	"time"
)

// Handshake describes the details of the HTTP request that established a
// connection — the Go equivalent of Socket.IO's socket.handshake object. It is
// captured once, at connection time, and never changes for the life of the
// socket. Middleware and connection handlers inspect it to authorize or
// annotate a connection (origin, forwarded address, query string, auth
// payload).
type Handshake struct {
	// Headers is the set of HTTP request headers.
	Headers http.Header
	// Time is when the handshake was processed.
	Time time.Time
	// Address is the remote network address the connection originated from
	// (honoring X-Forwarded-For when present).
	Address string
	// XDomain reports whether the request carried an Origin header (a
	// cross-origin, browser-initiated request).
	XDomain bool
	// Secure reports whether the request arrived over TLS (https/wss).
	Secure bool
	// Issued is the handshake time in Unix milliseconds, matching the JS
	// handshake.issued field.
	Issued int64
	// URL is the request URI (path and query) of the handshake request.
	URL string
	// Query holds the parsed query-string parameters.
	Query url.Values
	// Auth is the authentication payload the client supplied with CONNECT.
	Auth any
}

// NewHandshake builds a Handshake from an incoming HTTP request and the auth
// payload sent with the Socket.IO CONNECT packet. The remote address prefers the
// first entry of an X-Forwarded-For header (for connections behind a proxy) and
// otherwise falls back to the request's RemoteAddr with any port stripped.
func NewHandshake(r *http.Request, auth any) *Handshake {
	now := time.Now()
	h := &Handshake{
		Headers: r.Header,
		Time:    now,
		Address: handshakeAddress(r),
		XDomain: r.Header.Get("Origin") != "",
		Secure:  r.TLS != nil,
		Issued:  now.UnixMilli(),
		URL:     r.RequestURI,
		Query:   r.URL.Query(),
		Auth:    auth,
	}
	return h
}

// Get returns the first value of a request header (case-insensitive), or "" if
// absent — a convenience over reaching into Headers directly.
func (h *Handshake) Get(header string) string {
	if h.Headers == nil {
		return ""
	}
	return h.Headers.Get(header)
}

// Param returns the first value of a query-string parameter, or "" if absent.
func (h *Handshake) Param(key string) string {
	if h.Query == nil {
		return ""
	}
	return h.Query.Get(key)
}

// handshakeAddress derives the client address, preferring the first hop in an
// X-Forwarded-For header and stripping the port from RemoteAddr otherwise.
func handshakeAddress(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		if i := indexByteHS(fwd, ','); i >= 0 {
			return trimSpaceHS(fwd[:i])
		}
		return trimSpaceHS(fwd)
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

func indexByteHS(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func trimSpaceHS(s string) string {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
