---
decomposed_from: .claude/plans/2026-03-21-proxy-coin-full-implementation.md
date: "2026-03-21"
project: "proxy-coin"
wave: 9
task_id: "smart-contracts"
created_at: "2026-03-21T00:00:00Z"
---

# Wave 09 — Smart Contracts

**Phase**: 3 — Token Launch
**Estimated Total**: 11h
**Dependencies**: Wave 01 only (Foundry project initialized)
**Agent**: Contract Specialist (all 6 tasks)

Important: Wave 9 is independent of the Phase 1-2 backend/Android work. It can start as soon as Wave 01 completes — run this as a parallel effort during Phase 2.

Internal ordering: 9.1 (Token) first, then 9.2, 9.3, 9.4 in parallel (all depend on Token). 9.5 (deploy script) after all contracts. 9.6 (distribute script) is short and can follow 9.2.

---

## Tasks

### 9.1 — Contracts: ProxyCoinToken.sol
- **Agent**: Contracts
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 1.4 (Foundry project)
- **Files**: `contracts/src/ProxyCoinToken.sol`

**Acceptance Criteria**:
- ERC-20 with `AccessControl` from OpenZeppelin
- Roles: `MINTER_ROLE`, `BURNER_ROLE`
- `MAX_SUPPLY` = 1,000,000,000 × 10^18 (1B tokens)
- `mint(address to, uint256 amount)` — checks supply cap, requires `MINTER_ROLE`
- `burn(address from, uint256 amount)` — requires `BURNER_ROLE`
- Constructor: grants `DEFAULT_ADMIN_ROLE` to deployer
- Events: `Transfer`, `Approval` (inherited), `RoleGranted`, `RoleRevoked`

**Technical Notes**: Inherit from `ERC20`, `AccessControl`. Use `ERC20Permit` for gasless approvals. Token name: "ProxyCoin", symbol: "PRXY", decimals: 18.

---

### 9.2 — Contracts: RewardDistributor.sol
- **Agent**: Contracts
- **Estimate**: 3h
- **Complexity**: Standard
- **Depends on**: 9.1 (references Token contract)
- **Files**: `contracts/src/RewardDistributor.sol`

**Acceptance Criteria**:
- Merkle-based cumulative claims per SMART-CONTRACTS.md
- `setMerkleRoot(bytes32 root)` — owner only, updates merkle root
- `claim(uint256 cumulativeAmount, bytes32[] calldata proof)` — verifies with `MerkleProof.verify`, cumulative tracking via `claimed[address]` mapping, transfers `cumulativeAmount - claimed[msg.sender]` tokens
- `getClaimable(address wallet, uint256 cumulativeAmount, bytes32[] calldata proof)` — view function
- Events: `MerkleRootUpdated(bytes32 oldRoot, bytes32 newRoot)`, `RewardsClaimed(address indexed wallet, uint256 amount)`

**Technical Notes**: Leaf encoding: `keccak256(abi.encodePacked(wallet, cumulativeAmount))`. Use OpenZeppelin `MerkleProof`. Cumulative design prevents double-claim automatically — claimed[address] tracks total claimed so far.

---

### 9.3 — Contracts: Staking.sol
- **Agent**: Contracts
- **Estimate**: 3h
- **Complexity**: Standard
- **Depends on**: 9.1 (references Token contract)
- **Files**: `contracts/src/Staking.sol`

**Acceptance Criteria**:
- 4-tier staking per SMART-CONTRACTS.md
- Tier thresholds: Bronze 1K, Silver 5K, Gold 10K, Platinum 50K PRXY
- Lock periods: Bronze/Silver 30 days, Gold 60 days, Platinum 90 days
- `stake(uint256 amount)` — transfer tokens from user, record stake, determine tier
- `unstake()` — after lock period, returns tokens; reverts if locked
- `slash(address staker, string calldata reason)` — requires `SLASHER_ROLE`, burns 50% of stake
- `getTier(address staker)` — view function returning current tier
- Events: `Staked`, `Unstaked`, `Slashed`

**Technical Notes**: Reject staking if already staked (must unstake first). Store: `stakedAmount`, `stakedAt`, `unlockAt`, `tier` per address. Grant `SLASHER_ROLE` to a backend service account for automated slashing.

---

### 9.4 — Contracts: Vesting.sol
- **Agent**: Contracts
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 9.1 (references Token contract)
- **Files**: `contracts/src/Vesting.sol`

**Acceptance Criteria**:
- Linear vesting with cliff per SMART-CONTRACTS.md
- `createSchedule(address beneficiary, uint256 totalAmount, uint256 startTime, uint256 cliffDuration, uint256 vestingDuration, bool revocable)` — owner only
- `release(address beneficiary)` — calculates vested amount, transfers releasable tokens
- `revoke(address beneficiary)` — owner only, revocable schedules only, returns unvested to owner
- `getReleasable(address beneficiary)` — view function
- Events: `ScheduleCreated`, `TokensReleased`, `ScheduleRevoked`

**Technical Notes**: Vested amount formula: if `block.timestamp < cliff` → 0; if `>= cliff + vestingDuration` → total; else linear interpolation. `released` mapping tracks already-released amounts.

---

### 9.5 — Contracts: Deploy script
- **Agent**: Contracts
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 9.1, 9.2, 9.3, 9.4 (all contracts must exist)
- **Files**: `contracts/script/Deploy.s.sol`

**Acceptance Criteria**:
- `script/Deploy.s.sol` — deploys all 4 contracts in order
- Deploy sequence: Token → RewardDistributor → Staking → Vesting
- Grant `MINTER_ROLE` to RewardDistributor address
- Grant `BURNER_ROLE` to Staking address
- Verify all addresses are non-zero post-deploy
- Output deployment addresses to console
- Works against local anvil fork and Base Sepolia

**Technical Notes**: Use Foundry `Script` base class with `vm.startBroadcast(deployerPrivateKey)`. Check `vm.envUint("PRIVATE_KEY")` for key. Log all deployed addresses.

---

### 9.6 — Contracts: DistributeRewards script
- **Agent**: Contracts
- **Estimate**: 1h
- **Complexity**: Simple
- **Depends on**: 9.2 (RewardDistributor must exist)
- **Files**: `contracts/script/DistributeRewards.s.sol`

**Acceptance Criteria**:
- `script/DistributeRewards.s.sol` — calls `setMerkleRoot(root)` on RewardDistributor
- Takes root as `vm.envBytes32("MERKLE_ROOT")` input
- Takes `REWARD_DISTRIBUTOR_ADDRESS` from env
- Logs transaction hash on success
- Used by backend's daily Merkle generation job (Wave 11.6)

**Technical Notes**: Simple script — few lines. The main value is that it's tested and the environment variable interface is documented for the backend team.

---

## Phase 3 — Wave 9 Success Gate

- [ ] All 4 contracts compile without warnings
- [ ] Deploy script runs against local anvil (fork of Base)
- [ ] Token mints correctly with supply cap enforced
- [ ] RewardDistributor merkle root can be set
- [ ] Staking tiers work at boundary amounts
- [ ] Vesting calculates correctly at cliff/mid-vest/post-vest timestamps

## Dependencies

- **Requires**: Wave 01 (Foundry project) only
- **Can run in parallel with**: Waves 02-08 (completely independent of backend/Android work)
- **Unblocks**: Wave 10 (Contract Testing)

## Technical Notes

- Smart contracts are the highest-stakes code in the project — bugs here can lose user funds
- Write contracts defensively: check-effects-interactions pattern, no re-entrancy, explicit access control
- OpenZeppelin libraries handle most security concerns — do not reinvent ERC-20 or access control
- Test against an Anvil local fork of Base mainnet to simulate real conditions before Sepolia deploy
