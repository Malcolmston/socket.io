package socketio

import "encoding/json"

// Broadcaster fans broadcasts out to other server instances. Installing one
// (Server.SetBroadcaster) turns a single-node server into a cluster member: a
// broadcast is published once and each node delivers it to its own local
// sockets. The reference implementation is the Redis adapter in the redis
// subpackage, but any pub/sub transport can implement this interface.
type Broadcaster interface {
	// Publish sends a serialized broadcast to all instances (including this
	// one — pub/sub echoes to the publisher).
	Publish(data []byte) error
	// OnMessage registers the handler invoked for every received broadcast.
	OnMessage(func(data []byte))
	// Close shuts the broadcaster down.
	Close() error
}

// broadcastMessage is the wire form of a cross-node broadcast.
type broadcastMessage struct {
	Namespace string   `json:"nsp"`
	Rooms     []string `json:"rooms"`
	Except    []string `json:"except"`
	Event     string   `json:"event"`
	Args      []any    `json:"args"`
}

// SetBroadcaster installs a cluster broadcaster. Once set, every room/namespace
// broadcast is published through it and delivered to local sockets when the
// message is received back, so all nodes in the cluster stay in sync.
func (s *Server) SetBroadcaster(b Broadcaster) *Server {
	s.mu.Lock()
	s.broadcaster = b
	s.mu.Unlock()
	if b != nil {
		b.OnMessage(s.handleRemoteBroadcast)
	}
	return s
}

// handleRemoteBroadcast delivers a broadcast received from another node to this
// node's local sockets.
func (s *Server) handleRemoteBroadcast(data []byte) {
	var msg broadcastMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return
	}
	ns := s.namespace(msg.Namespace)
	if ns == nil {
		return
	}
	except := make(map[string]struct{}, len(msg.Except))
	for _, id := range msg.Except {
		except[id] = struct{}{}
	}
	(&BroadcastOperator{ns: ns, rooms: msg.Rooms, except: except}).emitLocal(msg.Event, msg.Args...)
}
