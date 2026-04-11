---
decomposed_from: .claude/plans/2026-03-21-proxy-coin-full-implementation.md
date: "2026-03-21"
project: "proxy-coin"
wave: 2
task_id: "foundation-layer"
created_at: "2026-03-21T00:00:00Z"
---

# Wave 02 — Foundation Layer

**Phase**: 0 — Foundation
**Estimated Total**: 10h
**Dependencies**: Wave 01 (all tasks)
**Agent Mix**: Infrastructure (1 task), Backend (2 tasks), Android (3 tasks)

Tasks 2.1, 2.2, and 2.4/2.5/2.6 can run in parallel. Task 2.3 depends on 2.2 completing (config needed before migrations).

---

## Tasks

### 2.1 — Protocol: Compile + verify generated code
- **Agent**: Infrastructure
- **Estimate**: 1h
- **Complexity**: Simple
- **Depends on**: 1.1, 1.6
- **Files**: `backend/pkg/protocol/*.go`, `android/app/src/main/java/.../protocol/`

**Acceptance Criteria**:
- `generate-proto.sh` runs cleanly
- Go code generated in `backend/pkg/protocol/`
- Kotlin code generated for Android
- Generated code compiles in both Go and Kotlin projects

**Technical Notes**: Run script, verify output, fix any protoc issues. This unblocks task 3.3 (WebSocket server needs protocol types).

---

### 2.2 — Backend: Config package
- **Agent**: Backend
- **Estimate**: 1h
- **Complexity**: Simple
- **Depends on**: 1.2
- **Files**: `backend/internal/config/config.go`

**Acceptance Criteria**:
- `internal/config/config.go` with `Config` struct
- Loads from env vars: `DATABASE_URL`, `REDIS_URL`, `BASE_RPC_URL`, `LISTEN_ADDR`, `WS_ADDR`, `JWT_SECRET`
- Sensible defaults for development
- Validation of required fields

**Technical Notes**: Use envconfig or manual `os.Getenv`. Keep it simple, no YAML config files.

---

### 2.3 — Backend: Database migrations
- **Agent**: Backend
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 1.2, 1.5 (Postgres must exist)
- **Files**: `backend/migrations/001_initial.up.sql`, `backend/migrations/001_initial.down.sql`

**Acceptance Criteria**:
- `migrations/001_initial.up.sql` with all 7+ tables from BACKEND.md
- Tables: nodes, metering (partitioned), earnings, claimable_rewards, customers, customer_usage, fraud_events, referrals
- All indexes from BACKEND.md schema
- `migrations/001_initial.down.sql` for rollback
- Migrations apply successfully against Docker Postgres

**Technical Notes**: Use golang-migrate for migration runner. Set up partitioning for the metering table (partitioned by created_at month). Include referrals table even though it's needed in a later wave.

---

### 2.4 — Android: Navigation + base screens
- **Agent**: Android
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 1.3
- **Files**: `android/app/src/main/.../navigation/NavGraph.kt`, screen stubs, `Theme.kt`

**Acceptance Criteria**:
- `NavGraph.kt` with routes for all 6 screens (Dashboard, Earnings, Wallet, Settings, Onboarding, WalletSetup)
- Empty Composable stubs for each screen
- Bottom navigation bar (Home, Earn, Wallet, Settings)
- Theme applied (Color.kt, Type.kt, Theme.kt)
- App launches and navigates between screens

**Technical Notes**: Use Compose Navigation. Material3 bottom nav. Define routes as a sealed class or object with string constants.

---

### 2.5 — Android: Room database setup
- **Agent**: Android
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 1.3
- **Files**: `android/app/src/main/.../data/local/AppDatabase.kt`, entity files, DAOs

**Acceptance Criteria**:
- `AppDatabase.kt` with 3 entities per ANDROID-APP.md
- `EarningsEntity`, `MeteringEntity`, `TransactionEntity` with all fields
- `EarningsDao`, `MeteringDao`, `TransactionDao` with CRUD queries
- `DatabaseModule` providing database instance via Hilt
- Database created successfully on app launch

**Technical Notes**: Room version from `libs.versions.toml`. Use KSP for Room compiler (not KAPT).

---

### 2.6 — Android: Network module
- **Agent**: Android
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 1.3
- **Files**: `android/app/src/main/.../data/remote/ApiService.kt`, `NetworkModule.kt`, `AuthInterceptor.kt`

**Acceptance Criteria**:
- `ApiService.kt` interface with all endpoints from ANDROID-APP.md
- `OkHttpClient` with logging interceptor in `NetworkModule`
- Retrofit instance configured for JSON + protobuf
- `AuthInterceptor` skeleton (adds JWT to requests)
- Base URL configurable via `BuildConfig`

**Technical Notes**: Use Moshi or Kotlinx Serialization for JSON. Protobuf converter for binary endpoints.

---

## Wave 2 Success Gate

- [ ] `docker-compose -f infrastructure/docker-compose.dev.yml up` starts Postgres+Redis
- [ ] `go build ./...` compiles all backend packages
- [ ] Gradle sync succeeds for Android project
- [ ] Proto generation works end-to-end
- [ ] Database migrations apply cleanly to local Postgres
- [ ] Android app launches and shows navigation skeleton

## Dependencies

- **Requires**: Wave 01 complete (all 6 tasks)
- **Unblocks**: Waves 03 and 04 (can start in parallel after Wave 02)

## Technical Notes

- Wave 2 completion is the "foundation gate" — nothing in Phases 1-4 can start without this
- Backend tasks (2.2, 2.3) and Android tasks (2.4, 2.5, 2.6) are fully parallel with each other
- Task 2.1 (proto compile) must finish before 3.3 (WebSocket server) starts
- Task 2.3 (migrations) requires both 1.2 (Go structure for migration runner) and 1.5 (Docker Postgres)
