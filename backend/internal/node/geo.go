// Package node manages the node registry and lifecycle for Android proxy nodes.
package node

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// GeoNodeCount represents the count of active nodes for a specific location.
type GeoNodeCount struct {
	Country string         `json:"country"`
	Total   int            `json:"total"`
	Regions map[string]int `json:"regions"`
}

// GeoDistribution maps country code → region → node count.
type GeoDistribution map[string]map[string]int

// GetNodeCountsByGeo returns the number of active nodes grouped by country and region.
// The outer map key is ISO 3166-1 alpha-2 country code; the inner map key is region
// name; the value is the count of active nodes.
func (r *Registry) GetNodeCountsByGeo(ctx context.Context) (GeoDistribution, error) {
	const q = `
		SELECT
			COALESCE(country, 'unknown') AS country,
			COALESCE(region,  'unknown') AS region,
			COUNT(*) AS cnt
		FROM nodes
		WHERE status = 'active'
		GROUP BY country, region
		ORDER BY country, region
	`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("node: geo distribution query: %w", err)
	}
	defer rows.Close()

	dist := make(GeoDistribution)
	for rows.Next() {
		var country, region string
		var cnt int
		if err := rows.Scan(&country, &region, &cnt); err != nil {
			return nil, fmt.Errorf("node: scan geo row: %w", err)
		}
		if _, ok := dist[country]; !ok {
			dist[country] = make(map[string]int)
		}
		dist[country][region] = cnt
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("node: iterate geo rows: %w", err)
	}

	return dist, nil
}

// StatusResponse is the payload for GET /v1/status.
type StatusResponse struct {
	TotalNodes   int                       `json:"total_nodes"`
	ActiveNodes  int                       `json:"active_nodes"`
	Countries    int                       `json:"countries"`
	Distribution map[string]GeoNodeCount   `json:"distribution"`
}

// GeoHandler exposes an HTTP handler for geographic status information.
type GeoHandler struct {
	registry *Registry
}

// NewGeoHandler creates a GeoHandler backed by the given registry.
func NewGeoHandler(registry *Registry) *GeoHandler {
	return &GeoHandler{registry: registry}
}

// HandleStatus handles GET /v1/status.
// Returns total/active node counts and geographic distribution.
func (h *GeoHandler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	dist, err := h.registry.GetNodeCountsByGeo(r.Context())
	if err != nil {
		http.Error(w, `{"error":"failed to fetch node distribution"}`, http.StatusInternalServerError)
		return
	}

	// Aggregate totals.
	totalActive := 0
	distribution := make(map[string]GeoNodeCount, len(dist))
	for country, regions := range dist {
		countryTotal := 0
		for _, cnt := range regions {
			countryTotal += cnt
			totalActive += cnt
		}
		distribution[country] = GeoNodeCount{
			Country: country,
			Total:   countryTotal,
			Regions: regions,
		}
	}

	resp := StatusResponse{
		TotalNodes:   totalActive,
		ActiveNodes:  totalActive,
		Countries:    len(dist),
		Distribution: distribution,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, `{"error":"encoding failed"}`, http.StatusInternalServerError)
	}
}
