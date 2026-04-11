// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "forge-std/Test.sol";
import "../src/ProxyCoinToken.sol";
import "../src/Vesting.sol";

contract VestingTest is Test {
    ProxyCoinToken public token;
    Vesting public vesting;

    address public admin = address(this);
    address public beneficiary = address(0x7);

    uint256 constant TOTAL = 100_000 * 1e18;
    uint256 constant CLIFF = 180 days;    // 6 months
    uint256 constant DURATION = 730 days; // 2 years

    // -------------------------------------------------------------------------
    // Setup
    // -------------------------------------------------------------------------

    function setUp() public {
        token = new ProxyCoinToken(admin);
        vesting = new Vesting(address(token));

        token.grantRole(token.MINTER_ROLE(), admin);
        token.mint(address(vesting), TOTAL);
    }

    // -------------------------------------------------------------------------
    // Helpers
    // -------------------------------------------------------------------------

    function _createDefaultSchedule(bool revocable) internal {
        vesting.createSchedule(
            beneficiary,
            TOTAL,
            block.timestamp, // start = now
            CLIFF,
            DURATION,
            revocable
        );
    }

    // -------------------------------------------------------------------------
    // createSchedule
    // -------------------------------------------------------------------------

    function test_CreateSchedule() public {
        _createDefaultSchedule(false);

        (
            uint256 total,
            uint256 released,
            uint256 start,
            uint256 cliff,
            uint256 duration,
            bool revocable,
            bool revoked
        ) = vesting.schedules(beneficiary);

        assertEq(total, TOTAL);
        assertEq(released, 0);
        assertEq(start, block.timestamp);
        assertEq(cliff, CLIFF);
        assertEq(duration, DURATION);
        assertFalse(revocable);
        assertFalse(revoked);
    }

    function test_CreateScheduleEmitsEvent() public {
        vm.expectEmit(true, false, false, true);
        emit Vesting.VestingCreated(beneficiary, TOTAL);
        _createDefaultSchedule(false);
    }

    function test_CreateScheduleDuplicate() public {
        _createDefaultSchedule(false);

        vm.expectRevert("Schedule exists");
        _createDefaultSchedule(false);
    }

    function test_CreateSchedule_ZeroAmount() public {
        vm.expectRevert("Zero amount");
        vesting.createSchedule(beneficiary, 0, block.timestamp, CLIFF, DURATION, false);
    }

    function test_CreateSchedule_ZeroDuration() public {
        vm.expectRevert("Zero duration");
        vesting.createSchedule(beneficiary, TOTAL, block.timestamp, CLIFF, 0, false);
    }

    function test_CreateSchedule_Unauthorized() public {
        vm.prank(beneficiary);
        vm.expectRevert();
        vesting.createSchedule(
            beneficiary,
            TOTAL,
            block.timestamp,
            CLIFF,
            DURATION,
            false
        );
    }

    // -------------------------------------------------------------------------
    // release — before cliff
    // -------------------------------------------------------------------------

    function test_ReleaseBeforeCliff() public {
        _createDefaultSchedule(false);

        // Advance to just before cliff
        vm.warp(block.timestamp + CLIFF - 1);

        vm.prank(beneficiary);
        vm.expectRevert("Nothing to release");
        vesting.release();
    }

    function test_ReleaseAtStart() public {
        _createDefaultSchedule(false);

        // No time has passed
        vm.prank(beneficiary);
        vm.expectRevert("Nothing to release");
        vesting.release();
    }

    // -------------------------------------------------------------------------
    // release — at cliff
    // -------------------------------------------------------------------------

    function test_ReleaseAtCliff() public {
        uint256 startTs = block.timestamp;
        _createDefaultSchedule(false);

        vm.warp(startTs + CLIFF);

        vm.prank(beneficiary);
        vesting.release();

        // At cliff (180/730 of DURATION elapsed), vested = TOTAL * 180 / 730
        uint256 expectedVested = (TOTAL * CLIFF) / DURATION;
        assertEq(token.balanceOf(beneficiary), expectedVested);

        (, uint256 released, , , , , ) = vesting.schedules(beneficiary);
        assertEq(released, expectedVested);
    }

    // -------------------------------------------------------------------------
    // release — mid vesting
    // -------------------------------------------------------------------------

    function test_ReleaseDuringVesting() public {
        uint256 startTs = block.timestamp;
        _createDefaultSchedule(false);

        // Advance to 365 days (halfway through 730-day duration)
        uint256 elapsed = 365 days;
        vm.warp(startTs + elapsed);

        vm.prank(beneficiary);
        vesting.release();

        uint256 expectedVested = (TOTAL * elapsed) / DURATION;
        assertEq(token.balanceOf(beneficiary), expectedVested);
    }

    function test_ReleaseTwiceDuringVesting() public {
        uint256 startTs = block.timestamp;
        _createDefaultSchedule(false);

        // First release at cliff
        vm.warp(startTs + CLIFF);
        vm.prank(beneficiary);
        vesting.release();
        uint256 firstRelease = token.balanceOf(beneficiary);

        // Second release at 1 year
        vm.warp(startTs + 365 days);
        vm.prank(beneficiary);
        vesting.release();
        uint256 secondRelease = token.balanceOf(beneficiary) - firstRelease;

        uint256 expectedSecond = (TOTAL * 365 days) / DURATION - firstRelease;
        assertEq(secondRelease, expectedSecond);
    }

    // -------------------------------------------------------------------------
    // release — after full vest
    // -------------------------------------------------------------------------

    function test_ReleaseAfterFullVest() public {
        uint256 startTs = block.timestamp;
        _createDefaultSchedule(false);

        vm.warp(startTs + DURATION);

        vm.prank(beneficiary);
        vesting.release();

        assertEq(token.balanceOf(beneficiary), TOTAL);

        (, uint256 released, , , , , ) = vesting.schedules(beneficiary);
        assertEq(released, TOTAL);
    }

    function test_ReleaseAfterFullVest_NothingLeft() public {
        uint256 startTs = block.timestamp;
        _createDefaultSchedule(false);

        vm.warp(startTs + DURATION);

        vm.prank(beneficiary);
        vesting.release();

        // Second release should revert
        vm.prank(beneficiary);
        vm.expectRevert("Nothing to release");
        vesting.release();
    }

    function test_ReleaseEmitsEvent() public {
        uint256 startTs = block.timestamp;
        _createDefaultSchedule(false);

        vm.warp(startTs + DURATION);

        uint256 expectedRelease = TOTAL;
        vm.expectEmit(true, false, false, true);
        emit Vesting.TokensReleased(beneficiary, expectedRelease);

        vm.prank(beneficiary);
        vesting.release();
    }

    // -------------------------------------------------------------------------
    // No schedule
    // -------------------------------------------------------------------------

    function test_ReleaseNoSchedule() public {
        vm.prank(address(0x8));
        vm.expectRevert("No schedule");
        vesting.release();
    }

    // -------------------------------------------------------------------------
    // revoke
    // -------------------------------------------------------------------------

    function test_Revoke() public {
        uint256 startTs = block.timestamp;
        _createDefaultSchedule(true); // revocable

        // Advance to halfway
        vm.warp(startTs + 365 days);

        uint256 vestedBeforeRevoke = (TOTAL * 365 days) / DURATION;
        uint256 unvestedBeforeRevoke = TOTAL - vestedBeforeRevoke;

        uint256 ownerBalanceBefore = token.balanceOf(admin);

        vesting.revoke(beneficiary);

        // Unvested tokens returned to owner
        assertEq(token.balanceOf(admin), ownerBalanceBefore + unvestedBeforeRevoke);

        // Schedule marked revoked, total reduced to vested amount
        (uint256 total, , , , , , bool revoked) = vesting.schedules(beneficiary);
        assertTrue(revoked);
        assertEq(total, vestedBeforeRevoke);
    }

    function test_RevokeEmitsEvent() public {
        _createDefaultSchedule(true);
        vm.warp(block.timestamp + 365 days);

        vm.expectEmit(true, false, false, false);
        emit Vesting.VestingRevoked(beneficiary);
        vesting.revoke(beneficiary);
    }

    function test_RevokeNonRevocable() public {
        _createDefaultSchedule(false); // NOT revocable

        vm.expectRevert("Not revocable");
        vesting.revoke(beneficiary);
    }

    function test_RevokeAlreadyRevoked() public {
        _createDefaultSchedule(true);
        vm.warp(block.timestamp + 365 days);

        vesting.revoke(beneficiary);

        vm.expectRevert("Already revoked");
        vesting.revoke(beneficiary);
    }

    function test_RevokeUnauthorized() public {
        _createDefaultSchedule(true);

        vm.prank(beneficiary);
        vm.expectRevert();
        vesting.revoke(beneficiary);
    }

    function test_ReleaseAfterRevoke() public {
        uint256 startTs = block.timestamp;
        _createDefaultSchedule(true);

        vm.warp(startTs + 365 days);
        vesting.revoke(beneficiary);

        // Beneficiary cannot release after revocation
        vm.prank(beneficiary);
        vm.expectRevert("Revoked");
        vesting.release();
    }

    function test_RevokeAtStartNothingVested() public {
        _createDefaultSchedule(true);

        // Revoke at t=0 — no cliff reached, nothing vested
        uint256 ownerBefore = token.balanceOf(admin);
        vesting.revoke(beneficiary);

        // Entire TOTAL should be returned to owner (0 vested)
        assertEq(token.balanceOf(admin), ownerBefore + TOTAL);
    }

    // -------------------------------------------------------------------------
    // getReleasable
    // -------------------------------------------------------------------------

    function test_GetReleasable() public {
        uint256 startTs = block.timestamp;
        _createDefaultSchedule(false);

        vm.warp(startTs + DURATION);

        uint256 releasable = vesting.getReleasable(beneficiary);
        assertEq(releasable, TOTAL);
    }

    function test_GetReleasable_BeforeCliff() public {
        _createDefaultSchedule(false);
        vm.warp(block.timestamp + CLIFF - 1);

        uint256 releasable = vesting.getReleasable(beneficiary);
        assertEq(releasable, 0);
    }

    function test_GetReleasable_AfterRevoke() public {
        _createDefaultSchedule(true);
        vm.warp(block.timestamp + 365 days);

        vesting.revoke(beneficiary);

        uint256 releasable = vesting.getReleasable(beneficiary);
        assertEq(releasable, 0);
    }

    function test_GetReleasable_AfterPartialRelease() public {
        uint256 startTs = block.timestamp;
        _createDefaultSchedule(false);

        vm.warp(startTs + CLIFF);
        vm.prank(beneficiary);
        vesting.release();

        uint256 firstVested = (TOTAL * CLIFF) / DURATION;

        vm.warp(startTs + 365 days);
        uint256 releasable = vesting.getReleasable(beneficiary);

        uint256 expectedReleasable = (TOTAL * 365 days) / DURATION - firstVested;
        assertEq(releasable, expectedReleasable);
    }

    // -------------------------------------------------------------------------
    // Future start
    // -------------------------------------------------------------------------

    function test_CreateSchedule_FutureStart() public {
        uint256 futureStart = block.timestamp + 30 days;
        vesting.createSchedule(
            beneficiary,
            TOTAL,
            futureStart,
            CLIFF,
            DURATION,
            false
        );

        // Release should revert — cliff not yet reached
        vm.prank(beneficiary);
        vm.expectRevert("Nothing to release");
        vesting.release();
    }

    function test_GetReleasable_NoSchedule() public view {
        // Address with no schedule: getReleasable should return 0 (total=0, released=0)
        uint256 releasable = vesting.getReleasable(address(0x99));
        assertEq(releasable, 0);
    }
}
