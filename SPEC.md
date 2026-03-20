# Proxy Coin - Master Specification

## Vision

Proxy Coin is a decentralized bandwidth and compute marketplace where Android device owners earn crypto tokens (PRXY) in exchange for sharing their idle device resources - primarily network bandwidth for proxied web traffic, and secondarily processing power for lightweight compute tasks.

## Problem Statement

The residential proxy market (~$1.2B, growing 20%+ annually) relies on users sharing their bandwidth in exchange for cash payouts. Current solutions suffer from:

1. **Low payouts** - Centralized operators capture most of the value
2. **Opaque economics** - Users can't verify how their resources are being used or valued
3. **No ownership** - Users are renters, not stakeholders in the network
4. **Limited platforms** - Most are browser extensions (Grass) or desktop-only (PacketStream)

Proxy Coin solves this by:
- Paying with a transparent on-chain token with verifiable supply and distribution
- Making users stakeholders (token holders benefit from network growth)
- Building native Android-first (deeper device integration than browser extensions)
- Adding compute resource sharing (WASM-based) as a differentiator

## System Overview

```
                    +------------------+
                    |  Bandwidth       |
                    |  Customers       |
                    |  (API clients)   |
                    +--------+---------+
                             |
                             | REST API (proxy requests)
                             v
                    +------------------+
                    |  Backend         |
                    |  Orchestrator    |
                    |  (Go services)   |
                    +--------+---------+
                             |
                    WebSocket tunnels (multiplexed)
                    /        |        \
                   v         v         v
              +--------+ +--------+ +--------+
              |Android | |Android | |Android |
              | Node 1 | | Node 2 | | Node N |
              +--------+ +--------+ +--------+
                   |         |         |
                   v         v         v
              (exit traffic from device IPs)
                   |         |         |
              example.com  site.org  api.dev

                    +------------------+
                    |  Base L2         |
                    |  Smart Contracts |
                    |  (PRXY Token)    |
                    +------------------+
```

## Tech Stack

| Layer | Technology | Rationale |
|-------|-----------|-----------|
| **Android App** | Kotlin, Jetpack Compose, Hilt | Modern Android stack, declarative UI, standard DI |
| **Backend** | Go 1.22+ | Excellent concurrency (goroutines), low memory, strong networking stdlib |
| **Database** | PostgreSQL 16 | ACID, JSON support, proven at scale |
| **Cache** | Redis 7 | Real-time node status, session state, pub/sub |
| **Protocol** | Protocol Buffers v3 | Efficient binary serialization for WebSocket messages |
| **Blockchain** | Base L2 (Coinbase) | Sub-cent fees, EVM, Coinbase wallet/offramp integration |
| **Smart Contracts** | Solidity 0.8.x, Foundry | Industry standard, excellent tooling |
| **Compute Sandbox** | WebAssembly (Wasmtime) | Sandboxed, portable, near-native performance |

### Why These Choices

**Go over Node/Python for backend**: Proxy routing is I/O-intensive with thousands of concurrent WebSocket connections. Go's goroutine model handles this naturally without callback hell or the GIL. Single binary deployment simplifies ops.

**Base L2 over Solana/Polygon**: Coinbase ecosystem provides the easiest fiat offramp for non-crypto-native users. Sub-cent transaction fees make batch reward distribution economical. EVM compatibility means standard Solidity tooling.

**Protobuf over JSON for wire protocol**: Each proxy request generates multiple messages. At 1000+ nodes with 5 concurrent requests each, JSON serialization overhead becomes significant. Protobuf is ~3-10x more compact and ~10-100x faster to parse.

**Kotlin/Compose over Flutter/React Native**: Native Android gives direct access to Foreground Services, Android Keystore, Play Integrity API, and fine-grained power management. Cross-platform frameworks add abstraction layers that hurt reliability for always-on background services.

## Monorepo Structure

See [ARCHITECTURE.md](ARCHITECTURE.md) for detailed project structure.

```
proxy-coin/
├── android/          # Android app (Kotlin)
├── backend/          # Backend services (Go)
├── contracts/        # Smart contracts (Solidity)
├── protocol/         # Shared protobuf definitions
├── docs/             # Documentation
├── infrastructure/   # Deployment configs
└── scripts/          # Utility scripts
```

## Related Specifications

| Document | Contents |
|----------|----------|
| [ARCHITECTURE.md](ARCHITECTURE.md) | System architecture, data flows, component interactions |
| [ANDROID-APP.md](ANDROID-APP.md) | Android app modules, screens, services |
| [BACKEND.md](BACKEND.md) | Backend services, API design, database schema |
| [TOKENOMICS.md](TOKENOMICS.md) | Token economics, rewards, emission schedule |
| [SMART-CONTRACTS.md](SMART-CONTRACTS.md) | Contract specifications and deployment |
| [SECURITY-AND-COMPLIANCE.md](SECURITY-AND-COMPLIANCE.md) | Security, legal, compliance |
| [ROADMAP.md](ROADMAP.md) | Development phases and milestones |
| [COMPETITIVE-ANALYSIS.md](COMPETITIVE-ANALYSIS.md) | Market analysis and differentiation |
