// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

/// @title Token Vesting
/// @notice Linear vesting for team, investors, and treasury tokens
contract Vesting is Ownable {
    using SafeERC20 for IERC20;

    IERC20 public token;

    struct Schedule {
        uint256 total;     // Total tokens to vest
        uint256 released;  // Already released
        uint256 start;     // Vesting start timestamp
        uint256 cliff;     // Cliff duration (seconds)
        uint256 duration;  // Total vesting duration (seconds)
        bool revocable;    // Can owner revoke unvested tokens
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
