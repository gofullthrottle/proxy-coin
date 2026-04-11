---
decomposed_from: .claude/plans/2026-03-21-proxy-coin-full-implementation.md
date: "2026-03-21"
project: "proxy-coin"
wave: 8
task_id: "android-ui"
created_at: "2026-03-21T00:00:00Z"
---

# Wave 08 — Android UI

**Phase**: 2 — Alpha
**Estimated Total**: 13h
**Dependencies**: Waves 06 and 07
**Agent**: Android Specialist (all 5 tasks)

Task 8.2 (DashboardViewModel) should be implemented alongside 8.1 (Dashboard screen). Task 8.3 (Earnings), 8.4 (Settings), and 8.5 (Notifications) are fully independent of each other.

---

## Tasks

### 8.1 — Android: Dashboard screen
- **Agent**: Android
- **Estimate**: 4h
- **Complexity**: Complex
- **Depends on**: 7.5 (connection state), 5.3 (earnings API)
- **Files**: `android/app/src/main/.../ui/dashboard/DashboardScreen.kt`

**Acceptance Criteria**:
- Connection status toggle (tap to start/stop proxy service)
- Today's earnings + all-time earnings with USD estimate
- Bandwidth shared progress bar (up/down bytes)
- Live stats row: uptime, requests served, avg latency, trust score
- 7-day earnings trend chart
- Material3 design throughout
- Loading states for all async data
- Error states with retry option

**Technical Notes**: Use `rememberCoroutineScope` for the service toggle. Charting: use a Compose-native library (e.g., `co.yml.charts` or `com.patrykandpatrick.vico`). Trust score displayed as a colored badge (green/yellow/red).

---

### 8.2 — Android: DashboardViewModel
- **Agent**: Android
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 8.1 (provides the UiState structure)
- **Files**: `android/app/src/main/.../ui/dashboard/DashboardViewModel.kt`

**Acceptance Criteria**:
- Collects data from: `ProxyForegroundService` (connection state, live stats via bound service), `EarningsRepository` (today/all-time from Room + API), API (token price, trust score)
- Exposes `DashboardUiState` as `StateFlow`
- Auto-refresh: live stats every 30s, earnings every 5 min, trust score every 10 min
- Service toggle action: calls `startService`/`stopService` and updates state

**Technical Notes**: `DashboardUiState` is a data class with: `connectionState`, `todayEarnings`, `allTimeEarnings`, `bandwidthUp`, `bandwidthDown`, `requestsServed`, `uptimeSeconds`, `avgLatencyMs`, `trustScore`, `earningsTrend`.

---

### 8.3 — Android: Earnings screen
- **Agent**: Android
- **Estimate**: 3h
- **Complexity**: Standard
- **Depends on**: 5.3 (earnings API), 2.5 (Room DB for local earnings)
- **Files**: `android/app/src/main/.../ui/earnings/EarningsScreen.kt`, `EarningsViewModel.kt`

**Acceptance Criteria**:
- Tab bar: Day / Week / Month / All
- Earnings chart (line chart) for selected period
- Breakdown table: bandwidth earnings, uptime bonus, quality bonus, referral earnings
- Pending amount (off-chain, not yet claimed) vs claimed amount (on-chain)
- "Claim Rewards" button — placeholder for Phase 3, shows coming soon
- Pull-to-refresh

**Technical Notes**: Earnings data comes from both local Room DB (recent, for speed) and API (authoritative, for history). Merge the two sources in the ViewModel.

---

### 8.4 — Android: Settings screen (full)
- **Agent**: Android
- **Estimate**: 3h
- **Complexity**: Standard
- **Depends on**: 5.4 (basic settings foundation)
- **Files**: `android/app/src/main/.../ui/settings/SettingsScreen.kt` (expansion of existing)

**Acceptance Criteria**:
- Bandwidth section: max bandwidth slider (Mbps), WiFi-only toggle, cellular toggle, cellular data cap (GB/month)
- Power section: battery threshold slider (10-50%), charging-only mode toggle
- Notifications section: earnings milestones toggle, service status toggle, weekly summary toggle
- Account section: device ID (copy), node ID (copy), trust score, referral code (copy + share)
- About section: version, Terms of Service link, Privacy Policy link, open source licenses
- All settings persisted in DataStore and reactive

**Technical Notes**: Use `HiltViewModel` and `SettingsRepository`. Group settings into `PreferenceCategory`-style sections. All sliders have labels showing the current value.

---

### 8.5 — Android: Notification enhancements
- **Agent**: Android
- **Estimate**: 1h
- **Complexity**: Simple
- **Depends on**: 4.1 (foreground notification base)
- **Files**: `android/app/src/main/.../service/ProxyForegroundService.kt` (notification update)

**Acceptance Criteria**:
- Rich notification showing: connection state icon, today's earnings (PRXY + USD), bandwidth shared, uptime
- Updated every 30s while service is running
- Action buttons in notification: "Pause" / "Resume" (pending intents)
- Low-priority notification channel to avoid user annoyance
- Expanded notification style (BigText or custom) showing more detail

**Technical Notes**: Use `NotificationCompat.Builder`. Expanded layout via `setStyle(NotificationCompat.BigTextStyle(...))`. Pause/resume via `PendingIntent` to a `BroadcastReceiver`.

---

## Phase 2 Success Gate

- [ ] Dashboard screen shows real-time connection status, earnings, and bandwidth
- [ ] Earnings screen shows history with chart, breakdown, and pending/claimed amounts
- [ ] Settings screen fully functional with all preferences persisted
- [ ] Notification shows live stats with pause/resume actions
- [ ] All screens follow Material3 design language
- [ ] No loading spinners lasting more than 2 seconds (use skeleton UI)

## Dependencies

- **Requires**: Wave 06 (HTTPS + Security) AND Wave 07 (Attestation + Monitoring) — both complete
- **Unblocks**: Waves 11 (Wallet + Merkle, combined with Wave 10) and 13 (Customer API) and 14 (Anti-Fraud)

## Technical Notes

- This wave represents the visible face of Phase 2 — real users interact with these screens
- Dashboard (8.1) is the highest-value screen and should be polished even if others are rough
- The "Claim Rewards" button in Earnings (8.3) is a placeholder that gets wired up in Wave 12
- Material3 dynamic color theming (monet) should be enabled for Android 12+ devices
- Test all screens with accessibility tools (TalkBack) before marking complete
