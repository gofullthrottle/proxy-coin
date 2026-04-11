---
decomposed_from: .claude/plans/2026-03-21-proxy-coin-full-implementation.md
date: "2026-03-21"
project: "proxy-coin"
wave: 6
task_id: "https-security"
created_at: "2026-03-21T00:00:00Z"
---

# Wave 06 — HTTPS + Security

**Phase**: 2 — Alpha
**Estimated Total**: 15h
**Dependencies**: Wave 05
**Agent Mix**: Backend (4 tasks), Android (2 tasks)

Backend tasks 6.1, 6.2, 6.5 can run in parallel. Task 6.3 (blocklist distribution) depends on 6.2 (blocklist management). Android tasks 6.4 and 6.5 are coupled and should be done together.

---

## Tasks

### 6.1 — Backend: HTTPS CONNECT tunnel
- **Agent**: Backend
- **Estimate**: 4h
- **Complexity**: Complex
- **Depends on**: 5.1, 5.2 (base proxy pipeline)
- **Files**: `backend/internal/proxy/https_handler.go`

**Acceptance Criteria**:
- Orchestrator handles `CONNECT` method proxy requests
- Establish TCP tunnel through WebSocket to Android node
- Node opens TLS connection to destination, relays encrypted bytes bidirectionally
- No MITM — backend and node never see plaintext HTTPS content
- Proper cleanup: close tunnel on disconnect, timeout (default 60s idle), size limit
- Integration test: verify HTTPS site accessible through tunnel

**Technical Notes**: `CONNECT` establishes a raw TCP tunnel, not HTTP. Backend sends `CONNECT_REQUEST` proto message to node. Node upgrades to raw byte relay mode. Use separate proto message type for tunnel data chunks.

---

### 6.2 — Backend: Domain blocklist management
- **Agent**: Backend
- **Estimate**: 3h
- **Complexity**: Standard
- **Depends on**: 2.3 (DB for blocklist storage), 2.2 (config)
- **Files**: `backend/internal/proxy/filter.go`, `backend/internal/api/blocklist_handler.go`

**Acceptance Criteria**:
- `internal/proxy/filter.go` — Load blocklist from config/database
- Hash-based matching: SHA-256 of normalized domain
- Categories per SECURITY.md: CSAM, malware C2, gov/mil, financial, healthcare
- Rate-based blocking: > 100 req/min to one domain from one node
- Admin API to add/remove blocklist entries: `POST /admin/blocklist`, `DELETE /admin/blocklist/{domain}`
- Blocklist version hash for change detection

**Technical Notes**: Store blocklist in PostgreSQL (for admin management) and cache in Redis (for fast lookup). Use bloom filter for high-speed membership check before hash lookup.

---

### 6.3 — Backend: Blocklist distribution
- **Agent**: Backend
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 6.2 (blocklist must exist), 3.4 (WebSocket pool for distribution)
- **Files**: `backend/internal/node/config_pusher.go`

**Acceptance Criteria**:
- On node `REGISTER`: send current blocklist hash set to node via `CONFIG_UPDATE`
- Periodic update: every 1 hour, push updated blocklist to all connected nodes
- Include blocklist version hash — nodes skip download if version matches
- Redis pub/sub for broadcasting updates across multiple orchestrator instances
- Nodes acknowledge receipt

**Technical Notes**: Use Redis pub/sub channel `config_updates` for multi-instance orchestrator coordination. Blocklist is sent as compact hash set (not full domain list).

---

### 6.4 — Android: HTTPS CONNECT in ProxyEngine
- **Agent**: Android
- **Estimate**: 3h
- **Complexity**: Standard
- **Depends on**: 4.3 (ProxyEngine base), 6.1 (backend must support CONNECT)
- **Files**: `android/app/src/main/.../proxy/ProxyEngine.kt` (extension)

**Acceptance Criteria**:
- Handle `CONNECT_REQUEST` proto messages separately from HTTP proxy requests
- Open raw TCP socket to destination (not OkHttp — raw socket)
- Relay bytes bidirectionally through WebSocket tunnel
- Proper cleanup: close socket on tunnel close message, idle timeout, max bytes
- Integration test: HTTPS site reachable through full tunnel

**Technical Notes**: Use `java.net.Socket` for raw TCP. Run relay on two coroutines (read-from-socket → write-to-ws, read-from-ws → write-to-socket). Signal tunnel close to backend on socket EOF or error.

---

### 6.5 — Android: Domain filter
- **Agent**: Android
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 4.3 (ProxyEngine checks filter before executing)
- **Files**: `android/app/src/main/.../proxy/DomainFilter.kt`

**Acceptance Criteria**:
- `DomainFilter` class — receives blocklist hash set from backend via `CONFIG_UPDATE`
- Before executing any proxy request, hash destination domain (SHA-256, normalized)
- Check against in-memory hash set — reject blocked domains with `403 Forbidden` response
- Update blocklist on `CONFIG_UPDATE` messages atomically (swap reference, not modify)
- Log all blocked domain attempts (domain hash only, not full domain)

**Technical Notes**: Store hash set as `Set<String>` in a `@Volatile` reference for lock-free reads. Domain normalization: lowercase, strip port, strip `www.` prefix.

---

### 6.6 — Backend: Server-side metering
- **Agent**: Backend
- **Estimate**: 3h
- **Complexity**: Standard
- **Depends on**: 3.4 (connection pool where bytes flow), 3.6 (metering publisher)
- **Files**: `backend/internal/metering/counter.go`

**Acceptance Criteria**:
- `internal/metering/counter.go` — count bytes flowing through WebSocket in real-time
- Intercept WebSocket message reads/writes to count payload bytes
- Compare with client-reported metrics — flag > 10% divergence
- Feed authoritative server-measured bytes into `MeteringEvent`
- Metrics exposed: bytes_in_total, bytes_out_total per node per session

**Technical Notes**: Wrap the gorilla/websocket connection in a byte-counting wrapper. Use atomic int64 counters (no mutex needed for accumulation). Reset per-session counters on reconnect.

---

## Phase 2 — Wave 6 Success Gate

- [ ] HTTPS proxy working through CONNECT tunnel (test with `curl --proxy` against HTTPS site)
- [ ] Domain blocklist blocking 100% of test blocked domains
- [ ] Blocklist distributed to nodes on registration and update
- [ ] Server-side metering counting bytes within 5% of actual

## Dependencies

- **Requires**: Wave 05 complete
- **Parallel with**: Wave 07 (Attestation + Monitoring) — both depend on Wave 05
- **Unblocks**: Wave 08 (Android UI) — requires both Wave 06 and Wave 07

## Technical Notes

- HTTPS CONNECT tunnel (6.1 + 6.4) is the most complex pair in this wave — the no-MITM requirement means the backend cannot inspect HTTPS content, which requires a raw byte relay architecture
- Blocklist must be enforced on the Android node side (6.5) not just server side — malicious nodes could bypass server-only filtering
- Server-side metering (6.6) is the anti-fraud anchor — client-reported bytes are advisory only
