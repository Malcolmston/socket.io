import { io } from "socket.io-client";
const socket = io("http://127.0.0.1:9731", { forceNew: true, reconnection: false }); // default: polling then upgrade
let transportChanges = [];
socket.io.engine.on("upgrade", (t) => transportChanges.push(t.name));
await new Promise((res, rej) => { socket.on("connect", res); socket.on("connect_error", e=>rej(e)); setTimeout(()=>rej(new Error("timeout")), 5000); });
const startTransport = socket.io.engine.transport.name;
// give the upgrade a moment
await new Promise(r => setTimeout(r, 800));
const nowTransport = socket.io.engine.transport.name;
const echoP = new Promise(res => socket.once("echo", (...a)=>res(a)));
socket.emit("echo", "after-upgrade");
const echoed = await Promise.race([echoP, new Promise(r=>setTimeout(()=>r(null),2000))]);
console.log("start transport:", startTransport);
console.log("upgraded to:", nowTransport, "| upgrade events:", JSON.stringify(transportChanges));
console.log("echo after upgrade:", JSON.stringify(echoed));
const ok = nowTransport === "websocket" && echoed && echoed[0] === "after-upgrade";
console.log(ok ? "UPGRADE PASS" : "UPGRADE FAIL");
socket.close();
process.exit(ok?0:1);
