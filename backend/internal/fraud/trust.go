// Package fraud provides anti-fraud detection for the Proxy Coin network.
package fraud

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// trustRecommendation is the action recommended based on a trust score threshold.
type trustRecommendation string

const (
	trustRecommendationOK      trustRecommendation = "ok"
	trustRecommendationReview  trustRecommendation = "review"
	trustRecommendationSuspend trustRecommendation = "suspend"
	trustRecommendationBan     trustRecommendation = "ban"
)

// TrustScore is the output of the trust calculation.
type TrustScore struct {
	NodeID         string              `json:"node_id"`
	Score          float64             `json:"score"` // 0.0 – 1.0
	Recommendation trustRecommendation `json:"recommendation"`
	Components     TrustComponents     `json:"components"`
	CalculatedAt   time.Time           `json:"calculated_at"`
}

// TrustComponents breaks down the weighted contribution of each factor.
type TrustComponents struct {
	Attestation  float64 `json:"attestation"`   // weight 0.15
	Uptime       float64 `json:"uptime"`        // weight 0.20
	Verification float64 `json:"verification"`  // weight 0.25 (no fraud events)
	IP           float64 `json:"ip"`            // weight 0.15 (residential)
	Bandwidth    float64 `json:"bandwidth"`     // weight 0.15 (consistent)
	Age          float64 `json:"age"`           // weight 0.10 (account age)
}

// weights must sum to 1.0.
var trustWeights = TrustComponents{
	Attestation:  0.15,
	Uptime:       0.20,
	Verification: 0.25,
	IP:           0.15,
	Bandwidth:    0.15,
	Age:          0.10,
}

// TrustCalculator computes trust scores for proxy nodes using a weighted model.
type TrustCalculator struct {
	pool *pgxpool.Pool
}

// NewTrustCalculator creates a TrustCalculator backed by the given pool.
func NewTrustCalculator(pool *pgxpool.Pool) *TrustCalculator {
	return &TrustCalculator{pool: pool}
}

// CalculateTrustScore computes a composite trust score for the given node.
//
// The model uses six weighted components (see TrustComponents for weights):
//
//	1. Attestation score  (Play Integrity validity)
//	2. Uptime score       (fraction of hours active in the last 7 days)
//	3. Verification score (absence of fraud events)
//	4. IP score           (residential classification)
//	5. Bandwidth score    (consistency of throughput)
//	6. Age score          (account age, capped at 90 days)
//
// Score thresholds:
//   - ≥ 0.3: OK (allow rewards)
//   - < 0.3: recommend suspend
//   - < 0.1: recommend ban
func (tc *TrustCalculator) CalculateTrustScore(ctx context.Context, nodeID string) (*TrustScore, error) {
	raw, err := tc.fetchRawMetrics(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("trust: fetch metrics for node %s: %w", nodeID, err)
	}

	components := TrustComponents{
		Attestation:  clamp01(raw.attestationScore),
		Uptime:       clamp01(raw.uptimeFraction),
		Verification: clamp01(raw.verificationScore),
		IP:           clamp01(raw.ipScore),
		Bandwidth:    clamp01(raw.bandwidthScore),
		Age:          clamp01(raw.ageScore),
	}

	score := components.Attestation*trustWeights.Attestation +
		components.Uptime*trustWeights.Uptime +
		components.Verification*trustWeights.Verification +
		components.IP*trustWeights.IP +
		components.Bandwidth*trustWeights.Bandwidth +
		components.Age*trustWeights.Age

	score = clamp01(score)

	recommendation := trustRecommendationOK
	switch {
	case score < 0.1:
		recommendation = trustRecommendationBan
	case score < 0.3:
		recommendation = trustRecommendationSuspend
	case score < 0.5:
		recommendation = trustRecommendationReview
	}

	ts := &TrustScore{
		NodeID:         nodeID,
		Score:          score,
		Recommendation: recommendation,
		Components:     components,
		CalculatedAt:   time.Now().UTC(),
	}

	// Persist updated trust score back to the nodes table.
	if err := tc.persistScore(ctx, nodeID, score); err != nil {
		log.Printf("trust: WARNING failed to persist score for node %s: %v", nodeID, err)
	}

	return ts, nil
}

// rawNodeMetrics holds the raw (unweighted) metric values for one node.
type rawNodeMetrics struct {
	attestationScore  float64
	uptimeFraction    float64
	verificationScore float64
	ipScore           float64
	bandwidthScore    float64
	ageScore          float64
}

// fetchRawMetrics queries all the data needed to compute the trust score.
func (tc *TrustCalculator) fetchRawMetrics(ctx context.Context, nodeID string) (*rawNodeMetrics, error) {
	const q = `
		WITH node_info AS (
			SELECT
				id,
				is_residential,
				registered_at,
				-- Hours since registration, capped at 2160 (90 days)
				LEAST(EXTRACT(EPOCH FROM (NOW() - registered_at)) / 3600, 2160) AS age_hours
			FROM nodes
			WHERE id = $1
		),
		uptime_info AS (
			SELECT
				COALESCE(
					COUNT(*) FILTER (WHERE bytes_proxied > 0)::float /
					NULLIF(COUNT(*), 0),
					0
				) AS uptime_fraction
			FROM earnings
			WHERE node_id = $1
			  AND epoch >= NOW() - INTERVAL '7 days'
		),
		fraud_info AS (
			SELECT COUNT(*) AS fraud_count
			FROM fraud_events
			WHERE node_id = $1
			  AND detected_at >= NOW() - INTERVAL '30 days'
			  AND severity IN ('high', 'critical')
		),
		bandwidth_info AS (
			SELECT
				COALESCE(STDDEV(bytes_proxied), 0) AS bandwidth_stddev,
				COALESCE(AVG(bytes_proxied),    1) AS bandwidth_avg
			FROM earnings
			WHERE node_id = $1
			  AND epoch >= NOW() - INTERVAL '7 days'
		),
		attestation_info AS (
			-- Attestation score: 1.0 if trust_score ≥ 0.5 (as a proxy for a valid
			-- Play Integrity attestation on file; proper integration in a later wave).
			SELECT COALESCE(CAST(trust_score AS float8), 0) AS attestation_proxy
			FROM nodes
			WHERE id = $1
		)
		SELECT
			ni.is_residential,
			ni.age_hours,
			COALESCE(ui.uptime_fraction, 0),
			COALESCE(fi.fraud_count, 0),
			COALESCE(bi.bandwidth_stddev, 0),
			COALESCE(bi.bandwidth_avg,    1),
			COALESCE(ai.attestation_proxy, 0)
		FROM node_info ni
		CROSS JOIN uptime_info ui
		CROSS JOIN fraud_info  fi
		CROSS JOIN bandwidth_info bi
		CROSS JOIN attestation_info ai
	`

	var (
		isResidential    bool
		ageHours         float64
		uptimeFraction   float64
		fraudCount       int
		bandwidthStddev  float64
		bandwidthAvg     float64
		attestationProxy float64
	)

	row := tc.pool.QueryRow(ctx, q, nodeID)
	if err := row.Scan(
		&isResidential,
		&ageHours,
		&uptimeFraction,
		&fraudCount,
		&bandwidthStddev,
		&bandwidthAvg,
		&attestationProxy,
	); err != nil {
		return nil, fmt.Errorf("trust: scan metrics: %w", err)
	}

	// Derive component scores.

	// Attestation: use the stored attestation proxy (proper Play Integrity in wave 14).
	attestationScore := clamp01(attestationProxy)

	// IP score: 1.0 for residential, 0.0 for datacenter.
	ipScore := 0.0
	if isResidential {
		ipScore = 1.0
	}

	// Age score: linear ramp from 0 → 1 over 90 days (2160 hours).
	ageScore := clamp01(ageHours / 2160.0)

	// Verification: penalise recent high-severity fraud events.
	// 0 events → 1.0; each event subtracts 0.25, capped at 0.
	verificationScore := clamp01(1.0 - float64(fraudCount)*0.25)

	// Bandwidth consistency: coefficient of variation (lower CV = more consistent).
	// CV = stddev/mean; CV < 1 → consistent → high score.
	cv := 0.0
	if bandwidthAvg > 0 {
		cv = bandwidthStddev / bandwidthAvg
	}
	bandwidthScore := clamp01(1.0 - math.Min(cv, 1.0))

	return &rawNodeMetrics{
		attestationScore:  attestationScore,
		uptimeFraction:    uptimeFraction,
		verificationScore: verificationScore,
		ipScore:           ipScore,
		bandwidthScore:    bandwidthScore,
		ageScore:          ageScore,
	}, nil
}

// persistScore writes the calculated trust score back to the nodes table.
func (tc *TrustCalculator) persistScore(ctx context.Context, nodeID string, score float64) error {
	const q = `
		UPDATE nodes
		SET trust_score = $2,
		    updated_at  = now()
		WHERE id = $1
	`
	if _, err := tc.pool.Exec(ctx, q, nodeID, score); err != nil {
		return fmt.Errorf("trust: persist score for %s: %w", nodeID, err)
	}
	return nil
}

// clamp01 clamps a float to the [0.0, 1.0] range.
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
