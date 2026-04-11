// Package node manages the node registry and lifecycle for Android proxy nodes.
package node

import (
	"context"
	"errors"
	"math/rand"
	"sort"
	"time"
)

// ErrNoAvailableNodes is returned when no eligible node can be selected.
var ErrNoAvailableNodes = errors.New("no available nodes matching criteria")

// ScoredNode pairs a Node with its computed selection score.
type ScoredNode struct {
	Node  Node
	Score float64
}

// Selector chooses an appropriate proxy node for an incoming request using a
// weighted-random strategy that favours high-trust, low-load, residential nodes.
type Selector struct {
	registry *Registry
}

// NewSelector creates a Selector backed by the given Registry.
func NewSelector(registry *Registry) *Selector {
	return &Selector{registry: registry}
}

// SelectNode returns the best available node matching filter.
// It scores every candidate, sorts descending, then performs a weighted random
// pick over the top 5 to avoid thundering-herd on the single highest scorer.
func (s *Selector) SelectNode(ctx context.Context, filter NodeFilter) (*Node, error) {
	candidates, err := s.registry.FindNodes(ctx, filter)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, ErrNoAvailableNodes
	}

	scored := make([]ScoredNode, len(candidates))
	for i, n := range candidates {
		scored[i] = ScoredNode{Node: n, Score: calculateScore(n)}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	// Limit the weighted random pool to the top 5 candidates.
	top := scored
	if len(top) > 5 {
		top = top[:5]
	}

	return weightedRandom(top), nil
}

// calculateScore computes a composite score in [0, 1] for a node candidate.
//
// Component weights:
//
//	Trust score                                     0.4
//	Load factor  (1 - activeRequests/maxConcurrent) 0.3
//	Residential bonus                               0.2
//	Uptime bonus  (hours since registration, ≤1.0)  0.1
func calculateScore(n Node) float64 {
	// Trust component (already in [0,1]).
	trust := n.TrustScore * 0.4

	// Load component: lower utilisation → higher score.
	loadFactor := 1.0
	if n.MaxConcurrent > 0 {
		utilisation := float64(n.ActiveRequests) / float64(n.MaxConcurrent)
		if utilisation > 1.0 {
			utilisation = 1.0
		}
		loadFactor = 1.0 - utilisation
	}
	load := loadFactor * 0.3

	// Residential bonus.
	residential := 0.3
	if n.IsResidential {
		residential = 1.0
	}
	res := residential * 0.2

	// Uptime bonus: hours since registration, capped at 720 h (30 days) → 1.0.
	const capHours = 720.0
	hours := time.Since(n.RegisteredAt).Hours()
	if hours < 0 {
		hours = 0
	}
	uptime := hours / capHours
	if uptime > 1.0 {
		uptime = 1.0
	}
	up := uptime * 0.1

	return trust + load + res + up
}

// weightedRandom selects one node from scored using the scores as relative
// weights.  All scores must be non-negative; a zero-sum pool falls back to
// uniform random selection.
func weightedRandom(nodes []ScoredNode) *Node {
	total := 0.0
	for _, sn := range nodes {
		total += sn.Score
	}

	if total <= 0 {
		// Fall back to uniform random if all scores are zero.
		idx := rand.Intn(len(nodes))
		n := nodes[idx].Node
		return &n
	}

	r := rand.Float64() * total
	cumulative := 0.0
	for i := range nodes {
		cumulative += nodes[i].Score
		if r <= cumulative {
			n := nodes[i].Node
			return &n
		}
	}

	// Should not reach here; return the last node as a safety net.
	n := nodes[len(nodes)-1].Node
	return &n
}
