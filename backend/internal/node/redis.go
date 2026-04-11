// Package node manages the node registry and lifecycle for Android proxy nodes.
package node

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	sessionTTL = 5 * time.Minute
)

// RedisStore manages real-time node state in Redis.
//
// Key schema (matching BACKEND.md):
//
//	node:{nodeID}:status      → string status value with TTL
//	node:{nodeID}:requests    → integer active request counter
//	node:{nodeID}:heartbeat   → Unix timestamp of last heartbeat
//	session:{sessionID}       → nodeID with 5-minute TTL
//	ws:{nodeID}               → serverInstance identifier
type RedisStore struct {
	client *redis.Client
}

// NewRedisStore creates a RedisStore backed by the given Redis client.
func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{client: client}
}

// SetNodeStatus persists the status for a node with the given TTL.
func (s *RedisStore) SetNodeStatus(ctx context.Context, nodeID, status string, ttl time.Duration) error {
	key := fmt.Sprintf("node:%s:status", nodeID)
	if err := s.client.Set(ctx, key, status, ttl).Err(); err != nil {
		return fmt.Errorf("redis: set node status for %s: %w", nodeID, err)
	}
	return nil
}

// GetNodeStatus returns the current status string for the node.
// Returns redis.Nil if the key does not exist (node status expired or never set).
func (s *RedisStore) GetNodeStatus(ctx context.Context, nodeID string) (string, error) {
	key := fmt.Sprintf("node:%s:status", nodeID)
	val, err := s.client.Get(ctx, key).Result()
	if err != nil {
		return "", fmt.Errorf("redis: get node status for %s: %w", nodeID, err)
	}
	return val, nil
}

// IncrActiveRequests atomically increments the active request counter for a
// node and returns the new value.
func (s *RedisStore) IncrActiveRequests(ctx context.Context, nodeID string) (int64, error) {
	key := fmt.Sprintf("node:%s:requests", nodeID)
	val, err := s.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("redis: incr active requests for %s: %w", nodeID, err)
	}
	return val, nil
}

// DecrActiveRequests atomically decrements the active request counter for a
// node and returns the new value.  The counter is clamped to 0 via a Lua
// script to prevent underflow on restarts.
func (s *RedisStore) DecrActiveRequests(ctx context.Context, nodeID string) (int64, error) {
	key := fmt.Sprintf("node:%s:requests", nodeID)
	// Use a Lua script so the decrement never goes below zero.
	const luaScript = `
		local v = redis.call('GET', KEYS[1])
		if v == false or tonumber(v) <= 0 then
			redis.call('SET', KEYS[1], 0)
			return 0
		end
		return redis.call('DECR', KEYS[1])
	`
	val, err := s.client.Eval(ctx, luaScript, []string{key}).Int64()
	if err != nil {
		return 0, fmt.Errorf("redis: decr active requests for %s: %w", nodeID, err)
	}
	return val, nil
}

// GetActiveRequests returns the current active request count for a node.
// Returns 0 if the key does not exist.
func (s *RedisStore) GetActiveRequests(ctx context.Context, nodeID string) (int, error) {
	key := fmt.Sprintf("node:%s:requests", nodeID)
	val, err := s.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("redis: get active requests for %s: %w", nodeID, err)
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("redis: parse active requests for %s: %w", nodeID, err)
	}
	return n, nil
}

// SetHeartbeat records the current time as the node's last heartbeat.
// The key is stored without expiry so it survives across restarts; the
// orchestrator uses the timestamp value to detect stale nodes.
func (s *RedisStore) SetHeartbeat(ctx context.Context, nodeID string) error {
	key := fmt.Sprintf("node:%s:heartbeat", nodeID)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	if err := s.client.Set(ctx, key, ts, 0).Err(); err != nil {
		return fmt.Errorf("redis: set heartbeat for %s: %w", nodeID, err)
	}
	return nil
}

// GetHeartbeat returns the last heartbeat time for the node.
// Returns a zero time.Time and redis.Nil error if the key does not exist.
func (s *RedisStore) GetHeartbeat(ctx context.Context, nodeID string) (time.Time, error) {
	key := fmt.Sprintf("node:%s:heartbeat", nodeID)
	val, err := s.client.Get(ctx, key).Result()
	if err != nil {
		return time.Time{}, fmt.Errorf("redis: get heartbeat for %s: %w", nodeID, err)
	}
	unixSec, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("redis: parse heartbeat timestamp for %s: %w", nodeID, err)
	}
	return time.Unix(unixSec, 0), nil
}

// SetSessionBinding binds a proxy session to the node that is serving it.
// The binding expires after 5 minutes so stale sessions are automatically
// cleaned up without requiring an explicit delete.
func (s *RedisStore) SetSessionBinding(ctx context.Context, sessionID, nodeID string) error {
	key := fmt.Sprintf("session:%s", sessionID)
	if err := s.client.Set(ctx, key, nodeID, sessionTTL).Err(); err != nil {
		return fmt.Errorf("redis: set session binding %s→%s: %w", sessionID, nodeID, err)
	}
	return nil
}

// GetSessionBinding returns the nodeID that owns the given session.
func (s *RedisStore) GetSessionBinding(ctx context.Context, sessionID string) (string, error) {
	key := fmt.Sprintf("session:%s", sessionID)
	val, err := s.client.Get(ctx, key).Result()
	if err != nil {
		return "", fmt.Errorf("redis: get session binding for %s: %w", sessionID, err)
	}
	return val, nil
}

// SetWSMapping records which backend server instance holds the WebSocket
// connection for a given node.  This enables cross-instance message routing.
func (s *RedisStore) SetWSMapping(ctx context.Context, nodeID, serverInstance string) error {
	key := fmt.Sprintf("ws:%s", nodeID)
	if err := s.client.Set(ctx, key, serverInstance, 0).Err(); err != nil {
		return fmt.Errorf("redis: set ws mapping for node %s: %w", nodeID, err)
	}
	return nil
}

// GetWSMapping returns the server instance that owns the WebSocket connection
// for the given node.
func (s *RedisStore) GetWSMapping(ctx context.Context, nodeID string) (string, error) {
	key := fmt.Sprintf("ws:%s", nodeID)
	val, err := s.client.Get(ctx, key).Result()
	if err != nil {
		return "", fmt.Errorf("redis: get ws mapping for node %s: %w", nodeID, err)
	}
	return val, nil
}

// DeleteNodeKeys removes all Redis keys associated with a node.  This is
// called when a node is banned or permanently deregistered.
func (s *RedisStore) DeleteNodeKeys(ctx context.Context, nodeID string) error {
	keys := []string{
		fmt.Sprintf("node:%s:status", nodeID),
		fmt.Sprintf("node:%s:requests", nodeID),
		fmt.Sprintf("node:%s:heartbeat", nodeID),
		fmt.Sprintf("ws:%s", nodeID),
	}
	if err := s.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("redis: delete node keys for %s: %w", nodeID, err)
	}
	return nil
}
