// Package node manages the node registry and lifecycle for Android proxy nodes.
package node

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Node represents a registered Android proxy node.
type Node struct {
	ID             string    `db:"id"`
	DeviceID       string    `db:"device_id"`
	WalletAddress  string    `db:"wallet_address"`
	IP             string    `db:"ip"`
	Country        string    `db:"country"`
	Region         string    `db:"region"`
	City           string    `db:"city"`
	ISP            string    `db:"isp"`
	ASN            int       `db:"asn"`
	IsResidential  bool      `db:"is_residential"`
	NetworkType    string    `db:"network_type"`
	TrustScore     float64   `db:"trust_score"`
	Status         string    `db:"status"`
	MaxConcurrent  int       `db:"max_concurrent"`
	StakedAmount   float64   `db:"staked_amount"`
	ReferralCode   string    `db:"referral_code"`
	ReferredBy     *string   `db:"referred_by"`
	RegisteredAt   time.Time `db:"registered_at"`
	LastHeartbeat  time.Time `db:"last_heartbeat"`
	ActiveRequests int       // in-memory only (populated from Redis)
}

// NodeFilter defines criteria for querying nodes from the registry.
type NodeFilter struct {
	Country     string
	Region      string
	Status      string
	MinTrust    float64
	NetworkType string
	Limit       int
}

// Registry manages node lifecycle in PostgreSQL.
type Registry struct {
	pool *pgxpool.Pool
}

// NewRegistry creates a Registry backed by the given connection pool.
func NewRegistry(pool *pgxpool.Pool) *Registry {
	return &Registry{pool: pool}
}

// Register inserts a new node or updates the wallet address and IP for an existing
// device.  It generates a unique referral code on first registration.
func (r *Registry) Register(ctx context.Context, deviceID, walletAddress, ip string) (*Node, error) {
	referralCode := strings.ReplaceAll(uuid.New().String(), "-", "")[:8]

	const q = `
		INSERT INTO nodes (device_id, wallet_address, ip, referral_code)
		VALUES ($1, $2, $3::inet, $4)
		ON CONFLICT (device_id) DO UPDATE
		    SET wallet_address = EXCLUDED.wallet_address,
		        ip             = EXCLUDED.ip,
		        updated_at     = now()
		RETURNING
		    id, device_id, wallet_address,
		    COALESCE(host(ip), '')          AS ip,
		    COALESCE(country,  '')          AS country,
		    COALESCE(region,   '')          AS region,
		    COALESCE(city,     '')          AS city,
		    COALESCE(isp,      '')          AS isp,
		    COALESCE(asn,      0)           AS asn,
		    is_residential, network_type,
		    CAST(trust_score AS float8)     AS trust_score,
		    status, max_concurrent,
		    CAST(staked_amount AS float8)   AS staked_amount,
		    COALESCE(referral_code, '')     AS referral_code,
		    CAST(referred_by AS text)       AS referred_by,
		    registered_at,
		    COALESCE(last_heartbeat, registered_at) AS last_heartbeat
	`

	row := r.pool.QueryRow(ctx, q, deviceID, walletAddress, ip, referralCode)
	return scanNode(row)
}

// GetByID retrieves a node by its UUID primary key.
func (r *Registry) GetByID(ctx context.Context, id string) (*Node, error) {
	const q = `
		SELECT
		    id, device_id, wallet_address,
		    COALESCE(host(ip), '')          AS ip,
		    COALESCE(country,  '')          AS country,
		    COALESCE(region,   '')          AS region,
		    COALESCE(city,     '')          AS city,
		    COALESCE(isp,      '')          AS isp,
		    COALESCE(asn,      0)           AS asn,
		    is_residential, network_type,
		    CAST(trust_score AS float8)     AS trust_score,
		    status, max_concurrent,
		    CAST(staked_amount AS float8)   AS staked_amount,
		    COALESCE(referral_code, '')     AS referral_code,
		    CAST(referred_by AS text)       AS referred_by,
		    registered_at,
		    COALESCE(last_heartbeat, registered_at) AS last_heartbeat
		FROM nodes
		WHERE id = $1
	`

	row := r.pool.QueryRow(ctx, q, id)
	return scanNode(row)
}

// GetByDeviceID retrieves a node by its unique device identifier.
func (r *Registry) GetByDeviceID(ctx context.Context, deviceID string) (*Node, error) {
	const q = `
		SELECT
		    id, device_id, wallet_address,
		    COALESCE(host(ip), '')          AS ip,
		    COALESCE(country,  '')          AS country,
		    COALESCE(region,   '')          AS region,
		    COALESCE(city,     '')          AS city,
		    COALESCE(isp,      '')          AS isp,
		    COALESCE(asn,      0)           AS asn,
		    is_residential, network_type,
		    CAST(trust_score AS float8)     AS trust_score,
		    status, max_concurrent,
		    CAST(staked_amount AS float8)   AS staked_amount,
		    COALESCE(referral_code, '')     AS referral_code,
		    CAST(referred_by AS text)       AS referred_by,
		    registered_at,
		    COALESCE(last_heartbeat, registered_at) AS last_heartbeat
		FROM nodes
		WHERE device_id = $1
	`

	row := r.pool.QueryRow(ctx, q, deviceID)
	return scanNode(row)
}

// UpdateHeartbeat records a node's latest heartbeat timestamp along with any
// observed change in IP address or network type.
func (r *Registry) UpdateHeartbeat(ctx context.Context, nodeID, ip, networkType string) error {
	const q = `
		UPDATE nodes
		SET last_heartbeat = now(),
		    ip             = $2::inet,
		    network_type   = $3,
		    updated_at     = now()
		WHERE id = $1
	`

	tag, err := r.pool.Exec(ctx, q, nodeID, ip, networkType)
	if err != nil {
		return fmt.Errorf("node: update heartbeat for %s: %w", nodeID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("node: no node with id %s", nodeID)
	}
	return nil
}

// UpdateStatus sets the operational status of a node (e.g. "active", "suspended").
func (r *Registry) UpdateStatus(ctx context.Context, nodeID, status string) error {
	const q = `
		UPDATE nodes
		SET status     = $2,
		    updated_at = now()
		WHERE id = $1
	`

	tag, err := r.pool.Exec(ctx, q, nodeID, status)
	if err != nil {
		return fmt.Errorf("node: update status for %s: %w", nodeID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("node: no node with id %s", nodeID)
	}
	return nil
}

// UpdateTrustScore sets the trust score for a node.  Score must be in [0, 1].
func (r *Registry) UpdateTrustScore(ctx context.Context, nodeID string, score float64) error {
	const q = `
		UPDATE nodes
		SET trust_score = $2,
		    updated_at  = now()
		WHERE id = $1
	`

	tag, err := r.pool.Exec(ctx, q, nodeID, score)
	if err != nil {
		return fmt.Errorf("node: update trust score for %s: %w", nodeID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("node: no node with id %s", nodeID)
	}
	return nil
}

// FindNodes returns nodes matching the supplied filter.  All filter fields are
// optional; omitting a field (zero value) disables that criterion.
func (r *Registry) FindNodes(ctx context.Context, filter NodeFilter) ([]Node, error) {
	args := []any{}
	conds := []string{}
	argIdx := 1

	if filter.Country != "" {
		conds = append(conds, fmt.Sprintf("country = $%d", argIdx))
		args = append(args, filter.Country)
		argIdx++
	}
	if filter.Region != "" {
		conds = append(conds, fmt.Sprintf("region = $%d", argIdx))
		args = append(args, filter.Region)
		argIdx++
	}
	if filter.Status != "" {
		conds = append(conds, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, filter.Status)
		argIdx++
	}
	if filter.MinTrust > 0 {
		conds = append(conds, fmt.Sprintf("CAST(trust_score AS float8) >= $%d", argIdx))
		args = append(args, filter.MinTrust)
		argIdx++
	}
	if filter.NetworkType != "" {
		conds = append(conds, fmt.Sprintf("network_type = $%d", argIdx))
		args = append(args, filter.NetworkType)
		argIdx++
	}

	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}

	limit := 100
	if filter.Limit > 0 {
		limit = filter.Limit
	}

	q := fmt.Sprintf(`
		SELECT
		    id, device_id, wallet_address,
		    COALESCE(host(ip), '')          AS ip,
		    COALESCE(country,  '')          AS country,
		    COALESCE(region,   '')          AS region,
		    COALESCE(city,     '')          AS city,
		    COALESCE(isp,      '')          AS isp,
		    COALESCE(asn,      0)           AS asn,
		    is_residential, network_type,
		    CAST(trust_score AS float8)     AS trust_score,
		    status, max_concurrent,
		    CAST(staked_amount AS float8)   AS staked_amount,
		    COALESCE(referral_code, '')     AS referral_code,
		    CAST(referred_by AS text)       AS referred_by,
		    registered_at,
		    COALESCE(last_heartbeat, registered_at) AS last_heartbeat
		FROM nodes
		%s
		ORDER BY trust_score DESC
		LIMIT %d
	`, where, limit)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("node: find nodes: %w", err)
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		n, err := scanNode(rows)
		if err != nil {
			return nil, fmt.Errorf("node: scan row: %w", err)
		}
		nodes = append(nodes, *n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("node: iterate rows: %w", err)
	}
	return nodes, nil
}

// CountByCountry returns the number of nodes grouped by country code.  It is
// used by the status endpoint to show geographic distribution.
func (r *Registry) CountByCountry(ctx context.Context) (map[string]int, error) {
	const q = `
		SELECT COALESCE(country, 'unknown') AS country, COUNT(*) AS cnt
		FROM nodes
		GROUP BY country
	`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("node: count by country: %w", err)
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var country string
		var cnt int
		if err := rows.Scan(&country, &cnt); err != nil {
			return nil, fmt.Errorf("node: scan country row: %w", err)
		}
		result[country] = cnt
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("node: iterate country rows: %w", err)
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// scanNode reads a single Node from any pgx Row/Rows source.
func scanNode(scanner interface {
	Scan(dest ...any) error
}) (*Node, error) {
	n := &Node{}
	var referredBy *string

	err := scanner.Scan(
		&n.ID,
		&n.DeviceID,
		&n.WalletAddress,
		&n.IP,
		&n.Country,
		&n.Region,
		&n.City,
		&n.ISP,
		&n.ASN,
		&n.IsResidential,
		&n.NetworkType,
		&n.TrustScore,
		&n.Status,
		&n.MaxConcurrent,
		&n.StakedAmount,
		&n.ReferralCode,
		&referredBy,
		&n.RegisteredAt,
		&n.LastHeartbeat,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("node: not found")
		}
		return nil, fmt.Errorf("node: scan: %w", err)
	}

	n.ReferredBy = referredBy
	return n, nil
}
