// Package auth handles authentication for node operators and API customers.
package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// planLimits maps a plan name to its daily request allowance.
// Keys must match the plan names stored in the customers table.
var planLimits = map[string]int{
	"free":       100,
	"starter":    10_000,
	"pro":        100_000,
	"enterprise": 1_000_000,
}

// RateLimiter enforces per-customer daily request quotas using Redis.
// Keys use the pattern:  ratelimit:{apiKey}:{YYYY-MM-DD}
// Each key has a 24-hour TTL so cleanup is automatic.
type RateLimiter struct {
	client *redis.Client
}

// NewRateLimiter creates a RateLimiter backed by the given Redis client.
func NewRateLimiter(client *redis.Client) *RateLimiter {
	return &RateLimiter{client: client}
}

// CheckResult holds the outcome of a rate limit check.
type CheckResult struct {
	Allowed   bool
	Remaining int
	Limit     int
	ResetAt   time.Time // midnight UTC of the next day
}

// Check determines whether the customer may make another request today.
//
//   - apiKey: the customer's raw API key (used as part of the Redis key)
//   - plan:   the customer's plan name (free/starter/pro/enterprise)
//
// It atomically increments the daily counter and returns whether the new
// value is within the plan's limit.
func (l *RateLimiter) Check(ctx context.Context, apiKey, plan string) (allowed bool, remaining int, err error) {
	result, err := l.CheckFull(ctx, apiKey, plan)
	if err != nil {
		return false, 0, err
	}
	return result.Allowed, result.Remaining, nil
}

// CheckFull is like Check but returns the full CheckResult including the limit
// and reset time, suitable for populating X-RateLimit-* response headers.
func (l *RateLimiter) CheckFull(ctx context.Context, apiKey, plan string) (*CheckResult, error) {
	limit, ok := planLimits[plan]
	if !ok {
		limit = planLimits["free"]
	}

	now := time.Now().UTC()
	dateStr := now.Format("2006-01-02")
	key := fmt.Sprintf("ratelimit:%s:%s", apiKey, dateStr)

	// Increment the counter atomically.
	count, err := l.client.Incr(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("ratelimit: incr key %s: %w", key, err)
	}

	// Set the 24-hour TTL on first access (EXPIRE is a no-op if already set
	// and the Redis INCR call already did the increment).
	if count == 1 {
		// Key was just created; set the TTL.
		if err := l.client.Expire(ctx, key, 24*time.Hour).Err(); err != nil {
			// Non-fatal: the counter will still work; worst case the key persists.
			_ = err
		}
	}

	// Compute reset time: midnight UTC tomorrow.
	resetAt := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)

	intCount := int(count)
	remaining := limit - intCount
	if remaining < 0 {
		remaining = 0
	}

	return &CheckResult{
		Allowed:   intCount <= limit,
		Remaining: remaining,
		Limit:     limit,
		ResetAt:   resetAt,
	}, nil
}

// Reset zeroes the counter for a given API key and date (today by default).
// Used by admin tooling and tests.
func (l *RateLimiter) Reset(ctx context.Context, apiKey string) error {
	now := time.Now().UTC()
	key := fmt.Sprintf("ratelimit:%s:%s", apiKey, now.Format("2006-01-02"))
	if err := l.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("ratelimit: reset key %s: %w", key, err)
	}
	return nil
}
