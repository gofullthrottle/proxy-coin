# Proxy Coin

Decentralized bandwidth and compute marketplace. An Android app that rewards users with PRXY tokens for sharing idle device resources.

## Overview

Proxy Coin creates a peer-to-peer proxy network where Android device owners earn ERC-20 tokens (PRXY on Base L2) in exchange for routing web traffic through their devices. Bandwidth buyers (web scraping companies, ad verification platforms, market researchers) pay for residential proxy access via API.

## Architecture

```
Customers (API) → Backend Orchestrator → WebSocket Tunnels → Android Nodes
                         ↕                                        ↕
                    PostgreSQL/Redis                         Device Network
                         ↕
                  Base L2 Smart Contracts (PRXY Token)
```

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Android | Kotlin, Jetpack Compose, Hilt, Room, Web3j |
| Backend | Go 1.22+, gorilla/websocket, pgx, protobuf |
| Blockchain | Solidity 0.8.x, Foundry, Base L2 |
| Database | PostgreSQL 16, Redis 7 |
| Protocol | Protocol Buffers v3 |

## Documentation

| Document | Description |
|----------|-------------|
| [SPEC.md](SPEC.md) | Master specification and vision |
| [ARCHITECTURE.md](ARCHITECTURE.md) | System architecture and data flows |
| [ANDROID-APP.md](ANDROID-APP.md) | Android app modules, screens, services |
| [BACKEND.md](BACKEND.md) | Backend services, API, database schema |
| [TOKENOMICS.md](TOKENOMICS.md) | Token economics and reward mechanics |
| [SMART-CONTRACTS.md](SMART-CONTRACTS.md) | Solidity contract specifications |
| [SECURITY-AND-COMPLIANCE.md](SECURITY-AND-COMPLIANCE.md) | Security, legal, compliance |
| [ROADMAP.md](ROADMAP.md) | Development phases and milestones |
| [COMPETITIVE-ANALYSIS.md](COMPETITIVE-ANALYSIS.md) | Market analysis |

## License

Proprietary. All rights reserved.
