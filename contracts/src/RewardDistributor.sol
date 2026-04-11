// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "@openzeppelin/contracts/utils/cryptography/MerkleProof.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

interface IProxyCoinToken {
    function mint(address to, uint256 amount) external;
}

/// @title Reward Distributor
/// @notice Merkle-tree-based batch reward distribution for proxy node operators
contract RewardDistributor is Ownable {
    IProxyCoinToken public token;

    bytes32 public merkleRoot;
    uint256 public epoch; // incremented each time root is updated

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
