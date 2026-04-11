---
decomposed_from: .claude/plans/2026-03-21-proxy-coin-full-implementation.md
date: "2026-03-21"
project: "proxy-coin"
wave: 5
task_id: "integration-points"
created_at: "2026-03-21T00:00:00Z"
---

# Wave 05 — Integration + Points

**Phase**: 1 — Proof of Concept
**Estimated Total**: 12h
**Dependencies**: Waves 03 and 04 (both must complete)
**Agent Mix**: Backend (4 tasks), Android (1 task), Infrastructure (1 task)

This wave wires together the backend and Android components into a working end-to-end flow. Tasks 5.1 and 5.2 are coupled (request + response handlers). Tasks 5.3, 5.4, 5.5, 5.6 can run in parallel after 5.1-5.2.

---

## Tasks

### 5.1 — Backend: Proxy request handler
- **Agent**: Backend
- **Estimate**: 3h
- **Complexity**: Standard
- **Depends on**: 3.2 (NodeSelector), 3.4 (connection pool)
- **Files**: `backend/internal/proxy/handler.go`

**Acceptance Criteria**:
- `internal/proxy/handler.go` — `HandleProxyRequest(req)` function
- Calls `NodeSelector.SelectNode(req)` to pick a node
- Sends `PROXY_REQUEST` via WebSocket to selected node
- Waits for response with configurable timeout (default 30s)
- Returns assembled response to caller
- Handles: node unavailable (`ErrNoAvailableNodes`), timeout, node error response

**Technical Notes**: Uses a request channel from the connection pool. On timeout, sends cancellation message to node. Log all proxy requests with latency.

---

### 5.2 — Backend: Proxy response handler
- **Agent**: Backend
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 5.1 (shares request/response lifecycle)
- **Files**: `backend/internal/proxy/router.go`

**Acceptance Criteria**:
- `internal/proxy/router.go` — assembles streamed `ProxyResponseChunk` messages into complete response
- Forwards assembled response to waiting request handler via channel
- Records `MeteringEvent` on completion (bytes, latency, status)
- Handles: partial responses, out-of-order chunks (via sequence numbers), connection drop mid-stream

**Technical Notes**: Buffer chunks by request_id. Use `ProxyResponseEnd` as signal to flush and complete. Out-of-order tolerance: buffer up to 10 chunks, then fail.

---

### 5.3 — Backend: Points ledger
- **Agent**: Backend
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 2.3 (DB schema has earnings table)
- **Files**: `backend/internal/reward/ledger.go`, `backend/internal/api/earnings_handler.go`

**Acceptance Criteria**:
- PostgreSQL-based points tracking (no blockchain yet)
- Insert earnings records per node per hour
- `GET /v1/earnings` — returns earnings for a node (requires node auth)
- `GET /v1/earnings/summary` — returns totals (today, week, all-time, pending)
- Points accumulate but are not yet claimable on-chain

**Technical Notes**: Earnings are in PRXY micro-units (1e-6). Store as integers to avoid floating point. Conversion to USD via configurable rate.

---

### 5.4 — Android: Basic settings
- **Agent**: Android
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 4.1 (service reads settings), 2.4 (settings screen stub)
- **Files**: `android/app/src/main/.../ui/settings/SettingsScreen.kt`, DataStore preferences

**Acceptance Criteria**:
- Settings screen with: WiFi-only toggle (default on), on/off switch for proxy service
- Current bandwidth usage display (today's bytes shared)
- Settings persisted in `DataStore<Preferences>`
- `ProxyForegroundService` reads settings reactively via `Flow`
- Toggle immediately starts/stops service

**Technical Notes**: Use `DataStore` (not SharedPreferences). Expose settings as `Flow<AppSettings>` from a `SettingsRepository`.

---

### 5.5 — Integration: End-to-end test harness
- **Agent**: Backend
- **Estimate**: 3h
- **Complexity**: Standard
- **Depends on**: 5.1, 5.2 (full proxy pipeline needed)
- **Files**: `scripts/test-proxy.sh` and/or `backend/tests/e2e/proxy_test.go`

**Acceptance Criteria**:
- Test harness starts all backend services (or connects to running stack)
- Simulates a proxy request to a known URL (e.g., `httpbin.org/get`)
- Verifies response matches direct request (status code, body structure)
- Tests with 10 concurrent requests
- Measures latency overhead (target: < 500ms additional vs direct)
- Reports success rate — must be > 90%
- Output is human-readable pass/fail summary

**Technical Notes**: Use a Go integration test with `testing.T` or a bash script that curls the orchestrator API. Requires a mock Android node or a real Android device running the app.

---

### 5.6 — Infrastructure: Full dev Docker Compose
- **Agent**: Infrastructure
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 1.5 (base compose), all backend tasks (need service configs)
- **Files**: `infrastructure/docker-compose.yml`

**Acceptance Criteria**:
- `infrastructure/docker-compose.yml` with all 3 backend services + Postgres + Redis
- Services: `orchestrator`, `api`, `metering`, `postgres`, `redis`
- Volume mounts for code hot-reload (Go binary rebuilt on change)
- Health checks for all services
- Port mapping: orchestrator WS on 8080, API on 8081, metering on 8082
- Environment variables configured via `.env` file
- Single `docker-compose up` starts entire stack

**Technical Notes**: Use multi-stage Dockerfile for Go services. `depends_on` with health checks so services wait for DB.

---

## Phase 1 Success Gate

- [ ] Backend WebSocket server accepts connections from Android client
- [ ] Android app proxies HTTP requests through device successfully
- [ ] End-to-end test harness passes with > 90% success rate
- [ ] Points accumulate in PostgreSQL per node
- [ ] Auto-reconnect works after network switch
- [ ] Docker Compose starts full stack in one command
- [ ] Settings screen toggles proxy service on/off

## Dependencies

- **Requires**: Wave 03 (Backend Orchestrator) AND Wave 04 (Android Core) — both complete
- **Unblocks**: Waves 06 and 07 (Phase 2, can run in parallel)

## Technical Notes

- This wave is the "Phase 1 milestone" — completing it means a working end-to-end proxy
- The test harness (5.5) requires either a real Android device running the Wave 4 services, or a simulated node
- Consider writing a mock Android node in Go for testing purposes — it can simulate WebSocket registration and proxy execution
- Points ledger (5.3) is intentionally simple — full blockchain integration comes in Phase 3
