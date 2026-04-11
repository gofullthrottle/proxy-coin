// Package metering tracks bandwidth and compute usage per proxy node.
package metering

import (
	"context"
	"log"
	"sync"
	"time"
)

// Aggregator collects UsageSnapshots from all active nodes and periodically
// flushes them to the Reporter for downstream reward calculation.
//
// Full flush-to-stream logic is implemented in Wave 6; this file provides the
// structure and placeholder stubs needed by the rest of the system.
type Aggregator struct {
	mu       sync.Mutex
	counters map[string]*UsageCounter
	period   time.Duration

	reporter *Reporter // set via WithReporter; may be nil before Wave 6
}

// NewAggregator creates an Aggregator with the given flush period.
func NewAggregator(period time.Duration) *Aggregator {
	return &Aggregator{
		counters: make(map[string]*UsageCounter),
		period:   period,
	}
}

// WithReporter sets the Reporter that the Aggregator will use when flushing
// snapshots to Redis Streams. This wires Task 3.6 into the aggregation loop.
func (a *Aggregator) WithReporter(r *Reporter) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.reporter = r
}

// Track returns (or lazily creates) the UsageCounter for nodeID.
func (a *Aggregator) Track(nodeID string) *UsageCounter {
	a.mu.Lock()
	defer a.mu.Unlock()
	if c, ok := a.counters[nodeID]; ok {
		return c
	}
	c := NewUsageCounter(nodeID)
	a.counters[nodeID] = c
	return c
}

// Flush snapshots all counters, resets them, and returns the snapshots.
// If a Reporter is configured it also publishes the snapshots as metering
// events (full implementation in Wave 6).
func (a *Aggregator) Flush() []UsageSnapshot {
	a.mu.Lock()
	defer a.mu.Unlock()

	snapshots := make([]UsageSnapshot, 0, len(a.counters))
	for nodeID, c := range a.counters {
		snapshots = append(snapshots, c.Snapshot())
		a.counters[nodeID] = NewUsageCounter(nodeID)
	}
	return snapshots
}

// FlushToStream flushes all counters and publishes each snapshot as a
// metering Event to Redis Streams via the configured Reporter.
// This is a placeholder for Wave 6 full implementation.
func (a *Aggregator) FlushToStream(ctx context.Context) error {
	snapshots := a.Flush()

	if a.reporter == nil || len(snapshots) == 0 {
		return nil
	}

	events := make([]Event, 0, len(snapshots))
	for _, s := range snapshots {
		events = append(events, Event{
			NodeID:    s.NodeID,
			BytesIn:   s.BytesIn,
			BytesOut:  s.BytesOut,
			Timestamp: s.PeriodEnd,
		})
	}
	return a.reporter.PublishBatch(ctx, events)
}

// Run starts the periodic flush loop. It blocks until ctx is cancelled.
// This is a placeholder for the Wave 6 scheduler integration.
func (a *Aggregator) Run(ctx context.Context) {
	ticker := time.NewTicker(a.period)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := a.FlushToStream(ctx); err != nil {
				log.Printf("metering: aggregator flush error: %v", err)
			}
		case <-ctx.Done():
			return
		}
	}
}
