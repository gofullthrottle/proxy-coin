---
decomposed_from: .claude/plans/2026-03-21-proxy-coin-full-implementation.md
date: "2026-03-21"
project: "proxy-coin"
wave: 15
task_id: "beta-features"
created_at: "2026-03-21T00:00:00Z"
---

# Wave 15 — Beta Features

**Phase**: 4 — Beta
**Estimated Total**: 14h
**Dependencies**: Waves 13 and 14
**Agent Mix**: Android (2 tasks), Backend (4 tasks)

All 6 tasks are effectively parallel — they cover distinct features with no internal dependencies. Android tasks (15.1, 15.2) and Backend tasks (15.3, 15.4, 15.5, 15.6) can all proceed simultaneously.

---

## Tasks

### 15.1 — Android: Onboarding flow
- **Agent**: Android
- **Estimate**: 3h
- **Complexity**: Standard
- **Depends on**: 12.5 (WalletSetupScreen), 2.4 (navigation)
- **Files**: `android/app/src/main/.../ui/onboarding/WelcomeScreen.kt`, `PermissionsScreen.kt`, `OnboardingViewModel.kt`

**Acceptance Criteria**:
- `WelcomeScreen`: logo, tagline, 3 value propositions, "Get Started" CTA
- `PermissionsScreen`: battery optimization exemption, notifications permission — each with plain-language explanation
- Flow: Welcome → Permissions → WalletSetup → Dashboard
- Only shown on first launch (persist `onboardingCompleted: true` in DataStore)
- Skip button on permissions screen (not required, just recommended)
- Smooth animated transitions between screens

**Technical Notes**: Use `AnimatedNavHost` or `Crossfade` for transitions. Battery optimization: launch `ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS`. Notification: use `ActivityCompat.requestPermissions` for API 33+.

---

### 15.2 — Android: Referral screen
- **Agent**: Android
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 8.4 (settings screen, referral code displayed there too), 15.3 (referral API)
- **Files**: `android/app/src/main/.../ui/referral/ReferralScreen.kt`

**Acceptance Criteria**:
- Display unique referral code (from `GET /v1/referral/code`)
- Share button: deep link `https://proxycoin.io/join?ref={code}` via Android share sheet
- Referral stats: total referrals (count), total earnings from referrals (PRXY + USD)
- Referral list: each referee's join date and their contribution to referrer earnings
- Pull-to-refresh

**Technical Notes**: Referral deep link should be handled by the app (intent filter in manifest). Use `Intent.ACTION_SEND` for share sheet. Stats from `GET /v1/referral/stats`.

---

### 15.3 — Backend: Referral system
- **Agent**: Backend
- **Estimate**: 3h
- **Complexity**: Standard
- **Depends on**: 2.3 (referrals table), 11.5 (reward calculator for referral earnings)
- **Files**: `backend/internal/customer/referral.go`, `backend/internal/api/referral_handler.go`

**Acceptance Criteria**:
- Generate unique referral codes per node on registration (6-char alphanumeric, stored in nodes table)
- `POST /v1/referral/apply` — link referee (new node) to referrer (existing node), record in referrals table. One-time only.
- Referral earnings: 5% of referee's earnings shared to referrer (calculated in reward calculator)
- `GET /v1/referral/stats` — referrer sees: code, count, total_earned
- `GET /v1/referral/code` — returns current node's referral code
- Referral earnings show in earnings breakdown (separate line item in response)

**Technical Notes**: Referral code collision: retry with new code if collision detected (unlikely with 6 chars but handle it). Referral earnings are computed in the reward calculator (Wave 11.5) — add 5% of referee reward to referrer's claimable_rewards.

---

### 15.4 — Backend: Billing system
- **Agent**: Backend
- **Estimate**: 3h
- **Complexity**: Standard
- **Depends on**: 13.5 (usage tracking), 13.1 (customer auth)
- **Files**: `backend/internal/customer/billing.go`, `backend/internal/api/billing_handler.go`

**Acceptance Criteria**:
- Plan definitions: Free (100 req/day, $0/mo), Starter (10K req/day, $29/mo), Pro (100K req/day, $199/mo), Enterprise (custom)
- `GET /v1/billing` — returns current plan, usage this period, next billing date, estimated cost
- `GET /v1/billing/invoices` — returns list of past invoices (period, requests, bytes, total_cost)
- Invoice generation: simple calculation from daily usage records — no Stripe integration in this wave
- Plan upgrade placeholder: `POST /v1/billing/plan` (returns "coming soon" for actual payment)

**Technical Notes**: Invoices are generated monthly from `customer_usage` records. Store invoices in a new `customer_invoices` table (add migration). Cost calculation: configurable per-request and per-GB rates in config.

---

### 15.5 — Backend: Geographic targeting
- **Agent**: Backend
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 3.2 (NodeSelector), 14.2 (IP intelligence provides country data)
- **Files**: `backend/internal/node/selector.go` (enhancement)

**Acceptance Criteria**:
- Enhance `NodeSelector` to accept `geo: { country, region }` from customer proxy request
- Filter candidate nodes by country (ISO 3166-1 alpha-2) and optionally region
- Fall back to any country if no nodes available in requested geo
- `GET /v1/status` — public endpoint showing available node counts by country
  - Response: `{ countries: { "US": 45, "UK": 12, "DE": 8, ... } }`
- Update node registration to record country/region from IP geolocation (use MaxMind from 14.2)

**Technical Notes**: Country data comes from IP intelligence (MaxMind GeoLite2). Cache node-by-country counts in Redis (TTL 60s). `/v1/status` is unauthenticated — useful for customers before signing up.

---

### 15.6 — Backend: Sticky sessions
- **Agent**: Backend
- **Estimate**: 1h
- **Complexity**: Simple
- **Depends on**: 3.2 (NodeSelector), 3.5 (Redis)
- **Files**: `backend/internal/node/selector.go` (addition), `backend/internal/cache/redis.go` (addition)

**Acceptance Criteria**:
- Customer sends `session_id` field in proxy request (optional string)
- Node selector checks Redis: `session:{sessionID}` → nodeID, TTL 5 min
- If session exists: route to same node (if still available), else new node + update binding
- If session doesn't exist: select new node, store in Redis with 5-min TTL
- Session TTL resets on each request using that session

**Technical Notes**: 3 lines of logic in selector + Redis read/write. The complexity is in handling the case where the session-pinned node goes offline — fall through to normal selection and update the binding.

---

## Phase 4 — Wave 15 Success Gate

- [ ] Onboarding flow shown on fresh install, skipped on subsequent launches
- [ ] Referral codes generated for all nodes, referral earnings flowing
- [ ] Billing summary accurate against usage records
- [ ] Geographic filtering routes requests to correct country nodes
- [ ] Sticky sessions maintained across sequential requests with same `session_id`
- [ ] Referral screen shows correct stats

## Dependencies

- **Requires**: Wave 13 (Customer API) AND Wave 14 (Anti-Fraud) — both complete
- **Unblocks**: Wave 16 (Polish + Production)

## Technical Notes

- Onboarding (15.1) significantly impacts day-1 user retention — invest in smooth animations and clear copy
- Referral system (15.3) is a growth mechanism — ensure the 5% calculation is verified and visible to referrers
- Geographic targeting (15.5) is a key differentiator for enterprise customers who need specific country IPs
- Sticky sessions (15.6) is a small task but high-value for customers scraping or maintaining login sessions
