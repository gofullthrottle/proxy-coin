// Package customer handles customer-facing REST API operations.
package customer

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// referralEarningsSharePct is the percentage of the referee's earnings
	// shared back to the referrer as a bonus.
	referralEarningsSharePct = 5.0

	// referralCodeLength is the number of random bytes used for referral codes.
	// Each byte → 2 hex chars, so 8 bytes → 16-char code.
	referralCodeLength = 8
)

// ReferralStats holds statistics about a node's referral programme.
type ReferralStats struct {
	NodeID        string    `json:"node_id"`
	ReferralCode  string    `json:"referral_code"`
	TotalReferrals int      `json:"total_referrals"`
	// BonusEarnedTotal is the lifetime PRXY bonus earned from referrals (whole tokens).
	BonusEarnedTotal float64   `json:"bonus_earned_total"`
	// EarningsSharePct is the percentage shared from each referee's earnings.
	EarningsSharePct float64   `json:"earnings_share_pct"`
	GeneratedAt      time.Time `json:"generated_at"`
}

// ReferralManager manages the referral program for node operators.
type ReferralManager struct {
	pool *pgxpool.Pool
}

// NewReferralManager creates a ReferralManager backed by the given pool.
func NewReferralManager(pool *pgxpool.Pool) *ReferralManager {
	return &ReferralManager{pool: pool}
}

// GenerateReferralCode creates a unique referral code for the given node.
// If the node already has a code, the existing code is returned.
func (m *ReferralManager) GenerateReferralCode(ctx context.Context, nodeID string) (string, error) {
	// Check if a code already exists.
	const selectQ = `SELECT referral_code FROM nodes WHERE id = $1`
	var existingCode string
	err := m.pool.QueryRow(ctx, selectQ, nodeID).Scan(&existingCode)
	if err != nil && err != pgx.ErrNoRows {
		return "", fmt.Errorf("referral: check existing code for node %s: %w", nodeID, err)
	}
	if existingCode != "" {
		return existingCode, nil
	}

	// Generate a new unique code.
	var code string
	for attempt := 0; attempt < 5; attempt++ {
		raw := make([]byte, referralCodeLength)
		if _, err := rand.Read(raw); err != nil {
			return "", fmt.Errorf("referral: generate random code: %w", err)
		}
		code = hex.EncodeToString(raw)

		const updateQ = `
			UPDATE nodes
			SET referral_code = $2,
			    updated_at    = now()
			WHERE id = $1
			  AND (referral_code IS NULL OR referral_code = '')
		`
		tag, err := m.pool.Exec(ctx, updateQ, nodeID, code)
		if err != nil {
			continue // retry on conflict
		}
		if tag.RowsAffected() > 0 {
			return code, nil
		}

		// Another process set the code concurrently; fetch and return it.
		if err := m.pool.QueryRow(ctx, selectQ, nodeID).Scan(&existingCode); err == nil && existingCode != "" {
			return existingCode, nil
		}
	}

	return "", fmt.Errorf("referral: failed to generate unique code for node %s after 5 attempts", nodeID)
}

// ApplyReferralCode links a referee node to the referrer that owns the code.
// A node can only be referred once; subsequent calls return an error.
func (m *ReferralManager) ApplyReferralCode(ctx context.Context, nodeID, code string) error {
	// Resolve the referrer node from the code.
	const resolveQ = `SELECT id FROM nodes WHERE referral_code = $1`
	var referrerID string
	if err := m.pool.QueryRow(ctx, resolveQ, code).Scan(&referrerID); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("referral: code %q not found", code)
		}
		return fmt.Errorf("referral: resolve code %q: %w", code, err)
	}

	if referrerID == nodeID {
		return fmt.Errorf("referral: node cannot refer itself")
	}

	// Link the referee to the referrer. referred_by is stored as the referral code.
	const updateQ = `
		UPDATE nodes
		SET referred_by = $2,
		    updated_at  = now()
		WHERE id = $1
		  AND (referred_by IS NULL OR referred_by = '')
	`
	tag, err := m.pool.Exec(ctx, updateQ, nodeID, code)
	if err != nil {
		return fmt.Errorf("referral: apply code for node %s: %w", nodeID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("referral: node %s already has a referral code applied", nodeID)
	}

	return nil
}

// GetReferralStats returns referral statistics for the given node.
func (m *ReferralManager) GetReferralStats(ctx context.Context, nodeID string) (*ReferralStats, error) {
	const q = `
		WITH referrer AS (
			SELECT referral_code FROM nodes WHERE id = $1
		),
		referees AS (
			SELECT COUNT(*) AS total_referrals
			FROM nodes n
			JOIN referrer r ON n.referred_by = r.referral_code
		),
		bonus AS (
			-- Sum 5% of all earnings of referred nodes.
			SELECT COALESCE(
				SUM(e.final_reward) * ($2::float8 / 100.0),
				0
			) AS bonus_total
			FROM earnings e
			JOIN nodes n ON n.id = e.node_id
			JOIN referrer r ON n.referred_by = r.referral_code
		)
		SELECT
			r.referral_code,
			re.total_referrals,
			b.bonus_total
		FROM referrer r
		CROSS JOIN referees re
		CROSS JOIN bonus b
	`

	var (
		referralCode   string
		totalReferrals int
		bonusTotal     float64
	)

	row := m.pool.QueryRow(ctx, q, nodeID, referralEarningsSharePct)
	if err := row.Scan(&referralCode, &totalReferrals, &bonusTotal); err != nil {
		return nil, fmt.Errorf("referral: get stats for node %s: %w", nodeID, err)
	}

	return &ReferralStats{
		NodeID:           nodeID,
		ReferralCode:     referralCode,
		TotalReferrals:   totalReferrals,
		BonusEarnedTotal: bonusTotal,
		EarningsSharePct: referralEarningsSharePct,
		GeneratedAt:      time.Now().UTC(),
	}, nil
}

// CalculateReferralBonus returns the referral bonus owed to the referrer
// for one epoch. Called by the reward calculator during reward settlement.
func CalculateReferralBonus(refereeFinalReward float64) float64 {
	return refereeFinalReward * (referralEarningsSharePct / 100.0)
}
