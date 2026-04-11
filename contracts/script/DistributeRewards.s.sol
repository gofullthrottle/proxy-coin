// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "forge-std/Script.sol";
import "../src/RewardDistributor.sol";

contract DistributeRewards is Script {
    function run() external {
        uint256 deployerPrivateKey = vm.envUint("DEPLOYER_KEY");
        address distributorAddr = vm.envAddress("REWARD_DISTRIBUTOR_ADDRESS");
        bytes32 newRoot = vm.envBytes32("MERKLE_ROOT");

        vm.startBroadcast(deployerPrivateKey);

        RewardDistributor distributor = RewardDistributor(distributorAddr);
        distributor.setMerkleRoot(newRoot);

        console.log("Merkle root updated:");
        console.log("  Epoch:", distributor.epoch());
        console.log("  Root:", vm.toString(newRoot));

        vm.stopBroadcast();
    }
}
