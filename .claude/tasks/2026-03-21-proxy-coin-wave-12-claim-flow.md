---
decomposed_from: .claude/plans/2026-03-21-proxy-coin-full-implementation.md
date: "2026-03-21"
project: "proxy-coin"
wave: 12
task_id: "claim-flow"
created_at: "2026-03-21T00:00:00Z"
---

# Wave 12 — Claim Flow + Wallet UI

**Phase**: 3 — Token Launch
**Estimated Total**: 12h
**Dependencies**: Wave 11
**Agent Mix**: Backend (3 tasks), Android (3 tasks)

Backend tasks 12.1 and 12.2 should be done together (publisher + API are tightly coupled). Android tasks 12.3 and 12.4 are coupled (screen + ViewModel). Task 12.5 (WalletSetupScreen) is independent. Task 12.6 (E2E test) comes last.

---

## Tasks

### 12.1 — Backend: Merkle root publisher
- **Agent**: Backend
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 11.6 (Merkle generator produces root), 10.6 (contract on Sepolia)
- **Files**: `backend/internal/reward/distributor.go`

**Acceptance Criteria**:
- `internal/reward/distributor.go` — `PublishMerkleRoot(root [32]byte)` function
- Calls `setMerkleRoot(root)` on RewardDistributor contract via Go Ethereum client (`go-ethereum`)
- Store transaction hash in database after success
- Triggered automatically after daily Merkle generation job (11.6)
- Error handling: retry up to 3 times on gas estimation or nonce issues
- Alert on failure (log + optionally notify admin)

**Technical Notes**: Use `go-ethereum` `ethclient` for contract calls. Requires a funded service wallet for gas (Base has very low fees). ABI binding generated via `abigen` from RewardDistributor ABI.

---

### 12.2 — Backend: Claim proof API
- **Agent**: Backend
- **Estimate**: 1h
- **Complexity**: Simple
- **Depends on**: 11.6 (proofs stored in DB), 12.1 (root published)
- **Files**: `backend/internal/api/rewards_handler.go`

**Acceptance Criteria**:
- `GET /v1/rewards/proof` — authenticated request, returns Merkle proof for requesting node's wallet
- Response: `{ cumulativeAmount, proof: [], currentEpoch, merkleRoot }`
- `GET /v1/rewards/history` — claim history with tx hashes, amounts, timestamps
- Returns 404 if no pending rewards for wallet

**Technical Notes**: Look up wallet address from node's auth token. Query `merkle_proofs` table. Proofs are returned as hex-encoded `bytes32[]` for Web3j consumption.

---

### 12.3 — Android: Wallet screen
- **Agent**: Android
- **Estimate**: 3h
- **Complexity**: Standard
- **Depends on**: 11.1 (WalletManager), 11.4 (TransactionBuilder), 12.2 (claim proof API)
- **Files**: `android/app/src/main/.../ui/wallet/WalletScreen.kt`

**Acceptance Criteria**:
- PRXY balance (on-chain via Web3j)
- Pending balance (off-chain, from `GET /v1/earnings/summary`)
- Wallet address with QR code + copy button
- "Claim Rewards" button — fetches proof, builds tx, signs, submits, shows confirmation
- "Send PRXY" button — opens send flow (address + amount input)
- Transaction history list (local Room DB + on-chain via explorer API)
- Token price in USD (from DEX price feed or CoinGecko API)
- Block explorer link (Basescan)
- Loading states for all async operations

**Technical Notes**: QR code: use `zxing-android-embedded` or Compose-native QR library. Claim button shows step progress: Fetching proof → Building transaction → Signing → Submitting → Confirmed.

---

### 12.4 — Android: WalletViewModel
- **Agent**: Android
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 12.3 (UiState structure defined by screen)
- **Files**: `android/app/src/main/.../ui/wallet/WalletViewModel.kt`

**Acceptance Criteria**:
- Coordinates: `WalletManager` (on-chain balance), API (pending earnings, claim proof), `TransactionBuilder` (claim/send)
- Exposes `WalletUiState` as `StateFlow`
- Claim flow state machine: `IDLE` → `FETCHING_PROOF` → `BUILDING_TX` → `SIGNING` → `SUBMITTING` → `CONFIRMING` → `SUCCESS`/`ERROR`
- Handle claim errors with user-friendly messages (insufficient gas, proof invalid, already claimed)
- Auto-refresh balance after confirmed transaction

**Technical Notes**: `WalletUiState` data class includes: `address`, `onChainBalance`, `pendingBalance`, `usdPrice`, `transactions`, `claimState`, `errorMessage`.

---

### 12.5 — Android: WalletSetupScreen
- **Agent**: Android
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 11.3 (MnemonicGenerator), 2.4 (navigation)
- **Files**: `android/app/src/main/.../ui/wallet/WalletSetupScreen.kt`

**Acceptance Criteria**:
- Shown during onboarding if no wallet exists
- Option A: "Create New Wallet" → generates mnemonic → shows MnemonicBackupScreen → confirm → wallet created
- Option B: "Import Existing" → paste 12-word phrase or 64-char hex private key → validate → import
- Wallet address displayed after creation/import with copy button
- Navigate to Dashboard after setup
- Persist wallet creation state: `walletCreated: true` in DataStore

**Technical Notes**: Skip screen if wallet already exists (check DataStore on app launch). Use `AnimatedVisibility` for transition between create/import options.

---

### 12.6 — Integration: End-to-end claim flow
- **Agent**: Backend
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 12.1, 12.2, 12.3, 12.4 (full claim pipeline)
- **Files**: `scripts/test-claim-flow.sh` or `backend/tests/e2e/claim_test.go`

**Acceptance Criteria**:
- Full cycle test: node earns rewards → reward calculator runs → Merkle tree generated → root published on-chain → node fetches proof → node claims on testnet → verify tokens received
- Test with actual Base Sepolia testnet (or local Anvil fork)
- Verify: token balance increased by claimed amount
- Test: double-claim attempt is rejected by contract
- Performance: Merkle generation for 1,000 test nodes completes in < 1 minute

**Technical Notes**: Requires: running backend services, Sepolia RPC, funded service wallet. Can use `vm.ffi()` in Foundry to call backend if running fully local. Document test environment setup clearly.

---

## Phase 3 Success Gate

- [ ] All 4 contracts deployed to Base Sepolia
- [ ] All contract tests passing, > 95% coverage
- [ ] Wallet generation + key storage verified secure (manual security review)
- [ ] Merkle tree generation < 5 minutes for 10K nodes (< 1 min for 1K)
- [ ] End-to-end claim flow working on testnet (tokens received in wallet)
- [ ] Gas cost per claim < $0.01 on Base (verify on Sepolia)
- [ ] WalletSetupScreen tested on physical device with real Keystore

## Dependencies

- **Requires**: Wave 11 (Wallet + Merkle Backend) complete
- **Unblocks**: Waves 13, 14 (Phase 4 — depend on Wave 08 not Wave 12)

## Technical Notes

- The claim flow (12.6) is the end-to-end proof that the token economy works
- Gas on Base L2 is very cheap (< $0.01 per transaction at normal gas prices) — verify this assumption on Sepolia before committing to the economics
- The wallet setup screen (12.5) is security-critical UX — users must understand they need to back up their mnemonic before it's discarded from memory
