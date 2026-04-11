---
decomposed_from: .claude/plans/2026-03-21-proxy-coin-full-implementation.md
date: "2026-03-21"
project: "proxy-coin"
wave: 7
task_id: "attestation-monitoring"
created_at: "2026-03-21T00:00:00Z"
---

# Wave 07 — Attestation + Monitoring

**Phase**: 2 — Alpha
**Estimated Total**: 11h
**Dependencies**: Wave 05
**Agent Mix**: Android (3 tasks), Backend (2 tasks)

All 5 tasks can run in parallel. Tasks 7.1 (Play Integrity) and 7.2 (backend verification) are coupled and should coordinate on the API contract. Tasks 7.3, 7.4, 7.5 are fully independent.

---

## Tasks

### 7.1 — Android: Play Integrity integration
- **Agent**: Android
- **Estimate**: 3h
- **Complexity**: Standard
- **Depends on**: 4.2 (WebSocket client, registration flow)
- **Files**: `android/app/src/main/.../security/IntegrityChecker.kt`

**Acceptance Criteria**:
- Integrate Google Play Integrity API (`com.google.android.play:integrity`)
- Generate integrity token on node registration
- Send token to backend via `POST /v1/node/attest`
- Periodic re-attestation at configurable interval (default 24h)
- Handle API errors gracefully: retry with backoff, continue operation if attestation service unavailable
- Attestation token included in `REGISTER` message or sent separately

**Technical Notes**: Use `IntegrityManagerFactory.create()`. `StandardIntegrityManager` for repeated checks (caches token). Token is opaque JWT — do not parse client-side. Pass to backend for Google API verification.

---

### 7.2 — Backend: Attestation verification
- **Agent**: Backend
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 3.1 (node registry for status updates)
- **Files**: `backend/internal/fraud/attestation.go`

**Acceptance Criteria**:
- `internal/fraud/attestation.go` — verify Play Integrity token via Google's API
- Check verdicts: `MEETS_DEVICE_INTEGRITY`, `MEETS_BASIC_INTEGRITY`, genuine APK signature
- On failure: suspend node earnings (update trust_score), log `fraud_event`, require re-attestation
- Periodic re-check schedule (not just on registration)
- `POST /v1/node/attest` endpoint — accepts token, returns verification result

**Technical Notes**: Call `https://playintegrity.googleapis.com/v1/{package_name}:decodeIntegrityToken`. Requires Google Cloud project with Play Integrity API enabled. Store service account key in config.

---

### 7.3 — Android: ResourceMonitor enhancements
- **Agent**: Android
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 4.4 (base ResourceMonitor)
- **Files**: `android/app/src/main/.../monitor/ResourceMonitor.kt` (enhancement)

**Acceptance Criteria**:
- CPU temperature monitoring via Thermal API (Android 11+, API 30+)
- Network speed estimation (measure via periodic small test download)
- Memory pressure detection (`ActivityManager.MemoryInfo.lowMemory`)
- Enhanced `ResourceState` with: battery_level, is_charging, network_type, cpu_temp_celsius, available_memory_mb, is_low_memory
- More granular pause/resume logic: pause on thermal throttling, pause on low memory

**Technical Notes**: `PowerManager.ThermalStatusCallback` for thermal events. Degrade gracefully on older Android versions (API < 30) by omitting temperature. Network speed: measure over 5s window.

---

### 7.4 — Backend: Node config push
- **Agent**: Backend
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 3.4 (WebSocket pool), 6.3 (config push infrastructure)
- **Files**: `backend/internal/node/config.go`

**Acceptance Criteria**:
- `internal/node/config.go` — `PushConfig(nodeID, config NodeConfig)` method
- Sends `CONFIG_UPDATE` proto message via WebSocket to specified node
- `NodeConfig` fields: earning_rate_per_mb, blocklist_hash, max_concurrent_requests, heartbeat_interval_s, attestation_interval_h
- Redis pub/sub for broadcasting to all orchestrator instances (multi-node deployment)
- `POST /admin/node/{id}/config` — admin endpoint to update specific node config

**Technical Notes**: Reuse the Redis pub/sub infrastructure from blocklist distribution (6.3). Config updates are non-critical — fire-and-forget with re-send on next heartbeat if missed.

---

### 7.5 — Android: Connection resilience
- **Agent**: Android
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 4.2 (base WebSocket client)
- **Files**: `android/app/src/main/.../network/WebSocketClient.kt` (enhancement)

**Acceptance Criteria**:
- Jitter added to reconnect backoff (±20% of base interval) to prevent thundering herd
- Session resume within 30s window: reuse node registration (same node ID, no re-registration)
- Explicit connection state machine: `DISCONNECTED` → `CONNECTING` → `CONNECTED` → `RECONNECTING` → back to `CONNECTING`
- State machine exposed as `StateFlow<ConnectionState>` to UI
- Backoff counter resets after 5 minutes of stable connection (not just on connect)

**Technical Notes**: Use `kotlin.random.Random.nextDouble(0.8, 1.2) * backoffMs` for jitter. Session token stored in `DataStore` with 30s TTL check on reconnect.

---

## Phase 2 — Wave 7 Success Gate

- [ ] Play Integrity tokens generated and sent to backend
- [ ] Backend verifies tokens against Google API
- [ ] Fake/rooted devices have earnings suspended
- [ ] ResourceMonitor emitting temperature and memory state (Android 11+ devices)
- [ ] Connection state machine transitions correctly on network events
- [ ] Node config pushed successfully to connected devices

## Dependencies

- **Requires**: Wave 05 complete
- **Parallel with**: Wave 06 (HTTPS + Security) — both depend on Wave 05
- **Unblocks**: Wave 08 (Android UI) — requires both Wave 06 and Wave 07

## Technical Notes

- Play Integrity requires a real Android device and a Google Play Console app registration — emulator will always fail attestation
- Tasks 7.1 and 7.2 must coordinate on the API contract: what fields are in the token request, what does the response body look like
- Connection resilience (7.5) is particularly important for mobile users who switch between WiFi and cellular — the 30s session resume window is critical for user experience
