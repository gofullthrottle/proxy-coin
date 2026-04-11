// Package node manages the node registry and lifecycle for Android proxy nodes.
package node

import "time"

// HealthCheck represents a single health probe result for a node.
type HealthCheck struct {
	NodeID    string        `json:"node_id"`
	Timestamp time.Time     `json:"timestamp"`
	Latency   time.Duration `json:"latency_ms"`
	Success   bool          `json:"success"`
	Error     string        `json:"error,omitempty"`
}
