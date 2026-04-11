---
decomposed_from: .claude/plans/2026-03-21-proxy-coin-full-implementation.md
date: "2026-03-21"
project: "proxy-coin"
wave: 10
task_id: "contract-testing"
created_at: "2026-03-21T00:00:00Z"
---

# Wave 10 — Contract Testing + Deploy

**Phase**: 3 — Token Launch
**Estimated Total**: 12h
**Dependencies**: Wave 09
**Agent**: Contract Specialist (all 6 tasks)

Tasks 10.1 through 10.4 are fully parallel (one test file per contract). Task 10.5 (fuzz tests) can overlap with 10.1-10.4. Task 10.6 (Sepolia deploy) must come last.

---

## Tasks

### 10.1 — Tests: ProxyCoinToken
- **Agent**: Contracts
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 9.1
- **Files**: `contracts/test/ProxyCoinToken.t.sol`

**Acceptance Criteria**:
- Test: `mint` — success path, exceeds MAX_SUPPLY reverts, unauthorized reverts
- Test: `burn` — success path, unauthorized reverts, burns correct amount
- Test: role management — `grantRole`, `revokeRole`, `renounceRole`
- Test: standard ERC-20 — `transfer`, `transferFrom`, `approve`, `allowance`
- Test: `ERC20Permit` — `permit` with valid/invalid signatures
- 100% function coverage (`forge coverage`)

**Technical Notes**: Use `vm.expectRevert()` for expected failures. Use `makeAddr()` for test addresses. Check events with `vm.expectEmit()`.

---

### 10.2 — Tests: RewardDistributor
- **Agent**: Contracts
- **Estimate**: 3h
- **Complexity**: Standard
- **Depends on**: 9.2
- **Files**: `contracts/test/RewardDistributor.t.sol`

**Acceptance Criteria**:
- Test: `setMerkleRoot` — owner succeeds, non-owner reverts
- Test: `claim` with valid proof — tokens transferred, `claimed` mapping updated, event emitted
- Test: `claim` with invalid proof — reverts
- Test: double-claim prevention — second claim for same epoch reverts
- Test: cumulative accounting — claim 50 tokens, then remaining 50 from total of 100
- Test: `getClaimable` view accuracy
- Generate test Merkle trees in Solidity using OpenZeppelin `MerkleTree` or off-chain in test setup

**Technical Notes**: Build Merkle trees off-chain in test using `murky` library or manual keccak256 construction. Test with 3+ leaves to verify proof paths.

---

### 10.3 — Tests: Staking
- **Agent**: Contracts
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 9.3
- **Files**: `contracts/test/Staking.t.sol`

**Acceptance Criteria**:
- Test: `stake` for each tier (boundary values: exactly 1K, 5K, 10K, 50K PRXY)
- Test: `unstake` before lock period expires — reverts
- Test: `unstake` after lock period — tokens returned, state cleared
- Test: `slash` — 50% burned, 50% to what? verify per spec. Unauthorized reverts.
- Test: `getTier` boundary values (999 → none, 1000 → Bronze, 4999 → Bronze, 5000 → Silver)
- Test: already-staked rejection (second `stake` call reverts)

**Technical Notes**: Use `vm.warp()` to test time-based lock expiry. Use `vm.expectEmit()` for all events.

---

### 10.4 — Tests: Vesting
- **Agent**: Contracts
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 9.4
- **Files**: `contracts/test/Vesting.t.sol`

**Acceptance Criteria**:
- Test: `createSchedule` — success path, unauthorized reverts, duplicate beneficiary handling
- Test: `release` before cliff — zero tokens released
- Test: `release` during vesting — correct linear interpolation (25%, 50%, 75%)
- Test: `release` after full vesting — full amount released
- Test: `revoke` — revocable schedule returns unvested, non-revocable reverts
- Test: `getReleasable` matches `release` behavior at all checkpoints

**Technical Notes**: Use `vm.warp()` extensively. Create a helper function `warpTo(startTime + duration)` for readability. Verify exact token amounts (no approximation in assertions).

---

### 10.5 — Tests: Fuzz testing
- **Agent**: Contracts
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 10.1, 10.2, 10.3, 10.4
- **Files**: `contracts/test/Fuzz.t.sol`

**Acceptance Criteria**:
- Fuzz test: mint amounts (bound to valid range) — total minted never exceeds MAX_SUPPLY
- Fuzz test: claim amounts — claimed never exceeds cumulativeAmount from valid proof
- Fuzz test: stake amounts — getTier always returns correct tier for any amount
- Fuzz test: vesting calculations — getReleasable never negative, never exceeds total
- Invariant test: `totalMinted <= MAX_SUPPLY` across all operations
- Invariant test: `claimed[wallet] <= cumulativeAllocation[wallet]`
- 1000 fuzz runs (configured in `foundry.toml`)

**Technical Notes**: Use `bound(x, min, max)` for constrained fuzzing. Invariant tests use the `invariant_` prefix. Use `excludeContract()` for test helpers.

---

### 10.6 — Contracts: Deploy to Base Sepolia
- **Agent**: Contracts
- **Estimate**: 1h
- **Complexity**: Simple
- **Depends on**: 10.1, 10.2, 10.3, 10.4, 10.5 (all tests must pass first)
- **Files**: `contracts/deployments/base-sepolia.json`

**Acceptance Criteria**:
- Run `Deploy.s.sol` against Base Sepolia testnet
- All 4 contracts deployed and verified on Basescan
- Addresses recorded in `contracts/deployments/base-sepolia.json`
- Roles granted: `MINTER_ROLE` to RewardDistributor, `BURNER_ROLE` to Staking
- Manual test: mint 1000 PRXY, stake in Bronze tier, verify on Basescan
- Manual test: set a test Merkle root and claim tokens

**Technical Notes**: Use `forge script --verify --rpc-url base_sepolia`. Basescan API key in env. Gas on Base Sepolia is cheap — no need to optimize for testnet.

---

## Phase 3 — Wave 10 Success Gate

- [ ] `forge test` passes all tests (0 failures)
- [ ] `forge coverage` shows > 95% line coverage
- [ ] `forge test --fuzz-runs 1000` passes all fuzz tests
- [ ] All 4 contracts deployed to Base Sepolia
- [ ] All contracts verified on Basescan
- [ ] Manual claim test succeeds on testnet
- [ ] Deployment addresses saved to `base-sepolia.json`

## Dependencies

- **Requires**: Wave 09 (Smart Contracts) complete
- **Unblocks**: Wave 11 (Wallet + Merkle Backend) — combined with Wave 08 completion

## Technical Notes

- Tests should be written before Wave 10.6 (deploy) — this is the gate condition
- Fuzz tests often surface edge cases that unit tests miss — run them with high iteration counts during development
- Basescan verification can fail on first try due to rate limits — retry with `--delay 5`
- Save the deployer private key separately from the testnet faucet key — deployer will eventually become the admin/owner for mainnet contracts
