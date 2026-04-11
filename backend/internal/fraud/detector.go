// Package fraud provides anti-fraud detection for the Proxy Coin network.
package fraud

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Verdict is the outcome of a fraud evaluation.
type Verdict string

const (
	VerdictAllow  Verdict = "allow"
	VerdictReview Verdict = "review"
	VerdictBlock  Verdict = "block"
)

// FraudEvent records a detected fraud incident for a node.
type FraudEvent struct {
	ID         int64     `json:"id"`
	NodeID     string    `json:"node_id"`
	EventType  string    `json:"event_type"`  // e.g. "ip_datacenter", "bandwidth_spike"
	Severity   string    `json:"severity"`    // low | medium | high | critical
	Detail     string    `json:"detail"`
	DetectedAt time.Time `json:"detected_at"`
	Action     string    `json:"action"`      // none | review | suspend | ban
}

// Signal aggregates all fraud signals for a single node event.
type Signal struct {
	NodeID      string
	IPAddress   string
	BytesTotal  int64
	RequestRate float64 // requests per second
	IsMobile    bool
}

// Requests is a helper that returns total request count from the signal.
// Signal.Requests is not stored directly; populated by metering in a later wave.
func (s Signal) Requests() int64 {
	return 0
}

// Detector coordinates multiple fraud checks and returns a final verdict.
type Detector struct {
	ipIntel     *IPIntelligence
	behavioral  *BehavioralAnalyzer
	attestation *AttestationVerifier
	pool        *pgxpool.Pool
}

// NewDetector creates a fraud Detector wiring together all sub-components.
func NewDetector(
	pool *pgxpool.Pool,
	ipIntel *IPIntelligence,
	behavioral *BehavioralAnalyzer,
	attestation *AttestationVerifier,
) *Detector {
	return &Detector{
		pool:        pool,
		ipIntel:     ipIntel,
		behavioral:  behavioral,
		attestation: attestation,
	}
}

// Evaluate runs all fraud checks against the given signal and returns a
// consolidated verdict along with a human-readable reason.
// This is the synchronous path used for real-time request gating.
func (d *Detector) Evaluate(sig Signal) (Verdict, string) {
	if d.ipIntel != nil {
		if v, reason := d.ipIntel.Check(sig.IPAddress); v == VerdictBlock {
			return VerdictBlock, reason
		}
	}

	if d.behavioral != nil {
		if v, reason := d.behavioral.Analyze(sig); v == VerdictBlock {
			return VerdictBlock, reason
		}
	}

	if d.attestation != nil {
		if v, reason := d.attestation.Verify(sig.NodeID); v != VerdictAllow {
			return v, reason
		}
	}

	return VerdictAllow, ""
}

// Analyze runs all fraud checks for a node and returns the detected FraudEvents.
// This is the async analysis path used by the background job.
func (d *Detector) Analyze(ctx context.Context, nodeID string) ([]FraudEvent, error) {
	var events []FraudEvent

	// IP intelligence check.
	if d.ipIntel != nil {
		ipEvents, err := d.ipIntel.AnalyzeNode(ctx, nodeID)
		if err != nil {
			log.Printf("fraud: ip analysis for node %s: %v", nodeID, err)
		} else {
			events = append(events, ipEvents...)
		}
	}

	// Behavioral check.
	if d.behavioral != nil {
		behavioralEvents, err := d.behavioral.DetectAnomalies(ctx, nodeID)
		if err != nil {
			log.Printf("fraud: behavioral analysis for node %s: %v", nodeID, err)
		} else {
			events = append(events, behavioralEvents...)
		}
	}

	return events, nil
}

// LogEvent inserts a fraud event into the fraud_events table.
func (d *Detector) LogEvent(ctx context.Context, event FraudEvent) error {
	if d.pool == nil {
		return nil // no DB configured (e.g. in tests)
	}
	const q = `
		INSERT INTO fraud_events (node_id, event_type, severity, detail, detected_at, action)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`
	now := time.Now().UTC()
	if event.DetectedAt.IsZero() {
		event.DetectedAt = now
	}
	if err := d.pool.QueryRow(ctx, q,
		event.NodeID,
		event.EventType,
		event.Severity,
		event.Detail,
		event.DetectedAt,
		event.Action,
	).Scan(&event.ID); err != nil {
		return fmt.Errorf("fraud: log event for node %s: %w", event.NodeID, err)
	}
	return nil
}

// AutoAction applies consequences based on event severity.
// It updates the node's status in the database to reflect the action taken.
func (d *Detector) AutoAction(ctx context.Context, event FraudEvent) error {
	if d.pool == nil {
		return nil
	}

	var status string
	switch event.Severity {
	case "critical":
		status = "banned"
	case "high":
		status = "suspended"
	case "medium":
		status = "review"
	default:
		// Low severity — no automatic status change.
		return nil
	}

	const q = `
		UPDATE nodes
		SET status     = $2,
		    updated_at = now()
		WHERE id = $1
	`
	if _, err := d.pool.Exec(ctx, q, event.NodeID, status); err != nil {
		return fmt.Errorf("fraud: auto action for node %s (set %s): %w", event.NodeID, status, err)
	}
	log.Printf("fraud: auto-action node %s → status=%s (event_type=%s)", event.NodeID, status, event.EventType)
	return nil
}
