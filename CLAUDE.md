# Proxy Coin

Decentralized bandwidth and compute marketplace. Android app that rewards users with PRXY tokens (ERC-20 on Base L2) for sharing device resources — primarily network bandwidth for proxied traffic, secondarily processing power via WASM.


## ClickUp Integration

@.clickup_state.json

## Project Structure

```
proxy-coin/
├── android/          # Android app (Kotlin, Jetpack Compose, Hilt)
├── backend/          # Backend services (Go)
├── contracts/        # Smart contracts (Solidity, Foundry)
├── protocol/         # Shared protobuf definitions
├── infrastructure/   # Docker/K8s deployment configs
├── scripts/          # Utility scripts
└── docs/             # Documentation
```

## Tech Stack

- **Android**: Kotlin, Jetpack Compose, Hilt, Room, Retrofit, Web3j
- **Backend**: Go 1.22+, gorilla/websocket, pgx, protobuf
- **Blockchain**: Solidity 0.8.x, Foundry, Base L2 (Coinbase)
- **Database**: PostgreSQL 16, Redis 7
- **Protocol**: Protocol Buffers v3

## Development Guidelines

- Follow conventional commits: `feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`
- Feature branches for all changes, never commit directly to main
- Use uv for Python scripts (PEP 723 headers)

## Key Specifications

| Document | Contents |
|----------|----------|
| SPEC.md | Master spec, vision, tech stack rationale |
| ARCHITECTURE.md | System diagrams, data flow, directory structure |
| ANDROID-APP.md | App modules, screens, services, security |
| BACKEND.md | Go services, API design, database schema |
| TOKENOMICS.md | Token economics, rewards, emission schedule |
| SMART-CONTRACTS.md | Solidity contracts, deployment plan |
| SECURITY-AND-COMPLIANCE.md | Traffic filtering, anti-fraud, legal |
| ROADMAP.md | Development phases, milestones, risks |
| COMPETITIVE-ANALYSIS.md | Market analysis, differentiation, revenue |
