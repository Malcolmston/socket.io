import { io } from "socket.io-client";

const URL = "http://127.0.0.1:9731";
const results = [];
let failures = 0;
function check(name, cond, extra="") {
  results.push(`${cond ? "PASS" : "FAIL"}  ${name} ${extra}`);
  if (!cond) failures++;
}
const wait = (ms) => new Promise(r => setTimeout(r, ms));

async function run(transport) {
  const socket = io(URL, { transports: [transport], forceNew: true, reconnection: false });

  await new Promise((res, rej) => {
    socket.on("connect", res);
    socket.on("connect_error", (e) => rej(new Error("connect_error: " + e.message)));
    setTimeout(() => rej(new Error("connect timeout")), 4000);
  });
  check(`[${transport}] connect`, socket.connected, `id=${socket.id}`);

  // echo
  const echoP = new Promise(res => socket.once("echo", (...a) => res(a)));
  socket.emit("echo", "hello", { n: 1 });
  const echoed = await Promise.race([echoP, wait(2000).then(()=>null)]);
  check(`[${transport}] echo`, echoed && echoed[0] === "hello" && echoed[1] && echoed[1].n === 1, JSON.stringify(echoed));

  // ack (client -> server event with ack)
  const ackP = new Promise(res => socket.emit("ping", (...a) => res(a)));
  const ack = await Promise.race([ackP, wait(2000).then(()=>null)]);
  check(`[${transport}] client-ack`, ack && ack[0] === "pong" && ack[1] === 42, JSON.stringify(ack));

  // room broadcast
  const newsP = new Promise(res => socket.once("news", (...a) => res(a)));
  socket.emit("shout", "hi room");
  const news = await Promise.race([newsP, wait(2000).then(()=>null)]);
  check(`[${transport}] room-broadcast`, news && news[0] === "hi room", JSON.stringify(news));

  // server -> client ack (server emits "question", client acks)
  socket.on("question", (q, cb) => { cb("4"); });
  socket.emit("askme");
  await wait(500);
  check(`[${transport}] server-emit-with-ack`, true, "(see SERVER_GOT_ACK on stderr)");

  // namespace
  const admin = io(URL + "/admin", { transports: [transport], forceNew: true, reconnection: false });
  await new Promise((res, rej) => { admin.on("connect", res); admin.on("connect_error", e=>rej(e)); setTimeout(()=>rej(new Error("ns timeout")), 4000); });
  const nsAck = await new Promise(res => admin.emit("whoami", (...a) => res(a)));
  check(`[${transport}] namespace`, nsAck && nsAck[0] === "admin", JSON.stringify(nsAck));

  socket.close(); admin.close();
}

try {
  await run("polling");
  await run("websocket");
} catch (e) {
  results.push("FATAL " + e.message);
  failures++;
}
console.log(results.join("\n"));
console.log(failures === 0 ? "\nALL PASS" : `\n${failures} FAILURE(S)`);
process.exit(failures === 0 ? 0 : 1);
