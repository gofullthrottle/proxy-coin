// Package config loads and validates backend service configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all runtime configuration for the Proxy Coin backend services.
type Config struct {
	// Database
	DatabaseURL string

	// Redis
	RedisURL string

	// Service listen addresses
	OrchestratorAddr string
	APIAddr          string
	MeteringAddr     string
	WSAddr           string

	// Auth
	JWTSecret          string
	JWTExpiry          time.Duration
	RefreshTokenExpiry time.Duration

	// Blockchain
	BaseRPCURL               string
	TokenAddress             string
	RewardDistributorAddress string
	StakingAddress           string
	VestingAddress           string

	// Proxy
	MaxConcurrentPerNode int
	RequestTimeoutMs     int
	MaxResponseBytes     int64

	// Metering / Reward
	PRXYPerMB          float64
	RewardCalcInterval time.Duration
	MerkleGenInterval  time.Duration

	// Node health
	HeartbeatIntervalMs int
	HeartbeatTimeoutMs  int
	MinTrustScore       float64
}

// Load reads configuration from environment variables, applying defaults where the variable
// is absent.  It returns an error if any required variable is missing or invalid.
func Load() (*Config, error) {
	cfg := &Config{
		// Database — required at runtime; default eases local development.
		DatabaseURL: getEnv("DATABASE_URL", "postgres://proxycoin:proxycoin_dev@localhost:5432/proxycoin?sslmode=disable"),

		// Redis — required at runtime; default eases local development.
		RedisURL: getEnv("REDIS_URL", "redis://localhost:6379"),

		// Service addresses
		OrchestratorAddr: getEnv("ORCHESTRATOR_ADDR", ":8080"),
		APIAddr:          getEnv("API_ADDR", ":8081"),
		MeteringAddr:     getEnv("METERING_ADDR", ":8082"),
		WSAddr:           getEnv("WS_ADDR", ":8080"),

		// Auth
		JWTSecret:          getEnv("JWT_SECRET", ""),
		JWTExpiry:          getDurationEnv("JWT_EXPIRY", 15*time.Minute),
		RefreshTokenExpiry: getDurationEnv("REFRESH_TOKEN_EXPIRY", 7*24*time.Hour),

		// Blockchain
		BaseRPCURL:               getEnv("BASE_RPC_URL", "https://sepolia.base.org"),
		TokenAddress:             getEnv("TOKEN_ADDRESS", ""),
		RewardDistributorAddress: getEnv("REWARD_DISTRIBUTOR_ADDRESS", ""),
		StakingAddress:           getEnv("STAKING_ADDRESS", ""),
		VestingAddress:           getEnv("VESTING_ADDRESS", ""),

		// Proxy limits
		MaxConcurrentPerNode: getIntEnv("MAX_CONCURRENT_PER_NODE", 5),
		RequestTimeoutMs:     getIntEnv("REQUEST_TIMEOUT_MS", 30000),
		MaxResponseBytes:     getInt64Env("MAX_RESPONSE_BYTES", 10*1024*1024), // 10 MiB

		// Metering / Reward parameters
		PRXYPerMB:          getFloatEnv("PRXY_PER_MB", 10.0),
		RewardCalcInterval: getDurationEnv("REWARD_CALC_INTERVAL", 1*time.Hour),
		MerkleGenInterval:  getDurationEnv("MERKLE_GEN_INTERVAL", 24*time.Hour),

		// Node heartbeat
		HeartbeatIntervalMs: getIntEnv("HEARTBEAT_INTERVAL_MS", 30000),
		HeartbeatTimeoutMs:  getIntEnv("HEARTBEAT_TIMEOUT_MS", 90000),
		MinTrustScore:       getFloatEnv("MIN_TRUST_SCORE", 0.3),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// validate checks that all required fields are populated and that numeric bounds make sense.
func (c *Config) validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("config: DATABASE_URL is required")
	}
	if c.RedisURL == "" {
		return fmt.Errorf("config: REDIS_URL is required")
	}
	if c.JWTExpiry <= 0 {
		return fmt.Errorf("config: JWT_EXPIRY must be a positive duration, got %v", c.JWTExpiry)
	}
	if c.RefreshTokenExpiry <= 0 {
		return fmt.Errorf("config: REFRESH_TOKEN_EXPIRY must be a positive duration, got %v", c.RefreshTokenExpiry)
	}
	if c.MaxConcurrentPerNode <= 0 {
		return fmt.Errorf("config: MAX_CONCURRENT_PER_NODE must be positive, got %d", c.MaxConcurrentPerNode)
	}
	if c.RequestTimeoutMs <= 0 {
		return fmt.Errorf("config: REQUEST_TIMEOUT_MS must be positive, got %d", c.RequestTimeoutMs)
	}
	if c.MaxResponseBytes <= 0 {
		return fmt.Errorf("config: MAX_RESPONSE_BYTES must be positive, got %d", c.MaxResponseBytes)
	}
	if c.PRXYPerMB < 0 {
		return fmt.Errorf("config: PRXY_PER_MB must be non-negative, got %f", c.PRXYPerMB)
	}
	if c.RewardCalcInterval <= 0 {
		return fmt.Errorf("config: REWARD_CALC_INTERVAL must be a positive duration, got %v", c.RewardCalcInterval)
	}
	if c.MerkleGenInterval <= 0 {
		return fmt.Errorf("config: MERKLE_GEN_INTERVAL must be a positive duration, got %v", c.MerkleGenInterval)
	}
	if c.HeartbeatIntervalMs <= 0 {
		return fmt.Errorf("config: HEARTBEAT_INTERVAL_MS must be positive, got %d", c.HeartbeatIntervalMs)
	}
	if c.HeartbeatTimeoutMs <= c.HeartbeatIntervalMs {
		return fmt.Errorf("config: HEARTBEAT_TIMEOUT_MS (%d) must be greater than HEARTBEAT_INTERVAL_MS (%d)",
			c.HeartbeatTimeoutMs, c.HeartbeatIntervalMs)
	}
	if c.MinTrustScore < 0 || c.MinTrustScore > 1 {
		return fmt.Errorf("config: MIN_TRUST_SCORE must be in [0, 1], got %f", c.MinTrustScore)
	}
	return nil
}

// RequestTimeout returns RequestTimeoutMs as a time.Duration.
func (c *Config) RequestTimeout() time.Duration {
	return time.Duration(c.RequestTimeoutMs) * time.Millisecond
}

// HeartbeatInterval returns HeartbeatIntervalMs as a time.Duration.
func (c *Config) HeartbeatInterval() time.Duration {
	return time.Duration(c.HeartbeatIntervalMs) * time.Millisecond
}

// HeartbeatTimeout returns HeartbeatTimeoutMs as a time.Duration.
func (c *Config) HeartbeatTimeout() time.Duration {
	return time.Duration(c.HeartbeatTimeoutMs) * time.Millisecond
}

// ---------------------------------------------------------------------------
// Private helpers
// ---------------------------------------------------------------------------

// getEnv returns the value of the named environment variable, or defaultVal if
// the variable is unset or empty.
func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// getIntEnv parses key as a base-10 integer, returning defaultVal on absence or
// parse failure.
func getIntEnv(key string, defaultVal int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return n
}

// getInt64Env parses key as a base-10 int64, returning defaultVal on absence or
// parse failure.
func getInt64Env(key string, defaultVal int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return defaultVal
	}
	return n
}

// getFloatEnv parses key as a float64, returning defaultVal on absence or parse
// failure.
func getFloatEnv(key string, defaultVal float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return defaultVal
	}
	return f
}

// getDurationEnv parses key using time.ParseDuration, returning defaultVal on
// absence or parse failure.  Values must use Go duration syntax, e.g. "15m",
// "24h", "90s".
func getDurationEnv(key string, defaultVal time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return defaultVal
	}
	return d
}
