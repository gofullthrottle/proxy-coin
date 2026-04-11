---
decomposed_from: .claude/plans/2026-03-21-proxy-coin-full-implementation.md
date: "2026-03-21"
project: "proxy-coin"
wave: 13
task_id: "customer-api"
created_at: "2026-03-21T00:00:00Z"
---

# Wave 13 — Customer API

**Phase**: 4 — Beta
**Estimated Total**: 14h
**Dependencies**: Wave 08 (Phase 2 proxy pipeline complete)
**Agent**: Backend Specialist (all 6 tasks)

Tasks 13.1 and 13.2 are sequential (auth infrastructure before endpoints). Tasks 13.3 and 13.4 (proxy endpoints) require 13.1. Tasks 13.5 and 13.6 (usage + rate limiting) can run in parallel with 13.3-13.4.

---

## Tasks

### 13.1 — Backend: Customer auth
- **Agent**: Backend
- **Estimate**: 3h
- **Complexity**: Standard
- **Depends on**: 2.3 (customers table in DB), 2.2 (config for JWT secret)
- **Files**: `backend/internal/auth/customer_auth.go`

**Acceptance Criteria**:
- Customer registration: email + bcrypt password (cost factor 12)
- JWT token generation: 15-minute expiry, `sub` = customer ID
- Refresh tokens: 7-day expiry, stored in DB, rotated on use
- API key generation: format `prxy_live_{40 random hex chars}`, stored as bcrypt hash in DB
- API key rotation: `DELETE /v1/auth/apikey` + `POST /v1/auth/apikey`
- Auth middleware for both JWT and API key authentication

**Technical Notes**: Use `golang-jwt/jwt/v5`. `bcrypt.GenerateFromPassword` for passwords and API key hashes. Rate limit login attempts at middleware level (see 13.6). API keys are shown only once on creation.

---

### 13.2 — Backend: Customer registration + login
- **Agent**: Backend
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 13.1 (auth infrastructure)
- **Files**: `backend/internal/api/auth_handler.go`

**Acceptance Criteria**:
- `POST /v1/auth/register` — email + password → create customer, return JWT + refresh token
- `POST /v1/auth/login` — email + password → verify bcrypt, return JWT + refresh token
- `POST /v1/auth/refresh` — refresh token → new JWT
- `POST /v1/auth/apikey` — authenticated, return newly generated API key (shown once)
- `DELETE /v1/auth/apikey` — revoke current API key
- Input validation: email format, password min 12 chars
- Rate limiting: 5 login attempts per IP per minute (Redis-based)

**Technical Notes**: Email verification is out of scope for beta — just validate format. Store refresh tokens in `customer_refresh_tokens` table (or add to customers table).

---

### 13.3 — Backend: Proxy request endpoint
- **Agent**: Backend
- **Estimate**: 3h
- **Complexity**: Standard
- **Depends on**: 13.1 (auth), 5.1 (proxy handler)
- **Files**: `backend/internal/api/proxy_handler.go`

**Acceptance Criteria**:
- `POST /v1/proxy` — per BACKEND.md specification
- Auth: API key or JWT Bearer token
- Request body: `{ url, method, headers, body, geo: { country, region }, timeout_ms, session_id }`
- Validate: URL is HTTP/HTTPS, method is allowed, timeout within limits (max 60s)
- Pass to Orchestrator's proxy handler, wait for response
- Response: `{ status_code, headers, body_b64, node_id, latency_ms, bytes_out }`
- Error responses: 408 timeout, 503 no nodes available, 429 rate limited, 422 invalid request

**Technical Notes**: Body is base64 encoded in response to handle binary content. `session_id` passed to node selector for sticky sessions. Log all proxy requests with customer_id, node_id, latency, status.

---

### 13.4 — Backend: Batch proxy endpoint
- **Agent**: Backend
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 13.3 (single proxy endpoint logic)
- **Files**: `backend/internal/api/proxy_handler.go` (addition)

**Acceptance Criteria**:
- `POST /v1/proxy/batch` — accept JSON array of proxy requests (max 10)
- Execute concurrently using goroutines + WaitGroup
- Return array of responses in same order as requests
- Individual request failures don't cancel others — include error in that slot's response
- Aggregate metrics: total_requests, successful, failed, avg_latency_ms
- Same auth and validation as single endpoint

**Technical Notes**: Use `errgroup.Group` for concurrent execution with context. Cap at 10 parallel requests — return 422 if array exceeds limit.

---

### 13.5 — Backend: Usage tracking
- **Agent**: Backend
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 2.3 (customer_usage table), 13.1 (customer auth)
- **Files**: `backend/internal/customer/usage.go`, `backend/internal/api/usage_handler.go`

**Acceptance Criteria**:
- `internal/customer/usage.go` — `RecordUsage(customerID, requests, bytesOut int)` called after each proxy request
- Upsert into `customer_usage` table: daily record per customer
- `GET /v1/usage` — returns current period usage (requests, bytes, cost_usd, plan_limit)
- `GET /v1/usage/daily` — returns daily breakdown for past 30 days
- Enforce plan limits: reject requests when daily limit exceeded
- Cost calculation: requests × $0.001 + bytes × $0.0001/MB (configurable)

**Technical Notes**: Use PostgreSQL `INSERT ... ON CONFLICT DO UPDATE` for upsert. Check limit before proxy execution (not after). Daily usage resets at UTC midnight.

---

### 13.6 — Backend: Rate limiting
- **Agent**: Backend
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 3.5 (Redis client), 13.1 (customer auth)
- **Files**: `backend/internal/auth/ratelimit.go`

**Acceptance Criteria**:
- Redis-based rate limiting per API key per day
- Plan limits: Free 100 req/day, Starter 10K/day, Pro 100K/day, Enterprise: custom
- Return `429 Too Many Requests` with `Retry-After` header (seconds until reset)
- Redis key: `ratelimit:{apiKey}:{YYYY-MM-DD}`, TTL set to midnight
- Burst allowance: up to 5 requests per second per key (INCR + expire sliding window)
- Login rate limit: 5 attempts per IP per minute (separate Redis key)

**Technical Notes**: Use `INCR` + check against limit (not Lua scripts for simplicity). Two separate limiters: daily quota and burst rate. Middleware wraps all `/v1/proxy` routes.

---

## Phase 4 — Wave 13 Success Gate

- [ ] Customer can register, log in, get JWT and API key
- [ ] `POST /v1/proxy` returns proxied response with customer auth
- [ ] `POST /v1/proxy/batch` processes up to 10 concurrent requests
- [ ] Usage tracked per customer per day in PostgreSQL
- [ ] Rate limiting enforces plan limits with correct 429 response
- [ ] API latency p95 < 2 seconds for single proxy request

## Dependencies

- **Requires**: Wave 08 (Phase 2 proxy pipeline) — specifically the working proxy flow from Waves 3-5
- **Parallel with**: Wave 14 (Anti-Fraud) — both depend on Wave 08
- **Unblocks**: Wave 15 (Beta Features, requires both Wave 13 and Wave 14)

## Technical Notes

- The Customer API is the revenue-generating component — rate limiting and usage tracking must be accurate
- API keys should be generated with sufficient entropy to be unguessable (40 hex chars = 160 bits)
- Consider adding Swagger/OpenAPI documentation as part of this wave (feeds Wave 16.5)
- Usage tracking must be consistent even if the backend crashes mid-request — consider recording at the start of billing (optimistic) vs end (conservative)
