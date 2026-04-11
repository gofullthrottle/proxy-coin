package config

import (
	"os"
	"testing"
	"time"
)

// setenv sets a collection of environment variables for the duration of the
// test and restores (or clears) them via t.Cleanup.
func setenv(t *testing.T, kvs map[string]string) {
	t.Helper()
	for k, v := range kvs {
		prev, existed := os.LookupEnv(k)
		if err := os.Setenv(k, v); err != nil {
			t.Fatalf("os.Setenv(%q): %v", k, err)
		}
		k, prev, existed := k, prev, existed // capture
		t.Cleanup(func() {
			if existed {
				os.Setenv(k, prev) //nolint:errcheck
			} else {
				os.Unsetenv(k) //nolint:errcheck
			}
		})
	}
}

// unsetenv ensures the given keys are absent for the duration of the test.
func unsetenv(t *testing.T, keys ...string) {
	t.Helper()
	for _, k := range keys {
		prev, existed := os.LookupEnv(k)
		os.Unsetenv(k) //nolint:errcheck
		k, prev, existed := k, prev, existed
		t.Cleanup(func() {
			if existed {
				os.Setenv(k, prev) //nolint:errcheck
			}
		})
	}
}

// TestLoad_Defaults verifies that Load() succeeds with no environment variables
// set beyond the bare minimum required for validation and returns expected
// default values for every field.
func TestLoad_Defaults(t *testing.T) {
	// Clear any ambient env that might bleed in from the test runner.
	unsetenv(t,
		"DATABASE_URL", "REDIS_URL",
		"ORCHESTRATOR_ADDR", "API_ADDR", "METERING_ADDR", "WS_ADDR",
		"JWT_SECRET", "JWT_EXPIRY", "REFRESH_TOKEN_EXPIRY",
		"BASE_RPC_URL", "TOKEN_ADDRESS", "REWARD_DISTRIBUTOR_ADDRESS",
		"STAKING_ADDRESS", "VESTING_ADDRESS",
		"MAX_CONCURRENT_PER_NODE", "REQUEST_TIMEOUT_MS", "MAX_RESPONSE_BYTES",
		"PRXY_PER_MB", "REWARD_CALC_INTERVAL", "MERKLE_GEN_INTERVAL",
		"HEARTBEAT_INTERVAL_MS", "HEARTBEAT_TIMEOUT_MS", "MIN_TRUST_SCORE",
	)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	tests := []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{"DatabaseURL", cfg.DatabaseURL, "postgres://proxycoin:proxycoin_dev@localhost:5432/proxycoin?sslmode=disable"},
		{"RedisURL", cfg.RedisURL, "redis://localhost:6379"},
		{"OrchestratorAddr", cfg.OrchestratorAddr, ":8080"},
		{"APIAddr", cfg.APIAddr, ":8081"},
		{"MeteringAddr", cfg.MeteringAddr, ":8082"},
		{"WSAddr", cfg.WSAddr, ":8080"},
		{"JWTSecret", cfg.JWTSecret, ""},
		{"JWTExpiry", cfg.JWTExpiry, 15 * time.Minute},
		{"RefreshTokenExpiry", cfg.RefreshTokenExpiry, 7 * 24 * time.Hour},
		{"BaseRPCURL", cfg.BaseRPCURL, "https://sepolia.base.org"},
		{"TokenAddress", cfg.TokenAddress, ""},
		{"RewardDistributorAddress", cfg.RewardDistributorAddress, ""},
		{"StakingAddress", cfg.StakingAddress, ""},
		{"VestingAddress", cfg.VestingAddress, ""},
		{"MaxConcurrentPerNode", cfg.MaxConcurrentPerNode, 5},
		{"RequestTimeoutMs", cfg.RequestTimeoutMs, 30000},
		{"MaxResponseBytes", cfg.MaxResponseBytes, int64(10 * 1024 * 1024)},
		{"PRXYPerMB", cfg.PRXYPerMB, 10.0},
		{"RewardCalcInterval", cfg.RewardCalcInterval, 1 * time.Hour},
		{"MerkleGenInterval", cfg.MerkleGenInterval, 24 * time.Hour},
		{"HeartbeatIntervalMs", cfg.HeartbeatIntervalMs, 30000},
		{"HeartbeatTimeoutMs", cfg.HeartbeatTimeoutMs, 90000},
		{"MinTrustScore", cfg.MinTrustScore, 0.3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %v, want %v", tt.got, tt.want)
			}
		})
	}
}

// TestLoad_EnvOverrides verifies that each environment variable is read and
// overrides the corresponding default.
func TestLoad_EnvOverrides(t *testing.T) {
	setenv(t, map[string]string{
		"DATABASE_URL":               "postgres://user:pass@db.example.com:5432/prod",
		"REDIS_URL":                  "redis://redis.example.com:6379/1",
		"ORCHESTRATOR_ADDR":          ":9090",
		"API_ADDR":                   ":9091",
		"METERING_ADDR":              ":9092",
		"WS_ADDR":                    ":9090",
		"JWT_SECRET":                 "super-secret-key",
		"JWT_EXPIRY":                 "30m",
		"REFRESH_TOKEN_EXPIRY":       "14d", // invalid — should fall back to default
		"BASE_RPC_URL":               "https://mainnet.base.org",
		"TOKEN_ADDRESS":              "0xTOKEN",
		"REWARD_DISTRIBUTOR_ADDRESS": "0xDISTRIBUTOR",
		"STAKING_ADDRESS":            "0xSTAKING",
		"VESTING_ADDRESS":            "0xVESTING",
		"MAX_CONCURRENT_PER_NODE":    "10",
		"REQUEST_TIMEOUT_MS":         "5000",
		"MAX_RESPONSE_BYTES":         "5242880", // 5 MiB
		"PRXY_PER_MB":                "25.5",
		"REWARD_CALC_INTERVAL":       "2h",
		"MERKLE_GEN_INTERVAL":        "48h",
		"HEARTBEAT_INTERVAL_MS":      "15000",
		"HEARTBEAT_TIMEOUT_MS":       "45000",
		"MIN_TRUST_SCORE":            "0.5",
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	if cfg.DatabaseURL != "postgres://user:pass@db.example.com:5432/prod" {
		t.Errorf("DatabaseURL: got %q", cfg.DatabaseURL)
	}
	if cfg.RedisURL != "redis://redis.example.com:6379/1" {
		t.Errorf("RedisURL: got %q", cfg.RedisURL)
	}
	if cfg.OrchestratorAddr != ":9090" {
		t.Errorf("OrchestratorAddr: got %q", cfg.OrchestratorAddr)
	}
	if cfg.APIAddr != ":9091" {
		t.Errorf("APIAddr: got %q", cfg.APIAddr)
	}
	if cfg.MeteringAddr != ":9092" {
		t.Errorf("MeteringAddr: got %q", cfg.MeteringAddr)
	}
	if cfg.JWTSecret != "super-secret-key" {
		t.Errorf("JWTSecret: got %q", cfg.JWTSecret)
	}
	if cfg.JWTExpiry != 30*time.Minute {
		t.Errorf("JWTExpiry: got %v, want 30m", cfg.JWTExpiry)
	}
	// "14d" is not valid Go duration syntax — getDurationEnv should fall back.
	if cfg.RefreshTokenExpiry != 7*24*time.Hour {
		t.Errorf("RefreshTokenExpiry fallback: got %v, want 168h", cfg.RefreshTokenExpiry)
	}
	if cfg.BaseRPCURL != "https://mainnet.base.org" {
		t.Errorf("BaseRPCURL: got %q", cfg.BaseRPCURL)
	}
	if cfg.TokenAddress != "0xTOKEN" {
		t.Errorf("TokenAddress: got %q", cfg.TokenAddress)
	}
	if cfg.RewardDistributorAddress != "0xDISTRIBUTOR" {
		t.Errorf("RewardDistributorAddress: got %q", cfg.RewardDistributorAddress)
	}
	if cfg.StakingAddress != "0xSTAKING" {
		t.Errorf("StakingAddress: got %q", cfg.StakingAddress)
	}
	if cfg.VestingAddress != "0xVESTING" {
		t.Errorf("VestingAddress: got %q", cfg.VestingAddress)
	}
	if cfg.MaxConcurrentPerNode != 10 {
		t.Errorf("MaxConcurrentPerNode: got %d", cfg.MaxConcurrentPerNode)
	}
	if cfg.RequestTimeoutMs != 5000 {
		t.Errorf("RequestTimeoutMs: got %d", cfg.RequestTimeoutMs)
	}
	if cfg.MaxResponseBytes != 5242880 {
		t.Errorf("MaxResponseBytes: got %d", cfg.MaxResponseBytes)
	}
	if cfg.PRXYPerMB != 25.5 {
		t.Errorf("PRXYPerMB: got %f", cfg.PRXYPerMB)
	}
	if cfg.RewardCalcInterval != 2*time.Hour {
		t.Errorf("RewardCalcInterval: got %v", cfg.RewardCalcInterval)
	}
	if cfg.MerkleGenInterval != 48*time.Hour {
		t.Errorf("MerkleGenInterval: got %v", cfg.MerkleGenInterval)
	}
	if cfg.HeartbeatIntervalMs != 15000 {
		t.Errorf("HeartbeatIntervalMs: got %d", cfg.HeartbeatIntervalMs)
	}
	if cfg.HeartbeatTimeoutMs != 45000 {
		t.Errorf("HeartbeatTimeoutMs: got %d", cfg.HeartbeatTimeoutMs)
	}
	if cfg.MinTrustScore != 0.5 {
		t.Errorf("MinTrustScore: got %f", cfg.MinTrustScore)
	}
}

// TestLoad_ValidationErrors verifies that invalid or missing required
// configuration values cause Load() to return descriptive errors.
func TestLoad_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		// DATABASE_URL="" causes getEnv to fall back to the development default,
		// so the way to force the validation error is to explicitly test validate()
		// with an empty value.  We cover that via the validate() unit test below.
		// Here we verify that a truly invalid (zero-length after trimming) custom
		// value still propagates: set to a non-empty env var that we override in
		// validate via a negative MaxConcurrentPerNode to keep the test table tidy.
		// The DATABASE_URL validation path is exercised by TestValidate_Empty below.
		{
			name:    "negative MaxConcurrentPerNode",
			env:     map[string]string{"MAX_CONCURRENT_PER_NODE": "-1"},
			wantErr: "MAX_CONCURRENT_PER_NODE must be positive",
		},
		{
			name:    "zero RequestTimeoutMs",
			env:     map[string]string{"REQUEST_TIMEOUT_MS": "0"},
			wantErr: "REQUEST_TIMEOUT_MS must be positive",
		},
		{
			name:    "negative MaxResponseBytes",
			env:     map[string]string{"MAX_RESPONSE_BYTES": "-100"},
			wantErr: "MAX_RESPONSE_BYTES must be positive",
		},
		{
			name:    "MinTrustScore above 1",
			env:     map[string]string{"MIN_TRUST_SCORE": "1.5"},
			wantErr: "MIN_TRUST_SCORE must be in [0, 1]",
		},
		{
			name:    "MinTrustScore below 0",
			env:     map[string]string{"MIN_TRUST_SCORE": "-0.1"},
			wantErr: "MIN_TRUST_SCORE must be in [0, 1]",
		},
		{
			name: "heartbeat timeout not greater than interval",
			env: map[string]string{
				"HEARTBEAT_INTERVAL_MS": "60000",
				"HEARTBEAT_TIMEOUT_MS":  "60000",
			},
			wantErr: "HEARTBEAT_TIMEOUT_MS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Start from a clean-ish environment with valid defaults, then
			// overlay the overrides that should trigger the error.
			unsetenv(t,
				"DATABASE_URL", "REDIS_URL",
				"MAX_CONCURRENT_PER_NODE", "REQUEST_TIMEOUT_MS", "MAX_RESPONSE_BYTES",
				"PRXY_PER_MB", "HEARTBEAT_INTERVAL_MS", "HEARTBEAT_TIMEOUT_MS",
				"MIN_TRUST_SCORE",
			)
			setenv(t, tt.env)

			_, err := Load()
			if err == nil {
				t.Fatal("Load() should have returned an error but did not")
			}
			// Check the error message contains the expected fragment.
			if msg := err.Error(); len(msg) == 0 {
				t.Error("error message is empty")
			} else {
				found := false
				for i := 0; i < len(msg)-len(tt.wantErr)+1; i++ {
					if msg[i:i+len(tt.wantErr)] == tt.wantErr {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("error %q does not contain %q", msg, tt.wantErr)
				}
			}
		})
	}
}

// TestLoad_DurationHelpers exercises the convenience Duration methods.
func TestLoad_DurationHelpers(t *testing.T) {
	unsetenv(t,
		"DATABASE_URL", "REDIS_URL",
		"REQUEST_TIMEOUT_MS", "HEARTBEAT_INTERVAL_MS", "HEARTBEAT_TIMEOUT_MS",
	)
	setenv(t, map[string]string{
		"REQUEST_TIMEOUT_MS":    "5000",
		"HEARTBEAT_INTERVAL_MS": "10000",
		"HEARTBEAT_TIMEOUT_MS":  "30000",
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	if got := cfg.RequestTimeout(); got != 5*time.Second {
		t.Errorf("RequestTimeout(): got %v, want 5s", got)
	}
	if got := cfg.HeartbeatInterval(); got != 10*time.Second {
		t.Errorf("HeartbeatInterval(): got %v, want 10s", got)
	}
	if got := cfg.HeartbeatTimeout(); got != 30*time.Second {
		t.Errorf("HeartbeatTimeout(): got %v, want 30s", got)
	}
}

// TestHelpers_Fallback tests the private helpers directly through their public
// effect on Load() — ensures they fall back gracefully on unparseable values.
func TestHelpers_Fallback(t *testing.T) {
	unsetenv(t,
		"DATABASE_URL", "REDIS_URL",
		"MAX_CONCURRENT_PER_NODE", "MAX_RESPONSE_BYTES", "PRXY_PER_MB",
		"REWARD_CALC_INTERVAL",
	)
	setenv(t, map[string]string{
		"MAX_CONCURRENT_PER_NODE": "notanint",
		"MAX_RESPONSE_BYTES":      "notanint64",
		"PRXY_PER_MB":             "notafloat",
		"REWARD_CALC_INTERVAL":    "notaduration",
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	if cfg.MaxConcurrentPerNode != 5 {
		t.Errorf("MaxConcurrentPerNode fallback: got %d, want 5", cfg.MaxConcurrentPerNode)
	}
	if cfg.MaxResponseBytes != 10*1024*1024 {
		t.Errorf("MaxResponseBytes fallback: got %d, want 10485760", cfg.MaxResponseBytes)
	}
	if cfg.PRXYPerMB != 10.0 {
		t.Errorf("PRXYPerMB fallback: got %f, want 10.0", cfg.PRXYPerMB)
	}
	if cfg.RewardCalcInterval != 1*time.Hour {
		t.Errorf("RewardCalcInterval fallback: got %v, want 1h", cfg.RewardCalcInterval)
	}
}

// TestValidate_Empty directly exercises the validate() checks for DatabaseURL
// and RedisURL.  Because getEnv falls back to a non-empty dev default when the
// env variable is set to "", we test validate() by constructing a Config with
// the fields zeroed out and calling validate() directly.
func TestValidate_Empty(t *testing.T) {
	t.Run("empty DatabaseURL", func(t *testing.T) {
		c := &Config{
			DatabaseURL:          "",
			RedisURL:             "redis://localhost:6379",
			JWTExpiry:            15 * time.Minute,
			RefreshTokenExpiry:   7 * 24 * time.Hour,
			MaxConcurrentPerNode: 5,
			RequestTimeoutMs:     1000,
			MaxResponseBytes:     1024,
			PRXYPerMB:            10,
			RewardCalcInterval:   time.Hour,
			MerkleGenInterval:    24 * time.Hour,
			HeartbeatIntervalMs:  30000,
			HeartbeatTimeoutMs:   90000,
			MinTrustScore:        0.3,
		}
		if err := c.validate(); err == nil {
			t.Fatal("validate() should error on empty DatabaseURL")
		} else if msg := err.Error(); len(msg) == 0 {
			t.Error("error message is empty")
		}
	})

	t.Run("empty RedisURL", func(t *testing.T) {
		c := &Config{
			DatabaseURL:          "postgres://localhost/proxycoin",
			RedisURL:             "",
			JWTExpiry:            15 * time.Minute,
			RefreshTokenExpiry:   7 * 24 * time.Hour,
			MaxConcurrentPerNode: 5,
			RequestTimeoutMs:     1000,
			MaxResponseBytes:     1024,
			PRXYPerMB:            10,
			RewardCalcInterval:   time.Hour,
			MerkleGenInterval:    24 * time.Hour,
			HeartbeatIntervalMs:  30000,
			HeartbeatTimeoutMs:   90000,
			MinTrustScore:        0.3,
		}
		if err := c.validate(); err == nil {
			t.Fatal("validate() should error on empty RedisURL")
		}
	})
}
