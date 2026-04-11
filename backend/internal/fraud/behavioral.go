// Package fraud provides anti-fraud detection for the Proxy Coin network.
package fraud

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// bandwidthSpikeMultiplier is the factor above average at which bandwidth
	// is considered suspicious.
	bandwidthSpikeMultiplier = 10.0

	// newNodeGraceDays is the number of days a new node earns at reduced rate.
	newNodeGraceDays = 7

	// newNodeRampMultiplier is the reward multiplier applied during the grace period.
	newNodeRampMultiplier = 0.5

	// sleepCycleWindowHours is the window (hours) in which to expect at least
	// one idle period.  Nodes with no idle periods are likely bots.
	sleepCycleWindowHours = 24
)

// BehavioralAnalyzer detects anomalous traffic patterns that indicate
// automated or inflated usage rather than genuine residential proxying.
type BehavioralAnalyzer struct {
	// maxRequestsPerSecond is the threshold above which traffic is suspicious.
	maxRequestsPerSecond float64
	// minBytesPerRequest filters out nodes generating empty/synthetic requests.
	minBytesPerRequest int64
	// pool is used for database-backed anomaly detection in the async path.
	pool *pgxpool.Pool
}

// NewBehavioralAnalyzer creates an analyzer with the given thresholds.
func NewBehavioralAnalyzer(maxRPS float64, minBytesPerReq int64) *BehavioralAnalyzer {
	return &BehavioralAnalyzer{
		maxRequestsPerSecond: maxRPS,
		minBytesPerRequest:   minBytesPerReq,
	}
}

// NewBehavioralAnalyzerWithPool creates an analyzer with DB access for async checks.
func NewBehavioralAnalyzerWithPool(maxRPS float64, minBytesPerReq int64, pool *pgxpool.Pool) *BehavioralAnalyzer {
	return &BehavioralAnalyzer{
		maxRequestsPerSecond: maxRPS,
		minBytesPerRequest:   minBytesPerReq,
		pool:                 pool,
	}
}

// Analyze inspects a Signal for behavioral anomalies and returns a verdict.
// This is the synchronous real-time path.
func (b *BehavioralAnalyzer) Analyze(sig Signal) (Verdict, string) {
	if sig.RequestRate > b.maxRequestsPerSecond {
		return VerdictBlock, "behavioral: request rate exceeds threshold"
	}

	if sig.Requests() > 0 && sig.BytesTotal/sig.Requests() < b.minBytesPerRequest {
		return VerdictReview, "behavioral: suspiciously low bytes per request"
	}

	return VerdictAllow, ""
}

// DetectAnomalies is the async path that queries the database for behavioral
// anomalies for a specific node over the past 24-hour window.
// It emits FraudEvents for each anomaly detected.
func (b *BehavioralAnalyzer) DetectAnomalies(ctx context.Context, nodeID string) ([]FraudEvent, error) {
	if b.pool == nil {
		return nil, nil
	}

	var events []FraudEvent

	// Check 1: Bandwidth spike (>10x average over last 7 days vs last hour).
	spikeEvent, err := b.checkBandwidthSpike(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("behavioral: bandwidth spike check for %s: %w", nodeID, err)
	}
	if spikeEvent != nil {
		events = append(events, *spikeEvent)
	}

	// Check 2: No sleep cycles (no idle period in last 24 hours).
	sleepEvent, err := b.checkSleepCycles(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("behavioral: sleep cycle check for %s: %w", nodeID, err)
	}
	if sleepEvent != nil {
		events = append(events, *sleepEvent)
	}

	// Check 3: Same-subnet clustering (many nodes from the same /24 subnet).
	clusterEvent, err := b.checkIPClustering(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("behavioral: ip clustering check for %s: %w", nodeID, err)
	}
	if clusterEvent != nil {
		events = append(events, *clusterEvent)
	}

	return events, nil
}

// IsNewNode reports whether the node is within the grace ramp-up period.
// New nodes earn at newNodeRampMultiplier for the first newNodeGraceDays.
func (b *BehavioralAnalyzer) IsNewNode(ctx context.Context, nodeID string) (bool, error) {
	if b.pool == nil {
		return false, nil
	}
	const q = `SELECT registered_at FROM nodes WHERE id = $1`
	var registeredAt time.Time
	if err := b.pool.QueryRow(ctx, q, nodeID).Scan(&registeredAt); err != nil {
		return false, fmt.Errorf("behavioral: get registered_at for %s: %w", nodeID, err)
	}
	return time.Since(registeredAt) < time.Duration(newNodeGraceDays)*24*time.Hour, nil
}

// RampMultiplier returns the reward multiplier for the given node.
// New nodes within the grace period earn at 0.5x.
func (b *BehavioralAnalyzer) RampMultiplier(ctx context.Context, nodeID string) (float64, error) {
	isNew, err := b.IsNewNode(ctx, nodeID)
	if err != nil {
		return 1.0, err
	}
	if isNew {
		return newNodeRampMultiplier, nil
	}
	return 1.0, nil
}

// ---------------------------------------------------------------------------
// Internal anomaly checks
// ---------------------------------------------------------------------------

// checkBandwidthSpike compares the last-hour bandwidth to the 7-day rolling average.
// A spike of >10x is flagged as suspicious.
func (b *BehavioralAnalyzer) checkBandwidthSpike(ctx context.Context, nodeID string) (*FraudEvent, error) {
	const q = `
		SELECT
			COALESCE(AVG(bytes_proxied) FILTER (
				WHERE epoch >= NOW() - INTERVAL '7 days'
				  AND epoch <  NOW() - INTERVAL '1 hour'
			), 0) AS avg_7d,
			COALESCE(MAX(bytes_proxied) FILTER (
				WHERE epoch >= NOW() - INTERVAL '1 hour'
			), 0) AS last_hour
		FROM earnings
		WHERE node_id = $1
	`
	var avg7d, lastHour float64
	if err := b.pool.QueryRow(ctx, q, nodeID).Scan(&avg7d, &lastHour); err != nil {
		return nil, fmt.Errorf("behavioral: bandwidth spike query: %w", err)
	}

	if avg7d > 0 && lastHour/avg7d > bandwidthSpikeMultiplier {
		return &FraudEvent{
			NodeID:    nodeID,
			EventType: "bandwidth_spike",
			Severity:  "high",
			Detail:    fmt.Sprintf("last_hour=%.0f bytes is %.1fx above 7-day avg=%.0f", lastHour, lastHour/avg7d, avg7d),
			Action:    "review",
		}, nil
	}
	return nil, nil
}

// checkSleepCycles looks for any epoch in the last 24 hours where the node
// had zero traffic — a proxy for whether the device actually sleeps.
// Bots tend to run 24/7 with no idle periods.
func (b *BehavioralAnalyzer) checkSleepCycles(ctx context.Context, nodeID string) (*FraudEvent, error) {
	const q = `
		SELECT COUNT(*) AS active_hours
		FROM earnings
		WHERE node_id = $1
		  AND epoch >= NOW() - INTERVAL '24 hours'
		  AND bytes_proxied > 0
	`
	var activeHours int
	if err := b.pool.QueryRow(ctx, q, nodeID).Scan(&activeHours); err != nil {
		return nil, fmt.Errorf("behavioral: sleep cycle query: %w", err)
	}

	// If a node was active for 23+ of the last 24 hours, flag it.
	if activeHours >= 23 {
		return &FraudEvent{
			NodeID:    nodeID,
			EventType: "no_sleep_cycle",
			Severity:  "medium",
			Detail:    fmt.Sprintf("node active for %d/24 hours — possible bot", activeHours),
			Action:    "review",
		}, nil
	}
	return nil, nil
}

// checkIPClustering detects when too many nodes share the same /24 subnet,
// which indicates a coordinated farm rather than organic residential devices.
func (b *BehavioralAnalyzer) checkIPClustering(ctx context.Context, nodeID string) (*FraudEvent, error) {
	const q = `
		WITH node_subnet AS (
			SELECT
				id,
				-- Derive /24 subnet by zeroing the last octet of the IPv4 address.
				network(set_masklen(ip::inet, 24)) AS subnet
			FROM nodes
			WHERE id = $1
		),
		siblings AS (
			SELECT COUNT(*) AS cnt
			FROM nodes n
			JOIN node_subnet ns ON network(set_masklen(n.ip::inet, 24)) = ns.subnet
			WHERE n.status = 'active'
			  AND n.id <> $1
		)
		SELECT cnt FROM siblings
	`
	var siblingCount int
	if err := b.pool.QueryRow(ctx, q, nodeID).Scan(&siblingCount); err != nil {
		// Fail open: if we can't run the query (e.g. no inet column), skip this check.
		return nil, nil //nolint:nilerr
	}

	// More than 10 sibling nodes in the same /24 is suspicious for a
	// "residential" device.
	if siblingCount > 10 {
		return &FraudEvent{
			NodeID:    nodeID,
			EventType: "ip_cluster",
			Severity:  "medium",
			Detail:    fmt.Sprintf("%d other active nodes share the same /24 subnet", siblingCount),
			Action:    "review",
		}, nil
	}
	return nil, nil
}
