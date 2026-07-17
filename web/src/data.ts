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
  blurb:"A dependency-free Socket.IO server and Go client speaking the real wire protocol — Engine.IO v4 "+
    "(long-polling + WebSocket + upgrade) and Socket.IO v5 (namespaces, rooms, acks, binary attachments). The "+
    "WebSocket transport is RFC 6455 written from scratch on the standard library alone, and it interoperates "+
    "with the official socket.io-client@4 in the browser.",
  tags:["Engine.IO v4","Socket.IO v5","RFC6455 WS","namespaces","rooms","acks","binary","Redis scale-out","zero deps"],
  features:[
    "Both transports — HTTP long-polling and WebSocket — with the seamless polling→WS upgrade",
    "Namespaces via <code>io.Of</code>, rooms with <code>Socket.Join</code>/<code>Leave</code>, and room-scoped broadcasts",
    "Acknowledgements both ways: return a value from <code>On</code>, or call <code>EmitWithAck</code>/<code>EmitAck</code>",
    "Per-socket data store — <code>Set</code>/<code>Get</code>/<code>Data</code>, the equivalent of <code>socket.data</code>",
    "Connection middleware (<code>io.Use</code>) and binary attachments (nested <code>[]byte</code> args)",
    "Chainable broadcast operators: <code>To</code>, <code>In</code>, <code>Except</code>, <code>Volatile</code>, <code>Compress</code>",
    "Pluggable <code>Adapter</code> plus a stdlib Redis <code>Broadcaster</code> for multi-node scale-out",
    "Server-to-server events via <code>OnServerEvent</code> and <code>ServerSideEmit</code>",
    "Mounts alongside any handler with <code>io.Handler(next)</code> or <code>io.Attach(mux)</code>",
    "Ships a Go client (<code>client.Dial</code>) with automatic reconnection and acks"
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
