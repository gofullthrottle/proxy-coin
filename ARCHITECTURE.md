# Proxy Coin - Technical Architecture

## System Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        CUSTOMER LAYER                               │
│                                                                     │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐              │
│  │ Web Scraping │  │ Ad Verify    │  │ Market       │  ...         │
│  │ Company      │  │ Platform     │  │ Research     │              │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘              │
│         └──────────────────┼──────────────────┘                     │
│                            │ REST/gRPC API                          │
└────────────────────────────┼────────────────────────────────────────┘
                             │
┌────────────────────────────┼────────────────────────────────────────┐
│                     BACKEND LAYER                                   │
│                            │                                        │
│  ┌─────────────────────────▼──────────────────────────┐            │
│  │              API Gateway (Nginx/Caddy)              │            │
│  │         Rate limiting, TLS termination, auth        │            │
│  └──────────┬──────────────┬──────────────┬───────────┘            │
│             │              │              │                         │
│  ┌──────────▼──┐  ┌───────▼──────┐  ┌───▼───────────┐            │
│  │ Customer    │  │ Orchestrator │  │ Metering      │            │
│  │ API         │  │ Service      │  │ Service       │            │
│  │             │  │              │  │               │            │
│  │ • Auth      │  │ • Node       │  │ • Byte count  │            │
│  │ • Proxy req │  │   registry   │  │ • Aggregation │            │
│  │ • Billing   │  │ • Request    │  │ • Fraud det.  │            │
│  │ • Dashboard │  │   routing    │  │ • Merkle gen  │            │
│  └──────┬──────┘  │ • WebSocket  │  │ • Rewards     │            │
│         │         │   manager    │  └───────┬───────┘            │
│         │         │ • Load bal.  │          │                     │
│         │         └──────┬───────┘          │                     │
│         │                │                  │                     │
│  ┌──────▼────────────────▼──────────────────▼──────┐              │
│  │              PostgreSQL + Redis                  │              │
│  │    Nodes, sessions, metering, users, billing     │              │
│  └──────────────────────┬──────────────────────────┘              │
│                         │                                          │
└─────────────────────────┼──────────────────────────────────────────┘
                          │
         WebSocket tunnels (wss://, multiplexed, protobuf)
         ┌────────────────┼────────────────┐
         │                │                │
┌────────▼───┐   ┌───────▼────┐   ┌───────▼────┐
│  Android   │   │  Android   │   │  Android   │
│  Node 1    │   │  Node 2    │   │  Node N    │
│            │   │            │   │            │
│ ┌────────┐ │   │ ┌────────┐ │   │ ┌────────┐ │
│ │Proxy   │ │   │ │Proxy   │ │   │ │Proxy   │ │
│ │Engine  │ │   │ │Engine  │ │   │ │Engine  │ │
│ └───┬────┘ │   │ └───┬────┘ │   │ └───┬────┘ │
│     │      │   │     │      │   │     │      │
│ ┌───▼────┐ │   │ ┌───▼────┐ │   │ ┌───▼────┐ │
│ │Wallet  │ │   │ │Wallet  │ │   │ │Wallet  │ │
│ │Manager │ │   │ │Manager │ │   │ │Manager │ │
│ └────────┘ │   │ └────────┘ │   │ └────────┘ │
└────────────┘   └────────────┘   └────────────┘
         │                │                │
         ▼                ▼                ▼
    (exit traffic from residential IPs)

┌─────────────────────────────────────────────────────────────────────┐
│                     BLOCKCHAIN LAYER (Base L2)                      │
│                                                                     │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐              │
│  │ ProxyCoin    │  │ Reward       │  │ Staking      │              │
│  │ Token (ERC20)│  │ Distributor  │  │ Contract     │              │
│  └──────────────┘  └──────────────┘  └──────────────┘              │
│                                                                     │
│  ┌──────────────┐                                                   │
│  │ Vesting      │                                                   │
│  │ Contract     │                                                   │
│  └──────────────┘                                                   │
└─────────────────────────────────────────────────────────────────────┘
```

## Data Flow: Proxy Request Lifecycle

```
1. Customer sends proxy request
   POST /v1/proxy
   {
     "url": "https://example.com/api/data",
     "method": "GET",
     "geo": "US-CA",
     "headers": {"Accept": "application/json"}
   }

2. API Gateway authenticates customer (API key + JWT)

3. Customer API validates request, passes to Orchestrator

4. Orchestrator selects optimal node:
   - Filter: geo=US-CA, status=active, trust_score>0.5
   - Sort: by load (ascending), latency (ascending)
   - Select: top candidate with capacity

5. Orchestrator sends PROXY_REQUEST via WebSocket to selected node:
   ProxyRequest {
     request_id: "req_abc123"
     method: GET
     url: "https://example.com/api/data"
     headers: [("Accept", "application/json")]
   }

6. Android node's ProxyEngine:
   a. Validates URL against blocklist
   b. Executes HTTP request from device
   c. Streams response back via WebSocket:
      ProxyResponseStart { request_id, status: 200, headers, content_length }
      ProxyResponseChunk { request_id, chunk_index: 0, data: bytes[0..64KB] }
      ProxyResponseChunk { request_id, chunk_index: 1, data: bytes[64KB..128KB] }
      ProxyResponseEnd   { request_id, total_bytes: 128000 }

7. Orchestrator forwards response to Customer API → Customer

8. Metering Service records:
   - node_id, request_id, bytes_in, bytes_out, latency_ms, success
   - Server-side byte count (authoritative, not client-reported)

9. Periodically (hourly), Reward Calculator:
   - Aggregates metering data per node
   - Applies trust_score multiplier
   - Updates off-chain balance in PostgreSQL
   - App polls for updated earnings
```

## Component Interaction Map

| Component | Communicates With | Protocol | Purpose |
|-----------|------------------|----------|---------|
| Customer API | API Gateway | HTTP/gRPC | Receive proxy requests |
| Customer API | Orchestrator | Internal gRPC | Route proxy requests |
| Orchestrator | Android Nodes | WebSocket (protobuf) | Send/receive proxy traffic |
| Orchestrator | Redis | TCP | Node status, session state |
| Orchestrator | PostgreSQL | TCP | Node registry, config |
| Metering Service | PostgreSQL | TCP | Store usage records |
| Metering Service | Redis Streams | TCP | Consume metering events |
| Reward Calculator | PostgreSQL | TCP | Read metering, write rewards |
| Reward Calculator | Base L2 RPC | HTTPS | Publish Merkle roots |
| Android App | Backend API | HTTPS | Auth, earnings, config |
| Android App | Orchestrator | WebSocket | Proxy traffic |
| Android App | Base L2 RPC | HTTPS | Wallet balance, claim tx |

## Project Directory Structure

```
proxy-coin/
├── android/                              # Android Application
│   ├── app/
│   │   ├── src/
│   │   │   ├── main/
│   │   │   │   ├── java/com/proxycoin/app/
│   │   │   │   │   ├── ProxyCoinApp.kt
│   │   │   │   │   ├── di/                          # Hilt dependency injection
│   │   │   │   │   │   ├── AppModule.kt
│   │   │   │   │   │   ├── NetworkModule.kt
│   │   │   │   │   │   ├── DatabaseModule.kt
│   │   │   │   │   │   └── CryptoModule.kt
│   │   │   │   │   ├── ui/                           # Jetpack Compose UI
│   │   │   │   │   │   ├── theme/
│   │   │   │   │   │   │   ├── Theme.kt
│   │   │   │   │   │   │   ├── Color.kt
│   │   │   │   │   │   │   └── Type.kt
│   │   │   │   │   │   ├── navigation/
│   │   │   │   │   │   │   └── NavGraph.kt
│   │   │   │   │   │   ├── onboarding/
│   │   │   │   │   │   │   ├── WelcomeScreen.kt
│   │   │   │   │   │   │   ├── PermissionsScreen.kt
│   │   │   │   │   │   │   └── WalletSetupScreen.kt
│   │   │   │   │   │   ├── dashboard/
│   │   │   │   │   │   │   ├── DashboardScreen.kt
│   │   │   │   │   │   │   └── DashboardViewModel.kt
│   │   │   │   │   │   ├── earnings/
│   │   │   │   │   │   │   ├── EarningsScreen.kt
│   │   │   │   │   │   │   └── EarningsViewModel.kt
│   │   │   │   │   │   ├── wallet/
│   │   │   │   │   │   │   ├── WalletScreen.kt
│   │   │   │   │   │   │   └── WalletViewModel.kt
│   │   │   │   │   │   ├── settings/
│   │   │   │   │   │   │   ├── SettingsScreen.kt
│   │   │   │   │   │   │   └── SettingsViewModel.kt
│   │   │   │   │   │   └── referral/
│   │   │   │   │   │       ├── ReferralScreen.kt
│   │   │   │   │   │       └── ReferralViewModel.kt
│   │   │   │   │   ├── service/                      # Background services
│   │   │   │   │   │   ├── ProxyForegroundService.kt
│   │   │   │   │   │   ├── ProxyEngine.kt
│   │   │   │   │   │   ├── WebSocketClient.kt
│   │   │   │   │   │   ├── ResourceMonitor.kt
│   │   │   │   │   │   └── BootReceiver.kt
│   │   │   │   │   ├── domain/                       # Business logic
│   │   │   │   │   │   ├── model/
│   │   │   │   │   │   │   ├── Node.kt
│   │   │   │   │   │   │   ├── Earnings.kt
│   │   │   │   │   │   │   ├── ProxyRequest.kt
│   │   │   │   │   │   │   └── WalletInfo.kt
│   │   │   │   │   │   ├── usecase/
│   │   │   │   │   │   │   ├── StartProxyUseCase.kt
│   │   │   │   │   │   │   ├── StopProxyUseCase.kt
│   │   │   │   │   │   │   ├── GetEarningsUseCase.kt
│   │   │   │   │   │   │   ├── ClaimRewardsUseCase.kt
│   │   │   │   │   │   │   └── CheckResourcesUseCase.kt
│   │   │   │   │   │   └── repository/
│   │   │   │   │   │       ├── NodeRepository.kt
│   │   │   │   │   │       ├── EarningsRepository.kt
│   │   │   │   │   │       ├── WalletRepository.kt
│   │   │   │   │   │       └── SettingsRepository.kt
│   │   │   │   │   ├── data/                         # Data layer
│   │   │   │   │   │   ├── local/
│   │   │   │   │   │   │   ├── AppDatabase.kt
│   │   │   │   │   │   │   ├── dao/
│   │   │   │   │   │   │   │   ├── EarningsDao.kt
│   │   │   │   │   │   │   │   ├── MeteringDao.kt
│   │   │   │   │   │   │   │   └── TransactionDao.kt
│   │   │   │   │   │   │   └── entity/
│   │   │   │   │   │   │       ├── EarningsEntity.kt
│   │   │   │   │   │   │       ├── MeteringEntity.kt
│   │   │   │   │   │   │       └── TransactionEntity.kt
│   │   │   │   │   │   ├── remote/
│   │   │   │   │   │   │   ├── ApiService.kt
│   │   │   │   │   │   │   ├── dto/
│   │   │   │   │   │   │   │   ├── RegisterNodeDto.kt
│   │   │   │   │   │   │   │   ├── EarningsDto.kt
│   │   │   │   │   │   │   │   └── ClaimRewardDto.kt
│   │   │   │   │   │   │   └── interceptor/
│   │   │   │   │   │   │       ├── AuthInterceptor.kt
│   │   │   │   │   │   │       └── CertPinningInterceptor.kt
│   │   │   │   │   │   ├── repository/
│   │   │   │   │   │   │   ├── NodeRepositoryImpl.kt
│   │   │   │   │   │   │   ├── EarningsRepositoryImpl.kt
│   │   │   │   │   │   │   ├── WalletRepositoryImpl.kt
│   │   │   │   │   │   │   └── SettingsRepositoryImpl.kt
│   │   │   │   │   │   └── preferences/
│   │   │   │   │   │       └── EncryptedPrefsManager.kt
│   │   │   │   │   ├── crypto/                       # Wallet operations
│   │   │   │   │   │   ├── WalletManager.kt
│   │   │   │   │   │   ├── KeystoreHelper.kt
│   │   │   │   │   │   ├── MnemonicGenerator.kt
│   │   │   │   │   │   └── TransactionBuilder.kt
│   │   │   │   │   └── util/
│   │   │   │   │       ├── NetworkUtil.kt
│   │   │   │   │       ├── BatteryUtil.kt
│   │   │   │   │       └── DeviceInfo.kt
│   │   │   │   ├── res/
│   │   │   │   │   ├── values/
│   │   │   │   │   ├── drawable/
│   │   │   │   │   └── xml/
│   │   │   │   └── AndroidManifest.xml
│   │   │   └── test/                                 # Unit tests
│   │   └── build.gradle.kts
│   ├── gradle/
│   │   └── libs.versions.toml                        # Version catalog
│   ├── build.gradle.kts                              # Root build
│   ├── settings.gradle.kts
│   └── gradle.properties
│
├── backend/                              # Backend Services (Go)
│   ├── cmd/
│   │   ├── orchestrator/                 # WebSocket + routing server
│   │   │   └── main.go
│   │   ├── api/                          # Customer-facing REST API
│   │   │   └── main.go
│   │   └── metering/                     # Metering + reward calculation
│   │       └── main.go
│   ├── internal/
│   │   ├── node/                         # Node registry & lifecycle
│   │   │   ├── registry.go
│   │   │   ├── health.go
│   │   │   └── selector.go
│   │   ├── proxy/                        # Proxy request handling
│   │   │   ├── router.go
│   │   │   ├── handler.go
│   │   │   └── filter.go                # Domain blocklist
│   │   ├── websocket/                    # WebSocket management
│   │   │   ├── server.go
│   │   │   ├── connection.go
│   │   │   └── pool.go
│   │   ├── metering/                     # Usage tracking
│   │   │   ├── counter.go
│   │   │   ├── aggregator.go
│   │   │   └── reporter.go
│   │   ├── reward/                       # Reward calculation
│   │   │   ├── calculator.go
│   │   │   ├── merkle.go
│   │   │   └── distributor.go
│   │   ├── fraud/                        # Anti-fraud
│   │   │   ├── detector.go
│   │   │   ├── ip_intelligence.go
│   │   │   ├── behavioral.go
│   │   │   └── attestation.go
│   │   ├── auth/                         # Authentication
│   │   │   ├── jwt.go
│   │   │   ├── apikey.go
│   │   │   └── device.go
│   │   ├── customer/                     # Customer management
│   │   │   ├── handler.go
│   │   │   ├── billing.go
│   │   │   └── usage.go
│   │   └── config/
│   │       └── config.go
│   ├── pkg/
│   │   ├── protocol/                     # Generated protobuf code
│   │   │   ├── node.pb.go
│   │   │   └── metering.pb.go
│   │   └── blockchain/                   # On-chain interactions
│   │       ├── client.go
│   │       ├── token.go
│   │       └── distributor.go
│   ├── migrations/
│   │   ├── 001_initial.up.sql
│   │   └── 001_initial.down.sql
│   ├── go.mod
│   ├── go.sum
│   └── Dockerfile
│
├── contracts/                            # Smart Contracts (Foundry)
│   ├── src/
│   │   ├── ProxyCoinToken.sol
│   │   ├── RewardDistributor.sol
│   │   ├── Staking.sol
│   │   └── Vesting.sol
│   ├── test/
│   │   ├── ProxyCoinToken.t.sol
│   │   ├── RewardDistributor.t.sol
│   │   ├── Staking.t.sol
│   │   └── Vesting.t.sol
│   ├── script/
│   │   ├── Deploy.s.sol
│   │   └── DistributeRewards.s.sol
│   ├── foundry.toml
│   └── remappings.txt
│
├── protocol/                             # Shared Protobuf Definitions
│   └── proto/
│       ├── node.proto
│       ├── proxy.proto
│       └── metering.proto
│
├── infrastructure/
│   ├── docker-compose.yml
│   ├── docker-compose.dev.yml
│   ├── nginx/
│   │   └── nginx.conf
│   └── k8s/
│       ├── orchestrator-deployment.yaml
│       ├── api-deployment.yaml
│       ├── metering-deployment.yaml
│       └── ingress.yaml
│
├── scripts/
│   ├── generate-proto.sh
│   ├── deploy-contracts.sh
│   └── generate-merkle.py
│
├── docs/
│   ├── api-reference.md
│   └── legal/
│       ├── terms-of-service.md
│       ├── privacy-policy.md
│       └── acceptable-use.md
│
├── SPEC.md
├── ARCHITECTURE.md                       # (this file)
├── ANDROID-APP.md
├── BACKEND.md
├── TOKENOMICS.md
├── SMART-CONTRACTS.md
├── SECURITY-AND-COMPLIANCE.md
├── ROADMAP.md
└── COMPETITIVE-ANALYSIS.md
```

## Key Architectural Decisions

### 1. WebSocket Tunnel (not VPN, not direct proxy)

**Decision**: Devices connect outbound to the backend via WebSocket. The backend pushes proxy requests through this tunnel.

**Why**:
- Works behind any NAT/firewall (outbound connection from device)
- No port forwarding needed
- No conflict with user's existing VPN
- No special Android permissions beyond INTERNET
- This is how Grass, Honeygain, and EarnApp all work

**Trade-off**: Slightly higher latency than direct proxy (~10-50ms overhead). Acceptable for web scraping use cases.

### 2. Server-Side Metering (not client-reported)

**Decision**: The backend counts bytes flowing through the WebSocket, not the app reporting its own usage.

**Why**: Client-reported metering is trivially fakeable. Server-side metering is authoritative and tamper-proof.

### 3. Off-Chain Earnings + On-Chain Claims

**Decision**: Earnings accumulate off-chain in PostgreSQL. Users claim on-chain via Merkle proofs.

**Why**: On-chain per-request rewards would cost more in gas than the reward value. Batch settlement via Merkle trees amortizes gas across thousands of claims.

### 4. Go for Backend (not Node.js, not Rust)

**Decision**: Go for all backend services.

**Why**: Goroutines handle thousands of concurrent WebSocket connections naturally. Simpler than Rust with comparable performance for I/O-bound work. Better concurrency than Node.js. Single binary deployment.

### 5. Base L2 (not Solana, not Polygon)

**Decision**: Deploy token on Coinbase's Base L2.

**Why**: Sub-cent fees, EVM compatibility, seamless Coinbase wallet integration for fiat offramp. Best onboarding experience for non-crypto users.
