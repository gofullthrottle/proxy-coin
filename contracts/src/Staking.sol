// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import "@openzeppelin/contracts/access/AccessControl.sol";

interface IBurnableToken {
    function burn(uint256 amount) external;
}

/// @title PRXY Staking
/// @notice Stake PRXY tokens to increase earning multiplier, with fraud slashing
contract Staking is AccessControl {
    using SafeERC20 for IERC20;

    bytes32 public constant SLASHER_ROLE = keccak256("SLASHER_ROLE");

    IERC20 public token;

    struct Stake {
        uint256 amount;
        uint256 lockUntil; // timestamp
        uint256 tier;      // 0-3 (determines multiplier)
    }

    mapping(address => Stake) public stakes;

    // Tier thresholds and lock periods
    uint256[4] public tierThresholds = [
        1_000 * 1e18,   // Tier 0: 1,000 PRXY → 1.1x, 30 days
        5_000 * 1e18,   // Tier 1: 5,000 PRXY → 1.2x, 30 days
        10_000 * 1e18,  // Tier 2: 10,000 PRXY → 1.3x, 60 days
        50_000 * 1e18   // Tier 3: 50,000 PRXY → 1.5x, 90 days
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
