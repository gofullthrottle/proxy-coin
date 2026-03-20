# Proxy Coin - Token Economics

## Token Overview

| Property | Value |
|----------|-------|
| **Name** | Proxy Coin |
| **Symbol** | PRXY |
| **Chain** | Base L2 (Coinbase) |
| **Standard** | ERC-20 |
| **Total Supply** | 1,000,000,000 (1 billion) |
| **Decimals** | 18 |
| **Initial Circulating** | ~50,000,000 (5%) |

## Supply Allocation

```
Node Rewards    ████████████████████████████████████████  40%  (400M)
Treasury/DAO    ████████████████████                      20%  (200M)
Team            ███████████████                           15%  (150M)
Liquidity       ██████████                                10%  (100M)
Partners        ██████████                                10%  (100M)
Community       █████                                      5%  ( 50M)
```

| Allocation | Amount | Vesting |
|-----------|--------|---------|
| **Node Rewards** | 400,000,000 | Emitted over 5 years, decreasing schedule |
| **Treasury/DAO** | 200,000,000 | Governance-controlled, 6-month cliff then linear 3 years |
| **Team** | 150,000,000 | 1-year cliff, then linear over 3 years (4 years total) |
| **Liquidity** | 100,000,000 | 25% at TGE, 75% over 12 months |
| **Partners/Investors** | 100,000,000 | 6-month cliff, then linear over 18 months |
| **Community** | 50,000,000 | Airdrops, grants, bounties — discretionary |

## Emission Schedule (Node Rewards)

The 400M node rewards are distributed on a decreasing emission schedule:

| Year | Daily Emission | Annual Total | Cumulative |
|------|---------------|-------------|------------|
| 1 | 328,767 PRXY | 120,000,000 | 120,000,000 |
| 2 | 246,575 PRXY | 90,000,000 | 210,000,000 |
| 3 | 184,932 PRXY | 67,500,000 | 277,500,000 |
| 4 | 136,986 PRXY | 50,000,000 | 327,500,000 |
| 5 | 109,589 PRXY | 40,000,000 | 367,500,000 |
| 5+ | Governance decision | 32,500,000 remaining | 400,000,000 |

**Design rationale**: Front-loaded emission rewards early adopters. Decreasing schedule creates scarcity over time. Year 5+ remaining tokens are governed by DAO vote.

## How Users Earn PRXY

### Base Earning Rate

| Resource | Rate | Example |
|----------|------|---------|
| Bandwidth | 10 PRXY per GB proxied | 5 GB/day = 50 PRXY/day |
| Uptime | 1 PRXY per hour connected | 12 hrs/day = 12 PRXY/day |
| Compute (Phase 2) | 5 PRXY per CPU-hour | 2 hrs/day = 10 PRXY/day |

*Rates are illustrative and will be adjusted based on token price, network size, and revenue.*

### Earning Multipliers

| Factor | Range | How to Improve |
|--------|-------|---------------|
| **Trust Score** | 0.5x - 2.0x | Consistent uptime, pass attestations, residential IP |
| **Uptime Bonus** | 1.0x - 1.5x | Stay connected 20+ hours/day |
| **Quality Bonus** | 0.8x - 1.3x | Low latency, high success rate |
| **Staking Bonus** | 1.0x - 1.5x | Stake PRXY tokens (see Staking below) |
| **Referral Bonus** | +5% | Earn 5% of each referral's earnings |

### Example Monthly Earnings

**Casual user** (8 hrs/day, WiFi, 2 GB/day):
- Bandwidth: 2 GB × 10 PRXY × 30 days = 600 PRXY
- Uptime: 8 hrs × 1 PRXY × 30 days = 240 PRXY
- Trust multiplier: 1.0x (average)
- **Total: ~840 PRXY/month**

**Power user** (20 hrs/day, WiFi, 10 GB/day, staked 10K PRXY):
- Bandwidth: 10 GB × 10 PRXY × 30 days = 3,000 PRXY
- Uptime: 20 hrs × 1 PRXY × 30 days = 600 PRXY
- Trust multiplier: 1.5x (high trust, long history)
- Staking multiplier: 1.3x (10K staked)
- **Total: ~7,020 PRXY/month**

## Staking Mechanism

Users can stake PRXY to increase their earning multiplier:

| Staked Amount | Multiplier | Lock Period |
|--------------|------------|-------------|
| 1,000 PRXY | 1.1x | 30 days |
| 5,000 PRXY | 1.2x | 30 days |
| 10,000 PRXY | 1.3x | 60 days |
| 50,000 PRXY | 1.5x | 90 days |

**Slashing**: If a node is caught committing fraud (self-proxying, emulator, bandwidth inflation), up to 50% of staked PRXY is slashed and burned.

## Revenue Model and Token Value Accrual

### Revenue Sources

```
Customer pays $X per GB of proxy bandwidth
         │
         ├─ 70% → Buy PRXY from market → Distribute to nodes (buy pressure)
         ├─ 20% → Operations (infrastructure, development, support)
         └─ 10% → Treasury (governed by DAO)
```

### Buy-and-Distribute Mechanism

Instead of paying nodes directly in USD, revenue is used to buy PRXY from the open market (DEX) and distribute to nodes. This creates continuous buy pressure proportional to network revenue.

### Token Value Drivers

1. **Demand side**: Staking requirement for higher earnings creates demand
2. **Supply side**: Decreasing emission reduces new supply over time
3. **Burn mechanism**: Slashed stakes are burned, permanently reducing supply
4. **Revenue buyback**: 70% of revenue buys PRXY from market
5. **Utility**: PRXY is required to use premium features (geo-targeting, sticky sessions)

## Anti-Inflation Mechanisms

1. **Fixed supply**: Hard cap at 1 billion tokens
2. **Decreasing emission**: Year-over-year reduction in new tokens
3. **Staking locks**: Staked tokens are illiquid
4. **Burn events**: Fraud slashing permanently removes tokens
5. **Revenue buyback**: Offsets emission with market purchases

## Claiming Process

1. Earnings accumulate off-chain (visible in app in real-time)
2. Daily: Backend generates Merkle tree of all pending rewards
3. Merkle root published on-chain (Base L2)
4. User initiates claim in app:
   a. App fetches Merkle proof from backend API
   b. App constructs claim transaction
   c. User signs with wallet
   d. Transaction submitted to RewardDistributor contract
   e. Contract verifies proof against root
   f. PRXY transferred to user wallet
5. Minimum claim: 100 PRXY (prevents dust claims wasting gas)

## Token Distribution Timeline

```
Month 0 (TGE):
├── Liquidity: 25M (initial DEX liquidity)
├── Community: 10M (initial airdrop to early testers)
└── Circulating: ~35M

Month 6:
├── Partners begin unlock: +~4.2M/month
├── Treasury begins unlock: +~5.6M/month
├── Node emissions: ~10M/month
└── Circulating: ~120M

Month 12:
├── Team begins unlock: +~3.1M/month
├── Liquidity fully unlocked: 100M total
├── Node emissions: ~10M/month
└── Circulating: ~250M

Year 2:
├── All vesting active
├── Node emission decreases to 7.5M/month
└── Circulating: ~450M

Year 5:
├── Most vesting complete
├── Node emission at 3.3M/month
└── Circulating: ~850M
```

## Governance (Future)

After sufficient decentralization:
- PRXY holders can vote on parameter changes (emission rates, staking requirements)
- Treasury spending proposals
- New feature prioritization
- Protocol upgrades
- 1 PRXY = 1 vote (staked PRXY gets 2x voting power)
