// Package customer handles customer-facing REST API operations.
package customer

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DailyUsage represents aggregated usage for a single calendar day.
type DailyUsage struct {
	Date     string  `json:"date"`      // YYYY-MM-DD
	Requests int64   `json:"requests"`
	Bytes    int64   `json:"bytes"`
	CostUSD  float64 `json:"cost_usd"`
}

// UsageTracker records and queries per-customer bandwidth usage.
type UsageTracker struct {
	pool *pgxpool.Pool
}

// NewUsageTracker creates a UsageTracker backed by the given connection pool.
func NewUsageTracker(pool *pgxpool.Pool) *UsageTracker {
	return &UsageTracker{pool: pool}
}

// RecordUsage inserts a usage event for the given customer.
// costUSD is the per-request cost in US dollars.
func (u *UsageTracker) RecordUsage(ctx context.Context, customerID string, requests, bytes int64, costUSD float64) error {
	const q = `
		INSERT INTO customer_usage (customer_id, requests, bytes, cost_usd, recorded_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := u.pool.Exec(ctx, q, customerID, requests, bytes, costUSD, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("usage: record for customer %s: %w", customerID, err)
	}
	return nil
}

// GetUsage returns an aggregated usage summary for the customer (all time).
func (u *UsageTracker) GetUsage(ctx context.Context, customerID string) (*UsageSummary, error) {
	const q = `
		SELECT
			COALESCE(SUM(requests), 0) AS requests,
			COALESCE(SUM(bytes),    0) AS bytes
		FROM customer_usage
		WHERE customer_id = $1
	`
	var summary UsageSummary
	summary.CustomerID = customerID
	summary.PeriodStart = time.Time{}
	summary.PeriodEnd = time.Now().UTC()

	if err := u.pool.QueryRow(ctx, q, customerID).Scan(
		&summary.Requests,
		&summary.BytesUsed,
	); err != nil {
		return nil, fmt.Errorf("usage: get for customer %s: %w", customerID, err)
	}
	return &summary, nil
}

// GetDailyUsage returns per-day usage breakdown for a customer within a date range.
// from and to are date strings in YYYY-MM-DD format.
func (u *UsageTracker) GetDailyUsage(ctx context.Context, customerID, from, to string) ([]DailyUsage, error) {
	const q = `
		SELECT
			DATE(recorded_at AT TIME ZONE 'UTC')::text AS date,
			SUM(requests)                              AS requests,
			SUM(bytes)                                 AS bytes,
			SUM(cost_usd)                              AS cost_usd
		FROM customer_usage
		WHERE customer_id = $1
		  AND DATE(recorded_at AT TIME ZONE 'UTC') >= $2::date
		  AND DATE(recorded_at AT TIME ZONE 'UTC') <= $3::date
		GROUP BY DATE(recorded_at AT TIME ZONE 'UTC')
		ORDER BY date ASC
	`
	rows, err := u.pool.Query(ctx, q, customerID, from, to)
	if err != nil {
		return nil, fmt.Errorf("usage: daily query for customer %s: %w", customerID, err)
	}
	defer rows.Close()

	var result []DailyUsage
	for rows.Next() {
		var d DailyUsage
		if err := rows.Scan(&d.Date, &d.Requests, &d.Bytes, &d.CostUSD); err != nil {
			return nil, fmt.Errorf("usage: scan daily row: %w", err)
		}
		result = append(result, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("usage: iterate daily rows: %w", err)
	}
	if result == nil {
		result = []DailyUsage{}
	}
	return result, nil
}

// CheckRateLimit reports whether the customer may make a request given their plan.
// This is a database-side check using the daily usage counter, complementing
// the Redis-based RateLimiter in the auth package.
//
// plan must be one of: free, starter, pro, enterprise.
func (u *UsageTracker) CheckRateLimit(ctx context.Context, customerID string, plan string) (bool, error) {
	dailyLimits := map[string]int64{
		"free":       100,
		"starter":    10_000,
		"pro":        100_000,
		"enterprise": 1_000_000,
	}
	limit, ok := dailyLimits[plan]
	if !ok {
		limit = dailyLimits["free"]
	}

	const q = `
		SELECT COALESCE(SUM(requests), 0)
		FROM customer_usage
		WHERE customer_id = $1
		  AND DATE(recorded_at AT TIME ZONE 'UTC') = CURRENT_DATE
	`
	var total int64
	if err := u.pool.QueryRow(ctx, q, customerID).Scan(&total); err != nil {
		return false, fmt.Errorf("usage: check rate limit for customer %s: %w", customerID, err)
	}
	return total < limit, nil
}
