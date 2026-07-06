package redis_test

import (
	"log"

	socketio "github.com/malcolmston/socketio"
	"github.com/malcolmston/socketio/redis"
)

// ExampleNew shows how to turn a single-node Socket.IO server into a cluster
// member. It calls New to connect to Redis and subscribe to a shared pub/sub
// channel, returning a *Broadcaster that satisfies socketio.Broadcaster; the
// deferred Close tears the connections down on exit. It then creates a
// socketio.Server and installs the broadcaster with SetBroadcaster, after which
// every broadcast — here io.To("room1").Emit — is published once to Redis and
// delivered to the matching local sockets of every node subscribed to the same
// channel, including this one. Run this same code on each instance behind your
// load balancer and a message emitted on any node reaches clients connected to
// all of them. The reader should take away the three-line scale-out recipe:
// redis.New, SetBroadcaster, then broadcast as usual. (The example is compiled
// to verify the API but not executed here, as it needs a running Redis server.)
func ExampleNew() {
	bc, err := redis.New(redis.Options{
		Addr:    "localhost:6379",
		Channel: "socket.io",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer bc.Close()

	io := socketio.New()
	io.SetBroadcaster(bc)

	// Fans out across every node subscribed to the "socket.io" channel.
	io.To("room1").Emit("news", "hello cluster")
}
