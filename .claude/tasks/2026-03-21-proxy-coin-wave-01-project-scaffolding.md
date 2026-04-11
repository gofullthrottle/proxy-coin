---
decomposed_from: .claude/plans/2026-03-21-proxy-coin-full-implementation.md
date: "2026-03-21"
project: "proxy-coin"
wave: 1
task_id: "project-scaffolding"
created_at: "2026-03-21T00:00:00Z"
---

# Wave 01 — Project Scaffolding

**Phase**: 0 — Foundation
**Estimated Total**: 10h
**Dependencies**: None — all tasks are fully parallel
**Agent Mix**: Infrastructure (3 tasks), Backend (1 task), Android (1 task), Contracts (1 task)

All 6 tasks in this wave are independent and can execute in parallel.

---

## Tasks

### 1.1 — Protocol: Create protobuf definitions
- **Agent**: Infrastructure
- **Estimate**: 2h
- **Complexity**: Standard
- **Files**: `protocol/node.proto`, `protocol/proxy.proto`, `protocol/metering.proto`

**Acceptance Criteria**:
- `node.proto` with Register, Registered, Heartbeat, ConfigUpdate, EarningsUpdate messages
- `proxy.proto` with ProxyRequest, ProxyResponseStart, ProxyResponseChunk, ProxyResponseEnd messages
- `metering.proto` with MeteringEvent, MeteringReport messages
- All messages use proto3 syntax with appropriate field types

**Technical Notes**: Reference ARCHITECTURE.md data flow section for message types. Use proto3 syntax. Package: `proxycoin.protocol.v1`

---

### 1.2 — Backend: Init Go module + project structure
- **Agent**: Backend
- **Estimate**: 2h
- **Complexity**: Standard
- **Files**: `backend/go.mod`, `backend/cmd/*/main.go`, `backend/internal/`, `backend/Makefile`

**Acceptance Criteria**:
- `go.mod` with Go 1.22+ and module path `github.com/gofullthrottle/proxy-coin/backend`
- `cmd/{orchestrator,api,metering}/main.go` entry points
- `internal/` packages: node, proxy, websocket, metering, reward, fraud, auth, customer, config
- `pkg/` packages: protocol, blockchain
- `Makefile` with build, test, lint, proto-gen targets
- `go build ./...` compiles without errors

**Technical Notes**: Follow standard Go project layout. Use `cmd/` for binaries, `internal/` for private packages, `pkg/` for importable packages.

---

### 1.3 — Android: Init Gradle project + Hilt DI
- **Agent**: Android
- **Estimate**: 3h
- **Complexity**: Complex
- **Files**: `android/build.gradle.kts`, `android/app/libs.versions.toml`, `android/app/src/main/.../ProxyCoinApp.kt`

**Acceptance Criteria**:
- `android/` directory with proper Gradle project structure
- `build.gradle.kts` with all dependencies from ANDROID-APP.md `libs.versions.toml`
- `ProxyCoinApp.kt` application class with `@HiltAndroidApp`
- Hilt modules: AppModule, NetworkModule, DatabaseModule, CryptoModule
- `AndroidManifest.xml` with all required permissions
- `compileSdk 35`, `minSdk 26`, `targetSdk 35`
- Gradle sync succeeds without errors

**Technical Notes**: Use version catalog (`libs.versions.toml`). Kotlin 2.0.21, Compose BOM 2024.12.01, Hilt 2.51.1.

---

### 1.4 — Contracts: Init Foundry project
- **Agent**: Contracts
- **Estimate**: 1h
- **Complexity**: Simple
- **Files**: `contracts/foundry.toml`, `contracts/remappings.txt`, `contracts/src/` stubs

**Acceptance Criteria**:
- `foundry.toml` per SMART-CONTRACTS.md (solc 0.8.24, optimizer 200 runs, cancun EVM)
- OpenZeppelin contracts installed via `forge install`
- `remappings.txt` configured
- Empty contract stubs in `src/`
- `forge build` succeeds

**Technical Notes**: Fuzz runs = 1000. RPC endpoints for `base_sepolia` and `base_mainnet`. Etherscan config for verification.

---

### 1.5 — Infrastructure: Docker Compose dev environment
- **Agent**: Infrastructure
- **Estimate**: 1h
- **Complexity**: Simple
- **Files**: `infrastructure/docker-compose.dev.yml`, `.env.example`

**Acceptance Criteria**:
- `infrastructure/docker-compose.dev.yml` with PostgreSQL 16 + Redis 7
- PostgreSQL: `proxycoin` DB, `proxycoin` user, health check
- Redis: health check, port 6379 exposed
- Named volumes for data persistence
- `docker-compose up` starts both services successfully

**Technical Notes**: Use alpine images for smaller size. Add `.env.example` with default values.

---

### 1.6 — Scripts: Protobuf generation script
- **Agent**: Infrastructure
- **Estimate**: 1h
- **Complexity**: Simple
- **Files**: `scripts/generate-proto.sh`

**Acceptance Criteria**:
- `scripts/generate-proto.sh` compiles all `.proto` files
- Go output to `backend/pkg/protocol/`
- Kotlin lite output to `android/app/src/main/java/.../protocol/`
- Script is executable and documented

**Technical Notes**: Requires protoc, protoc-gen-go, protoc-gen-kotlin. Add dependency check at script start.

---

## Acceptance Criteria (Wave Level)

- [ ] All 4 subsystems have their scaffold in place
- [ ] `docker-compose -f infrastructure/docker-compose.dev.yml up` starts without errors
- [ ] `generate-proto.sh` runs cleanly (even if proto files are incomplete stubs)
- [ ] `go build ./...` compiles (even empty main functions)
- [ ] Android Gradle sync succeeds
- [ ] `forge build` succeeds

## Dependencies

None. This is the root wave. All tasks are fully parallel.

## Technical Notes

- This wave establishes the directory conventions that all future waves depend on
- Android task (1.3) is the longest at 3h due to Gradle/Hilt complexity
- Proto definitions (1.1) and generation script (1.6) are logically linked but can be authored simultaneously
- Docker Compose (1.5) should be validated with an actual `docker-compose up` before marking complete
