// Package socketio is a Go port of the Node.js Socket.IO server. It implements
// the Engine.IO v4 transport layer (HTTP long-polling and WebSocket, with the
// polling→websocket upgrade) and the Socket.IO v5 protocol (namespaces, rooms,
// events, and acknowledgements), exposing an API that mirrors socket.io:
//
//	io := socketio.New()
//	io.OnConnection(func(s *socketio.Socket) {
//		s.On("chat", func(args []any) []any {
//			io.To("room1").Emit("chat", args...)
//			return nil
//		})
//		s.Join("room1")
//	})
//	http.Handle("/socket.io/", io)
//	http.ListenAndServe(":3000", nil)
package socketio

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strings"
	"sync"
	"time"
)

// DefaultPath is the HTTP path Socket.IO serves from.
const DefaultPath = "/socket.io/"

// Options configures a Server.
type Options struct {
	// Path is the HTTP path the server handles (default "/socket.io/").
	Path string
	// PingInterval is how often the server sends heartbeat pings.
	PingInterval time.Duration
	// PingTimeout is how long the server waits for a pong before disconnecting.
	PingTimeout time.Duration
	// MaxPayload advertises the maximum HTTP payload size to clients.
	MaxPayload int
	// CheckOrigin, if set, authorizes cross-origin requests. When nil, all
	// origins are allowed.
	CheckOrigin func(r *http.Request) bool
}

func (o *Options) withDefaults() {
	if o.Path == "" {
		o.Path = DefaultPath
	}
	if o.PingInterval == 0 {
		o.PingInterval = 25 * time.Second
	}
	if o.PingTimeout == 0 {
		o.PingTimeout = 20 * time.Second
	}
	if o.MaxPayload == 0 {
		o.MaxPayload = 1000000
	}
}

// Server is a Socket.IO server. It implements http.Handler and should be
// mounted at its configured Path.
type Server struct {
	opts Options

	mu         sync.RWMutex
	namespaces map[string]*Namespace
	conns      map[string]*conn // engine.io sessions by sid
}

// New creates a Server. An optional Options may be supplied.
func New(opts ...Options) *Server {
	var o Options
	if len(opts) > 0 {
		o = opts[0]
	}
	o.withDefaults()
	s := &Server{
		opts:       o,
		namespaces: make(map[string]*Namespace),
		conns:      make(map[string]*conn),
	}
	// The default namespace always exists.
	s.namespaces["/"] = newNamespace(s, "/")
	return s
}

// Of returns the namespace with the given name, creating it if necessary. A
// name without a leading slash has one added.
func (s *Server) Of(name string) *Namespace {
	if name == "" {
		name = "/"
	}
	if !strings.HasPrefix(name, "/") {
		name = "/" + name
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ns, ok := s.namespaces[name]
	if !ok {
		ns = newNamespace(s, name)
		s.namespaces[name] = ns
	}
	return ns
}

// namespace returns an existing namespace or nil.
func (s *Server) namespace(name string) *Namespace {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.namespaces[name]
}

// OnConnection registers a handler invoked when a socket connects to the
// default namespace. It is the equivalent of io.on("connection", ...).
func (s *Server) OnConnection(fn func(*Socket)) {
	s.Of("/").OnConnection(fn)
}

// Emit broadcasts an event to every socket in the default namespace.
func (s *Server) Emit(event string, args ...any) {
	s.Of("/").Emit(event, args...)
}

// To returns a broadcast operator scoped to a room in the default namespace.
func (s *Server) To(room string) *BroadcastOperator {
	return s.Of("/").To(room)
}

// Sockets returns all connected sockets in the default namespace.
func (s *Server) Sockets() []*Socket {
	return s.Of("/").Sockets()
}

// ServeHTTP implements http.Handler and dispatches Engine.IO transport
// requests.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.opts.CheckOrigin != nil && r.Header.Get("Origin") != "" && !s.opts.CheckOrigin(r) {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return
	}
	// CORS: reflect the origin so browser clients can connect.
	if origin := r.Header.Get("Origin"); origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST")
		w.Header().Set("Access-Control-Allow-Headers", "content-type")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	transport := r.URL.Query().Get("transport")
	sid := r.URL.Query().Get("sid")

	switch transport {
	case "websocket":
		s.handleWebSocket(w, r, sid)
	case "polling", "":
		s.handlePolling(w, r, sid)
	default:
		http.Error(w, "unknown transport", http.StatusBadRequest)
	}
}

// getConn returns the session for a sid, or nil.
func (s *Server) getConn(sid string) *conn {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.conns[sid]
}

// newConn creates and registers a new engine.io session.
func (s *Server) newConn() *conn {
	c := &conn{
		sid:      newID(),
		server:   s,
		sockets:  make(map[string]*Socket),
		lastPong: time.Now(),
	}
	s.mu.Lock()
	s.conns[c.sid] = c
	s.mu.Unlock()
	return c
}

// removeConn deregisters a session and cleans up its sockets.
func (s *Server) removeConn(c *conn) {
	s.mu.Lock()
	delete(s.conns, c.sid)
	s.mu.Unlock()
	c.cleanup()
}

// newID generates a Socket.IO-style base64url identifier.
func newID() string {
	b := make([]byte, 15)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
