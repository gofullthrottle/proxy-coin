// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "forge-std/Test.sol";
import "../src/ProxyCoinToken.sol";
import "../src/RewardDistributor.sol";

contract RewardDistributorTest is Test {
    ProxyCoinToken public token;
    RewardDistributor public distributor;
    address public admin = address(this);

    address public alice = address(0xA11CE);
    address public bob = address(0xB0B);

    // -------------------------------------------------------------------------
    // Merkle tree helpers
    // -------------------------------------------------------------------------

    /// @dev Build a minimal 2-leaf Merkle tree matching the double-hash scheme
    ///      used in RewardDistributor:
    ///        leaf = keccak256(bytes.concat(keccak256(abi.encode(addr, amount))))
    ///      Leaves are sorted before hashing the root so that the ordering is
    ///      deterministic regardless of address/amount values.
    ///
    /// @return root        The Merkle root
    /// @return proof1      Proof for (addr1, amt1): [leaf2]
    /// @return proof2      Proof for (addr2, amt2): [leaf1]
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

        // Sort leaves for consistent root computation
        // (matches OpenZeppelin MerkleProof.verify which sorts pairs internally)
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

    // -------------------------------------------------------------------------
    // Setup
    // -------------------------------------------------------------------------

    function setUp() public {
        token = new ProxyCoinToken(admin);
        distributor = new RewardDistributor(address(token));
        token.grantRole(token.MINTER_ROLE(), address(distributor));
    }

    // -------------------------------------------------------------------------
    // setMerkleRoot
    // -------------------------------------------------------------------------

    function test_SetMerkleRoot() public {
        assertEq(distributor.epoch(), 0);

        bytes32 root = keccak256("root1");
        distributor.setMerkleRoot(root);

        assertEq(distributor.merkleRoot(), root);
        assertEq(distributor.epoch(), 1);
    }

    function test_SetMerkleRoot_EmitsEvent() public {
        bytes32 root = keccak256("root2");
        vm.expectEmit(true, false, false, true);
        emit RewardDistributor.MerkleRootUpdated(1, root);
        distributor.setMerkleRoot(root);
    }

    function test_SetMerkleRoot_NonOwner() public {
        vm.prank(alice);
        vm.expectRevert();
        distributor.setMerkleRoot(keccak256("root"));
    }

    function test_SetMerkleRoot_MultipleTimes() public {
        distributor.setMerkleRoot(keccak256("root1"));
        distributor.setMerkleRoot(keccak256("root2"));
        distributor.setMerkleRoot(keccak256("root3"));
        assertEq(distributor.epoch(), 3);
    }

    // -------------------------------------------------------------------------
    // claim — valid proof
    // -------------------------------------------------------------------------

    function test_Claim() public {
        uint256 aliceAmount = 1_000 * 1e18;
        uint256 bobAmount = 2_000 * 1e18;

        (bytes32 root, bytes32[] memory aliceProof, ) = _buildTree(
            alice,
            aliceAmount,
            bob,
            bobAmount
        );
        distributor.setMerkleRoot(root);

        vm.prank(alice);
        distributor.claim(aliceAmount, aliceProof);

        assertEq(token.balanceOf(alice), aliceAmount);
        assertEq(distributor.claimed(alice), aliceAmount);
    }

    function test_Claim_EmitsEvent() public {
        uint256 aliceAmount = 500 * 1e18;
        uint256 bobAmount = 100 * 1e18;

        (bytes32 root, bytes32[] memory aliceProof, ) = _buildTree(
            alice,
            aliceAmount,
            bob,
            bobAmount
        );
        distributor.setMerkleRoot(root);

        vm.expectEmit(true, false, false, true);
        emit RewardDistributor.RewardsClaimed(alice, aliceAmount, 1);

        vm.prank(alice);
        distributor.claim(aliceAmount, aliceProof);
    }

    // -------------------------------------------------------------------------
    // claim — invalid proof
    // -------------------------------------------------------------------------

    function test_Claim_InvalidProof() public {
        uint256 aliceAmount = 1_000 * 1e18;
        uint256 bobAmount = 2_000 * 1e18;

        (bytes32 root, , bytes32[] memory bobProof) = _buildTree(
            alice,
            aliceAmount,
            bob,
            bobAmount
        );
        distributor.setMerkleRoot(root);

        // Alice tries to claim using Bob's proof — must revert
        vm.prank(alice);
        vm.expectRevert("Invalid proof");
        distributor.claim(aliceAmount, bobProof);
    }

    function test_Claim_WrongAmount() public {
        uint256 aliceAmount = 1_000 * 1e18;
        uint256 bobAmount = 2_000 * 1e18;

        (bytes32 root, bytes32[] memory aliceProof, ) = _buildTree(
            alice,
            aliceAmount,
            bob,
            bobAmount
        );
        distributor.setMerkleRoot(root);

        // Alice tries to claim a different amount — invalid proof
        vm.prank(alice);
        vm.expectRevert("Invalid proof");
        distributor.claim(aliceAmount + 1, aliceProof);
    }

    function test_Claim_NoRootSet() public {
        bytes32[] memory emptyProof = new bytes32[](0);
        vm.prank(alice);
        vm.expectRevert("Invalid proof");
        distributor.claim(1_000 * 1e18, emptyProof);
    }

    // -------------------------------------------------------------------------
    // Double-claim prevention (cumulative model)
    // -------------------------------------------------------------------------

    function test_Claim_DoubleClaim() public {
        uint256 aliceAmount = 1_000 * 1e18;
        uint256 bobAmount = 500 * 1e18;

        (bytes32 root, bytes32[] memory aliceProof, ) = _buildTree(
            alice,
            aliceAmount,
            bob,
            bobAmount
        );
        distributor.setMerkleRoot(root);

        // First claim succeeds
        vm.prank(alice);
        distributor.claim(aliceAmount, aliceProof);
        assertEq(token.balanceOf(alice), aliceAmount);

        // Second claim with same cumulative amount has nothing to claim
        vm.prank(alice);
        vm.expectRevert("Nothing to claim");
        distributor.claim(aliceAmount, aliceProof);
    }

    // -------------------------------------------------------------------------
    // Partial then full claim
    // -------------------------------------------------------------------------

    function test_Claim_PartialThenFull() public {
        // Epoch 1: alice has 500 cumulative
        uint256 aliceEpoch1 = 500 * 1e18;
        uint256 bobEpoch1 = 100 * 1e18;

        (bytes32 root1, bytes32[] memory aliceProof1, ) = _buildTree(
            alice,
            aliceEpoch1,
            bob,
            bobEpoch1
        );
        distributor.setMerkleRoot(root1);

        vm.prank(alice);
        distributor.claim(aliceEpoch1, aliceProof1);
        assertEq(token.balanceOf(alice), aliceEpoch1);
        assertEq(distributor.claimed(alice), aliceEpoch1);

        // Epoch 2: alice now has 1_500 cumulative (earned 1_000 more)
        uint256 aliceEpoch2 = 1_500 * 1e18;
        uint256 bobEpoch2 = 200 * 1e18;

        (bytes32 root2, bytes32[] memory aliceProof2, ) = _buildTree(
            alice,
            aliceEpoch2,
            bob,
            bobEpoch2
        );
        distributor.setMerkleRoot(root2);

        vm.prank(alice);
        distributor.claim(aliceEpoch2, aliceProof2);

        // Should only receive the delta: 1_500 - 500 = 1_000
        assertEq(token.balanceOf(alice), aliceEpoch2);
        assertEq(distributor.claimed(alice), aliceEpoch2);
    }

    // -------------------------------------------------------------------------
    // getClaimable
    // -------------------------------------------------------------------------

    function test_GetClaimable() public {
        uint256 aliceAmount = 1_000 * 1e18;
        uint256 bobAmount = 2_000 * 1e18;

        (bytes32 root, bytes32[] memory aliceProof, ) = _buildTree(
            alice,
            aliceAmount,
            bob,
            bobAmount
        );
        distributor.setMerkleRoot(root);

        uint256 claimable = distributor.getClaimable(alice, aliceAmount, aliceProof);
        assertEq(claimable, aliceAmount);
    }

    function test_GetClaimable_AfterPartialClaim() public {
        uint256 aliceEpoch1 = 500 * 1e18;
        uint256 bobEpoch1 = 100 * 1e18;

        (bytes32 root1, bytes32[] memory aliceProof1, ) = _buildTree(
            alice,
            aliceEpoch1,
            bob,
            bobEpoch1
        );
        distributor.setMerkleRoot(root1);

        vm.prank(alice);
        distributor.claim(aliceEpoch1, aliceProof1);

        // After claiming 500, claimable with same proof should be 0
        uint256 claimable = distributor.getClaimable(alice, aliceEpoch1, aliceProof1);
        assertEq(claimable, 0);
    }

    function test_GetClaimable_InvalidProof() public {
        uint256 aliceAmount = 1_000 * 1e18;
        uint256 bobAmount = 2_000 * 1e18;

        (bytes32 root, , bytes32[] memory bobProof) = _buildTree(
            alice,
            aliceAmount,
            bob,
            bobAmount
        );
        distributor.setMerkleRoot(root);

        // Wrong proof returns 0
        uint256 claimable = distributor.getClaimable(alice, aliceAmount, bobProof);
        assertEq(claimable, 0);
    }

    // -------------------------------------------------------------------------
    // Epoch increment
    // -------------------------------------------------------------------------

    function test_EpochIncrement() public {
        assertEq(distributor.epoch(), 0);

        distributor.setMerkleRoot(keccak256("a"));
        assertEq(distributor.epoch(), 1);

        distributor.setMerkleRoot(keccak256("b"));
        assertEq(distributor.epoch(), 2);

        distributor.setMerkleRoot(keccak256("c"));
        assertEq(distributor.epoch(), 3);
    }

    // -------------------------------------------------------------------------
    // Both claimers in same tree
    // -------------------------------------------------------------------------

    function test_BothClaim() public {
        uint256 aliceAmount = 1_000 * 1e18;
        uint256 bobAmount = 2_500 * 1e18;

        (
            bytes32 root,
            bytes32[] memory aliceProof,
            bytes32[] memory bobProof
        ) = _buildTree(alice, aliceAmount, bob, bobAmount);
        distributor.setMerkleRoot(root);

        vm.prank(alice);
        distributor.claim(aliceAmount, aliceProof);

        vm.prank(bob);
        distributor.claim(bobAmount, bobProof);

        assertEq(token.balanceOf(alice), aliceAmount);
        assertEq(token.balanceOf(bob), bobAmount);
    }
}
