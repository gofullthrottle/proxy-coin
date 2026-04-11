---
decomposed_from: .claude/plans/2026-03-21-proxy-coin-full-implementation.md
date: "2026-03-21"
project: "proxy-coin"
wave: 11
task_id: "wallet-merkle"
created_at: "2026-03-21T00:00:00Z"
---

# Wave 11 — Wallet + Merkle Backend

**Phase**: 3 — Token Launch
**Estimated Total**: 16h
**Dependencies**: Wave 08 (Android UI complete) AND Wave 10 (contracts deployed to Sepolia)
**Agent Mix**: Android (4 tasks), Backend (2 tasks)

Android tasks 11.1-11.4 are sequential (each builds on prior). Backend tasks 11.5-11.6 are independent and can run in parallel with Android work.

---

## Tasks

### 11.1 — Android: WalletManager
- **Agent**: Android
- **Estimate**: 4h
- **Complexity**: Complex
- **Depends on**: 11.2 (KeystoreHelper needed for key storage)
- **Files**: `android/app/src/main/.../wallet/WalletManager.kt`

**Acceptance Criteria**:
- BIP-39 mnemonic generation (12 words) using `bitcoinj` or `web3j-crypto`
- BIP-44 key derivation path: `m/44'/60'/0'/0/0` (Ethereum)
- Ethereum keypair from derived private key
- Store private key encrypted in Android Keystore via `KeystoreHelper`
- `getAddress()` — returns EIP-55 checksummed address
- `getBalance()` — calls PRXY ERC-20 `balanceOf(address)` via Web3j
- `signMessage(message: ByteArray)` — personal sign
- RPC via Alchemy or Infura for Base L2 (configurable endpoint)

**Technical Notes**: Use Web3j for Ethereum operations. `Credentials` class from `web3j-crypto`. Private key must NEVER be logged or passed in plain text after initial generation.

---

### 11.2 — Android: KeystoreHelper
- **Agent**: Android
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 1.3 (Android project setup)
- **Files**: `android/app/src/main/.../security/KeystoreHelper.kt`

**Acceptance Criteria**:
- Android Keystore wrapper for AES-256-GCM encryption
- Generate AES key with `KeyPairGenerator` + `KeyGenParameterSpec` (hardware-backed when available)
- `encrypt(plaintext: ByteArray): EncryptedData` — returns ciphertext + IV
- `decrypt(encrypted: EncryptedData): ByteArray`
- Clear sensitive data from memory: zero-fill arrays after use
- Test: encrypt/decrypt round-trip on API 26+ emulator

**Technical Notes**: Use `KeyStore.getInstance("AndroidKeyStore")`. Key alias: `prxy_wallet_key`. Require user authentication for key usage (`setUserAuthenticationRequired(true)`) if biometric is available.

---

### 11.3 — Android: MnemonicGenerator + backup flow
- **Agent**: Android
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 11.1 (WalletManager generates mnemonic)
- **Files**: `android/app/src/main/.../ui/wallet/MnemonicBackupScreen.kt`, `MnemonicImportScreen.kt`

**Acceptance Criteria**:
- Generate BIP-39 mnemonic and display as word grid (12 words, 3×4)
- "Write this down" warning before showing mnemonic
- Backup confirmation: user must tap words in correct order (verify 3 random words)
- Import flow: paste 12-word phrase or raw private key (0x... format)
- Validate mnemonic checksum on import — reject invalid mnemonics
- After confirmation: persist wallet creation state in DataStore

**Technical Notes**: Never store the mnemonic itself — derive the private key immediately and store only the encrypted private key. Display only, then discard from memory.

---

### 11.4 — Android: TransactionBuilder
- **Agent**: Android
- **Estimate**: 2h
- **Complexity**: Standard
- **Depends on**: 11.1 (WalletManager for signing)
- **Files**: `android/app/src/main/.../wallet/TransactionBuilder.kt`

**Acceptance Criteria**:
- Build claim transaction: call `RewardDistributor.claim(cumulativeAmount, merkleProof)`
- Build send transaction: ERC-20 `transfer(to, amount)` on PRXY token
- Gas estimation: use `eth_estimateGas` via Web3j, add 20% buffer
- Gas price: use `eth_gasPrice` oracle, not hardcoded
- Wait for 1 block confirmation on Base (avg 2s block time)
- Return `TransactionReceipt` with status, hash, gas used

**Technical Notes**: Use Web3j `RawTransactionManager` with `Base` chain ID (8453) or Sepolia (84532). ABI-encode function calls manually or use Web3j code generation from ABI JSON.

---

### 11.5 — Backend: Reward calculator
- **Agent**: Backend
- **Estimate**: 3h
- **Complexity**: Standard
- **Depends on**: 2.3 (DB schema: earnings, metering), 3.6 (metering events)
- **Files**: `backend/internal/reward/calculator.go`

**Acceptance Criteria**:
- Hourly cron job: `internal/reward/calculator.go` runs via scheduler
- Aggregates metering data per node for the epoch (1 hour)
- Base reward: `bytes_served / 1_000_000 × PRXYPerMB` (configurable rate)
- Multipliers applied: trust (0.5x-2.0x), uptime (1.0x-1.5x), quality (0.8x-1.3x), staking tier (1.0x-1.5x)
- Insert into `earnings` table per epoch
- Update `claimable_rewards` cumulative total per node wallet
- Unit tests with known inputs and expected outputs

**Technical Notes**: `PRXYPerMB` configurable via node config. Trust multiplier: linear map from 0.3→0.5x, 1.0→2.0x. Quality: based on verification pass rate from fraud module.

---

### 11.6 — Backend: Merkle tree generator
- **Agent**: Backend
- **Estimate**: 3h
- **Complexity**: Standard
- **Depends on**: 11.5 (reward calculator populates claimable_rewards)
- **Files**: `backend/internal/reward/merkle.go`

**Acceptance Criteria**:
- Daily job: aggregates all pending rewards from `claimable_rewards` table
- Builds leaf nodes: `keccak256(abi.encodePacked(wallet_address, cumulative_amount))`
- Constructs Merkle tree (sorted pairs, standard Ethereum format)
- Stores proofs in database per wallet address (for claim API in Wave 12)
- Returns root hash
- Benchmark: < 5 minutes for 10,000 nodes
- Stores Merkle root history per epoch

**Technical Notes**: Use `wealdtech/go-merkletree` or implement manually. Sort leaves to ensure deterministic tree. Store as `[][]byte` for each proof level. Verify correctness: reconstruct random proofs and verify against root before publishing.

---

## Phase 3 — Wave 11 Success Gate

- [ ] WalletManager generates valid BIP-44 Ethereum keypair
- [ ] Private key encrypted in Android Keystore (hardware-backed on supported devices)
- [ ] Mnemonic backup flow completed without the mnemonic being stored
- [ ] TransactionBuilder submits a test claim on Base Sepolia
- [ ] Reward calculator produces correct amounts for known inputs
- [ ] Merkle tree generates and verifies correctly for 100+ nodes

## Dependencies

- **Requires**: Wave 08 (Android UI, provides screen infrastructure) AND Wave 10 (contracts on Sepolia)
- **Unblocks**: Wave 12 (Claim Flow + Wallet UI)

## Technical Notes

- WalletManager (11.1) and KeystoreHelper (11.2) contain the security-critical code — treat private key handling with extreme care
- Never log private keys, mnemonics, or derived key material
- Test on a physical Android device — hardware-backed Keystore is not available in emulators
- The reward calculator (11.5) will run hourly in production — ensure it handles DB failures gracefully and doesn't double-count epochs
