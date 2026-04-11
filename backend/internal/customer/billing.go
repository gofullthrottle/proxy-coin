// Package customer handles customer-facing REST API operations.
package customer

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ---------------------------------------------------------------------------
// Plan definitions
// ---------------------------------------------------------------------------

// Plan describes a customer pricing tier.
type Plan struct {
	Name           string  `json:"name"`
	DisplayName    string  `json:"display_name"`
	MonthlyUSD     float64 `json:"monthly_usd"`      // 0 = free; -1 = custom/contact sales
	DailyRequests  int64   `json:"daily_requests"`
	IncludedGB     int64   `json:"included_gb"`       // GB included in base price
	OverageUSDPerGB float64 `json:"overage_usd_per_gb"` // price per additional GB
}

// Plans contains all available pricing plans keyed by plan name.
var Plans = map[string]Plan{
	TierFree: {
		Name:            TierFree,
		DisplayName:     "Free",
		MonthlyUSD:      0,
		DailyRequests:   100,
		IncludedGB:      1,
		OverageUSDPerGB: 0,
	},
	TierStarter: {
		Name:            TierStarter,
		DisplayName:     "Starter",
		MonthlyUSD:      49,
		DailyRequests:   10_000,
		IncludedGB:      100,
		OverageUSDPerGB: 0.50,
	},
	TierPro: {
		Name:            TierPro,
		DisplayName:     "Pro",
		MonthlyUSD:      199,
		DailyRequests:   100_000,
		IncludedGB:      1_000,
		OverageUSDPerGB: 0.35,
	},
	TierEnterprise: {
		Name:            TierEnterprise,
		DisplayName:     "Enterprise",
		MonthlyUSD:      -1,
		DailyRequests:   1_000_000,
		IncludedGB:      10_000,
		OverageUSDPerGB: 0.20,
	},
}

// ---------------------------------------------------------------------------
// BillingRecord and BillingManager (retained for backward compatibility)
// ---------------------------------------------------------------------------

// BillingRecord tracks a customer's usage charges for a billing period.
type BillingRecord struct {
	CustomerID  string      `json:"customer_id"`
	Tier        PricingTier `json:"tier"`
	PeriodStart time.Time   `json:"period_start"`
	PeriodEnd   time.Time   `json:"period_end"`
	BytesUsed   int64       `json:"bytes_used"`
	// AmountCents is the total charge in USD cents.
	AmountCents int64 `json:"amount_cents"`
}

// BillingManager computes charges and manages subscription state.
type BillingManager struct {
	pool *pgxpool.Pool
}

// NewBillingManager creates a BillingManager.
func NewBillingManager(pool *pgxpool.Pool) *BillingManager {
	return &BillingManager{pool: pool}
}

// Calculate computes the charge for a billing period and writes it to rec.AmountCents.
func (b *BillingManager) Calculate(ctx context.Context, rec *BillingRecord) error {
	plan, ok := Plans[rec.Tier]
	if !ok {
		plan = Plans[TierFree]
	}

	// Bytes → GB
	gb := float64(rec.BytesUsed) / 1_000_000_000.0

	// Compute overage beyond the included GB allowance.
	overage := gb - float64(plan.IncludedGB)
	if overage < 0 {
		overage = 0
	}

	totalUSD := plan.MonthlyUSD + overage*plan.OverageUSDPerGB
	if totalUSD < 0 {
		totalUSD = 0 // enterprise: compute separately
	}

	rec.AmountCents = int64(totalUSD * 100)
	return nil
}

// ---------------------------------------------------------------------------
// BillingSummary and Invoice
// ---------------------------------------------------------------------------

// BillingSummary is the response payload for GET /v1/billing.
type BillingSummary struct {
	CustomerID      string    `json:"customer_id"`
	Plan            Plan      `json:"plan"`
	CurrentPeriod   string    `json:"current_period"` // YYYY-MM
	UsageBytes      int64     `json:"usage_bytes"`
	UsageRequests   int64     `json:"usage_requests"`
	EstimatedUSD    float64   `json:"estimated_usd"`
	NextBillingDate time.Time `json:"next_billing_date"`
}

// Invoice represents a completed billing invoice.
type Invoice struct {
	ID          string    `json:"id"`
	CustomerID  string    `json:"customer_id"`
	Period      string    `json:"period"` // YYYY-MM
	AmountUSD   float64   `json:"amount_usd"`
	UsageBytes  int64     `json:"usage_bytes"`
	CreatedAt   time.Time `json:"created_at"`
	PaidAt      *time.Time `json:"paid_at,omitempty"`
	Status      string    `json:"status"` // paid | open | void
}

// GetBillingSummary returns the current billing summary for a customer.
func (b *BillingManager) GetBillingSummary(ctx context.Context, customerID string) (*BillingSummary, error) {
	// Fetch plan and current period usage from the DB.
	const q = `
		SELECT c.plan,
		       COALESCE(SUM(u.bytes),    0) AS usage_bytes,
		       COALESCE(SUM(u.requests), 0) AS usage_requests
		FROM customers c
		LEFT JOIN customer_usage u
		    ON u.customer_id = c.id
		    AND DATE_TRUNC('month', u.recorded_at) = DATE_TRUNC('month', NOW())
		WHERE c.id = $1
		GROUP BY c.plan
	`

	var tierName string
	var usageBytes, usageRequests int64
	row := b.pool.QueryRow(ctx, q, customerID)
	if err := row.Scan(&tierName, &usageBytes, &usageRequests); err != nil {
		return nil, fmt.Errorf("billing: get summary for %s: %w", customerID, err)
	}

	plan, ok := Plans[tierName]
	if !ok {
		plan = Plans[TierFree]
	}

	now := time.Now().UTC()
	period := fmt.Sprintf("%d-%02d", now.Year(), now.Month())

	// Estimate overage charge.
	overageGB := float64(usageBytes)/1e9 - float64(plan.IncludedGB)
	if overageGB < 0 {
		overageGB = 0
	}
	estimatedUSD := plan.MonthlyUSD + overageGB*plan.OverageUSDPerGB

	// Next billing date: first of next month.
	nextBilling := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)

	return &BillingSummary{
		CustomerID:      customerID,
		Plan:            plan,
		CurrentPeriod:   period,
		UsageBytes:      usageBytes,
		UsageRequests:   usageRequests,
		EstimatedUSD:    estimatedUSD,
		NextBillingDate: nextBilling,
	}, nil
}

// GetInvoices returns invoices for a customer within the given date range.
// from and to are strings in YYYY-MM format.
func (b *BillingManager) GetInvoices(ctx context.Context, customerID, from, to string) ([]Invoice, error) {
	const q = `
		SELECT id, customer_id, period, amount_usd, usage_bytes, created_at, paid_at, status
		FROM invoices
		WHERE customer_id = $1
		  AND period >= $2
		  AND period <= $3
		ORDER BY period DESC
	`
	rows, err := b.pool.Query(ctx, q, customerID, from, to)
	if err != nil {
		return nil, fmt.Errorf("billing: get invoices for %s: %w", customerID, err)
	}
	defer rows.Close()

	var invoices []Invoice
	for rows.Next() {
		var inv Invoice
		if err := rows.Scan(
			&inv.ID,
			&inv.CustomerID,
			&inv.Period,
			&inv.AmountUSD,
			&inv.UsageBytes,
			&inv.CreatedAt,
			&inv.PaidAt,
			&inv.Status,
		); err != nil {
			return nil, fmt.Errorf("billing: scan invoice row: %w", err)
		}
		invoices = append(invoices, inv)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("billing: iterate invoice rows: %w", err)
	}
	if invoices == nil {
		invoices = []Invoice{}
	}
	return invoices, nil
}
