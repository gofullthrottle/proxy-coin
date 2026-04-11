// Package metering tracks bandwidth and compute usage per proxy node.
package metering

import (
	"sync/atomic"
	"time"
)

// UsageCounter accumulates byte counts for a single node within a period.
type UsageCounter struct {
	NodeID      string
	periodStart time.Time
	bytesIn     atomic.Int64
	bytesOut    atomic.Int64
	requests    atomic.Int64
}

// NewUsageCounter creates a counter for the given node starting now.
func NewUsageCounter(nodeID string) *UsageCounter {
	return &UsageCounter{
		NodeID:      nodeID,
		periodStart: time.Now(),
	}
}

// AddBytes records transferred bytes (in: bytes received, out: bytes sent).
func (c *UsageCounter) AddBytes(in, out int64) {
	c.bytesIn.Add(in)
	c.bytesOut.Add(out)
	c.requests.Add(1)
}

// Snapshot returns the current accumulated totals without resetting.
func (c *UsageCounter) Snapshot() UsageSnapshot {
	return UsageSnapshot{
		NodeID:      c.NodeID,
		PeriodStart: c.periodStart,
		PeriodEnd:   time.Now(),
		BytesIn:     c.bytesIn.Load(),
		BytesOut:    c.bytesOut.Load(),
		Requests:    c.requests.Load(),
	}
}

// UsageSnapshot is an immutable point-in-time view of usage for a node.
type UsageSnapshot struct {
	NodeID      string    `json:"node_id"`
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
	BytesIn     int64     `json:"bytes_in"`
	BytesOut    int64     `json:"bytes_out"`
	Requests    int64     `json:"requests"`
}

// TotalBytes returns the sum of inbound and outbound bytes.
func (s UsageSnapshot) TotalBytes() int64 {
	return s.BytesIn + s.BytesOut
}

// ---------------------------------------------------------------------------
// ByteCounter — authoritative server-side byte accounting (Task 6.6)
// ---------------------------------------------------------------------------

// ByteCounter tracks raw byte throughput for a single WebSocket connection
// using lock-free atomic operations. It is distinct from UsageCounter in
// that it does not track per-period windows; it accumulates monotonically
// and is read/reset by the Aggregator.
type ByteCounter struct {
	nodeID   string
	bytesIn  atomic.Int64
	bytesOut atomic.Int64
	requests atomic.Int64
}

// NewByteCounter creates a ByteCounter for the given node.
func NewByteCounter(nodeID string) *ByteCounter {
	return &ByteCounter{nodeID: nodeID}
}

// NodeID returns the node identifier this counter tracks.
func (b *ByteCounter) NodeID() string { return b.nodeID }

// Add records bytesIn inbound bytes and bytesOut outbound bytes for one
// proxied payload. The request counter is incremented once per call,
// providing an authoritative server-side view independent of node reports.
func (b *ByteCounter) Add(bytesIn, bytesOut int64) {
	b.bytesIn.Add(bytesIn)
	b.bytesOut.Add(bytesOut)
	b.requests.Add(1)
}

// BytesIn returns the total inbound bytes recorded since construction or the
// last Reset.
func (b *ByteCounter) BytesIn() int64 { return b.bytesIn.Load() }

// BytesOut returns the total outbound bytes recorded since construction or the
// last Reset.
func (b *ByteCounter) BytesOut() int64 { return b.bytesOut.Load() }

// Requests returns the total number of requests recorded.
func (b *ByteCounter) Requests() int64 { return b.requests.Load() }

// GetStats returns the current (in, out, requests) triple without resetting.
// This is the non-destructive read used for health monitoring.
func (b *ByteCounter) GetStats() (in, out, requests int64) {
	return b.bytesIn.Load(), b.bytesOut.Load(), b.requests.Load()
}

// Reset atomically zeroes all counters and returns the previous values as a
// (in, out, requests) triple. The caller is responsible for persisting the
// values before discarding this ByteCounter.
func (b *ByteCounter) Reset() (in, out, requests int64) {
	in = b.bytesIn.Swap(0)
	out = b.bytesOut.Swap(0)
	requests = b.requests.Swap(0)
	return
}

// ResetSnapshot is a convenience wrapper around Reset that returns a
// UsageSnapshot for consumers that expect that type.
func (b *ByteCounter) ResetSnapshot() UsageSnapshot {
	in, out, reqs := b.Reset()
	return UsageSnapshot{
		NodeID:   b.nodeID,
		BytesIn:  in,
		BytesOut: out,
		Requests: reqs,
	}
}

// ---------------------------------------------------------------------------
// ConnectionMeter — per-WebSocket-connection byte accounting (Task 6.6)
// ---------------------------------------------------------------------------

// ConnectionMeter wraps a ByteCounter and provides the same Add/GetStats/Reset
// interface at the connection level. It is embedded into the proxy path so
// every byte transiting a specific WebSocket connection is counted server-side,
// providing an authoritative measurement that cannot be spoofed by the node.
//
// Usage:
//
//	meter := metering.NewConnectionMeter(nodeID)
//	// On each relayed payload:
//	meter.RecordPayload(len(requestBody), len(responseBody))
//	// Periodically:
//	in, out, reqs := meter.Reset()
type ConnectionMeter struct {
	counter *ByteCounter
}

// NewConnectionMeter creates a ConnectionMeter for the given nodeID.
func NewConnectionMeter(nodeID string) *ConnectionMeter {
	return &ConnectionMeter{counter: NewByteCounter(nodeID)}
}

// NodeID returns the node identifier this meter tracks.
func (m *ConnectionMeter) NodeID() string { return m.counter.nodeID }

// RecordPayload records a single proxied exchange. bytesIn is the size of the
// request body (customer → target), bytesOut is the size of the response body
// (target → customer) as measured on the backend side.
func (m *ConnectionMeter) RecordPayload(bytesIn, bytesOut int64) {
	m.counter.Add(bytesIn, bytesOut)
}

// GetStats returns the current (in, out, requests) triple without resetting.
func (m *ConnectionMeter) GetStats() (in, out, requests int64) {
	return m.counter.GetStats()
}

// Reset atomically zeroes all counters and returns the previous (in, out, requests).
func (m *ConnectionMeter) Reset() (in, out, requests int64) {
	return m.counter.Reset()
}

// Snapshot returns a non-resetting UsageSnapshot for health monitoring.
func (m *ConnectionMeter) Snapshot() UsageSnapshot {
	in, out, reqs := m.counter.GetStats()
	return UsageSnapshot{
		NodeID:    m.counter.nodeID,
		PeriodEnd: time.Now(),
		BytesIn:   in,
		BytesOut:  out,
		Requests:  reqs,
	}
}
