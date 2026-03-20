# Proxy Coin - Backend Specification

## Service Architecture

Three independently deployable Go services sharing a common codebase:

```
┌──────────────┐  ┌──────────────────┐  ┌──────────────────┐
│ Customer API │  │   Orchestrator   │  │ Metering Service │
│              │  │                  │  │                  │
│ REST API for │  │ WebSocket server │  │ Usage aggregation│
│ proxy buyers │  │ Node registry    │  │ Reward calc      │
│ Auth, billing│  │ Request routing  │  │ Fraud detection  │
│ Usage dashboard│ │ Load balancing   │  │ Merkle generation│
└──────┬───────┘  └────────┬─────────┘  └────────┬─────────┘
       │                   │                     │
       └───────────────────┼─────────────────────┘
                           │
              ┌────────────┴────────────┐
              │   PostgreSQL + Redis    │
              └─────────────────────────┘
```

## 1. Orchestrator Service

The core service. Manages WebSocket connections to Android nodes and routes proxy requests.

### Node Registry

```go
type Node struct {
    ID            string    `db:"id"`             // UUID
    DeviceID      string    `db:"device_id"`      // Android device ID
    WalletAddress string    `db:"wallet_address"`
    IP            string    `db:"ip"`
    Country       string    `db:"country"`
    Region        string    `db:"region"`
    City          string    `db:"city"`
    ISP           string    `db:"isp"`
    ASN           int       `db:"asn"`
    IsResidential bool      `db:"is_residential"`
    NetworkType   string    `db:"network_type"`   // wifi, cellular
    TrustScore    float64   `db:"trust_score"`
    Status        string    `db:"status"`         // active, idle, suspended
    MaxConcurrent int       `db:"max_concurrent"`
    ActiveRequests int      // in-memory only (via Redis)
    LastHeartbeat time.Time `db:"last_heartbeat"`
    RegisteredAt  time.Time `db:"registered_at"`
}
```

### Node Selection Algorithm

```go
func (s *Selector) SelectNode(req ProxyRequest) (*Node, error) {
    // 1. Filter candidates
    candidates := s.registry.FindNodes(NodeFilter{
        Country:     req.GeoTarget.Country,
        Region:      req.GeoTarget.Region,
        Status:      "active",
        MinTrust:    0.3,
        NetworkType: "wifi", // prefer WiFi nodes
    })

    if len(candidates) == 0 {
        return nil, ErrNoAvailableNodes
    }

    // 2. Score candidates
    scored := make([]ScoredNode, len(candidates))
    for i, node := range candidates {
        scored[i] = ScoredNode{
            Node: node,
            Score: s.calculateScore(node),
        }
    }

    // 3. Weighted random selection from top 5
    //    (not strictly best — avoids overloading single node)
    sort.Slice(scored, func(i, j int) bool {
        return scored[i].Score > scored[j].Score
    })

    topN := scored[:min(5, len(scored))]
    return weightedRandom(topN), nil
}

func (s *Selector) calculateScore(node Node) float64 {
    loadFactor := 1.0 - float64(node.ActiveRequests)/float64(node.MaxConcurrent)
    return (node.TrustScore * 0.4) +
           (loadFactor * 0.3) +
           (residentialBonus(node) * 0.2) +
           (uptimeBonus(node) * 0.1)
}
```

### WebSocket Server

```go
type WSServer struct {
    upgrader    websocket.Upgrader
    connections sync.Map        // nodeID -> *Connection
    registry    *NodeRegistry
    config      *Config
}

type Connection struct {
    NodeID     string
    Conn       *websocket.Conn
    SendCh     chan []byte       // outbound messages
    ActiveReqs sync.Map         // requestID -> *PendingRequest
    mu         sync.Mutex
    lastPing   time.Time
}

// Message flow:
// 1. Node connects via wss://orchestrator.proxycoin.io/ws
// 2. Node sends REGISTER message
// 3. Server validates device attestation, creates/updates Node record
// 4. Server sends REGISTERED with config (earning rates, blocklist hash)
// 5. Node sends HEARTBEAT every 30s
// 6. Server sends PROXY_REQUEST when customer request arrives
// 7. Node sends PROXY_RESPONSE (streamed in chunks)
// 8. Server forwards response to Customer API
```

## 2. Customer API

REST API for businesses purchasing proxy bandwidth.

### Endpoints

```
POST   /v1/auth/register          # Create customer account
POST   /v1/auth/login             # Get JWT token
GET    /v1/auth/apikey             # Get/rotate API key

POST   /v1/proxy                  # Submit proxy request
POST   /v1/proxy/batch            # Submit batch proxy requests

GET    /v1/usage                  # Get usage statistics
GET    /v1/usage/daily            # Daily usage breakdown

GET    /v1/billing                # Billing summary
GET    /v1/billing/invoices       # Invoice history

GET    /v1/status                 # Network status (available nodes by geo)
```

### Proxy Request/Response

```
POST /v1/proxy
Authorization: Bearer <api_key>
Content-Type: application/json

Request:
{
  "url": "https://example.com/api/data",
  "method": "GET",
  "headers": {
    "Accept": "application/json",
    "User-Agent": "Mozilla/5.0 ..."
  },
  "body": null,
  "geo": {
    "country": "US",
    "region": "CA"           // optional
  },
  "session_id": "sess_abc",  // optional: sticky session (same exit IP)
  "timeout_ms": 15000        // optional: max wait time
}

Response:
{
  "request_id": "req_abc123",
  "status_code": 200,
  "headers": {
    "Content-Type": "application/json",
    "Content-Length": "1234"
  },
  "body": "<base64 encoded response body>",
  "node": {
    "country": "US",
    "region": "CA",
    "ip": "73.x.x.x"         // exit IP (customer needs this)
  },
  "metrics": {
    "total_ms": 842,
    "proxy_ms": 720,
    "bytes_transferred": 1234
  }
}
```

### Authentication

- **Customer accounts**: email + password, JWT tokens
- **API keys**: `prxy_live_xxxxxxxxxxxx` format, stored as bcrypt hash
- **Rate limiting**: per API key, configurable per plan
- **Plans**: Free (100 req/day), Starter ($49/mo, 10K req/day), Pro ($199/mo, 100K req/day), Enterprise (custom)

## 3. Metering Service

Processes usage events and calculates rewards.

### Event Pipeline

```
Orchestrator → Redis Stream → Metering Service → PostgreSQL
                                    │
                                    ├→ Earnings table (per node per hour)
                                    ├→ Fraud detection pipeline
                                    └→ Customer billing records
```

### Metering Event

```go
type MeteringEvent struct {
    RequestID    string    `json:"request_id"`
    NodeID       string    `json:"node_id"`
    CustomerID   string    `json:"customer_id"`
    BytesIn      int64     `json:"bytes_in"`      // request bytes
    BytesOut     int64     `json:"bytes_out"`      // response bytes
    LatencyMs    int       `json:"latency_ms"`
    StatusCode   int       `json:"status_code"`
    Success      bool      `json:"success"`
    Timestamp    time.Time `json:"timestamp"`
}
```

### Reward Calculation (runs hourly)

```go
func (c *Calculator) CalculateRewards(epoch time.Time) ([]NodeReward, error) {
    // 1. Aggregate metering for this epoch
    metrics := c.db.GetNodeMetrics(epoch)

    rewards := make([]NodeReward, 0, len(metrics))
    for _, m := range metrics {
        node := c.registry.GetNode(m.NodeID)

        // 2. Base reward: bytes proxied × rate
        baseReward := float64(m.TotalBytes) / 1e6 * c.config.PRXYPerMB

        // 3. Apply multipliers
        multiplier := 1.0
        multiplier *= trustMultiplier(node.TrustScore)   // 0.5x - 2.0x
        multiplier *= uptimeMultiplier(m.UptimePercent)   // 1.0x - 1.5x
        multiplier *= qualityMultiplier(m.SuccessRate, m.AvgLatency) // 0.8x - 1.3x

        // 4. Apply staking bonus (if node has staked PRXY)
        if node.StakedAmount > 0 {
            multiplier *= stakingMultiplier(node.StakedAmount) // 1.0x - 1.5x
        }

        finalReward := baseReward * multiplier

        rewards = append(rewards, NodeReward{
            NodeID:       m.NodeID,
            Epoch:        epoch,
            BaseReward:   baseReward,
            Multiplier:   multiplier,
            FinalReward:  finalReward,
            BytesProxied: m.TotalBytes,
        })
    }

    // 5. Persist to database
    c.db.InsertRewards(rewards)

    return rewards, nil
}
```

### Merkle Tree Generation (runs daily)

```go
func (d *Distributor) GenerateMerkleTree(date time.Time) (*MerkleRoot, error) {
    // 1. Get all pending (unclaimed) rewards
    pending := d.db.GetPendingRewards()

    // 2. Build leaf nodes: hash(nodeWallet, cumulativeAmount, nonce)
    leaves := make([][]byte, len(pending))
    for i, p := range pending {
        leaves[i] = keccak256(
            p.WalletAddress,
            p.CumulativeAmount,  // total claimable (not just today)
            p.Nonce,
        )
    }

    // 3. Build Merkle tree
    tree := merkle.NewTree(leaves)

    // 4. Publish root on-chain
    tx, err := d.contract.SetMerkleRoot(tree.Root())

    // 5. Store proofs in database (nodes query their proof via API)
    for i, p := range pending {
        proof := tree.GetProof(i)
        d.db.StoreProof(p.WalletAddress, proof)
    }

    return &MerkleRoot{Root: tree.Root(), TxHash: tx.Hash()}, nil
}
```

## Database Schema

```sql
-- 001_initial.up.sql

-- Node registry
CREATE TABLE nodes (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id       TEXT UNIQUE NOT NULL,
    wallet_address  TEXT NOT NULL,
    ip              INET,
    country         TEXT,
    region          TEXT,
    city            TEXT,
    isp             TEXT,
    asn             INTEGER,
    is_residential  BOOLEAN DEFAULT true,
    network_type    TEXT DEFAULT 'wifi',
    trust_score     DECIMAL(4,3) DEFAULT 0.500,
    status          TEXT DEFAULT 'pending',  -- pending, active, idle, suspended, banned
    max_concurrent  INTEGER DEFAULT 5,
    staked_amount   DECIMAL(36,18) DEFAULT 0,
    referral_code   TEXT UNIQUE,
    referred_by     UUID REFERENCES nodes(id),
    registered_at   TIMESTAMPTZ DEFAULT now(),
    last_heartbeat  TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_nodes_status ON nodes(status);
CREATE INDEX idx_nodes_country ON nodes(country);
CREATE INDEX idx_nodes_trust ON nodes(trust_score);
CREATE INDEX idx_nodes_wallet ON nodes(wallet_address);

-- Metering records (partitioned by month)
CREATE TABLE metering (
    id              BIGSERIAL,
    request_id      TEXT NOT NULL,
    node_id         UUID NOT NULL REFERENCES nodes(id),
    customer_id     UUID NOT NULL REFERENCES customers(id),
    bytes_in        BIGINT NOT NULL,
    bytes_out       BIGINT NOT NULL,
    latency_ms      INTEGER NOT NULL,
    status_code     INTEGER,
    success         BOOLEAN NOT NULL,
    created_at      TIMESTAMPTZ DEFAULT now(),
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- Create monthly partitions (run via cron)
-- CREATE TABLE metering_2026_01 PARTITION OF metering
--     FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');

-- Hourly earnings aggregation
CREATE TABLE earnings (
    id              BIGSERIAL PRIMARY KEY,
    node_id         UUID NOT NULL REFERENCES nodes(id),
    epoch           TIMESTAMPTZ NOT NULL,  -- hour boundary
    bytes_proxied   BIGINT NOT NULL,
    requests_served INTEGER NOT NULL,
    success_rate    DECIMAL(5,4),
    avg_latency_ms  INTEGER,
    base_reward     DECIMAL(36,18) NOT NULL,
    multiplier      DECIMAL(6,3) NOT NULL,
    final_reward    DECIMAL(36,18) NOT NULL,
    claimed         BOOLEAN DEFAULT false,
    created_at      TIMESTAMPTZ DEFAULT now(),
    UNIQUE(node_id, epoch)
);

CREATE INDEX idx_earnings_node ON earnings(node_id);
CREATE INDEX idx_earnings_unclaimed ON earnings(node_id) WHERE NOT claimed;

-- Cumulative claimable rewards (updated by reward calculator)
CREATE TABLE claimable_rewards (
    wallet_address  TEXT PRIMARY KEY,
    cumulative      DECIMAL(36,18) NOT NULL DEFAULT 0,
    claimed         DECIMAL(36,18) NOT NULL DEFAULT 0,
    pending         DECIMAL(36,18) GENERATED ALWAYS AS (cumulative - claimed) STORED,
    nonce           INTEGER NOT NULL DEFAULT 0,
    merkle_proof    JSONB,
    last_claim_tx   TEXT,
    updated_at      TIMESTAMPTZ DEFAULT now()
);

-- Customers (bandwidth buyers)
CREATE TABLE customers (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email           TEXT UNIQUE NOT NULL,
    password_hash   TEXT NOT NULL,
    api_key_hash    TEXT UNIQUE,
    plan            TEXT DEFAULT 'free',
    rate_limit      INTEGER DEFAULT 100,  -- requests per day
    balance_usd     DECIMAL(12,2) DEFAULT 0,
    created_at      TIMESTAMPTZ DEFAULT now()
);

-- Customer usage
CREATE TABLE customer_usage (
    id              BIGSERIAL PRIMARY KEY,
    customer_id     UUID NOT NULL REFERENCES customers(id),
    date            DATE NOT NULL,
    requests        INTEGER DEFAULT 0,
    bytes_total     BIGINT DEFAULT 0,
    cost_usd        DECIMAL(12,6) DEFAULT 0,
    UNIQUE(customer_id, date)
);

-- Fraud events
CREATE TABLE fraud_events (
    id              BIGSERIAL PRIMARY KEY,
    node_id         UUID NOT NULL REFERENCES nodes(id),
    event_type      TEXT NOT NULL,  -- self_proxy, emulator, bandwidth_inflation, etc.
    severity        TEXT NOT NULL,  -- warning, critical
    details         JSONB,
    action_taken    TEXT,           -- none, trust_reduction, suspension, ban
    created_at      TIMESTAMPTZ DEFAULT now()
);

-- Referral tracking
CREATE TABLE referrals (
    id              BIGSERIAL PRIMARY KEY,
    referrer_id     UUID NOT NULL REFERENCES nodes(id),
    referee_id      UUID NOT NULL REFERENCES nodes(id),
    earnings_shared DECIMAL(36,18) DEFAULT 0,
    created_at      TIMESTAMPTZ DEFAULT now(),
    UNIQUE(referee_id)
);
```

## Redis Usage

```
# Real-time node status
node:{nodeID}:status     → "active" | "idle" | TTL 60s
node:{nodeID}:requests   → count of active requests (INCR/DECR)
node:{nodeID}:heartbeat  → last heartbeat timestamp

# Session stickiness (for customers needing same exit IP)
session:{sessionID}      → nodeID, TTL 5min

# Rate limiting
ratelimit:{apiKey}:{date} → request count, TTL 24h

# WebSocket connection mapping
ws:{nodeID}              → serverInstance (for routing in multi-server)

# Metering event stream
STREAM metering_events   → MeteringEvent records consumed by Metering Service

# Pub/Sub
channel:node_config      → broadcast config updates to all orchestrator instances
```

## API Design: Node-Facing Endpoints

```
# Node registration and management
POST   /v1/node/register            # Register new node
PUT    /v1/node/heartbeat           # Update status
GET    /v1/node/config              # Get node configuration
POST   /v1/node/attest              # Submit Play Integrity token

# Earnings
GET    /v1/earnings                 # Get earnings history
GET    /v1/earnings/summary         # Get earnings summary
GET    /v1/earnings/pending         # Get pending claimable amount

# Rewards
GET    /v1/rewards/proof            # Get Merkle proof for claiming
GET    /v1/rewards/history          # Get claim history

# Metering
POST   /v1/metering/report          # Report local metering (supplementary)

# Referral
GET    /v1/referral/code            # Get referral code
GET    /v1/referral/stats           # Get referral statistics
POST   /v1/referral/apply           # Apply referral code
```

## Deployment

### Docker Compose (Development)

```yaml
version: '3.8'

services:
  orchestrator:
    build:
      context: .
      target: orchestrator
    ports:
      - "8080:8080"   # WebSocket
    environment:
      - DATABASE_URL=postgres://proxycoin:secret@postgres:5432/proxycoin
      - REDIS_URL=redis://redis:6379
    depends_on:
      - postgres
      - redis

  api:
    build:
      context: .
      target: api
    ports:
      - "8081:8081"   # REST API
    environment:
      - DATABASE_URL=postgres://proxycoin:secret@postgres:5432/proxycoin
      - REDIS_URL=redis://redis:6379
    depends_on:
      - postgres
      - redis

  metering:
    build:
      context: .
      target: metering
    environment:
      - DATABASE_URL=postgres://proxycoin:secret@postgres:5432/proxycoin
      - REDIS_URL=redis://redis:6379
      - BASE_RPC_URL=https://mainnet.base.org
    depends_on:
      - postgres
      - redis

  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: proxycoin
      POSTGRES_USER: proxycoin
      POSTGRES_PASSWORD: secret
    volumes:
      - pgdata:/var/lib/postgresql/data
    ports:
      - "5432:5432"

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"

volumes:
  pgdata:
```
