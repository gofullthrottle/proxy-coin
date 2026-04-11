// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "forge-std/Test.sol";
import "../src/ProxyCoinToken.sol";
import "../src/RewardDistributor.sol";
import "../src/Staking.sol";
import "../src/Vesting.sol";

/// @title Fuzz Tests for Proxy Coin contracts
/// @notice Property-based fuzz tests run with `forge test --fuzz-runs 1000`
contract FuzzTest is Test {
    ProxyCoinToken public token;
    RewardDistributor public distributor;
    Staking public staking;
    Vesting public vesting;

    address public admin = address(this);
    address public alice = address(0xA11CE);
    address public bob = address(0xB0B);
    address public slasher = address(0x5);

    // Tier thresholds (must match Staking.sol)
    uint256 constant TIER0 = 1_000 * 1e18;
    uint256 constant TIER1 = 5_000 * 1e18;
    uint256 constant TIER2 = 10_000 * 1e18;
    uint256 constant TIER3 = 50_000 * 1e18;

    // -------------------------------------------------------------------------
    // Setup
    // -------------------------------------------------------------------------

    function setUp() public {
        token = new ProxyCoinToken(admin);
        distributor = new RewardDistributor(address(token));
        staking = new Staking(address(token));
        vesting = new Vesting(address(token));

        token.grantRole(token.MINTER_ROLE(), admin);
        token.grantRole(token.MINTER_ROLE(), address(distributor));
        token.grantRole(token.BURNER_ROLE(), address(staking));

        staking.grantRole(staking.SLASHER_ROLE(), slasher);
    }

    // =========================================================================
    // ProxyCoinToken fuzz tests
    // =========================================================================

    /// @notice mint(amount) MUST revert if it would push totalSupply over MAX_SUPPLY;
    ///         otherwise totalSupply stays at or below MAX_SUPPLY.
    function testFuzz_MintCapped(uint256 amount) public {
        uint256 maxSupply = token.MAX_SUPPLY();

        // Bound to values that either fit or slightly exceed max supply
        amount = bound(amount, 0, maxSupply + 1);

        if (token.totalSupply() + amount > maxSupply) {
            vm.expectRevert("Exceeds max supply");
            token.mint(alice, amount);
        } else {
            token.mint(alice, amount);
            assertLe(token.totalSupply(), maxSupply);
        }
    }

    /// @notice Total supply never exceeds MAX_SUPPLY regardless of sequential mints.
    function testFuzz_TotalSupplyNeverExceedsMax(uint256 a, uint256 b) public {
        uint256 maxSupply = token.MAX_SUPPLY();
        a = bound(a, 0, maxSupply);
        b = bound(b, 0, maxSupply);

        token.mint(alice, a);

        if (token.totalSupply() + b > maxSupply) {
            vm.expectRevert("Exceeds max supply");
            token.mint(bob, b);
        } else {
            token.mint(bob, b);
        }

        assertLe(token.totalSupply(), maxSupply);
    }

    /// @notice Transfer preserves total balances: balanceOf(from) + balanceOf(to) unchanged.
    function testFuzz_TransferPreservesSum(uint256 mintAmount, uint256 transferAmount) public {
        mintAmount = bound(mintAmount, 1, token.MAX_SUPPLY());
        transferAmount = bound(transferAmount, 0, mintAmount);

        token.mint(alice, mintAmount);

        uint256 sumBefore = token.balanceOf(alice) + token.balanceOf(bob);

        vm.prank(alice);
        token.transfer(bob, transferAmount);

        uint256 sumAfter = token.balanceOf(alice) + token.balanceOf(bob);
        assertEq(sumAfter, sumBefore);
    }

    // =========================================================================
    // RewardDistributor fuzz tests
    // =========================================================================

    /// @notice Claimed amount after a valid claim equals exactly cumulativeAmount (first claim).
    function testFuzz_ClaimBounded(uint256 amount) public {
        amount = bound(amount, 1, token.MAX_SUPPLY() / 2);

        // Build a 2-leaf tree: alice with `amount`, bob with a different amount
        uint256 bobAmount = amount + 1;
        (bytes32 root, bytes32[] memory aliceProof, ) = _buildTree(
            alice,
            amount,
            bob,
            bobAmount
        );
        distributor.setMerkleRoot(root);

        vm.prank(alice);
        distributor.claim(amount, aliceProof);

        // claimed[alice] == amount (cumulative)
        assertEq(distributor.claimed(alice), amount);
        // token balance == amount
        assertEq(token.balanceOf(alice), amount);
    }

    /// @notice After claim, getClaimable returns 0 for the same cumulative amount.
    function testFuzz_GetClaimable_ZeroAfterFullClaim(uint256 amount) public {
        amount = bound(amount, 1, token.MAX_SUPPLY() / 2);

        uint256 bobAmount = amount + 7;
        (bytes32 root, bytes32[] memory aliceProof, ) = _buildTree(
            alice,
            amount,
            bob,
            bobAmount
        );
        distributor.setMerkleRoot(root);

        vm.prank(alice);
        distributor.claim(amount, aliceProof);

        uint256 claimable = distributor.getClaimable(alice, amount, aliceProof);
        assertEq(claimable, 0);
    }

    // =========================================================================
    // Staking fuzz tests
    // =========================================================================

    /// @notice Stake amount maps to the correct tier per the contract's thresholds.
    function testFuzz_StakeTier(uint256 amount) public {
        // Only test valid staking amounts (>= TIER0)
        amount = bound(amount, TIER0, 99_999 * 1e18);

        // Mint enough for the staker
        token.mint(alice, amount);

        vm.startPrank(alice);
        token.approve(address(staking), amount);
        staking.stake(amount);
        vm.stopPrank();

        Staking.Stake memory s = staking.getStake(alice);
        assertEq(s.amount, amount);

        uint256 expectedTier;
        if (amount >= TIER3) {
            expectedTier = 3;
        } else if (amount >= TIER2) {
            expectedTier = 2;
        } else if (amount >= TIER1) {
            expectedTier = 1;
        } else {
            expectedTier = 0;
        }

        assertEq(s.tier, expectedTier);
    }

    /// @notice Below-minimum stake always reverts.
    function testFuzz_StakeBelowMinimumReverts(uint256 amount) public {
        amount = bound(amount, 1, TIER0 - 1);

        token.mint(alice, amount);

        vm.startPrank(alice);
        token.approve(address(staking), amount);
        vm.expectRevert("Below minimum stake");
        staking.stake(amount);
        vm.stopPrank();
    }

    /// @notice lockUntil is always in the future when stake is created.
    function testFuzz_LockUntilInFuture(uint256 amount) public {
        amount = bound(amount, TIER0, 99_999 * 1e18);

        token.mint(alice, amount);

        vm.startPrank(alice);
        token.approve(address(staking), amount);
        staking.stake(amount);
        vm.stopPrank();

        Staking.Stake memory s = staking.getStake(alice);
        assertGt(s.lockUntil, block.timestamp);
    }

    /// @notice Slash reduces stake by exactly 50% and burns the same amount.
    function testFuzz_SlashIsHalf(uint256 amount) public {
        amount = bound(amount, TIER0, 99_999 * 1e18);
        // Ensure even amount for clean 50% check
        // (odd amounts round down due to integer division — that's acceptable)

        token.mint(alice, amount);

        vm.startPrank(alice);
        token.approve(address(staking), amount);
        staking.stake(amount);
        vm.stopPrank();

        uint256 supplyBefore = token.totalSupply();
        uint256 expectedSlash = (amount * 50) / 100;

        vm.prank(slasher);
        staking.slash(alice, "fuzz");

        Staking.Stake memory s = staking.getStake(alice);
        assertEq(s.amount, amount - expectedSlash);
        assertEq(token.totalSupply(), supplyBefore - expectedSlash);
    }

    // =========================================================================
    // Vesting fuzz tests
    // =========================================================================

    /// @notice Vested amount is always <= total at any point in time.
    function testFuzz_VestedBounded(uint256 elapsed) public {
        uint256 total = 1_000_000 * 1e18;
        uint256 cliff = 90 days;
        uint256 duration = 365 days;

        // Bound elapsed to a reasonable range (0 to 2× duration)
        elapsed = bound(elapsed, 0, duration * 2);

        token.mint(address(vesting), total);
        uint256 startTs = block.timestamp;

        vesting.createSchedule(
            alice,
            total,
            startTs,
            cliff,
            duration,
            false
        );

        vm.warp(startTs + elapsed);

        uint256 releasable = vesting.getReleasable(alice);
        assertLe(releasable, total);

        // If we release, the cumulative released should also be <= total
        if (releasable > 0) {
            vm.prank(alice);
            vesting.release();

            (, uint256 released, , , , , ) = vesting.schedules(alice);
            assertLe(released, total);
        }
    }

    /// @notice Releasing never produces more tokens than the schedule total.
    function testFuzz_ReleaseNeverExceedsTotal(uint256 total, uint256 elapsed) public {
        total = bound(total, 1e18, token.MAX_SUPPLY() / 10);
        uint256 duration = 365 days;
        uint256 cliff = 30 days;
        elapsed = bound(elapsed, cliff, duration * 3); // always past cliff

        token.mint(address(vesting), total);
        uint256 startTs = block.timestamp;

        vesting.createSchedule(alice, total, startTs, cliff, duration, false);

        vm.warp(startTs + elapsed);

        uint256 balanceBefore = token.balanceOf(alice);
        uint256 releasable = vesting.getReleasable(alice);

        if (releasable > 0) {
            vm.prank(alice);
            vesting.release();
        }

        assertLe(token.balanceOf(alice) - balanceBefore, total);
    }

    /// @notice Before cliff, releasable is always 0.
    function testFuzz_BeforeCliff_NothingReleasable(uint256 elapsed) public {
        uint256 total = 500_000 * 1e18;
        uint256 cliff = 180 days;
        uint256 duration = 730 days;

        elapsed = bound(elapsed, 0, cliff - 1);

        token.mint(address(vesting), total);
        uint256 startTs = block.timestamp;

        vesting.createSchedule(alice, total, startTs, cliff, duration, false);

        vm.warp(startTs + elapsed);

        assertEq(vesting.getReleasable(alice), 0);
    }

    // =========================================================================
    // Internal Merkle tree helper (mirrors RewardDistributor.t.sol)
    // =========================================================================

    function _buildTree(
        address addr1,
        uint256 amt1,
        address addr2,
        uint256 amt2
    )
        internal
        pure
        returns (
            bytes32 root,
            bytes32[] memory proof1,
            bytes32[] memory proof2
        )
    {
        bytes32 leaf1 = keccak256(
            bytes.concat(keccak256(abi.encode(addr1, amt1)))
        );
        bytes32 leaf2 = keccak256(
            bytes.concat(keccak256(abi.encode(addr2, amt2)))
        );

        if (leaf1 <= leaf2) {
            root = keccak256(abi.encodePacked(leaf1, leaf2));
        } else {
            root = keccak256(abi.encodePacked(leaf2, leaf1));
        }

        proof1 = new bytes32[](1);
        proof1[0] = leaf2;

        proof2 = new bytes32[](1);
        proof2[0] = leaf1;
    }
}
