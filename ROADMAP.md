# Proxy Coin - Development Roadmap

## Phase Overview

```
Phase 1: PoC          Phase 2: Alpha        Phase 3: Token       Phase 4: Beta         Phase 5: Growth
(4-6 weeks)           (4-6 weeks)           (4-6 weeks)          (6-8 weeks)           (ongoing)
─────────────────────────────────────────────────────────────────────────────────────────────────────►

• Android service      • HTTPS tunneling     • Deploy contracts    • Customer API        • iOS app
• WebSocket tunnel     • Domain filtering    • Wallet in app       • Anti-fraud v1       • Desktop app
• Basic proxy engine   • Resource monitor    • Merkle rewards      • Trust scoring       • Compute tasks
• Points tracking      • Play Integrity      • Claim flow          • Referral program    • DEX listing
• Manual testing       • Dashboard UI        • Testnet deploy      • Mainnet deploy      • DAO governance
```

## Phase 1: Proof of Concept (4-6 weeks)

**Goal**: Prove that proxying web traffic through Android devices via WebSocket tunnels works reliably.

### Deliverables

| Item | Description | Priority |
|------|-------------|----------|
| Foreground service | Persistent Android service with notification | P0 |
| WebSocket client | Connect to backend, auto-reconnect | P0 |
| Proxy engine (HTTP) | Execute HTTP requests from device, return response | P0 |
| Backend orchestrator | Single Go binary: WebSocket server + request routing | P0 |
| Protobuf protocol | Define message types, compile for Go + Kotlin | P0 |
| Points ledger | Track "points" in PostgreSQL (no blockchain yet) | P1 |
| Test harness | Script to send proxy requests and verify responses | P1 |
| Basic settings | WiFi-only toggle, on/off switch | P2 |

### Success Criteria

- [ ] 10 test devices connected simultaneously for 24 hours without crashes
- [ ] Proxy success rate > 90% for HTTP requests
- [ ] Latency overhead < 200ms (vs direct request)
- [ ] Battery drain < 5% per hour when active
- [ ] Auto-reconnect after network switch (WiFi → cellular → WiFi)

### Technical Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Android kills foreground service | Medium | High | Proper notification, WakeLock, battery exemption |
| WebSocket drops under cellular | Medium | Medium | Exponential backoff, session resume |
| High battery drain | Medium | High | Aggressive power monitoring, configurable limits |
| OkHttp connection pool issues | Low | Medium | Custom connection pool config, connection reuse |

## Phase 2: Alpha (4-6 weeks)

**Goal**: Production-quality proxy service with security controls and basic UI.

### Deliverables

| Item | Description | Priority |
|------|-------------|----------|
| HTTPS proxy (CONNECT) | Support HTTPS via CONNECT tunneling | P0 |
| Domain blocklist | Server-distributed hash-based blocklist | P0 |
| Resource monitor | Battery, network type, thermal monitoring | P0 |
| Play Integrity | Device attestation integration | P0 |
| Dashboard UI | Jetpack Compose: status, earnings, stats | P1 |
| Settings UI | Bandwidth cap, battery threshold, WiFi-only | P1 |
| Server-side metering | Authoritative byte counting on backend | P1 |
| Auto-start on boot | BootReceiver to restart service | P2 |
| Node config push | Backend pushes config updates to nodes | P2 |

### Success Criteria

- [ ] 50-100 devices running stably for 1 week
- [ ] HTTPS proxy success rate > 85%
- [ ] Play Integrity passing on all test devices
- [ ] Domain blocklist blocking 100% of test entries
- [ ] Battery drain < 3% per hour (optimized)
- [ ] Clean dashboard showing real-time stats

## Phase 3: Token Launch (4-6 weeks)

**Goal**: Deploy smart contracts, integrate wallet, enable earning and claiming PRXY.

### Deliverables

| Item | Description | Priority |
|------|-------------|----------|
| ProxyCoinToken.sol | Deploy ERC-20 to Base Sepolia | P0 |
| RewardDistributor.sol | Merkle-based claim contract | P0 |
| Wallet generation | BIP-39 mnemonic, Keystore storage | P0 |
| Claim flow | App fetches proof, signs tx, claims tokens | P0 |
| Earnings → rewards | Backend calculates PRXY rewards from metering | P0 |
| Merkle tree generator | Daily batch generation + on-chain root publish | P0 |
| Wallet UI | Balance, claim button, transaction history | P1 |
| Staking.sol | Stake for earning multiplier | P1 |
| Vesting.sol | Team/investor vesting schedules | P2 |
| Contract tests | >95% coverage, fuzz tests | P0 |

### Success Criteria

- [ ] Complete claim flow working on Base Sepolia testnet
- [ ] Wallet generation and key storage verified secure
- [ ] Merkle tree generation < 5 minutes for 10K nodes
- [ ] All contract tests passing, >95% coverage
- [ ] Gas cost per claim < $0.01 on Base

### Before Mainnet (Gating Checklist)

- [ ] Smart contract security audit completed
- [ ] Legal opinion on token classification obtained
- [ ] Terms of Service and Privacy Policy reviewed by legal
- [ ] Multisig wallet set up for admin operations
- [ ] Emergency pause mechanism tested

## Phase 4: Beta (6-8 weeks)

**Goal**: Launch to public with customer API, anti-fraud, and mainnet token.

### Deliverables

| Item | Description | Priority |
|------|-------------|----------|
| Customer API | REST API for proxy buyers | P0 |
| Customer auth | API keys, JWT, rate limiting | P0 |
| Anti-fraud v1 | Behavioral analysis, IP intelligence | P0 |
| Trust score | Multi-factor trust scoring system | P0 |
| Mainnet deploy | Deploy all contracts to Base mainnet | P0 |
| Referral program | Referral codes, 5% earnings share | P1 |
| Billing system | Customer billing and invoicing | P1 |
| Onboarding flow | Welcome, permissions, wallet setup screens | P1 |
| Customer dashboard | Usage stats, billing, API key management | P2 |
| Geographic targeting | Country/region node selection for customers | P1 |
| Sticky sessions | Same exit IP for session duration | P2 |

### Success Criteria

- [ ] 500-1000 nodes active
- [ ] First paying customer
- [ ] Anti-fraud catching >80% of simulated attacks
- [ ] Trust score accurately ranking nodes
- [ ] <1% false positive rate in fraud detection
- [ ] Mainnet claims working reliably
- [ ] Customer API latency p95 < 2 seconds

## Phase 5: Growth (Ongoing)

### Near-Term (3-6 months post-beta)

| Item | Priority | Description |
|------|----------|-------------|
| iOS app | P1 | Swift/SwiftUI native iOS client |
| Desktop app | P2 | macOS/Windows/Linux (Electron or native) |
| DEX listing | P1 | List PRXY on Uniswap (Base) |
| Staking UI | P1 | Full staking interface in app |
| Advanced anti-fraud | P1 | ML-based anomaly detection |
| Customer self-serve | P1 | Self-service signup, dashboard, billing |

### Medium-Term (6-12 months)

| Item | Priority | Description |
|------|----------|-------------|
| Compute tasks (WASM) | P2 | WebAssembly compute marketplace |
| DAO governance | P2 | On-chain voting for protocol parameters |
| Mobile SDK | P2 | SDK for other apps to integrate PRXY earning |
| Enterprise plans | P1 | Custom SLAs, dedicated nodes, higher throughput |
| CEX listing | P2 | List on centralized exchanges |

### Long-Term (12+ months)

| Item | Description |
|------|-------------|
| ML inference marketplace | Run AI models on-device for PRXY |
| Cross-chain bridges | PRXY on Ethereum mainnet, Solana |
| Protocol decentralization | Fully decentralized orchestrator |
| Node operator program | Professional node operators (not just phones) |

## Critical Path

```
Phase 1 → Phase 2 → Phase 3 → Phase 4
  ↓          ↓          ↓          ↓
Proxy     HTTPS     Contracts  Customer
Engine    + UI      + Wallet   API + Launch
(BLOCKS   (BLOCKS   (BLOCKS    (BLOCKS
EVERYTHING) PHASE 3)  PHASE 4)   REVENUE)
```

**Longest pole**: Phase 3 (token) is gated on legal opinion, which can take 4-8 weeks independently. Start legal process during Phase 1.

## Risk Register

| Risk | Probability | Impact | Mitigation | Owner |
|------|------------|--------|------------|-------|
| Play Store rejection | Medium | High | Prepare APK distribution, F-Droid | Android |
| Token classified as security | Medium | Critical | Early legal opinion, utility token design | Legal |
| Smart contract exploit | Low | Critical | Professional audit, bug bounty | Security |
| Insufficient demand (customers) | Medium | High | Validate demand with manual proxy sales first | Business |
| Android battery restrictions tighten | Medium | Medium | Active monitoring of Android dev blog, adaptive | Android |
| Competitor launches similar product | High | Medium | Speed to market, compute differentiation | Strategy |
| Fraud overwhelms anti-fraud system | Medium | High | Conservative early payouts, manual review | Backend |

## Milestones Summary

| Milestone | Target | Definition of Done |
|-----------|--------|-------------------|
| M1: PoC Works | Week 6 | 10 devices proxying HTTP for 24h |
| M2: Alpha Ready | Week 12 | 50 devices, HTTPS, UI, Play Integrity |
| M3: Token Live | Week 18 | Claim flow on testnet, audit initiated |
| M4: Beta Launch | Week 26 | Mainnet, customer API, 500+ nodes |
| M5: Revenue | Week 30 | First paying customer, positive unit economics |
| M6: 10K Nodes | Week 40 | Product-market fit validated |
