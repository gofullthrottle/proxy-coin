# Proxy Coin — Ultra-Marathon Task Reference

**Source Plan**: `.claude/plans/2026-03-21-proxy-coin-full-implementation.md`
**Source JSON**: `.claude/plans/2026-03-21-proxy-coin-decomposed.json`
**Created**: 2026-03-21
**Total Waves**: 16 | **Total Tasks**: 96 | **Agents**: 4

---

## Epic / Wave Summary Table

| Wave | Name | Phase | Est Hours | Depends On | Status |
|------|------|-------|-----------|------------|--------|
| 01 | Project Scaffolding | 0 — Foundation | 10h | — | pending |
| 02 | Foundation Layer | 0 — Foundation | 10h | Wave 01 | pending |
| 03 | Backend Orchestrator Core | 1 — Proof of Concept | 14h | Wave 02 | pending |
| 04 | Android Core Services | 1 — Proof of Concept | 16h | Wave 02 | pending |
| 05 | Integration + Points | 1 — Proof of Concept | 12h | Waves 03, 04 | pending |
| 06 | HTTPS + Security | 2 — Alpha | 15h | Wave 05 | pending |
| 07 | Attestation + Monitoring | 2 — Alpha | 11h | Wave 05 | pending |
| 08 | Android UI | 2 — Alpha | 13h | Waves 06, 07 | pending |
| 09 | Smart Contracts | 3 — Token Launch | 11h | Wave 01 | pending |
| 10 | Contract Testing + Deploy | 3 — Token Launch | 12h | Wave 09 | pending |
| 11 | Wallet + Merkle Backend | 3 — Token Launch | 16h | Waves 08, 10 | pending |
| 12 | Claim Flow + Wallet UI | 3 — Token Launch | 12h | Wave 11 | pending |
| 13 | Customer API | 4 — Beta | 14h | Wave 08 | pending |
| 14 | Anti-Fraud + Trust | 4 — Beta | 15h | Wave 08 | pending |
| 15 | Beta Features | 4 — Beta | 14h | Waves 13, 14 | pending |
| 16 | Polish + Production | 4 — Beta | 14h | Wave 15 | pending |

**Total estimated agent time**: ~199h

---

## Phase Groupings

### Phase 0 — Foundation (Waves 1-2)
Goal: Scaffold all 4 subsystems, dev environment running, protobuf compiling.
Success: `docker-compose up` starts Postgres+Redis, `go build ./...` compiles, Gradle sync succeeds.

### Phase 1 — Proof of Concept (Waves 3-5)
Goal: Proxy HTTP traffic through Android device via WebSocket tunnel. 10 devices, 24h, >90% success rate.
Success: End-to-end proxy working, points accumulating in PostgreSQL, auto-reconnect functional.

### Phase 2 — Alpha (Waves 6-8)
Goal: HTTPS proxy, security controls, production UI. 50-100 devices, 1 week stable.
Note: Wave 9 (Smart Contracts) can start in parallel with Phase 2 — it only depends on Wave 1.

### Phase 3 — Token Launch (Waves 9-12)
Goal: Smart contracts on testnet, wallet in app, end-to-end claim flow working.
Success: All 4 contracts deployed to Base Sepolia, claim flow working, gas < $0.01.

### Phase 4 — Beta (Waves 13-16)
Goal: Customer API, anti-fraud, referrals, billing. Ready for public launch.
Success: Customer API functioning, anti-fraud catching >80% simulated attacks, production deployment working.

---

## Dependency Graph

```
Wave 01 (Scaffold)
  ├── Wave 02 (Foundation)
  │     ├── Wave 03 (Backend Core) ──┐
  │     └── Wave 04 (Android Core) ──┤
  │                                   └── Wave 05 (Integration)
  │                                         ├── Wave 06 (HTTPS+Security) ──┐
  │                                         └── Wave 07 (Attestation) ─────┤
  │                                                                          └── Wave 08 (UI)
  │                                                                                ├── Wave 13 (Customer API) ──┐
  │                                                                                └── Wave 14 (Anti-Fraud) ────┤
  │                                                                                                              └── Wave 15 (Beta)
  │                                                                                                                    └── Wave 16 (Production)
  └── Wave 09 (Contracts)
        └── Wave 10 (Contract Tests)
              └── Wave 11 (Wallet+Merkle) ← also needs Wave 08
                    └── Wave 12 (Claim Flow)
```

---

## Agent Assignment Summary

| Agent | Primary Waves | Total Tasks | Skills |
|-------|--------------|-------------|--------|
| Backend Specialist | 2-3, 5-7, 11-14, 16 | ~42 | Go, PostgreSQL, Redis, WebSocket |
| Android Specialist | 2, 4, 7-8, 11-12, 15-16 | ~35 | Kotlin, Compose, Hilt, Room, Web3j |
| Contract Specialist | 9-10 | ~12 | Solidity, Foundry, OpenZeppelin |
| Infrastructure Specialist | 1-2, 5, 16 | ~8 | Docker, Protobuf, Nginx |

---

## Marathon Session Groupings

| Session | Waves | Est Agent Hours |
|---------|-------|----------------|
| Marathon 1 | 1-5 (Phase 0 + Phase 1) | ~62h |
| Marathon 2 | 6-9 (Phase 2 + start contracts) | ~50h |
| Marathon 3 | 10-12 (Phase 3 remainder) | ~40h |
| Marathon 4 | 13-16 (Phase 4) | ~57h |

Waves 9-10 (Smart Contracts) are independent and can run as a parallel marathon session alongside Phase 2.

---

## Wave Files

| File | Wave | Tasks |
|------|------|-------|
| `2026-03-21-proxy-coin-wave-01-project-scaffolding.md` | 1 | 1.1-1.6 |
| `2026-03-21-proxy-coin-wave-02-foundation-layer.md` | 2 | 2.1-2.6 |
| `2026-03-21-proxy-coin-wave-03-backend-orchestrator.md` | 3 | 3.1-3.6 |
| `2026-03-21-proxy-coin-wave-04-android-core-services.md` | 4 | 4.1-4.6 |
| `2026-03-21-proxy-coin-wave-05-integration-points.md` | 5 | 5.1-5.6 |
| `2026-03-21-proxy-coin-wave-06-https-security.md` | 6 | 6.1-6.6 |
| `2026-03-21-proxy-coin-wave-07-attestation-monitoring.md` | 7 | 7.1-7.5 |
| `2026-03-21-proxy-coin-wave-08-android-ui.md` | 8 | 8.1-8.5 |
| `2026-03-21-proxy-coin-wave-09-smart-contracts.md` | 9 | 9.1-9.6 |
| `2026-03-21-proxy-coin-wave-10-contract-testing.md` | 10 | 10.1-10.6 |
| `2026-03-21-proxy-coin-wave-11-wallet-merkle.md` | 11 | 11.1-11.6 |
| `2026-03-21-proxy-coin-wave-12-claim-flow.md` | 12 | 12.1-12.6 |
| `2026-03-21-proxy-coin-wave-13-customer-api.md` | 13 | 13.1-13.6 |
| `2026-03-21-proxy-coin-wave-14-anti-fraud.md` | 14 | 14.1-14.6 |
| `2026-03-21-proxy-coin-wave-15-beta-features.md` | 15 | 15.1-15.6 |
| `2026-03-21-proxy-coin-wave-16-polish-production.md` | 16 | 16.1-16.6 |

---

## Quick Commands

```bash
# Navigate to project
cd /Users/johnfreier/initiative-engine/workspaces/john-freier/orgs/gofullthrottle/initiatives/proxy-coin

# Start dev environment (required before backend tasks)
docker-compose -f infrastructure/docker-compose.dev.yml up -d

# Build backend
cd backend && go build ./...

# Generate protobufs
./scripts/generate-proto.sh

# Run contract tests
cd contracts && forge test

# Run backend tests
cd backend && go test ./...
```
