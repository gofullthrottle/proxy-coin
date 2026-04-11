---
decomposed_from: .claude/plans/2026-03-21-proxy-coin-full-implementation.md
date: "2026-03-21"
project: "proxy-coin"
wave: 16
task_id: "polish-production"
created_at: "2026-03-21T00:00:00Z"
---

# Wave 16 — Polish + Production

**Phase**: 4 — Beta
**Estimated Total**: 14h
**Dependencies**: Wave 15
**Agent Mix**: Android (2 tasks), Backend (2 tasks), Infrastructure (2 tasks)

Tasks 16.1, 16.2 (Android), 16.3 (API Gateway), and 16.4 (Production Deploy) are all parallel. Task 16.5 (Docs) can start at any point. Task 16.6 (Full E2E) must come last — it validates everything.

---

## Tasks

### 16.1 — Android: Security hardening
- **Agent**: Android
- **Estimate**: 3h
- **Complexity**: Standard
- **Depends on**: 11.2 (KeystoreHelper), 4.2 (WebSocket — cert pinning target)
- **Files**: `android/app/src/main/.../security/SecurityManager.kt`, `OkHttpClientProvider.kt` update

**Acceptance Criteria**:
- Certificate pinning via `OkHttp CertificatePinner` for backend domain
- Root detection: check for `su` binary, Magisk mount, `test-keys` in build tags
- Emulator detection: check build properties (`FINGERPRINT`, `PRODUCT`, telephony)
- APK signature verification at runtime: compare signing cert hash against expected
- Memory zeroing: zero-fill `ByteArray` and `CharArray` holding sensitive data after use
- On detection: suspend proxy operation, log to backend (but don't crash app — report gracefully)

**Technical Notes**: Certificate pins should be stored as SHA-256 of the public key (`sha256/...`). Include 2 backup pins in case of cert rotation. Root detection is best-effort — not a hard blocker (report as low trust signal). Never call `System.exit()`.

---

### 16.2 — Android: Release build config
- **Agent**: Android
- **Estimate**: 1h
- **Complexity**: Simple
- **Depends on**: 16.1 (security hardening should be in place before release build)
- **Files**: `android/app/build.gradle.kts`, `android/app/proguard-rules.pro`

**Acceptance Criteria**:
- ProGuard/R8 rules: aggressive obfuscation enabled (`minifyEnabled true`, `shrinkResources true`)
- Strip all `Log.*` calls in release build (via ProGuard rule)
- Signing config: keystore path + alias from environment variables (not hardcoded)
- Version name: `1.0.0-beta`, version code: 1
- Release build type with `debuggable false`, `allowBackup false`
- `./gradlew assembleRelease` succeeds and produces signed APK

**Technical Notes**: ProGuard rules for Room, Hilt, Retrofit, Web3j, OkHttp — these need keep rules or they'll break. Test the release build on a physical device before marking complete.

---

### 16.3 — Backend: API Gateway config
- **Agent**: Backend
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: all backend services (gates all inbound traffic)
- **Files**: `infrastructure/nginx.conf` or `infrastructure/Caddyfile`

**Acceptance Criteria**:
- TLS termination (Let's Encrypt or provided certificate)
- Rate limiting at gateway level: 100 req/s per IP burst, 1000 req/s global
- CORS: strict origin allowlist (no wildcard `*`)
- Request size limits: 1MB for proxy API, 10MB for batch
- WebSocket upgrade support at `/ws` path
- Health check endpoints: `GET /health` → 200 OK
- Compression (gzip) for API responses
- Security headers: `X-Content-Type-Options`, `X-Frame-Options`, `Strict-Transport-Security`

**Technical Notes**: Use Caddy if simplicity is preferred (automatic HTTPS). Use Nginx if more control needed. Either is fine — document choice in `infrastructure/README.md`.

---

### 16.4 — Infrastructure: Production deployment
- **Agent**: Infrastructure
- **Estimate**: 3h
- **Complexity**: Standard
- **Depends on**: 16.3 (API gateway), all services
- **Files**: `infrastructure/docker-compose.prod.yml` and/or `infrastructure/k8s/`

**Acceptance Criteria**:
- Docker Compose production config for initial deployment
- Separate configs for each service: `orchestrator`, `api`, `metering`
- Resource limits (CPU/memory) per service
- Health checks with restart policies (`restart: unless-stopped`)
- Log aggregation: stdout/stderr to centralized logging (ELK stack compatible)
- Environment variables via Docker secrets or `.env.prod` (not in compose file)
- Single command to start full production stack: `docker-compose -f docker-compose.prod.yml up -d`

**Technical Notes**: Start with Docker Compose for beta (simpler ops). K8s manifests are stretch goal. Use `--scale orchestrator=3` for horizontal scaling. Postgres should have daily backups configured.

---

### 16.5 — Docs: API reference + legal
- **Agent**: Infrastructure
- **Estimate**: 2h
- **Complexity**: Simple
- **Depends on**: Wave 13 (Customer API must be complete to document)
- **Files**: `docs/api-reference.md`, `docs/legal/terms-of-service.md`, `docs/legal/privacy-policy.md`, `docs/legal/acceptable-use.md`

**Acceptance Criteria**:
- `docs/api-reference.md` — all Customer API endpoints with: method, path, auth, request schema, response schema, error codes, example curl
- `docs/legal/terms-of-service.md` — skeleton document covering: service description, user obligations, node operator obligations, earnings, termination
- `docs/legal/privacy-policy.md` — skeleton covering: data collected (IP, device info, usage metrics), retention, sharing, GDPR/CCPA notes
- `docs/legal/acceptable-use.md` — prohibited use cases matching SECURITY.md blocklist categories
- All documents clearly marked "DRAFT - Not Legal Advice" pending legal review

**Technical Notes**: Legal documents are skeletons — they establish structure for future legal review, not final documents. API reference can be generated from Go struct comments using `swaggo/swag` if desired.

---

### 16.6 — Integration: Full system E2E tests
- **Agent**: Backend
- **Estimate**: 3h
- **Complexity**: Standard
- **Depends on**: everything — this is the final validation
- **Files**: `backend/tests/e2e/full_system_test.go`

**Acceptance Criteria**:
- Full cycle test: customer registers → gets API key → submits proxy request → node receives and executes → response returned → metering recorded → rewards calculated → Merkle generated → claim on testnet succeeds
- Test fraud detection: inject synthetic fraud signals, verify trust score decreases and earnings suspend
- Test rate limiting: exceed daily limit, verify 429 response with correct `Retry-After`
- Test geo-targeting: request US IP, verify node in US selected
- Test sticky sessions: 5 sequential requests with same `session_id`, verify same node used
- All tests pass in < 10 minutes total (use mocks/stubs for slow external services)

**Technical Notes**: Use `testcontainers-go` for Postgres and Redis. Mock Google Play Integrity API and Basescan API. Use a local Anvil fork for blockchain operations (fast, deterministic). Document how to run this test suite in `docs/testing.md`.

---

## Phase 4 Final Success Gate

- [ ] Customer API functioning with auth + rate limiting
- [ ] Anti-fraud catching > 80% of simulated attacks
- [ ] Trust score accurately ranking nodes (verified manually with known inputs)
- [ ] Referral system working end-to-end
- [ ] Customer API latency p95 < 2 seconds
- [ ] All security hardening measures active
- [ ] Production deployment config working (single-command start)
- [ ] Release APK builds and runs on physical device
- [ ] Full E2E test suite passing
- [ ] Legal docs drafted and API reference complete

## Dependencies

- **Requires**: Wave 15 (Beta Features) complete
- **Unblocks**: Public beta launch

## Technical Notes

- This wave is the launch gate — everything must work end-to-end before public launch
- The full E2E test (16.6) should be run against a staging environment (not production) before flipping DNS
- Release build (16.2) should be tested on multiple devices: budget (API 26), mid-range (API 31), flagship (API 35)
- Legal documents (16.5) must be reviewed by actual legal counsel before public launch — the skeletons here are a starting point only
- Certificate pinning (16.1) requires updating when backend certificates rotate — document this operational procedure
