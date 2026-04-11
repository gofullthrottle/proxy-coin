---
decomposed_from: .claude/plans/2026-03-21-proxy-coin-full-implementation.md
date: "2026-03-21"
project: "proxy-coin"
wave: 4
task_id: "android-core-services"
created_at: "2026-03-21T00:00:00Z"
---

# Wave 04 — Android Core Services

**Phase**: 1 — Proof of Concept
**Estimated Total**: 16h
**Dependencies**: Wave 02
**Agent**: Android Specialist (all 6 tasks)

Internal ordering: 4.1 (ForegroundService) should start first as it's the host for other services. 4.2 (WebSocket) and 4.3 (ProxyEngine) are tightly coupled and should be implemented together. 4.4, 4.5, 4.6 can follow in parallel.

---

## Tasks

### 4.1 — Android: ProxyForegroundService
- **Agent**: Android
- **Estimate**: 4h
- **Complexity**: Complex
- **Depends on**: 2.4 (navigation), 2.6 (network module)
- **Files**: `android/app/src/main/.../service/ProxyForegroundService.kt`

**Acceptance Criteria**:
- Persistent foreground service with notification channel
- `START_STICKY` restart behavior
- `WakeLock` and `WifiLock` acquired on start
- Notification showing connection status, updated every 30s
- Lifecycle: create notification → start foreground → connect WebSocket → listen for requests → update notification
- Service can be started/stopped from Settings screen toggle

**Technical Notes**: Use `ServiceCompat.startForeground()` for API 26+ compatibility. Notification channel ID: `proxy_service`. Show stats in notification body.

---

### 4.2 — Android: WebSocketClient
- **Agent**: Android
- **Estimate**: 4h
- **Complexity**: Complex
- **Depends on**: 2.1 (proto), 2.6 (network module)
- **Files**: `android/app/src/main/.../network/WebSocketClient.kt`

**Acceptance Criteria**:
- OkHttp WebSocket connecting to `wss://` backend endpoint
- Protobuf message serialization/deserialization
- Send `REGISTER` with device info on connect
- Handle `REGISTERED` response (store node ID)
- Heartbeat every 30s
- Reconnect with exponential backoff: 1s→2s→4s→8s→16s→30s max + jitter
- Session resume within 30s window
- Route `PROXY_REQUEST` messages to `ProxyEngine`

**Technical Notes**: OkHttp WebSocket listener runs on background thread. Use Kotlin Flow or callbacks to bridge to coroutines. Device info: Android ID, model, network type.

---

### 4.3 — Android: ProxyEngine
- **Agent**: Android
- **Estimate**: 3h
- **Complexity**: Standard
- **Depends on**: 4.2 (WebSocket for response routing)
- **Files**: `android/app/src/main/.../proxy/ProxyEngine.kt`

**Acceptance Criteria**:
- For each `PROXY_REQUEST`: validate URL against blocklist (placeholder), build OkHttp Request
- Execute from device network (not via WebSocket — use device's own connectivity)
- Stream response back in 64KB chunks: `ProxyResponseStart` → `ProxyResponseChunk[]` → `ProxyResponseEnd`
- `Semaphore` for concurrent request limit (default 5)
- 30s timeout per request
- 10MB max response size limit
- Error handling: timeout, DNS failure, connection refused — send error `ProxyResponseEnd`

**Technical Notes**: Separate OkHttp client for proxied requests (not the backend client). Set `readTimeout(30, SECONDS)`. Count bytes flowing through for local metering.

---

### 4.4 — Android: ResourceMonitor
- **Agent**: Android
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 4.1 (service hosts the monitor)
- **Files**: `android/app/src/main/.../monitor/ResourceMonitor.kt`

**Acceptance Criteria**:
- Monitor battery level (`BatteryManager`), charging state
- Monitor network type (`ConnectivityManager`: WiFi/cellular/metered)
- Monitor available memory
- Emit `ResourceState` every 10s via `StateFlow`
- `ProxyForegroundService` pauses proxy when:
  - Battery < threshold (default 20%)
  - Not on WiFi (if WiFi-only enabled in settings)
  - Thermal throttling detected

**Technical Notes**: Register `BroadcastReceiver` for `ACTION_BATTERY_CHANGED` and `CONNECTIVITY_ACTION`. Use `ConnectivityManager.NetworkCallback` for modern API.

---

### 4.5 — Android: BootReceiver
- **Agent**: Android
- **Estimate**: 1h
- **Complexity**: Simple
- **Depends on**: 4.1 (service to start)
- **Files**: `android/app/src/main/.../receiver/BootReceiver.kt`, `AndroidManifest.xml` update

**Acceptance Criteria**:
- `BroadcastReceiver` for `BOOT_COMPLETED`
- Starts `ProxyForegroundService` if user had it enabled
- Registered in `AndroidManifest.xml` with `RECEIVE_BOOT_COMPLETED` permission
- Respects user's "auto-start" preference from DataStore

**Technical Notes**: Use `WorkManager` or direct service start. Check `DataStore` for `autoStart` preference before starting.

---

### 4.6 — Android: MeteringService
- **Agent**: Android
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 2.5 (Room DB), 4.3 (ProxyEngine provides byte counts)
- **Files**: `android/app/src/main/.../service/MeteringService.kt`

**Acceptance Criteria**:
- Local metering tracker — records per-request stats in Room DB
- `MeteringEntity` fields: request_id, bytes_in, bytes_out, latency_ms, success, timestamp
- Periodic batch report to backend: `POST /v1/metering/report`
- Batch interval: every 5 minutes or 100 records, whichever first
- Supplementary to server-side metering (not authoritative)

**Technical Notes**: Use `WorkManager` for periodic batch uploads. Keep metering data for 30 days, then purge.

---

## Phase 1 — Wave 4 Success Gate

- [ ] `ProxyForegroundService` starts, shows notification, maintains foreground state
- [ ] `WebSocketClient` connects to backend, completes registration handshake
- [ ] `ProxyEngine` executes HTTP requests from device and streams responses back
- [ ] `ResourceMonitor` emits state and pauses service on low battery
- [ ] `BootReceiver` starts service after device reboot (if preference set)
- [ ] `MeteringService` records request stats in local Room DB

## Dependencies

- **Requires**: Wave 02 complete
- **Parallel with**: Wave 03 (Backend Orchestrator) — both depend only on Wave 02
- **Unblocks**: Wave 05 (Integration) — requires both Wave 03 and Wave 04

## Technical Notes

- This is the highest-hour wave in Phase 1 at 16h — plan accordingly
- Tasks 4.2 and 4.3 are tightly coupled: the WebSocket routes requests to the ProxyEngine; design the interface between them first
- The ProxyEngine uses the device's own network connection (not the WebSocket) to fetch resources — this is the core value proposition
- Android's battery and threading constraints make foreground services tricky — test on a real device, not just emulator
