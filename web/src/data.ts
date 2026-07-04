// Library content for the Socket.IO-for-Go landing site. This is the LIBS
// entry with id "socketio", copied verbatim from the malcolmston/go umbrella
// site's src/data.ts so the two stay in sync.
export interface Lib {
  id: string; name: string; icon: string; accent: string; pkg: string; node: string;
  repo: string; docs: string; tagline: string; blurb: string; tags: string[];
  features: string[]; node_code: string; go_code: string; integrate: string;
}
export const NODE_ACCENT = '#8cc84b';
export const LIB: Lib = {
  id:"socketio", name:"Socket.IO", icon:'<i class="fa-solid fa-bolt"></i>', accent:"#d2a8ff",
  pkg:"github.com/malcolmston/socketio", node:"socketio/socket.io",
  repo:"https://github.com/malcolmston/socket.io", docs:"https://malcolmston.github.io/socket.io/",
  tagline:"Bidirectional, low-latency, event-based communication.",
  blurb:"A dependency-free Socket.IO server (and Go client) speaking the real wire protocol — Engine.IO v4 "+
    "(polling + WebSocket + upgrade) and Socket.IO v5 (namespaces, rooms, acks, binary). The WebSocket layer is "+
    "RFC 6455 from scratch. Interoperates with socket.io-client@4.",
  tags:["Engine.IO v4","Socket.IO v5","RFC6455 WS","rooms","binary","Redis scale-out"],
  features:[
    "Both transports: HTTP long-polling and WebSocket, with the polling→WS upgrade",
    "Namespaces, rooms, broadcasting, acknowledgements (both directions)",
    "Per-socket data store — the equivalent of <code>socket.data</code>",
    "Connection middleware (<code>io.Use</code>), binary attachments",
    "Pluggable <code>Adapter</code> + a Redis <code>Broadcaster</code> for multi-node scale-out",
    "Mounts alongside Express via <code>io.Handler(app)</code>",
    "Ships a Go client with reconnection and <code>EmitWithAck</code>"
  ],
  node_code:
`const { Server } = require('socket.io')
const io = new Server(3000)

io.on('connection', (socket) => {
  socket.join('general')
  socket.on('chat', (msg) => {
    io.to('general').emit('chat', msg)
  })
})`,
  go_code:
`io := socketio.New()

io.OnConnection(func(s *socketio.Socket) {
    s.Join("general")
    s.On("chat", func(args []any) []any {
        io.To("general").Emit("chat", args...)
        return nil
    })
})
http.Handle(socketio.DefaultPath, io)`,
  integrate:
`<span class="tok-c">// Acks, per-socket data, and Redis scale-out</span>
io.OnConnection(func(s *socketio.Socket) {
    s.Set("user", currentUser)          <span class="tok-c">// socket.data</span>
    s.On("ping", func(args []any) []any {
        return []any{"pong"}            <span class="tok-c">// acknowledgement reply</span>
    })
})

bc, _ := redis.New(redis.Options{Addr: "localhost:6379"})
io.SetBroadcaster(bc)                    <span class="tok-c">// fan out across nodes</span>`
};
