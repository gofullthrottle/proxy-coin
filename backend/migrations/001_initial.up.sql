-- 001_initial.up.sql
-- Proxy Coin: initial database schema
-- PostgreSQL 16 · run via golang-migrate or psql

-- ---------------------------------------------------------------------------
-- Customers (bandwidth buyers)
-- Must be created before metering, which references it.
-- ---------------------------------------------------------------------------
CREATE TABLE customers (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT        UNIQUE NOT NULL,
    password_hash TEXT        NOT NULL,
    api_key_hash  TEXT        UNIQUE,                   -- bcrypt of "prxy_live_…" key
    plan          TEXT        NOT NULL DEFAULT 'free',  -- free | starter | pro | enterprise
    rate_limit    INTEGER     NOT NULL DEFAULT 100,     -- requests per day
    balance_usd   DECIMAL(12,2) NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- Node registry
-- ---------------------------------------------------------------------------
CREATE TABLE nodes (
    id             UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id      TEXT          UNIQUE NOT NULL,
    wallet_address TEXT          NOT NULL,
    ip             INET,
    country        TEXT,
    region         TEXT,
    city           TEXT,
    isp            TEXT,
    asn            INTEGER,
    is_residential BOOLEAN       NOT NULL DEFAULT true,
    network_type   TEXT          NOT NULL DEFAULT 'wifi',  -- wifi | cellular
    trust_score    DECIMAL(4,3)  NOT NULL DEFAULT 0.500,
    status         TEXT          NOT NULL DEFAULT 'pending',
                                    -- pending | active | idle | suspended | banned
    max_concurrent INTEGER       NOT NULL DEFAULT 5,
    staked_amount  DECIMAL(36,18) NOT NULL DEFAULT 0,
    referral_code  TEXT          UNIQUE,
    referred_by    UUID          REFERENCES nodes(id),
    registered_at  TIMESTAMPTZ   NOT NULL DEFAULT now(),
    last_heartbeat TIMESTAMPTZ,
    created_at     TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE INDEX idx_nodes_status  ON nodes(status);
CREATE INDEX idx_nodes_country ON nodes(country);
CREATE INDEX idx_nodes_trust   ON nodes(trust_score);
CREATE INDEX idx_nodes_wallet  ON nodes(wallet_address);

-- ---------------------------------------------------------------------------
-- Metering records (range-partitioned by month for efficient time-series queries)
-- ---------------------------------------------------------------------------
CREATE TABLE metering (
    id          BIGSERIAL,
    request_id  TEXT        NOT NULL,
    node_id     UUID        NOT NULL REFERENCES nodes(id),
    customer_id UUID        NOT NULL REFERENCES customers(id),
    bytes_in    BIGINT      NOT NULL,
    bytes_out   BIGINT      NOT NULL,
    latency_ms  INTEGER     NOT NULL,
    status_code INTEGER,
    success     BOOLEAN     NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- Default partition catches any rows whose created_at falls outside a named
-- monthly partition.  Monthly partitions are added by a cron job:
--   CREATE TABLE metering_YYYY_MM PARTITION OF metering
--       FOR VALUES FROM ('YYYY-MM-01') TO ('YYYY-MM+1-01');
CREATE TABLE metering_default PARTITION OF metering DEFAULT;

-- ---------------------------------------------------------------------------
-- Hourly earnings aggregation (one row per node per epoch hour)
-- ---------------------------------------------------------------------------
CREATE TABLE earnings (
    id              BIGSERIAL     PRIMARY KEY,
    node_id         UUID          NOT NULL REFERENCES nodes(id),
    epoch           TIMESTAMPTZ   NOT NULL,  -- truncated to the hour boundary
    bytes_proxied   BIGINT        NOT NULL,
    requests_served INTEGER       NOT NULL,
    success_rate    DECIMAL(5,4),
    avg_latency_ms  INTEGER,
    base_reward     DECIMAL(36,18) NOT NULL,
    multiplier      DECIMAL(6,3)  NOT NULL,
    final_reward    DECIMAL(36,18) NOT NULL,
    claimed         BOOLEAN       NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT now(),
    UNIQUE (node_id, epoch)
);

CREATE INDEX idx_earnings_node      ON earnings(node_id);
CREATE INDEX idx_earnings_unclaimed ON earnings(node_id) WHERE NOT claimed;

-- ---------------------------------------------------------------------------
-- Cumulative claimable rewards
-- Updated by the reward calculator after each Merkle tree generation cycle.
-- ---------------------------------------------------------------------------
CREATE TABLE claimable_rewards (
    wallet_address TEXT          PRIMARY KEY,
    cumulative     DECIMAL(36,18) NOT NULL DEFAULT 0,
    claimed        DECIMAL(36,18) NOT NULL DEFAULT 0,
    -- pending is derived; stored for fast reads by the claim API
    pending        DECIMAL(36,18) GENERATED ALWAYS AS (cumulative - claimed) STORED,
    nonce          INTEGER       NOT NULL DEFAULT 0,   -- incremented per claim batch
    merkle_proof   JSONB,
    last_claim_tx  TEXT,
    updated_at     TIMESTAMPTZ   NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- Customer usage (daily rollups)
-- ---------------------------------------------------------------------------
CREATE TABLE customer_usage (
    id          BIGSERIAL     PRIMARY KEY,
    customer_id UUID          NOT NULL REFERENCES customers(id),
    date        DATE          NOT NULL,
    requests    INTEGER       NOT NULL DEFAULT 0,
    bytes_total BIGINT        NOT NULL DEFAULT 0,
    cost_usd    DECIMAL(12,6) NOT NULL DEFAULT 0,
    UNIQUE (customer_id, date)
);

-- ---------------------------------------------------------------------------
-- Fraud events
-- ---------------------------------------------------------------------------
CREATE TABLE fraud_events (
    id           BIGSERIAL   PRIMARY KEY,
    node_id      UUID        NOT NULL REFERENCES nodes(id),
    event_type   TEXT        NOT NULL,  -- self_proxy | emulator | bandwidth_inflation | …
    severity     TEXT        NOT NULL,  -- warning | critical
    details      JSONB,
    action_taken TEXT,                  -- none | trust_reduction | suspension | ban
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_fraud_events_node     ON fraud_events(node_id);
CREATE INDEX idx_fraud_events_severity ON fraud_events(severity);

-- ---------------------------------------------------------------------------
-- Referral tracking
-- Each device (referee) can only be referred once (UNIQUE on referee_id).
-- ---------------------------------------------------------------------------
CREATE TABLE referrals (
    id              BIGSERIAL      PRIMARY KEY,
    referrer_id     UUID           NOT NULL REFERENCES nodes(id),
    referee_id      UUID           NOT NULL REFERENCES nodes(id),
    earnings_shared DECIMAL(36,18) NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ    NOT NULL DEFAULT now(),
    UNIQUE (referee_id)
);

CREATE INDEX idx_referrals_referrer ON referrals(referrer_id);
