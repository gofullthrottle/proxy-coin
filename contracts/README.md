# Proxy Coin Smart Contracts

Solidity smart contracts for the Proxy Coin (PRXY) network, built with [Foundry](https://book.getfoundry.sh/).

## Contracts

| Contract | Description |
|----------|-------------|
| `ProxyCoinToken` | ERC-20 PRXY token with controlled minting/burning |
| `RewardDistributor` | Merkle-tree-based batch reward distribution |
| `Staking` | PRXY staking with earning multipliers and fraud slashing |
| `Vesting` | Linear vesting for team, investors, and treasury |

## Prerequisites

Install Foundry (forge, cast, anvil):

```bash
curl -L https://foundry.paradigm.xyz | bash
foundryup
```

## Setup

Install dependencies after cloning:

```bash
cd contracts

# Install OpenZeppelin Contracts
forge install OpenZeppelin/openzeppelin-contracts --no-commit

# Install forge-std (testing framework)
forge install foundry-rs/forge-std --no-commit
```

## Environment Variables

Copy `.env.example` and fill in values:

```bash
cp ../.env.example .env
```

Required variables:

| Variable | Description |
|----------|-------------|
| `BASE_SEPOLIA_RPC` | Base Sepolia RPC URL (e.g. from Alchemy/Infura) |
| `BASE_MAINNET_RPC` | Base Mainnet RPC URL |
| `BASESCAN_API_KEY` | Basescan API key for contract verification |

## Build

```bash
forge build
```

## Test

```bash
# Run all tests
forge test

# Run with verbosity
forge test -vvv

# Run fuzz tests (1000 runs configured)
forge test --fuzz-runs 1000
```

## Deploy

```bash
# Deploy to Base Sepolia (testnet)
forge script script/Deploy.s.sol --rpc-url base_sepolia --broadcast --verify

# Deploy to Base Mainnet
forge script script/Deploy.s.sol --rpc-url base_mainnet --broadcast --verify
```

## Network Details

| Network | Chain ID | RPC |
|---------|----------|-----|
| Base Mainnet | 8453 | https://mainnet.base.org |
| Base Sepolia | 84532 | https://sepolia.base.org |

## Project Structure

```
contracts/
├── foundry.toml          # Foundry configuration
├── remappings.txt        # Import remappings
├── src/                  # Contract source files
│   ├── ProxyCoinToken.sol
│   ├── RewardDistributor.sol
│   ├── Staking.sol
│   └── Vesting.sol
├── test/                 # Forge tests
│   ├── ProxyCoinToken.t.sol
│   ├── RewardDistributor.t.sol
│   ├── Staking.t.sol
│   └── Vesting.t.sol
├── script/               # Deployment scripts
│   ├── Deploy.s.sol
│   └── DistributeRewards.s.sol
└── lib/                  # Forge dependencies (git submodules)
    ├── openzeppelin-contracts/
    └── forge-std/
```

## Implementation Status

Contract logic is stubbed out and scheduled for Wave 9. Tests are scheduled for Wave 10.
