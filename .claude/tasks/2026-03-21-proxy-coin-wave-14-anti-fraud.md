---
decomposed_from: .claude/plans/2026-03-21-proxy-coin-full-implementation.md
date: "2026-03-21"
project: "proxy-coin"
wave: 14
task_id: "anti-fraud"
created_at: "2026-03-21T00:00:00Z"
---

# Wave 14 — Anti-Fraud + Trust

**Phase**: 4 — Beta
**Estimated Total**: 15h
**Dependencies**: Wave 08 (metering data from Phase 2)
**Agent**: Backend Specialist (all 6 tasks)

Tasks 14.1 (fraud detector), 14.2 (IP intelligence), and 14.3 (behavioral analysis) are conceptually parallel but feed into task 14.4 (trust score). Task 14.5 (fraud logging) is a shared dependency — do it early. Task 14.6 (spot-check) is independent.

---

## Tasks

### 14.1 — Backend: Fraud detector
- **Agent**: Backend
- **Estimate**: 4h
- **Complexity**: Complex
- **Depends on**: 6.6 (server-side metering for byte comparison), 7.2 (attestation results)
- **Files**: `backend/internal/fraud/detector.go`

**Acceptance Criteria**:
- `internal/fraud/detector.go` — `Detect(nodeID) []FraudSignal` function
- Detect: emulator (via attestation failure — already covered by 7.2, wire results in)
- Detect: bandwidth inflation — server_bytes vs client_reported_bytes divergence > 10%
- Detect: Sybil attack — multiple nodes from same /24 IP subnet exceeding threshold (> 5 nodes)
- Detect: request pattern anomalies — node always proxies same customer (self-proxying signal)
- Log detected fraud signals as `FraudEvent` records (type, severity, details, node_id)
- Returns list of active fraud signals for a node

**Technical Notes**: Run fraud detection asynchronously — not in the request path. Triggered hourly by a background job. Use PostgreSQL queries aggregating metering and node data.

---

### 14.2 — Backend: IP intelligence
- **Agent**: Backend
- **Estimate**: 3h
- **Complexity**: Standard
- **Depends on**: 3.1 (node registry has IP data)
- **Files**: `backend/internal/fraud/ip_intelligence.go`

**Acceptance Criteria**:
- `internal/fraud/ip_intelligence.go` — classify IP as: residential, datacenter, VPN, proxy
- Use MaxMind GeoLite2 database (ASN + City) for classification
- ASN analysis: known datacenter ASNs (AWS, GCP, Azure, etc.) → datacenter classification
- Datacenter and VPN IPs: earn 0x (earnings multiplier)
- Cross-reference: claimed country from node registration vs IP geolocation — flag > 500km mismatch
- Cache IP classifications in Redis (TTL 24h) to avoid repeated DB lookups

**Technical Notes**: MaxMind GeoLite2 is free with registration. Use `oschwald/geoip2-golang`. ASN list of known datacenter ranges maintained in config or DB. Residential IP detection is heuristic — use ASN + IP range analysis.

---

### 14.3 — Backend: Behavioral analysis
- **Agent**: Backend
- **Estimate**: 3h
- **Complexity**: Standard
- **Depends on**: 3.6 (metering events), 2.3 (DB with metering history)
- **Files**: `backend/internal/fraud/behavioral.go`

**Acceptance Criteria**:
- Detect sudden bandwidth spikes: current hour > 10x 7-day average → flag
- Detect no sleep cycles: node active 24/7 for > 7 days with < 2h daily downtime → emulator signal
- Detect correlated nodes: multiple nodes from same IP range with identical traffic patterns → Sybil flag
- New node ramp-up: first 7 days of operation, earnings multiplier = 0.5x (trust building period)
- Output: list of behavioral signals per node, passed to trust score calculator

**Technical Notes**: Run against metering history in PostgreSQL. Use window functions for 7-day rolling averages. Sleep cycle detection: count hours with 0 requests per day.

---

### 14.4 — Backend: Trust score calculator
- **Agent**: Backend
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 14.1 (fraud signals), 14.2 (IP quality), 14.3 (behavioral), 7.2 (attestation)
- **Files**: `backend/internal/fraud/trust_score.go`

**Acceptance Criteria**:
- Per SECURITY.md trust model: weighted sum of components
- Component weights: device_attestation 0.15, uptime_consistency 0.20, verification_pass_rate 0.25, ip_quality 0.15, bandwidth_consistency 0.15, account_age 0.10
- Score range: 0.0 to 1.0
- Earnings multiplier: linear map from 0.3→0.5x, 1.0→2.0x
- Auto-actions: below 0.3 → suspend earnings, below 0.1 → ban node
- `UpdateTrustScore(nodeID)` recalculates and stores in `nodes` table
- Runs hourly, triggered after fraud detector

**Technical Notes**: Store component scores separately for auditability. Manual override capability (admin API) for edge cases.

---

### 14.5 — Backend: Fraud event logging
- **Agent**: Backend
- **Estimate**: 1h
- **Complexity**: Simple
- **Depends on**: 2.3 (fraud_events table)
- **Files**: `backend/internal/fraud/logger.go`

**Acceptance Criteria**:
- Store fraud events in `fraud_events` table (nodeID, type, severity, details JSON, timestamp)
- Severity levels: WARNING, CRITICAL, BAN
- Auto-actions on severity: WARNING → log only, CRITICAL → suspend + notify admin, BAN → disable node
- `GET /admin/fraud/events` — paginated list with filters (node, severity, date range)
- `GET /admin/fraud/queue` — pending manual review cases (CRITICAL without auto-action)

**Technical Notes**: Fraud events are immutable — never delete. Mark as reviewed/actioned, not deleted. Admin API is internal only (not customer-facing).

---

### 14.6 — Backend: Spot-check verification
- **Agent**: Backend
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 5.1 (proxy handler — spot-checks use same path), 3.1 (node registry)
- **Files**: `backend/internal/fraud/spot_check.go`

**Acceptance Criteria**:
- Periodic job: sends known URLs to random nodes and verifies response integrity
- Known URLs: list of stable test URLs with known response bodies (e.g., static files with known SHA-256)
- Compare response hash against expected — mismatch → fraud event (type: RESPONSE_TAMPERING)
- Track `verification_pass_rate` per node (rolling 30-day window)
- Frequency: 1 spot-check per node per hour on average (randomized timing)
- Failed spot-checks reduce trust score (via verification_pass_rate component)

**Technical Notes**: Use a small curated set of test URLs (10-20) that return deterministic responses. Store expected response hashes in config. Send spot-check requests through the normal proxy pipeline (indistinguishable from real customer traffic to the node).

---

## Phase 4 — Wave 14 Success Gate

- [ ] Fraud detector identifies bandwidth inflation with > 90% accuracy on synthetic tests
- [ ] IP intelligence correctly classifies datacenter IPs (verified against known AWS/GCP ranges)
- [ ] Behavioral analysis flags 24/7 uptime pattern
- [ ] Trust score updates hourly with correct weights
- [ ] Fraud events logged and admin review queue functional
- [ ] Spot-check verification catching response tampering

## Dependencies

- **Requires**: Wave 08 (Phase 2 proxy pipeline, metering data)
- **Parallel with**: Wave 13 (Customer API) — both depend on Wave 08
- **Unblocks**: Wave 15 (Beta Features) — requires both Wave 13 and Wave 14

## Technical Notes

- Anti-fraud is an adversarial system — assume sophisticated attackers will probe for weaknesses
- The spot-check mechanism (14.6) should be indistinguishable from real proxy traffic to prevent nodes from selectively behaving
- Trust score (14.4) is the central anti-fraud output — it gates earnings and should be the most carefully tested component
- Consider rate-limiting how fast a new node can earn to limit the damage from any single fraudulent node
