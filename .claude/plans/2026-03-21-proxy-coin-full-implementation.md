# Proxy Coin - Complete Implementation Plan

**Created**: 2026-03-21
**Approach**: Steel Thread First — build thinnest end-to-end path, then layer features
**Phases**: 5 (0-4), matching ROADMAP.md
**Waves**: 16 execution waves
**Tasks**: 96 granular tasks
**Agents**: 4 specialists (Backend, Android, Contracts, Infrastructure)

---

## Architecture Summary

```
Protocol (Protobuf) ──→ Backend (Go, 3 services) ──→ Android (Kotlin/Compose)
                                    │
                              PostgreSQL + Redis
                                    │
                        Smart Contracts (Solidity/Foundry, Base L2)
```

**Critical Path**: Protocol → Backend Orchestrator → Android Client → Integration Test

---

## Phase 0: Foundation (Waves 1-2)

**Goal**: Scaffold all 4 subsystems, dev environment running, protobuf compiling.

### Epic 0.1: Project Scaffolding

#### Wave 1 — All parallel, no dependencies

| ID | Task | Agent | Est | Acceptance Criteria |
|----|------|-------|-----|---------------------|
| 1.1 | **Protocol: Create protobuf definitions** | Infra | 2h | `node.proto`, `proxy.proto`, `metering.proto` with all message types from ARCHITECTURE.md (ProxyRequest, ProxyResponseStart, ProxyResponseChunk, ProxyResponseEnd, Register, Heartbeat, ConfigUpdate, EarningsUpdate) |
| 1.2 | **Backend: Init Go module + project structure** | Backend | 2h | `go.mod` with Go 1.22+, `cmd/{orchestrator,api,metering}/main.go`, `internal/` packages per ARCHITECTURE.md, `Makefile` with build/test/lint targets |
| 1.3 | **Android: Init Gradle project + Hilt DI** | Android | 3h | `android/` with `build.gradle.kts`, `libs.versions.toml` per ANDROID-APP.md, Hilt application class, `AppModule`, `NetworkModule`, `DatabaseModule`, `CryptoModule` |
| 1.4 | **Contracts: Init Foundry project** | Contracts | 1h | `foundry.toml` per SMART-CONTRACTS.md, OpenZeppelin installed via `forge install`, `remappings.txt`, empty contract stubs |
| 1.5 | **Infrastructure: Docker Compose dev environment** | Infra | 1h | `infrastructure/docker-compose.dev.yml` with Postgres 16 + Redis 7, health checks, volume mounts, ports exposed |
| 1.6 | **Scripts: Protobuf generation script** | Infra | 1h | `scripts/generate-proto.sh` compiling protos for Go (`protoc-gen-go`) and Kotlin (`protoc-gen-kotlin`, lite variant) |

### Epic 0.2: Foundation Layer

#### Wave 2 — Depends on Wave 1

| ID | Task | Agent | Est | Acceptance Criteria |
|----|------|-------|-----|---------------------|
| 2.1 | **Protocol: Compile + verify generated code** | Infra | 1h | `generate-proto.sh` runs cleanly, Go code in `backend/pkg/protocol/`, Kotlin code in `android/app/src/main/java/.../protocol/` |
| 2.2 | **Backend: Config package** | Backend | 1h | `internal/config/config.go` loading from env vars: DATABASE_URL, REDIS_URL, BASE_RPC_URL, LISTEN_ADDR, WS_ADDR, JWT_SECRET |
| 2.3 | **Backend: Database migrations** | Backend | 2h | `migrations/001_initial.up.sql` with all 7 tables from BACKEND.md (nodes, metering, earnings, claimable_rewards, customers, customer_usage, fraud_events, referrals), indexes, partition setup |
| 2.4 | **Android: Navigation + base screens** | Android | 2h | `NavGraph.kt` with routes for all 6 screens, empty Composable stubs for each, bottom nav bar (Home, Earn, Wallet, Settings) |
| 2.5 | **Android: Room database setup** | Android | 2h | `AppDatabase.kt` with 3 entities (EarningsEntity, MeteringEntity, TransactionEntity), DAOs with basic CRUD queries per ANDROID-APP.md |
| 2.6 | **Android: Network module** | Android | 2h | `ApiService.kt` interface with all endpoints from ANDROID-APP.md, OkHttp client with logging interceptor, Retrofit instance in `NetworkModule`, `AuthInterceptor` skeleton |

**Wave 2 Success Gate**: `docker-compose -f infrastructure/docker-compose.dev.yml up` starts Postgres+Redis, `go build ./...` compiles, Gradle sync succeeds, proto generation works.

---

## Phase 1: Proof of Concept (Waves 3-5)

**Goal**: Proxy HTTP traffic through Android device via WebSocket tunnel. 10 devices, 24h, >90% success rate.

### Epic 1.1: Backend Orchestrator

#### Wave 3 — Depends on Wave 2

| ID | Task | Agent | Est | Acceptance Criteria |
|----|------|-------|-----|---------------------|
| 3.1 | **Backend: Node registry** | Backend | 3h | `internal/node/registry.go` — CRUD operations on `nodes` table via pgx. Register node, update heartbeat, get by ID, find by filter (country, status, trust_score, network_type). Unit tests. |
| 3.2 | **Backend: Node selector algorithm** | Backend | 2h | `internal/node/selector.go` — `SelectNode(req)` implementing scored selection from BACKEND.md: filter candidates, calculate score (trust 0.4 + load 0.3 + residential 0.2 + uptime 0.1), weighted random from top 5. Unit tests with mock registry. |
| 3.3 | **Backend: WebSocket server** | Backend | 4h | `internal/websocket/server.go` — HTTP upgrade handler at `/ws`, gorilla/websocket connection lifecycle, protobuf message deserialization, `REGISTER`/`REGISTERED` handshake, `HEARTBEAT` handling (30s interval), connection tracking in `sync.Map`. |
| 3.4 | **Backend: Connection pool + message routing** | Backend | 3h | `internal/websocket/pool.go` + `connection.go` — Connection struct with SendCh, ActiveReqs map, mutex. Pool manages concurrent connections. Route incoming messages (PROXY_RESPONSE) to pending request channels. Back-pressure when node exceeds max_concurrent. |
| 3.5 | **Backend: Redis integration** | Backend | 2h | Node status keys (`node:{id}:status`, `node:{id}:requests`, `node:{id}:heartbeat`), session stickiness (`session:{id}` → nodeID, TTL 5min), WebSocket mapping (`ws:{id}` → server instance). go-redis/v9 client in config. |
| 3.6 | **Backend: Metering event publisher** | Backend | 2h | `internal/metering/reporter.go` — Publish `MeteringEvent` structs to Redis Stream `metering_events` after each proxy response completes. Event contains: request_id, node_id, customer_id, bytes_in, bytes_out, latency_ms, status_code, success, timestamp. |

### Epic 1.2: Android Core Services

#### Wave 4 — Depends on Wave 2; Wave 3 for integration

| ID | Task | Agent | Est | Acceptance Criteria |
|----|------|-------|-----|---------------------|
| 4.1 | **Android: ProxyForegroundService** | Android | 4h | Persistent foreground service with notification channel, `START_STICKY`, WakeLock, WifiLock. Shows connection status in notification (updated every 30s). Lifecycle: create notification → start foreground → connect WebSocket → listen for requests → update notification. |
| 4.2 | **Android: WebSocketClient** | Android | 4h | OkHttp WebSocket connecting to `wss://` backend. Protobuf message serialization. Send REGISTER with device info on connect. Handle REGISTERED response. Heartbeat every 30s. Reconnect with exponential backoff (1s→2s→4s→8s→16s→30s max) + jitter. Session resume within 30s window. Route PROXY_REQUEST to ProxyEngine. |
| 4.3 | **Android: ProxyEngine** | Android | 3h | For each PROXY_REQUEST: validate URL against blocklist (placeholder), build OkHttp Request, execute from device network, stream response back in 64KB chunks via WebSocket (ProxyResponseStart → ProxyResponseChunk[] → ProxyResponseEnd). Semaphore for concurrent limit (default 5). Timeout 30s. Max response 10MB. Error handling (timeout, DNS, connection refused). |
| 4.4 | **Android: ResourceMonitor** | Android | 2h | Monitor battery level (BatteryManager), charging state, network type (ConnectivityManager: WiFi/cellular/metered), available memory. Emit `ResourceState` every 10s via Flow. ProxyForegroundService pauses proxy when: battery < threshold (default 20%), not on WiFi (if WiFi-only enabled), thermal throttling detected. |
| 4.5 | **Android: BootReceiver** | Android | 1h | `BroadcastReceiver` for `BOOT_COMPLETED`. Starts ProxyForegroundService if user had it enabled. Registered in AndroidManifest. Respects user's "auto-start" preference from DataStore. |
| 4.6 | **Android: MeteringService** | Android | 2h | Local metering tracker. Records bytes_in, bytes_out, latency_ms, success per request in Room DB (MeteringEntity). Periodic batch report to backend via `POST /v1/metering/report` (supplementary to server-side metering). |

### Epic 1.3: Integration + Points

#### Wave 5 — Depends on Waves 3-4

| ID | Task | Agent | Est | Acceptance Criteria |
|----|------|-------|-----|---------------------|
| 5.1 | **Backend: Proxy request handler** | Backend | 3h | `internal/proxy/handler.go` — Receive proxy request (internal gRPC or HTTP from Customer API), call NodeSelector, send PROXY_REQUEST via WebSocket to selected node, wait for response with timeout, return to caller. Handle node unavailable, timeout, error cases. |
| 5.2 | **Backend: Proxy response handler** | Backend | 2h | `internal/proxy/router.go` — Assemble streamed ProxyResponseChunks into complete response. Forward to waiting request handler. Record MeteringEvent on completion. Handle partial responses, out-of-order chunks. |
| 5.3 | **Backend: Points ledger** | Backend | 2h | Simple PostgreSQL-based points tracking (no blockchain). Insert earnings records per node per hour. API endpoint `GET /v1/earnings` returns earnings for a node. `GET /v1/earnings/summary` returns totals. Points accumulate but are not yet claimable on-chain. |
| 5.4 | **Android: Basic settings** | Android | 2h | Settings screen with: WiFi-only toggle (default on), on/off switch for proxy service, bandwidth display. Settings persisted in DataStore. ProxyForegroundService reads settings reactively. |
| 5.5 | **Integration: End-to-end test harness** | Backend | 3h | `scripts/test-proxy.sh` or Go test — Start backend services, simulate a proxy request to a known URL (e.g., httpbin.org), verify response matches direct request. Test with multiple concurrent requests. Measure latency overhead. Report success rate. |
| 5.6 | **Infrastructure: Full dev Docker Compose** | Infra | 2h | `infrastructure/docker-compose.yml` with all 3 backend services + Postgres + Redis. Volume mounts for code hot-reload. Service health checks. Port mapping. Environment variables configured. Single `docker-compose up` starts everything. |

**Phase 1 Success Gate**:
- [ ] Backend WebSocket server accepts connections from Android client
- [ ] Android app proxies HTTP requests through device successfully
- [ ] End-to-end test harness passes with >90% success rate
- [ ] Points accumulate in PostgreSQL per node
- [ ] Auto-reconnect works after network switch
- [ ] Docker Compose starts full stack in one command

---

## Phase 2: Alpha (Waves 6-8)

**Goal**: HTTPS proxy, security controls, production UI. 50-100 devices, 1 week stable.

### Epic 2.1: HTTPS + Security

#### Wave 6 — Depends on Phase 1

| ID | Task | Agent | Est | Acceptance Criteria |
|----|------|-------|-----|---------------------|
| 6.1 | **Backend: HTTPS CONNECT tunnel** | Backend | 4h | Orchestrator handles CONNECT method proxy requests. Establish TCP tunnel through WebSocket to Android node. Node opens TLS connection to destination, relays encrypted bytes bidirectionally. No MITM — backend/node never sees plaintext HTTPS content. |
| 6.2 | **Backend: Domain blocklist management** | Backend | 3h | `internal/proxy/filter.go` — Load blocklist from config/database. Hash-based matching (SHA-256 of domain). Categories per SECURITY.md: CSAM, malware C2, gov/mil, financial, healthcare. Rate-based blocking (>100 req/min to one domain). Admin API to update blocklist. |
| 6.3 | **Backend: Blocklist distribution** | Backend | 2h | On node REGISTER and periodically via CONFIG_UPDATE, send blocklist hash set to nodes. Include blocklist version hash so nodes only download when changed. Redis pub/sub for broadcast config updates to all orchestrator instances. |
| 6.4 | **Android: HTTPS CONNECT in ProxyEngine** | Android | 3h | Handle CONNECT-type proxy requests. Open TCP socket to destination, relay bytes through WebSocket tunnel. Use OkHttp for initial handshake, raw socket for tunnel. Proper cleanup on timeout/disconnect. |
| 6.5 | **Android: Domain filter** | Android | 2h | `DomainFilter` class — receive blocklist hash set from backend. Before executing any proxy request, hash the destination domain and check against set. Reject blocked domains with appropriate error response. Update blocklist on CONFIG_UPDATE messages. |
| 6.6 | **Backend: Server-side metering** | Backend | 3h | `internal/metering/counter.go` — Count bytes flowing through WebSocket in real-time (authoritative, not client-reported). Compare with client-reported metrics. Flag >10% divergence. Feed into MeteringEvent with server-measured values. |

### Epic 2.2: Attestation + Monitoring

#### Wave 7 — Parallel with Wave 6

| ID | Task | Agent | Est | Acceptance Criteria |
|----|------|-------|-----|---------------------|
| 7.1 | **Android: Play Integrity integration** | Android | 3h | Integrate Google Play Integrity API. Generate integrity token on registration and periodically (configurable interval). Send token to backend via `POST /v1/node/attest`. Handle API errors gracefully. |
| 7.2 | **Backend: Attestation verification** | Backend | 2h | `internal/fraud/attestation.go` — Verify Play Integrity token via Google's API. Check: genuine device, genuine APK, not rooted. On failure: suspend node earnings, log fraud_event, require re-attestation. Periodic re-check (not just registration). |
| 7.3 | **Android: ResourceMonitor enhancements** | Android | 2h | Add CPU temperature monitoring (Thermal API, Android 11+), network speed estimation, memory pressure detection. Enhanced ResourceState with all fields. More granular pause/resume logic. |
| 7.4 | **Backend: Node config push** | Backend | 2h | `internal/node/config.go` — Push configuration updates to connected nodes via WebSocket CONFIG_UPDATE message. Config includes: earning rates, blocklist hash, max_concurrent, heartbeat interval. Redis pub/sub for multi-instance broadcast. |
| 7.5 | **Android: Connection resilience** | Android | 2h | Improve WebSocket reconnect: add jitter to prevent thundering herd, session resume within 30s window (reuse node registration), connection state machine (connecting→connected→reconnecting→disconnected), backoff reset on successful connection. |

### Epic 2.3: Android UI

#### Wave 8 — Depends on Waves 6-7 for data

| ID | Task | Agent | Est | Acceptance Criteria |
|----|------|-------|-----|---------------------|
| 8.1 | **Android: Dashboard screen** | Android | 4h | Full dashboard per ANDROID-APP.md wireframe: connection status toggle, today/all-time earnings with USD estimate, bandwidth shared progress bar (up/down), live stats (uptime, requests, latency, trust score), 7-day trend chart. Material3 design. |
| 8.2 | **Android: DashboardViewModel** | Android | 2h | Collect data from: ProxyForegroundService (connection state, live stats), EarningsRepository (today/all-time), API (token price, trust score). Expose as `DashboardUiState` StateFlow. Auto-refresh on service state changes. |
| 8.3 | **Android: Earnings screen** | Android | 3h | Tab bar (Day/Week/Month/All). Earnings chart (line chart using a Compose charting library). Breakdown table: bandwidth earnings, uptime bonus, quality bonus, referral earnings. Pending vs claimed amounts. Claim button (placeholder, wired in Phase 3). |
| 8.4 | **Android: Settings screen** | Android | 3h | Full settings per ANDROID-APP.md: Bandwidth (max slider, WiFi-only, cellular toggle, cellular cap), Power (battery threshold slider, charging-only), Notifications (milestones, status, weekly summary toggles), Account (device ID, node ID, trust score, referral code), About (version, ToS, privacy, licenses). All persisted in DataStore. |
| 8.5 | **Android: Notification enhancements** | Android | 1h | Rich notification showing: connection state icon, today's earnings, bandwidth shared, uptime. Updated every 30s. Action buttons: pause/resume. Low-priority channel to avoid user annoyance. |

**Phase 2 Success Gate**:
- [ ] HTTPS proxy working through CONNECT tunnel
- [ ] Domain blocklist blocking 100% of test entries
- [ ] Play Integrity attestation passing on real devices
- [ ] Dashboard showing real-time stats
- [ ] Resource monitor pausing proxy on low battery/cellular
- [ ] Server-side metering matching within 10% of actual bytes

---

## Phase 3: Token Launch (Waves 9-12)

**Goal**: Smart contracts on testnet, wallet in app, end-to-end claim flow working.

### Epic 3.1: Smart Contracts

#### Wave 9 — Can start during Phase 2 (independent)

| ID | Task | Agent | Est | Acceptance Criteria |
|----|------|-------|-----|---------------------|
| 9.1 | **Contracts: ProxyCoinToken.sol** | Contracts | 2h | ERC-20 with AccessControl per SMART-CONTRACTS.md. MINTER_ROLE, BURNER_ROLE. MAX_SUPPLY = 1B. `mint()` with supply cap check. `burn()` for slashing. Constructor grants DEFAULT_ADMIN_ROLE. |
| 9.2 | **Contracts: RewardDistributor.sol** | Contracts | 3h | Merkle-based claims per SMART-CONTRACTS.md. `setMerkleRoot()` (owner only), `claim(cumulativeAmount, proof)` with MerkleProof.verify, cumulative tracking via `claimed` mapping, `getClaimable()` view. Events: MerkleRootUpdated, RewardsClaimed. |
| 9.3 | **Contracts: Staking.sol** | Contracts | 3h | 4-tier staking per SMART-CONTRACTS.md. `stake(amount)` determines tier, locks tokens. `unstake()` after lock period. `slash(staker, reason)` with SLASHER_ROLE burns 50%. Tier thresholds: 1K/5K/10K/50K PRXY. Lock periods: 30/30/60/90 days. |
| 9.4 | **Contracts: Vesting.sol** | Contracts | 2h | Linear vesting per SMART-CONTRACTS.md. `createSchedule()` with beneficiary, total, start, cliff, duration, revocable. `release()` calculates vested amount. `revoke()` returns unvested to owner. `getReleasable()` view. |
| 9.5 | **Contracts: Deploy script** | Contracts | 2h | `script/Deploy.s.sol` — Deploy all 4 contracts in order: Token → RewardDistributor → Staking → Vesting. Grant MINTER_ROLE to RewardDistributor, BURNER_ROLE to Staking. Verify addresses. Output deployment addresses. |
| 9.6 | **Contracts: DistributeRewards script** | Contracts | 1h | `script/DistributeRewards.s.sol` — Script to call `setMerkleRoot()` on RewardDistributor. Takes root as input. Used by backend's daily Merkle generation job. |

### Epic 3.2: Contract Testing

#### Wave 10 — Depends on Wave 9

| ID | Task | Agent | Est | Acceptance Criteria |
|----|------|-------|-----|---------------------|
| 10.1 | **Tests: ProxyCoinToken** | Contracts | 2h | Test mint (success, exceeds supply, unauthorized), burn (success, unauthorized), role management (grant, revoke), transfer, approval. 100% function coverage. |
| 10.2 | **Tests: RewardDistributor** | Contracts | 3h | Test setMerkleRoot (owner only), claim with valid proof, claim with invalid proof, double-claim prevention, cumulative accounting (claim partial, then rest), getClaimable accuracy. Generate test Merkle trees. |
| 10.3 | **Tests: Staking** | Contracts | 2h | Test stake (all 4 tiers), unstake (before/after lock), slash (amount, burn, unauthorized), _getTier (boundary values), already-staked rejection. |
| 10.4 | **Tests: Vesting** | Contracts | 2h | Test createSchedule, release (before cliff, during vest, after full vest), revoke (revocable vs non-revocable), getReleasable at various timestamps. |
| 10.5 | **Tests: Fuzz testing** | Contracts | 2h | Fuzz tests (1000 runs) for: mint amounts, claim amounts, stake amounts, vesting calculations. Invariant tests: total minted <= MAX_SUPPLY, claimed <= cumulative, vested <= total. |
| 10.6 | **Contracts: Deploy to Base Sepolia** | Contracts | 1h | Run Deploy.s.sol against Base Sepolia testnet. Verify all contracts on Basescan. Record addresses in SMART-CONTRACTS.md. Grant roles. Test a manual claim on testnet. |

### Epic 3.3: Wallet + Merkle Backend

#### Wave 11 — Depends on Wave 10 (testnet deployed); Phase 2 backend

| ID | Task | Agent | Est | Acceptance Criteria |
|----|------|-------|-----|---------------------|
| 11.1 | **Android: WalletManager** | Android | 4h | Per ANDROID-APP.md: BIP-39 mnemonic generation (12 words), BIP-44 key derivation, Ethereum keypair. Store private key in Android Keystore (encrypted). `getAddress()`, `getBalance()` (ERC-20 balanceOf via Web3j), `signMessage()`. RPC via Alchemy/Infura for Base L2. |
| 11.2 | **Android: KeystoreHelper** | Android | 2h | Android Keystore wrapper. Generate AES key for encrypting wallet private key. Encrypt/decrypt operations. Hardware-backed when available. Clear sensitive data from memory after use. |
| 11.3 | **Android: MnemonicGenerator + backup flow** | Android | 2h | Generate BIP-39 mnemonic, display to user with "write this down" warning, require confirmation (select words in order). Import flow: paste mnemonic or private key. Validate mnemonic checksum. |
| 11.4 | **Android: TransactionBuilder** | Android | 2h | Build claim transaction: call `RewardDistributor.claim(cumulativeAmount, merkleProof)`. Build send transaction: ERC-20 `transfer(to, amount)`. Gas estimation via Web3j gas oracle. Wait for 1 block confirmation on Base. |
| 11.5 | **Backend: Reward calculator** | Backend | 3h | `internal/reward/calculator.go` — Runs hourly. Aggregates metering data per node for the epoch. Base reward = bytes/1e6 × PRXYPerMB. Apply multipliers: trust (0.5x-2.0x), uptime (1.0x-1.5x), quality (0.8x-1.3x), staking (1.0x-1.5x). Insert into earnings table. Update claimable_rewards cumulative. |
| 11.6 | **Backend: Merkle tree generator** | Backend | 3h | `internal/reward/merkle.go` — Runs daily. Get all pending rewards from claimable_rewards. Build leaf nodes: `keccak256(wallet, cumulativeAmount)`. Build Merkle tree. Store proofs in database per wallet. Return root hash. |

### Epic 3.4: Claim Flow + Wallet UI

#### Wave 12 — Depends on Wave 11

| ID | Task | Agent | Est | Acceptance Criteria |
|----|------|-------|-----|---------------------|
| 12.1 | **Backend: Merkle root publisher** | Backend | 2h | `internal/reward/distributor.go` — Call `setMerkleRoot(root)` on RewardDistributor contract via Go Ethereum client. Store tx hash. Triggered after daily Merkle generation. Error handling + retry. |
| 12.2 | **Backend: Claim proof API** | Backend | 1h | `GET /v1/rewards/proof` — Returns Merkle proof for requesting node's wallet. Include: cumulativeAmount, proof array, current epoch. `GET /v1/rewards/history` — Claim history with tx hashes. |
| 12.3 | **Android: Wallet screen** | Android | 3h | Per ANDROID-APP.md: PRXY balance (on-chain), pending balance (off-chain), wallet address with QR + copy, "Claim Rewards" button (fetches proof, builds tx, signs, submits), "Send PRXY" button, transaction history list, token price from DEX, block explorer link. |
| 12.4 | **Android: WalletViewModel** | Android | 2h | Coordinate: WalletManager (on-chain balance), API (pending earnings, claim proof), TransactionBuilder (claim/send). Expose WalletUiState with balance, pending, transactions, loading states. Handle claim flow: fetch proof → build tx → sign → submit → wait confirmation → refresh. |
| 12.5 | **Android: WalletSetupScreen** | Android | 2h | Onboarding wallet step: "Create New Wallet" (generate mnemonic, show backup, confirm) or "Import Existing" (paste mnemonic/key). Wallet address display with copy. Navigate to Dashboard after setup. Persist wallet creation state. |
| 12.6 | **Integration: End-to-end claim flow** | Backend | 2h | Test script: node earns rewards → reward calculator runs → Merkle tree generated → root published on-chain → node fetches proof → node claims on testnet → verify tokens received. Full cycle validation. |

**Phase 3 Success Gate**:
- [ ] All 4 contracts deployed to Base Sepolia
- [ ] All contract tests passing, >95% coverage
- [ ] Wallet generation + key storage verified secure
- [ ] Merkle tree generation < 5 minutes for 10K nodes
- [ ] End-to-end claim flow working on testnet
- [ ] Gas cost per claim < $0.01 on Base

---

## Phase 4: Beta (Waves 13-16)

**Goal**: Customer API, anti-fraud, referrals, billing. Ready for public launch.

### Epic 4.1: Customer API

#### Wave 13 — Depends on Phase 1-2 proxy pipeline

| ID | Task | Agent | Est | Acceptance Criteria |
|----|------|-------|-----|---------------------|
| 13.1 | **Backend: Customer auth** | Backend | 3h | `internal/auth/` — Customer registration (email + bcrypt password), JWT token generation (15 min expiry), refresh tokens (7 day), API key generation (`prxy_live_xxxx` format, stored as bcrypt hash), key rotation. |
| 13.2 | **Backend: Customer registration + login** | Backend | 2h | `POST /v1/auth/register` (email, password → create customer, return JWT), `POST /v1/auth/login` (email, password → verify, return JWT + refresh), `GET /v1/auth/apikey` (return/rotate API key). Input validation, rate limiting on login. |
| 13.3 | **Backend: Proxy request endpoint** | Backend | 3h | `POST /v1/proxy` — Per BACKEND.md: authenticate (API key or JWT), validate request body (url, method, geo, headers, timeout), pass to Orchestrator, wait for response, return formatted response with node info + metrics. Error responses for timeout, no nodes available, rate limit. |
| 13.4 | **Backend: Batch proxy endpoint** | Backend | 2h | `POST /v1/proxy/batch` — Accept array of proxy requests, execute concurrently (up to 10 parallel), return array of responses. Same auth and validation as single endpoint. Aggregate metrics. |
| 13.5 | **Backend: Usage tracking** | Backend | 2h | `internal/customer/usage.go` — Record per-customer daily usage (requests, bytes, cost_usd) in customer_usage table. `GET /v1/usage` returns usage stats. `GET /v1/usage/daily` returns daily breakdown. Enforce plan limits. |
| 13.6 | **Backend: Rate limiting** | Backend | 2h | Redis-based rate limiting per API key per day. Free: 100 req/day, Starter: 10K, Pro: 100K, Enterprise: custom. Return `429 Too Many Requests` with `Retry-After` header. Track in Redis: `ratelimit:{apiKey}:{date}`. |

### Epic 4.2: Anti-Fraud + Trust

#### Wave 14 — Depends on metering data (Phase 2)

| ID | Task | Agent | Est | Acceptance Criteria |
|----|------|-------|-----|---------------------|
| 14.1 | **Backend: Fraud detector** | Backend | 4h | `internal/fraud/detector.go` — Detect: self-proxying (impossible since backend controls requests — verify), emulator (via attestation), bandwidth inflation (server vs client byte divergence >10%), Sybil (multiple nodes same IP range). Log fraud_events with severity and details. |
| 14.2 | **Backend: IP intelligence** | Backend | 3h | `internal/fraud/ip_intelligence.go` — Classify IP: residential, datacenter, VPN, proxy using GeoIP database + ASN analysis. Datacenter/VPN IPs earn 0x. Cross-reference claimed location vs IP geolocation. Flag mismatches. |
| 14.3 | **Backend: Behavioral analysis** | Backend | 3h | `internal/fraud/behavioral.go` — Detect: sudden bandwidth spikes (>10x normal), no sleep cycles (24/7 activity = likely emulator), pattern correlations across nodes from same IP range. New node ramp-up: 0.5x earnings first 7 days. |
| 14.4 | **Backend: Trust score calculator** | Backend | 2h | Per SECURITY.md model: weighted sum of device_attestation (0.15), uptime_consistency (0.20), verification_pass_rate (0.25), ip_quality (0.15), bandwidth_consistency (0.15), account_age (0.10). Score 0.0-1.0. Multiplier: 0.5x (0.3) to 2.0x (1.0). Below 0.3: suspend. Below 0.1: ban. |
| 14.5 | **Backend: Fraud event logging** | Backend | 1h | Store fraud events in fraud_events table. Auto-actions: warning → trust reduction, critical → suspension/ban. Admin review queue for manual cases. |
| 14.6 | **Backend: Spot-check verification** | Backend | 2h | Periodically send known URLs to nodes, verify response integrity (hash match). Failed spot-checks reduce trust score. Track verification_pass_rate per node. |

### Epic 4.3: Beta Features

#### Wave 15 — Depends on Waves 13-14

| ID | Task | Agent | Est | Acceptance Criteria |
|----|------|-------|-----|---------------------|
| 15.1 | **Android: Onboarding flow** | Android | 3h | WelcomeScreen (logo, tagline, value props, "Get Started" CTA), PermissionsScreen (battery optimization, notifications with explanations), flow into WalletSetupScreen. Only shown on first launch. Onboarding state persisted. |
| 15.2 | **Android: Referral screen** | Android | 2h | Unique referral code display, share button (deep link), referral stats (count, total earnings), referral list with per-referral earnings. |
| 15.3 | **Backend: Referral system** | Backend | 3h | Generate unique referral codes per node. `POST /v1/referral/apply` — link referee to referrer in referrals table. 5% of referee's earnings shared to referrer. `GET /v1/referral/stats` + `GET /v1/referral/code`. Referral earnings show in earnings breakdown. |
| 15.4 | **Backend: Billing system** | Backend | 3h | `internal/customer/billing.go` — Plan management (free/starter/pro/enterprise). Billing summary endpoint. Invoice generation (simple: period, requests, bytes, cost). `GET /v1/billing` + `GET /v1/billing/invoices`. |
| 15.5 | **Backend: Geographic targeting** | Backend | 2h | Enhance NodeSelector to support country + region targeting from customer requests. `GET /v1/status` — Return available node counts by country/region for customers to check coverage before requesting. |
| 15.6 | **Backend: Sticky sessions** | Backend | 1h | `session:{sessionID}` → nodeID in Redis with TTL 5min. Customer sends `session_id` in proxy request. Selector checks Redis for existing session binding. Route to same node if available, else new node + update binding. |

### Epic 4.4: Polish + Production

#### Wave 16 — Final wave

| ID | Task | Agent | Est | Acceptance Criteria |
|----|------|-------|-----|---------------------|
| 16.1 | **Android: Security hardening** | Android | 3h | Certificate pinning (OkHttp CertificatePinner), root detection (su binary, Magisk, system mods), emulator detection (build props, sensors, telephony), APK signature verification at runtime, memory zeroing of sensitive data. |
| 16.2 | **Android: Release build config** | Android | 1h | ProGuard/R8 rules for aggressive obfuscation. Strip all Log calls in release. Signing config. Version name/code management. Release build type configuration. |
| 16.3 | **Backend: API Gateway config** | Backend | 2h | Nginx or Caddy config: TLS termination, rate limiting, CORS (strict origin allowlist), request size limits, WebSocket upgrade support, health check endpoints. |
| 16.4 | **Infrastructure: Production deployment** | Infra | 3h | Docker Compose production config OR K8s manifests per ARCHITECTURE.md. Service health checks, resource limits, restart policies, log aggregation. Separate configs for orchestrator, api, metering services. |
| 16.5 | **Docs: API reference + legal** | Infra | 2h | `docs/api-reference.md` (OpenAPI/Swagger for Customer API), `docs/legal/terms-of-service.md`, `docs/legal/privacy-policy.md`, `docs/legal/acceptable-use.md` — all skeleton documents per SECURITY.md requirements. |
| 16.6 | **Integration: Full system E2E tests** | Backend | 3h | Complete integration test suite: customer registers → gets API key → submits proxy request → node receives and executes → response returned → metering recorded → rewards calculated → Merkle generated → claim on testnet succeeds. Test fraud detection triggers. Test rate limiting. |

**Phase 4 Success Gate**:
- [ ] Customer API functioning with auth + rate limiting
- [ ] Anti-fraud catching >80% of simulated attacks
- [ ] Trust score accurately ranking nodes
- [ ] Referral system working end-to-end
- [ ] Customer API latency p95 < 2 seconds
- [ ] All security hardening measures active
- [ ] Production deployment config working

---

## Dependency Graph (Simplified)

```
Wave 1 (Scaffold)
  ├── Wave 2 (Foundation)
  │     ├── Wave 3 (Backend Core)
  │     │     └── Wave 5 (Integration)
  │     └── Wave 4 (Android Core)
  │           └── Wave 5 (Integration)
  │                 ├── Wave 6 (HTTPS+Security)  ── Phase 2
  │                 │     └── Wave 8 (UI)
  │                 └── Wave 7 (Attestation)  ── parallel
  │                       └── Wave 8 (UI)
  │
  └── Wave 9 (Contracts) ── can start early
        └── Wave 10 (Contract Tests)
              └── Wave 11 (Wallet+Merkle)
                    └── Wave 12 (Claim Flow)

Wave 8 (Phase 2 done) + Wave 12 (Phase 3 done)
  ├── Wave 13 (Customer API)
  │     └── Wave 15 (Beta Features)
  ├── Wave 14 (Anti-Fraud)
  │     └── Wave 15 (Beta Features)
  └── Wave 15 → Wave 16 (Production)
```

## Agent Assignments

| Agent | Waves | Total Tasks | Primary Skills |
|-------|-------|-------------|----------------|
| **Backend Specialist** | 2-3, 5-7, 11-14, 16 | ~42 tasks | Go, PostgreSQL, Redis, WebSocket, gRPC |
| **Android Specialist** | 2, 4, 7-8, 11-12, 15-16 | ~35 tasks | Kotlin, Compose, Hilt, Room, Web3j |
| **Contract Specialist** | 9-10 | ~12 tasks | Solidity, Foundry, OpenZeppelin |
| **Infrastructure Specialist** | 1-2, 5, 16 | ~8 tasks | Docker, Protobuf, Nginx, K8s |

## Execution Strategy

**Recommended**: Run each phase as a separate marathon session:
1. **Marathon 1**: Phase 0 + Phase 1 (Waves 1-5) — ~45h agent time
2. **Marathon 2**: Phase 2 (Waves 6-8) + start contracts (Wave 9) — ~35h agent time
3. **Marathon 3**: Phase 3 remainder (Waves 10-12) — ~25h agent time
4. **Marathon 4**: Phase 4 (Waves 13-16) — ~40h agent time

Smart contracts (Waves 9-10) are independent and can run as a parallel marathon alongside Phase 2.

## Risk Mitigations

| Risk | Mitigation |
|------|-----------|
| Android foreground service killed by OS | Proper notification, WakeLock, battery exemption, START_STICKY |
| WebSocket drops on cellular | Exponential backoff + jitter, session resume |
| High battery drain | ResourceMonitor with configurable thresholds |
| Play Store rejection | Prepare F-Droid, direct APK distribution |
| Smart contract exploit | Professional audit before mainnet (not in this plan) |
| Token classified as security | Legal opinion needed (parallel track, not in this plan) |

## Out of Scope (Phase 5+)

- iOS app
- Desktop app (macOS/Windows/Linux)
- WASM compute marketplace
- DEX listing + liquidity provision
- DAO governance
- ML-based fraud detection
- Mobile SDK for third-party integration
- CEX listing
- Cross-chain bridges

---

**Next Step**: Run `/ultra-decompose` on this plan to generate granular JSON task definitions for marathon execution.
