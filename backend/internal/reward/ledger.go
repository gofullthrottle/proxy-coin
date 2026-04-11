// Package reward calculates PRXY token rewards for proxy node operators.
package reward

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// EarningRecord represents the hourly earnings aggregation for a single node.
type EarningRecord struct {
	NodeID         string
	Epoch          time.Time
	BytesProxied   int64
	RequestsServed int
	SuccessRate    float64
	AvgLatencyMs   int
	BaseReward     float64
	Multiplier     float64
	FinalReward    float64
}

// EarningsSummary aggregates earnings data for API responses.
type EarningsSummary struct {
	TodayEarnings   float64
	AllTimeEarnings float64
	PendingClaim    float64
	TrustScore      float64
}

// Ledger manages earnings records in PostgreSQL (pre-blockchain point tracking).
// Each row in the earnings table represents one hourly epoch for one node.
type Ledger struct {
	pool *pgxpool.Pool
}

// NewLedger creates a Ledger backed by the given connection pool.
func NewLedger(pool *pgxpool.Pool) *Ledger {
	return &Ledger{pool: pool}
}

// RecordEarnings inserts or updates an hourly earnings record for a node.
// The (node_id, epoch) pair is unique; a conflict updates all mutable columns.
func (l *Ledger) RecordEarnings(ctx context.Context, record EarningRecord) error {
	const q = `
		INSERT INTO earnings (
			node_id, epoch,
			bytes_proxied, requests_served, success_rate, avg_latency_ms,
			base_reward, multiplier, final_reward
		) VALUES (
			$1, $2,
			$3, $4, $5, $6,
			$7, $8, $9
		)
		ON CONFLICT (node_id, epoch) DO UPDATE SET
			bytes_proxied   = EXCLUDED.bytes_proxied,
			requests_served = EXCLUDED.requests_served,
			success_rate    = EXCLUDED.success_rate,
			avg_latency_ms  = EXCLUDED.avg_latency_ms,
			base_reward     = EXCLUDED.base_reward,
			multiplier      = EXCLUDED.multiplier,
			final_reward    = EXCLUDED.final_reward
	`

	_, err := l.pool.Exec(ctx, q,
		record.NodeID,
		record.Epoch.UTC().Truncate(time.Hour),
		record.BytesProxied,
		record.RequestsServed,
		record.SuccessRate,
		record.AvgLatencyMs,
		record.BaseReward,
		record.Multiplier,
		record.FinalReward,
	)
	if err != nil {
		return fmt.Errorf("ledger: record earnings for node %s epoch %s: %w",
			record.NodeID, record.Epoch.Format(time.RFC3339), err)
	}
	return nil
}

// GetEarnings returns all earnings records for a node within a date range.
// from and to are ISO-8601 timestamps (e.g. "2026-01-01T00:00:00Z").
// Pass empty strings to remove the respective bound.
func (l *Ledger) GetEarnings(ctx context.Context, nodeID, from, to string) ([]EarningRecord, error) {
	const baseQ = `
		SELECT
			node_id, epoch,
			bytes_proxied, requests_served,
			COALESCE(CAST(success_rate AS float8), 0) AS success_rate,
			COALESCE(avg_latency_ms, 0)               AS avg_latency_ms,
			CAST(base_reward   AS float8),
			CAST(multiplier    AS float8),
			CAST(final_reward  AS float8)
		FROM earnings
		WHERE node_id = $1
	`

	args := []any{nodeID}
	argIdx := 2
	extra := ""

	if from != "" {
		extra += fmt.Sprintf(" AND epoch >= $%d", argIdx)
		args = append(args, from)
		argIdx++
	}
	if to != "" {
		extra += fmt.Sprintf(" AND epoch <= $%d", argIdx)
		args = append(args, to)
	}

	q := baseQ + extra + " ORDER BY epoch ASC"
	rows, err := l.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("ledger: get earnings for node %s: %w", nodeID, err)
	}
	defer rows.Close()

	var records []EarningRecord
	for rows.Next() {
		var r EarningRecord
		if err := rows.Scan(
			&r.NodeID,
			&r.Epoch,
			&r.BytesProxied,
			&r.RequestsServed,
			&r.SuccessRate,
			&r.AvgLatencyMs,
			&r.BaseReward,
			&r.Multiplier,
			&r.FinalReward,
		); err != nil {
			return nil, fmt.Errorf("ledger: scan earnings row: %w", err)
		}
		records = append(records, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ledger: iterate earnings rows: %w", err)
	}
	return records, nil
}

// GetSummary returns aggregated earnings statistics for a node, including
// today's earnings, all-time earnings, pending claimable amount, and the
// node's current trust score.
func (l *Ledger) GetSummary(ctx context.Context, nodeID string) (*EarningsSummary, error) {
	const q = `
		SELECT
			COALESCE(SUM(CAST(final_reward AS float8)) FILTER (
				WHERE epoch >= date_trunc('day', now() AT TIME ZONE 'UTC')
			), 0) AS today_earnings,
			COALESCE(SUM(CAST(final_reward AS float8)), 0) AS all_time_earnings,
			COALESCE(SUM(CAST(final_reward AS float8)) FILTER (
				WHERE NOT claimed
			), 0) AS pending_claim
		FROM earnings
		WHERE node_id = $1
	`

	var summary EarningsSummary
	row := l.pool.QueryRow(ctx, q, nodeID)
	if err := row.Scan(
		&summary.TodayEarnings,
		&summary.AllTimeEarnings,
		&summary.PendingClaim,
	); err != nil {
		return nil, fmt.Errorf("ledger: get summary for node %s: %w", nodeID, err)
	}

	// Fetch trust score separately from the nodes table.
	const trustQ = `
		SELECT CAST(trust_score AS float8)
		FROM nodes
		WHERE id = $1
	`
	if err := l.pool.QueryRow(ctx, trustQ, nodeID).Scan(&summary.TrustScore); err != nil {
		// Trust score is best-effort; do not fail the whole summary.
		summary.TrustScore = 0
	}

	return &summary, nil
}

// GetTodayEarnings returns the sum of final_reward for a node since midnight UTC.
func (l *Ledger) GetTodayEarnings(ctx context.Context, nodeID string) (float64, error) {
	const q = `
		SELECT COALESCE(SUM(CAST(final_reward AS float8)), 0)
		FROM earnings
		WHERE node_id = $1
		  AND epoch >= date_trunc('day', now() AT TIME ZONE 'UTC')
	`

	var total float64
	if err := l.pool.QueryRow(ctx, q, nodeID).Scan(&total); err != nil {
		return 0, fmt.Errorf("ledger: get today earnings for node %s: %w", nodeID, err)
	}
	return total, nil
}

// GetAllTimeEarnings returns the sum of final_reward across all epochs for a node.
func (l *Ledger) GetAllTimeEarnings(ctx context.Context, nodeID string) (float64, error) {
	const q = `
		SELECT COALESCE(SUM(CAST(final_reward AS float8)), 0)
		FROM earnings
		WHERE node_id = $1
	`

	var total float64
	if err := l.pool.QueryRow(ctx, q, nodeID).Scan(&total); err != nil {
		return 0, fmt.Errorf("ledger: get all-time earnings for node %s: %w", nodeID, err)
	}
	return total, nil
}
