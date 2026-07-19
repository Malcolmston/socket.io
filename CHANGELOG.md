# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.4.0] - 2026-07-19
### Added
- **Upstream-parity tests** for the `engineio` Engine.IO v4 packet codec, verified
  against socketio/engine.io-parser vectors; `parity.json` published.
### Changed
- 100% exported-symbol API-doc coverage across the module.

## [0.3.0] - 2026-07-18
### Added
- Streaming Socket.IO parser (`Encoder`/`Decoder`) mirroring `socket.io-parser`:
  encode a packet into its ordered transport frames and reassemble multi-frame
  binary packets (`BINARY_EVENT`/`BINARY_ACK`), plus `Packet.HasBinaryData`.
- Catch-all socket listeners (`Socket.OnAny`, `PrependAny`, `OffAny`,
  `ListenersAny`) — the equivalent of `socket.onAny`.
- Broadcast-flag and room parity: `BroadcastOperator.Local`, `Timeout`,
  `ExceptRoom`, and `In`/`Except`/`Local` entry points on `Namespace`, `Server`,
  and `Socket` (`io.in`, `io.except`, `io.local`, `socket.in`, `socket.except`).
- `Handshake` object and `NewHandshake` capturing request headers, address
  (honoring `X-Forwarded-For`), query, TLS state, and auth — like
  `socket.handshake`.
- Reserved-event guards: `IsReservedEvent`, `ValidateEventName`,
  `ReservedEvents`.
- Engine.IO WebSocket close codes (`CloseCode` + RFC 6455 constants,
  `EncodeCloseFrame`, `DecodeCloseFrame`, `IsValid`) and packet constructors
  (`NewPing`, `NewPong`, `NewClose`, `NewUpgrade`, `NewNoop`,
  `NewBinaryMessage`, `Packet.IsBinary`).
- Client reconnection `Backoff` (exponential delay with optional deterministic
  jitter), mirroring the JS client's `backo2`.

## [0.1.0] - 2026-07-04
### Added
- Initial public release — a from-scratch Go port of Socket.IO.
- Engine.IO v4 and Socket.IO v5 packet codecs.
- WebSocket (RFC 6455) transport and HTTP long-polling transport, both
  implemented on the standard library.
- Server API: namespaces, rooms, sockets, and acknowledgements; binary payloads;
  per-socket data store; client reconnection.
- Redis adapter for multi-node broadcasting.
- Verified interop with the real `socket.io-client@4`.
- Shared `Emitter` / `RoomTargeter` / `BroadcastTarget` interfaces documenting
  the emitter/broadcaster contract across Server, Namespace, Socket and
  BroadcastOperator.
- Automated releases (VERSION-driven tags + GitHub Releases, moving `stable` tag).
- CI: build/test matrix (Go 1.23 & 1.24), `-race` + coverage, golangci-lint v2,
  govulncheck, CodeQL, benchmarks, dependency review, and a stale bot.

[Unreleased]: https://github.com/malcolmston/socket.io/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/malcolmston/socket.io/compare/v0.1.0...v0.3.0
[0.1.0]: https://github.com/malcolmston/socket.io/releases/tag/v0.1.0
