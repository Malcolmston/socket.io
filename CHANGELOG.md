# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/malcolmston/socket.io/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/malcolmston/socket.io/releases/tag/v0.1.0
