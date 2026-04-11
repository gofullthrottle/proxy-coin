// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "forge-std/Script.sol";
import "../src/ProxyCoinToken.sol";
import "../src/RewardDistributor.sol";
import "../src/Staking.sol";
import "../src/Vesting.sol";

contract Deploy is Script {
    function run() external {
        uint256 deployerPrivateKey = vm.envUint("DEPLOYER_KEY");
        address deployer = vm.addr(deployerPrivateKey);

        vm.startBroadcast(deployerPrivateKey);

        // 1. Deploy token
        ProxyCoinToken token = new ProxyCoinToken(deployer);
        console.log("ProxyCoinToken deployed at:", address(token));

        // 2. Deploy RewardDistributor
        RewardDistributor distributor = new RewardDistributor(address(token));
        console.log("RewardDistributor deployed at:", address(distributor));

        // 3. Deploy Staking
        Staking staking = new Staking(address(token));
        console.log("Staking deployed at:", address(staking));

        // 4. Deploy Vesting
        Vesting vesting = new Vesting(address(token));
        console.log("Vesting deployed at:", address(vesting));

        // 5. Grant roles
        token.grantRole(token.MINTER_ROLE(), address(distributor));
        console.log("MINTER_ROLE granted to RewardDistributor");

        token.grantRole(token.BURNER_ROLE(), address(staking));
        console.log("BURNER_ROLE granted to Staking");

        vm.stopBroadcast();

        console.log("\n=== Deployment Complete ===");
        console.log("Token:", address(token));
        console.log("RewardDistributor:", address(distributor));
        console.log("Staking:", address(staking));
        console.log("Vesting:", address(vesting));
    }
}
