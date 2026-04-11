// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "forge-std/Test.sol";
import "../src/ProxyCoinToken.sol";
import "../src/Staking.sol";

contract StakingTest is Test {
    ProxyCoinToken public token;
    Staking public staking;

    address public admin = address(this);
    address public slasher = address(0x5);
    address public staker = address(0x6);

    uint256 constant STAKER_BALANCE = 100_000 * 1e18;

    // -------------------------------------------------------------------------
    // Setup
    // -------------------------------------------------------------------------

    function setUp() public {
        token = new ProxyCoinToken(admin);
        staking = new Staking(address(token));

        token.grantRole(token.MINTER_ROLE(), admin);
        token.grantRole(token.BURNER_ROLE(), address(staking));
        staking.grantRole(staking.SLASHER_ROLE(), slasher);

        token.mint(staker, STAKER_BALANCE);
    }

    // -------------------------------------------------------------------------
    // Internal helper: stake on behalf of `staker`
    // -------------------------------------------------------------------------

    function _stake(uint256 amount) internal {
        vm.startPrank(staker);
        token.approve(address(staking), amount);
        staking.stake(amount);
        vm.stopPrank();
    }

    // -------------------------------------------------------------------------
    // Tier assignment
    // -------------------------------------------------------------------------

    function test_StakeTier0() public {
        uint256 amount = 1_000 * 1e18; // exactly Tier 0 minimum
        _stake(amount);

        Staking.Stake memory s = staking.getStake(staker);
        assertEq(s.amount, amount);
        assertEq(s.tier, 0);
        assertEq(s.lockUntil, block.timestamp + 30 days);
    }

    function test_StakeTier0_Above() public {
        uint256 amount = 4_999 * 1e18; // below Tier 1 threshold
        _stake(amount);

        Staking.Stake memory s = staking.getStake(staker);
        assertEq(s.tier, 0);
    }

    function test_StakeTier1() public {
        uint256 amount = 5_000 * 1e18;
        _stake(amount);

        Staking.Stake memory s = staking.getStake(staker);
        assertEq(s.amount, amount);
        assertEq(s.tier, 1);
        assertEq(s.lockUntil, block.timestamp + 30 days);
    }

    function test_StakeTier2() public {
        uint256 amount = 10_000 * 1e18;
        _stake(amount);

        Staking.Stake memory s = staking.getStake(staker);
        assertEq(s.amount, amount);
        assertEq(s.tier, 2);
        assertEq(s.lockUntil, block.timestamp + 60 days);
    }

    function test_StakeTier3() public {
        uint256 amount = 50_000 * 1e18;
        _stake(amount);

        Staking.Stake memory s = staking.getStake(staker);
        assertEq(s.amount, amount);
        assertEq(s.tier, 3);
        assertEq(s.lockUntil, block.timestamp + 90 days);
    }

    function test_StakeTier3_Above() public {
        uint256 amount = 99_999 * 1e18; // above Tier 3 threshold, still Tier 3
        _stake(amount);

        Staking.Stake memory s = staking.getStake(staker);
        assertEq(s.tier, 3);
        assertEq(s.lockUntil, block.timestamp + 90 days);
    }

    // -------------------------------------------------------------------------
    // Stake — failure cases
    // -------------------------------------------------------------------------

    function test_StakeBelowMinimum() public {
        uint256 amount = 999 * 1e18; // below 1_000 PRXY minimum

        vm.startPrank(staker);
        token.approve(address(staking), amount);
        vm.expectRevert("Below minimum stake");
        staking.stake(amount);
        vm.stopPrank();
    }

    function test_StakeZero() public {
        vm.startPrank(staker);
        token.approve(address(staking), 0);
        vm.expectRevert("Cannot stake 0");
        staking.stake(0);
        vm.stopPrank();
    }

    function test_AlreadyStaked() public {
        _stake(1_000 * 1e18);

        // Second stake attempt without unstaking first must revert
        vm.startPrank(staker);
        token.approve(address(staking), 1_000 * 1e18);
        vm.expectRevert("Already staked, unstake first");
        staking.stake(1_000 * 1e18);
        vm.stopPrank();
    }

    function test_StakeTransfersTokens() public {
        uint256 amount = 5_000 * 1e18;
        uint256 balanceBefore = token.balanceOf(staker);

        _stake(amount);

        assertEq(token.balanceOf(staker), balanceBefore - amount);
        assertEq(token.balanceOf(address(staking)), amount);
    }

    // -------------------------------------------------------------------------
    // Unstake
    // -------------------------------------------------------------------------

    function test_UnstakeAfterLock() public {
        uint256 amount = 1_000 * 1e18; // Tier 0, 30-day lock
        _stake(amount);

        uint256 balanceBefore = token.balanceOf(staker);

        // Advance past lock period
        vm.warp(block.timestamp + 31 days);

        vm.prank(staker);
        staking.unstake();

        assertEq(token.balanceOf(staker), balanceBefore + amount);

        // Stake record cleared
        Staking.Stake memory s = staking.getStake(staker);
        assertEq(s.amount, 0);
        assertEq(s.tier, 0);
        assertEq(s.lockUntil, 0);
    }

    function test_UnstakeBeforeLock() public {
        _stake(1_000 * 1e18); // Tier 0, 30-day lock

        // Try to unstake 1 second before lock expires
        vm.warp(block.timestamp + 30 days - 1);

        vm.prank(staker);
        vm.expectRevert("Still locked");
        staking.unstake();
    }

    function test_UnstakeExactlyAtLockExpiry() public {
        _stake(1_000 * 1e18); // 30-day lock

        vm.warp(block.timestamp + 30 days);

        vm.prank(staker);
        staking.unstake();

        assertEq(token.balanceOf(staker), STAKER_BALANCE);
    }

    function test_UnstakeNoStake() public {
        vm.prank(staker);
        vm.expectRevert("No stake");
        staking.unstake();
    }

    function test_UnstakeEmitsEvent() public {
        uint256 amount = 1_000 * 1e18;
        _stake(amount);
        vm.warp(block.timestamp + 31 days);

        vm.expectEmit(true, false, false, true);
        emit Staking.Unstaked(staker, amount);

        vm.prank(staker);
        staking.unstake();
    }

    function test_StakeAfterUnstake() public {
        _stake(1_000 * 1e18);
        vm.warp(block.timestamp + 31 days);

        vm.prank(staker);
        staking.unstake();

        // Should be able to stake again
        _stake(5_000 * 1e18);
        Staking.Stake memory s = staking.getStake(staker);
        assertEq(s.tier, 1);
    }

    // -------------------------------------------------------------------------
    // Slash
    // -------------------------------------------------------------------------

    function test_Slash() public {
        uint256 amount = 10_000 * 1e18; // Tier 2
        _stake(amount);

        uint256 supplyBefore = token.totalSupply();
        uint256 expectedSlash = (amount * 50) / 100; // 50%

        vm.prank(slasher);
        staking.slash(staker, "fraud detected");

        Staking.Stake memory s = staking.getStake(staker);
        assertEq(s.amount, amount - expectedSlash);

        // Slashed tokens burned — total supply reduced
        assertEq(token.totalSupply(), supplyBefore - expectedSlash);
    }

    function test_SlashEmitsEvent() public {
        uint256 amount = 10_000 * 1e18;
        _stake(amount);

        uint256 expectedSlash = (amount * 50) / 100;

        vm.expectEmit(true, false, false, true);
        emit Staking.Slashed(staker, expectedSlash, "test");

        vm.prank(slasher);
        staking.slash(staker, "test");
    }

    function test_SlashUnauthorized() public {
        _stake(1_000 * 1e18);

        vm.prank(staker); // staker does not have SLASHER_ROLE
        vm.expectRevert();
        staking.slash(staker, "unauthorized attempt");
    }

    function test_SlashNoStake() public {
        vm.prank(slasher);
        vm.expectRevert("No stake to slash");
        staking.slash(staker, "no stake");
    }

    function test_SlashTier3() public {
        uint256 amount = 50_000 * 1e18; // Tier 3
        _stake(amount);

        vm.prank(slasher);
        staking.slash(staker, "fraud");

        Staking.Stake memory s = staking.getStake(staker);
        assertEq(s.amount, 25_000 * 1e18);
    }

    // -------------------------------------------------------------------------
    // getStake
    // -------------------------------------------------------------------------

    function test_GetStake() public {
        uint256 amount = 5_000 * 1e18;
        uint256 ts = block.timestamp;
        _stake(amount);

        Staking.Stake memory s = staking.getStake(staker);
        assertEq(s.amount, amount);
        assertEq(s.tier, 1);
        assertEq(s.lockUntil, ts + 30 days);
    }

    function test_GetStake_NoStake() public view {
        Staking.Stake memory s = staking.getStake(staker);
        assertEq(s.amount, 0);
        assertEq(s.tier, 0);
        assertEq(s.lockUntil, 0);
    }

    // -------------------------------------------------------------------------
    // Roles
    // -------------------------------------------------------------------------

    function test_SlasherRole() public view {
        assertTrue(staking.hasRole(staking.SLASHER_ROLE(), slasher));
    }

    function test_AdminRole() public view {
        assertTrue(staking.hasRole(staking.DEFAULT_ADMIN_ROLE(), admin));
    }

    function test_SlashPercentConstant() public view {
        assertEq(staking.SLASH_PERCENT(), 50);
    }

    // -------------------------------------------------------------------------
    // Token transfer integrity
    // -------------------------------------------------------------------------

    function test_StakingContractHoldsTokens() public {
        _stake(10_000 * 1e18);
        assertEq(token.balanceOf(address(staking)), 10_000 * 1e18);
    }

    function test_UnstakeReturnsExactAmount() public {
        uint256 amount = 50_000 * 1e18; // Tier 3
        _stake(amount);
        vm.warp(block.timestamp + 91 days);

        uint256 beforeUnstake = token.balanceOf(staker);
        vm.prank(staker);
        staking.unstake();
        assertEq(token.balanceOf(staker), beforeUnstake + amount);
    }
}
