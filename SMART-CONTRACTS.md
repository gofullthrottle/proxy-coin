# Proxy Coin - Smart Contract Specifications

## Overview

Four core contracts deployed on Base L2 (Coinbase), built with Foundry (Solidity 0.8.24+).

```
┌──────────────────┐
│ ProxyCoinToken   │  ERC-20 token with mint/burn
│ (PRXY)           │
└────────┬─────────┘
         │ transfers
┌────────▼─────────┐     ┌──────────────────┐
│ RewardDistributor│     │ Staking          │
│                  │     │                  │
│ Merkle claims    │     │ Lock PRXY for    │
│ Batch rewards    │     │ earning boost    │
└──────────────────┘     │ Slashing         │
                         └──────────────────┘
┌──────────────────┐
│ Vesting          │
│                  │
│ Team/investor    │
│ token locks      │
└──────────────────┘
```

## 1. ProxyCoinToken.sol

Standard ERC-20 with controlled minting and burning.

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/access/AccessControl.sol";

contract ProxyCoinToken is ERC20, AccessControl {
    bytes32 public constant MINTER_ROLE = keccak256("MINTER_ROLE");
    bytes32 public constant BURNER_ROLE = keccak256("BURNER_ROLE");

    uint256 public constant MAX_SUPPLY = 1_000_000_000 * 1e18; // 1 billion

    constructor(address admin) ERC20("Proxy Coin", "PRXY") {
        _grantRole(DEFAULT_ADMIN_ROLE, admin);
    }

    function mint(address to, uint256 amount) external onlyRole(MINTER_ROLE) {
        require(totalSupply() + amount <= MAX_SUPPLY, "Exceeds max supply");
        _mint(to, amount);
    }

    function burn(uint256 amount) external onlyRole(BURNER_ROLE) {
        _burn(msg.sender, amount);
    }
}
```

**Key properties**:
- Hard cap: 1 billion tokens (enforced in contract)
- MINTER_ROLE: granted to RewardDistributor and Vesting contracts
- BURNER_ROLE: granted to Staking contract (for slashing burns)
- Admin can grant/revoke roles (eventually transferred to DAO multisig)

## 2. RewardDistributor.sol

Merkle-tree-based batch reward distribution. Gas-efficient: one on-chain root update serves thousands of claims.

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "@openzeppelin/contracts/utils/cryptography/MerkleProof.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

interface IProxyCoinToken {
    function mint(address to, uint256 amount) external;
}

contract RewardDistributor is Ownable {
    IProxyCoinToken public token;

    bytes32 public merkleRoot;
    uint256 public epoch;  // incremented each time root is updated

    // Tracks cumulative claimed amount per wallet
    mapping(address => uint256) public claimed;

    event MerkleRootUpdated(uint256 indexed epoch, bytes32 root);
    event RewardsClaimed(address indexed wallet, uint256 amount, uint256 epoch);

    constructor(address _token) Ownable(msg.sender) {
        token = IProxyCoinToken(_token);
    }

    /// @notice Update the Merkle root (called daily by backend)
    /// @param _root New Merkle root
    function setMerkleRoot(bytes32 _root) external onlyOwner {
        merkleRoot = _root;
        epoch++;
        emit MerkleRootUpdated(epoch, _root);
    }

    /// @notice Claim rewards using Merkle proof
    /// @param cumulativeAmount Total earned to date (not just this claim)
    /// @param merkleProof Proof of inclusion in Merkle tree
    function claim(
        uint256 cumulativeAmount,
        bytes32[] calldata merkleProof
    ) external {
        // Verify proof
        bytes32 leaf = keccak256(
            bytes.concat(
                keccak256(abi.encode(msg.sender, cumulativeAmount))
            )
        );
        require(
            MerkleProof.verify(merkleProof, merkleRoot, leaf),
            "Invalid proof"
        );

        // Calculate amount to mint (cumulative - already claimed)
        uint256 claimable = cumulativeAmount - claimed[msg.sender];
        require(claimable > 0, "Nothing to claim");

        // Update claimed amount
        claimed[msg.sender] = cumulativeAmount;

        // Mint tokens to claimer
        token.mint(msg.sender, claimable);

        emit RewardsClaimed(msg.sender, claimable, epoch);
    }

    /// @notice Check claimable amount for a wallet
    function getClaimable(
        address wallet,
        uint256 cumulativeAmount,
        bytes32[] calldata merkleProof
    ) external view returns (uint256) {
        bytes32 leaf = keccak256(
            bytes.concat(
                keccak256(abi.encode(wallet, cumulativeAmount))
            )
        );
        if (!MerkleProof.verify(merkleProof, merkleRoot, leaf)) {
            return 0;
        }
        return cumulativeAmount - claimed[wallet];
    }
}
```

**How it works**:
1. Backend aggregates earnings per wallet
2. Backend builds Merkle tree: leaf = hash(wallet, cumulativeEarnings)
3. Backend calls `setMerkleRoot(root)` once per day
4. Users call `claim(cumulativeAmount, proof)` to mint their earnings
5. Contract verifies proof and mints `cumulativeAmount - alreadyClaimed`

**Why cumulative (not per-epoch)**:
- User can skip days and claim everything in one transaction
- No "use it or lose it" — earnings never expire
- Simpler accounting (no epoch tracking per user)

## 3. Staking.sol

Users stake PRXY to increase their earning multiplier. Staked tokens can be slashed for fraud.

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import "@openzeppelin/contracts/access/AccessControl.sol";

interface IBurnableToken {
    function burn(uint256 amount) external;
}

contract Staking is AccessControl {
    using SafeERC20 for IERC20;

    bytes32 public constant SLASHER_ROLE = keccak256("SLASHER_ROLE");

    IERC20 public token;

    struct Stake {
        uint256 amount;
        uint256 lockUntil;    // timestamp
        uint256 tier;         // 0-3 (determines multiplier)
    }

    mapping(address => Stake) public stakes;

    // Tier thresholds and lock periods
    uint256[4] public tierThresholds = [
        1_000 * 1e18,    // Tier 0: 1,000 PRXY → 1.1x, 30 days
        5_000 * 1e18,    // Tier 1: 5,000 PRXY → 1.2x, 30 days
        10_000 * 1e18,   // Tier 2: 10,000 PRXY → 1.3x, 60 days
        50_000 * 1e18    // Tier 3: 50,000 PRXY → 1.5x, 90 days
    ];

    uint256[4] public lockDurations = [
        30 days,
        30 days,
        60 days,
        90 days
    ];

    uint256 public constant SLASH_PERCENT = 50; // 50% slashed on fraud

    event Staked(address indexed staker, uint256 amount, uint256 tier);
    event Unstaked(address indexed staker, uint256 amount);
    event Slashed(address indexed staker, uint256 amount, string reason);

    constructor(address _token) {
        token = IERC20(_token);
        _grantRole(DEFAULT_ADMIN_ROLE, msg.sender);
    }

    function stake(uint256 amount) external {
        require(amount > 0, "Cannot stake 0");
        require(stakes[msg.sender].amount == 0, "Already staked, unstake first");

        // Determine tier
        uint256 tier = _getTier(amount);

        // Transfer tokens to contract
        token.safeTransferFrom(msg.sender, address(this), amount);

        stakes[msg.sender] = Stake({
            amount: amount,
            lockUntil: block.timestamp + lockDurations[tier],
            tier: tier
        });

        emit Staked(msg.sender, amount, tier);
    }

    function unstake() external {
        Stake storage s = stakes[msg.sender];
        require(s.amount > 0, "No stake");
        require(block.timestamp >= s.lockUntil, "Still locked");

        uint256 amount = s.amount;
        delete stakes[msg.sender];

        token.safeTransfer(msg.sender, amount);
        emit Unstaked(msg.sender, amount);
    }

    /// @notice Slash a staker's tokens (called by fraud detection backend)
    function slash(address staker, string calldata reason)
        external
        onlyRole(SLASHER_ROLE)
    {
        Stake storage s = stakes[staker];
        require(s.amount > 0, "No stake to slash");

        uint256 slashAmount = (s.amount * SLASH_PERCENT) / 100;
        s.amount -= slashAmount;

        // Burn slashed tokens
        IBurnableToken(address(token)).burn(slashAmount);

        emit Slashed(staker, slashAmount, reason);
    }

    function getStake(address staker) external view returns (Stake memory) {
        return stakes[staker];
    }

    function _getTier(uint256 amount) internal view returns (uint256) {
        for (uint256 i = 3; i > 0; i--) {
            if (amount >= tierThresholds[i]) return i;
        }
        if (amount >= tierThresholds[0]) return 0;
        revert("Below minimum stake");
    }
}
```

## 4. Vesting.sol

Linear vesting for team, investors, and treasury tokens.

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

contract Vesting is Ownable {
    using SafeERC20 for IERC20;

    IERC20 public token;

    struct Schedule {
        uint256 total;          // Total tokens to vest
        uint256 released;       // Already released
        uint256 start;          // Vesting start timestamp
        uint256 cliff;          // Cliff duration (seconds)
        uint256 duration;       // Total vesting duration (seconds)
        bool revocable;         // Can owner revoke unvested tokens
        bool revoked;
    }

    mapping(address => Schedule) public schedules;

    event VestingCreated(address indexed beneficiary, uint256 amount);
    event TokensReleased(address indexed beneficiary, uint256 amount);
    event VestingRevoked(address indexed beneficiary);

    constructor(address _token) Ownable(msg.sender) {
        token = IERC20(_token);
    }

    function createSchedule(
        address beneficiary,
        uint256 total,
        uint256 start,
        uint256 cliffDuration,
        uint256 vestingDuration,
        bool revocable
    ) external onlyOwner {
        require(schedules[beneficiary].total == 0, "Schedule exists");
        require(total > 0, "Zero amount");
        require(vestingDuration > 0, "Zero duration");

        schedules[beneficiary] = Schedule({
            total: total,
            released: 0,
            start: start,
            cliff: cliffDuration,
            duration: vestingDuration,
            revocable: revocable,
            revoked: false
        });

        emit VestingCreated(beneficiary, total);
    }

    function release() external {
        Schedule storage schedule = schedules[msg.sender];
        require(schedule.total > 0, "No schedule");
        require(!schedule.revoked, "Revoked");

        uint256 vested = _vestedAmount(schedule);
        uint256 releasable = vested - schedule.released;
        require(releasable > 0, "Nothing to release");

        schedule.released += releasable;
        token.safeTransfer(msg.sender, releasable);

        emit TokensReleased(msg.sender, releasable);
    }

    function revoke(address beneficiary) external onlyOwner {
        Schedule storage schedule = schedules[beneficiary];
        require(schedule.revocable, "Not revocable");
        require(!schedule.revoked, "Already revoked");

        uint256 vested = _vestedAmount(schedule);
        uint256 unvested = schedule.total - vested;

        schedule.revoked = true;
        schedule.total = vested;

        // Return unvested tokens to owner
        if (unvested > 0) {
            token.safeTransfer(owner(), unvested);
        }

        emit VestingRevoked(beneficiary);
    }

    function getReleasable(address beneficiary) external view returns (uint256) {
        Schedule storage schedule = schedules[beneficiary];
        if (schedule.revoked) return 0;
        return _vestedAmount(schedule) - schedule.released;
    }

    function _vestedAmount(Schedule storage schedule) internal view returns (uint256) {
        if (block.timestamp < schedule.start + schedule.cliff) {
            return 0; // Before cliff
        }

        uint256 elapsed = block.timestamp - schedule.start;
        if (elapsed >= schedule.duration) {
            return schedule.total; // Fully vested
        }

        return (schedule.total * elapsed) / schedule.duration;
    }
}
```

## Deployment Plan

### Testnet Deployment (Base Sepolia)

```bash
# 1. Deploy token
forge create src/ProxyCoinToken.sol:ProxyCoinToken \
  --constructor-args $ADMIN_ADDRESS \
  --rpc-url $BASE_SEPOLIA_RPC \
  --private-key $DEPLOYER_KEY

# 2. Deploy RewardDistributor
forge create src/RewardDistributor.sol:RewardDistributor \
  --constructor-args $TOKEN_ADDRESS \
  --rpc-url $BASE_SEPOLIA_RPC \
  --private-key $DEPLOYER_KEY

# 3. Deploy Staking
forge create src/Staking.sol:Staking \
  --constructor-args $TOKEN_ADDRESS \
  --rpc-url $BASE_SEPOLIA_RPC \
  --private-key $DEPLOYER_KEY

# 4. Deploy Vesting
forge create src/Vesting.sol:Vesting \
  --constructor-args $TOKEN_ADDRESS \
  --rpc-url $BASE_SEPOLIA_RPC \
  --private-key $DEPLOYER_KEY

# 5. Grant roles
cast send $TOKEN_ADDRESS "grantRole(bytes32,address)" \
  $(cast keccak "MINTER_ROLE") $REWARD_DISTRIBUTOR_ADDRESS

cast send $TOKEN_ADDRESS "grantRole(bytes32,address)" \
  $(cast keccak "BURNER_ROLE") $STAKING_ADDRESS
```

### Mainnet Deployment Checklist

- [ ] All tests passing (`forge test`)
- [ ] Gas optimization review
- [ ] Security audit by reputable firm (Trail of Bits, OpenZeppelin, Spearbit)
- [ ] Formal verification of critical paths (claim, slash)
- [ ] Admin keys in multisig (Gnosis Safe on Base)
- [ ] Timelock on admin functions (48h delay)
- [ ] Emergency pause mechanism
- [ ] Deployment script tested on fork (`forge script --fork-url`)
- [ ] Contract verification on Basescan

### Contract Addresses (to be filled after deployment)

| Contract | Testnet (Base Sepolia) | Mainnet (Base) |
|----------|----------------------|----------------|
| ProxyCoinToken | TBD | TBD |
| RewardDistributor | TBD | TBD |
| Staking | TBD | TBD |
| Vesting | TBD | TBD |

## Foundry Project Config

```toml
# foundry.toml
[profile.default]
src = "src"
out = "out"
libs = ["lib"]
solc = "0.8.24"
optimizer = true
optimizer_runs = 200
evm_version = "cancun"

[profile.default.fuzz]
runs = 1000

[rpc_endpoints]
base_sepolia = "${BASE_SEPOLIA_RPC}"
base_mainnet = "${BASE_MAINNET_RPC}"

[etherscan]
base_sepolia = { key = "${BASESCAN_API_KEY}", chain = 84532 }
base_mainnet = { key = "${BASESCAN_API_KEY}", chain = 8453 }
```

## Security Considerations

1. **Reentrancy**: All state changes before external calls (checks-effects-interactions)
2. **Integer overflow**: Solidity 0.8+ has built-in overflow checks
3. **Access control**: Role-based (OpenZeppelin AccessControl)
4. **Merkle proof**: Using OpenZeppelin's audited MerkleProof library
5. **Flash loan attacks**: Not applicable (no price oracle dependency)
6. **Front-running**: Claim function is user-specific (only msg.sender can claim their rewards)
7. **Upgrade path**: Contracts are NOT upgradeable by design — simplicity and trust. If critical bugs found, deploy new contracts and migrate via governance vote.
