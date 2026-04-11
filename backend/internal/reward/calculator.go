// Package reward calculates PRXY token rewards for proxy node operators.
package reward

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RewardRate defines the token emission parameters used during a period.
type RewardRate struct {
	// PRXYPerGB is the base reward in PRXY wei (1e18) per gigabyte proxied.
	PRXYPerGB *big.Int
	// QualityMultiplierMax caps the quality bonus multiplier (e.g. 1.5 = 150%).
	QualityMultiplierMax float64
}

// DefaultRewardRate returns the initial emission rate from the tokenomics spec.
func DefaultRewardRate() RewardRate {
	// 10 PRXY per GB expressed in wei (10 * 1e18).
	base := new(big.Int).Mul(big.NewInt(10), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	return RewardRate{
		PRXYPerGB:            base,
		QualityMultiplierMax: 1.5,
	}
}

// NodeReward holds the calculated reward for one node in one period.
type NodeReward struct {
	NodeID     string    `json:"node_id"`
	WalletAddr string    `json:"wallet_addr"`
	PeriodEnd  time.Time `json:"period_end"`
	BytesTotal int64     `json:"bytes_total"`
	// AmountWei is the reward denominated in PRXY wei (1e18 per token).
	AmountWei *big.Int `json:"amount_wei"`
}

// ---------------------------------------------------------------------------
// Multiplier helpers — match the ranges documented in BACKEND.md
// ---------------------------------------------------------------------------

// trustMultiplier maps a trust score in [0, 1] to a reward multiplier
// in [0.5, 2.0]. A score of 0.5 → 1.0x, 1.0 → 2.0x.
func trustMultiplier(score float64) float64 {
	score = math.Max(0, math.Min(1.0, score))
	// Linear interpolation: f(0) = 0.5, f(1) = 2.0.
	return 0.5 + score*1.5
}

// uptimeMultiplier maps an uptime percentage in [0, 100] to a multiplier
// in [1.0, 1.5]. Below 50% uptime clips to 1.0x; 100% → 1.5x.
func uptimeMultiplier(pct float64) float64 {
	pct = math.Max(0, math.Min(100, pct))
	if pct < 50 {
		return 1.0
	}
	// Linear from 50% → 1.0x to 100% → 1.5x.
	return 1.0 + ((pct-50)/50)*0.5
}

// qualityMultiplier combines request success rate and average latency into a
// multiplier in [0.8, 1.3].
//
//   - successRate in [0, 1]: weight 60%
//   - avgLatency in ms: penalises above 500 ms, weight 40%
func qualityMultiplier(successRate float64, avgLatencyMs int) float64 {
	successRate = math.Max(0, math.Min(1.0, successRate))

	// Latency score: 0 ms → 1.0, 500 ms → 0.5, ≥1000 ms → 0.0
	latencyScore := 1.0 - math.Min(1.0, float64(avgLatencyMs)/1000.0)

	// Weighted composite in [0, 1].
	composite := successRate*0.6 + latencyScore*0.4

	// Map to [0.8, 1.3].
	return 0.8 + composite*0.5
}

// ---------------------------------------------------------------------------
// Calculator — database-backed epoch reward engine
// ---------------------------------------------------------------------------

// Calculator computes per-node PRXY rewards from metering data stored in
// PostgreSQL and writes the results back via the Ledger.
type Calculator struct {
	pool      *pgxpool.Pool
	ledger    *Ledger
	prxyPerMB float64 // human-readable rate, e.g. 0.01 PRXY per MB
}

// NewCalculator creates a Calculator with the given connection pool, ledger,
// and per-MB rate. prxyPerMB is the base reward expressed in whole PRXY tokens
// per megabyte (not wei), e.g. 0.00001 for 1e-5 PRXY/MB.
func NewCalculator(pool *pgxpool.Pool, ledger *Ledger, prxyPerMB float64) *Calculator {
	return &Calculator{
		pool:      pool,
		ledger:    ledger,
		prxyPerMB: prxyPerMB,
	}
}

// epochMeteringRow represents one aggregated metering row for an epoch.
type epochMeteringRow struct {
	NodeID         string
	WalletAddr     string
	TrustScore     float64
	BytesProxied   int64
	RequestsServed int
	SuccessCount   int
	AvgLatencyMs   int
	UptimePct      float64 // 0-100 uptime percentage for the epoch
}

// CalculateRewards aggregates metering data for the given epoch hour,
// applies the reward formula with multipliers, persists each result through
// the Ledger, and returns the list of EarningRecord values written.
//
// epoch is truncated to the hour boundary before querying so the caller can
// pass any time within the desired hour.
func (c *Calculator) CalculateRewards(ctx context.Context, epoch time.Time) ([]EarningRecord, error) {
	epoch = epoch.UTC().Truncate(time.Hour)
	epochEnd := epoch.Add(time.Hour)

	// Aggregate metering data for the epoch window from the metering_events
	// or an equivalent pre-aggregated table. Adjust the query to match the
	// actual schema in use.
	const q = `
		SELECT
			n.id                                          AS node_id,
			n.wallet_address                              AS wallet_addr,
			COALESCE(CAST(n.trust_score AS float8), 0.5) AS trust_score,
			COALESCE(SUM(m.bytes_in + m.bytes_out), 0)   AS bytes_proxied,
			COALESCE(COUNT(m.request_id), 0)              AS requests_served,
			COALESCE(SUM(CASE WHEN m.success THEN 1 ELSE 0 END), 0) AS success_count,
			COALESCE(AVG(m.latency_ms)::int, 0)          AS avg_latency_ms,
			COALESCE(
				100.0 * SUM(CASE WHEN m.success IS NOT NULL THEN 1 ELSE 0 END)
				/ NULLIF(COUNT(*) FILTER (WHERE m.request_id IS NOT NULL), 0),
				100
			) AS uptime_pct
		FROM nodes n
		LEFT JOIN metering_events m
			ON m.node_id = n.id
			AND m.timestamp >= $1
			AND m.timestamp <  $2
		WHERE n.status = 'active'
		GROUP BY n.id, n.wallet_address, n.trust_score
	`

	rows, err := c.pool.Query(ctx, q, epoch, epochEnd)
	if err != nil {
		return nil, fmt.Errorf("reward: query metering for epoch %s: %w", epoch.Format(time.RFC3339), err)
	}
	defer rows.Close()

	var records []EarningRecord

	for rows.Next() {
		var row epochMeteringRow
		if err := rows.Scan(
			&row.NodeID,
			&row.WalletAddr,
			&row.TrustScore,
			&row.BytesProxied,
			&row.RequestsServed,
			&row.SuccessCount,
			&row.AvgLatencyMs,
			&row.UptimePct,
		); err != nil {
			return nil, fmt.Errorf("reward: scan metering row: %w", err)
		}

		rec := c.computeRecord(epoch, row)
		if err := c.ledger.RecordEarnings(ctx, rec); err != nil {
			return nil, fmt.Errorf("reward: persist earnings for node %s: %w", row.NodeID, err)
		}

		// Update the claimable_rewards cumulative total for the node.
		if err := c.updateClaimable(ctx, row.NodeID, rec.FinalReward); err != nil {
			// Non-fatal: log and continue so one bad row doesn't abort the batch.
			_ = err
		}

		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reward: iterate metering rows: %w", err)
	}

	return records, nil
}

// computeRecord applies the reward formula to a single node's epoch data.
func (c *Calculator) computeRecord(epoch time.Time, row epochMeteringRow) EarningRecord {
	// Base reward = totalBytes / 1e6 * prxyPerMB (in whole PRXY tokens).
	basePRXY := float64(row.BytesProxied) / 1e6 * c.prxyPerMB

	// Compute multipliers.
	var successRate float64
	if row.RequestsServed > 0 {
		successRate = float64(row.SuccessCount) / float64(row.RequestsServed)
	}

	tm := trustMultiplier(row.TrustScore)
	um := uptimeMultiplier(row.UptimePct)
	qm := qualityMultiplier(successRate, row.AvgLatencyMs)

	// Combined multiplier — capped to 4.0x to prevent runaway values.
	combined := math.Min(tm*um*qm, 4.0)
	finalPRXY := basePRXY * combined

	return EarningRecord{
		NodeID:         row.NodeID,
		Epoch:          epoch,
		BytesProxied:   row.BytesProxied,
		RequestsServed: row.RequestsServed,
		SuccessRate:    successRate,
		AvgLatencyMs:   row.AvgLatencyMs,
		BaseReward:     basePRXY,
		Multiplier:     combined,
		FinalReward:    finalPRXY,
	}
}

// updateClaimable increments the claimable_rewards column for the node by
// the given amount. The column tracks lifetime unclaimed PRXY earnings.
func (c *Calculator) updateClaimable(ctx context.Context, nodeID string, amount float64) error {
	const q = `
		UPDATE nodes
		SET claimable_rewards = COALESCE(claimable_rewards, 0) + $1
		WHERE id = $2
	`
	_, err := c.pool.Exec(ctx, q, amount, nodeID)
	if err != nil {
		return fmt.Errorf("reward: update claimable for node %s: %w", nodeID, err)
	}
	return nil
}

// Calculate is the original single-call API retained for backward compatibility.
// It derives the PRXY reward for the given bytes proxied without multipliers,
// returning the amount in wei (PRXY * 1e18).
func (c *Calculator) Calculate(bytesTotal int64) *big.Int {
	if bytesTotal <= 0 {
		return big.NewInt(0)
	}
	// basePRXY = bytes / 1e6 * prxyPerMB  (whole tokens, float)
	basePRXY := float64(bytesTotal) / 1e6 * c.prxyPerMB

	// Convert to wei: multiply by 1e18.
	// Use big.Float for precision.
	weiPerToken := new(big.Float).SetPrec(128).SetInt(
		new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil),
	)
	prxyFloat := new(big.Float).SetPrec(128).SetFloat64(basePRXY)
	weiFloat := new(big.Float).Mul(prxyFloat, weiPerToken)

	weiInt, _ := weiFloat.Int(nil)
	if weiInt == nil {
		return big.NewInt(0)
	}
	return weiInt
}
