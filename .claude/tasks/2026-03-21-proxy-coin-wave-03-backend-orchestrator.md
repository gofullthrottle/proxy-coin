---
decomposed_from: .claude/plans/2026-03-21-proxy-coin-full-implementation.md
date: "2026-03-21"
project: "proxy-coin"
wave: 3
task_id: "backend-orchestrator"
created_at: "2026-03-21T00:00:00Z"
---

# Wave 03 — Backend Orchestrator Core

**Phase**: 1 — Proof of Concept
**Estimated Total**: 14h
**Dependencies**: Wave 02
**Agent**: Backend Specialist (all 6 tasks)

Internal ordering: 3.1 must precede 3.2. 3.3 and 3.4 are coupled. 3.5 can run parallel to 3.1-3.4. 3.6 needs 3.5.

---

## Tasks

### 3.1 — Backend: Node registry
- **Agent**: Backend
- **Estimate**: 3h
- **Complexity**: Standard
- **Depends on**: 2.2 (config), 2.3 (DB schema)
- **Files**: `backend/internal/node/registry.go`, `backend/internal/node/registry_test.go`

**Acceptance Criteria**:
- `internal/node/registry.go` with `NodeRegistry` struct
- Methods: `RegisterNode`, `UpdateHeartbeat`, `GetByID`, `FindByFilter`
- `NodeFilter` supports: country, region, status, min `trust_score`, network_type
- Uses pgx for database operations
- Unit tests with mock DB or testcontainers

**Technical Notes**: Node struct matches BACKEND.md schema exactly. Use pgx/v5 with connection pool.

---

### 3.2 — Backend: Node selector algorithm
- **Agent**: Backend
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 3.1
- **Files**: `backend/internal/node/selector.go`, `backend/internal/node/selector_test.go`

**Acceptance Criteria**:
- `internal/node/selector.go` with `SelectNode(req ProxyRequest)` method
- Filter → Score → Weighted random from top 5 per BACKEND.md algorithm
- Score weights: trust 0.4, load 0.3, residential 0.2, uptime 0.1
- Returns `ErrNoAvailableNodes` when no candidates
- Unit tests with various node configurations

**Technical Notes**: Weighted random selection prevents overloading single best node. Test with mock registry.

---

### 3.3 — Backend: WebSocket server
- **Agent**: Backend
- **Estimate**: 4h
- **Complexity**: Complex
- **Depends on**: 2.1 (proto), 2.2 (config)
- **Files**: `backend/internal/websocket/server.go`, integration test

**Acceptance Criteria**:
- `internal/websocket/server.go` with HTTP upgrade handler at `/ws`
- Protobuf message deserialization for all message types
- `REGISTER`/`REGISTERED` handshake flow
- `HEARTBEAT` handling (30s interval, timeout detection)
- Connection tracking in `sync.Map` (nodeID → Connection)
- Graceful shutdown with connection draining
- Integration test with WebSocket client

**Technical Notes**: Use gorilla/websocket. Each connection gets dedicated read/write goroutines. `SendCh` buffered channel for outbound messages.

---

### 3.4 — Backend: Connection pool + message routing
- **Agent**: Backend
- **Estimate**: 3h
- **Complexity**: Complex
- **Depends on**: 3.3 (must design pool alongside server)
- **Files**: `backend/internal/websocket/pool.go`, `backend/internal/websocket/connection.go`

**Acceptance Criteria**:
- `Connection` struct with `SendCh`, `ActiveReqs` map, mutex
- `Pool` manages concurrent connections, `GetConnection(nodeID)` method
- Route incoming `PROXY_RESPONSE` messages to pending request channels
- Back-pressure: reject new requests when node exceeds `max_concurrent`
- Pool-level metrics: total connections, active requests

**Technical Notes**: `ActiveReqs map[requestID]chan ProxyResponse` for response routing. Use atomic counters for metrics.

---

### 3.5 — Backend: Redis integration
- **Agent**: Backend
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 2.2 (config), 1.5 (Redis running)
- **Files**: `backend/internal/cache/redis.go`, Redis key definitions

**Acceptance Criteria**:
- Node status keys: `node:{id}:status`, `node:{id}:requests`, `node:{id}:heartbeat`
- Session stickiness: `session:{id}` → nodeID, TTL 5 min
- WebSocket mapping: `ws:{nodeID}` → server instance identifier
- `go-redis/v9` client initialized from config
- Integration test confirming read/write operations

**Technical Notes**: Define all Redis key patterns as constants. TTL values configurable.

---

### 3.6 — Backend: Metering event publisher
- **Agent**: Backend
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 3.5 (Redis stream)
- **Files**: `backend/internal/metering/reporter.go`

**Acceptance Criteria**:
- `internal/metering/reporter.go` — `PublishMeteringEvent(event MeteringEvent)` method
- Publishes to Redis Stream `metering_events`
- `MeteringEvent` contains: request_id, node_id, customer_id, bytes_in, bytes_out, latency_ms, status_code, success, timestamp
- Consumer group configuration for metering service
- Unit test confirming publish + consume round-trip

**Technical Notes**: Redis Streams (XADD/XREAD) for reliable event delivery. Metering service reads from this stream asynchronously.

---

## Phase 1 — Wave 3 Success Gate

- [ ] WebSocket server accepts and maintains connections
- [ ] Node registration and heartbeat protocol working
- [ ] Node selector chooses nodes using weighted scoring
- [ ] Redis integration functional (all key patterns)
- [ ] Metering events publishing to Redis Stream
- [ ] All unit tests passing

## Dependencies

- **Requires**: Wave 02 complete
- **Parallel with**: Wave 04 (Android Core Services) — both depend only on Wave 02
- **Unblocks**: Wave 05 (requires both Wave 03 and Wave 04)

## Technical Notes

- This wave is the most technically dense of Phase 1 — 14h total, 2 complex tasks
- Task 3.3 (WebSocket server) is the critical-path item — start here
- Task 3.4 (connection pool) should be designed alongside 3.3, not after
- The metering publisher (3.6) feeds the separate metering service — keep it fire-and-forget with no blocking
