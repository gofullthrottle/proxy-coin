// Package auth handles authentication for node operators and API customers.
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// ErrAPIKeyNotFound is returned when a key lookup yields no result.
var ErrAPIKeyNotFound = errors.New("auth: API key not found")

// apiKeyPrefix is the human-readable prefix prepended to all issued keys.
// Customers can visually identify live production keys.
const apiKeyPrefix = "prxy_live_"

// bcryptCost is the bcrypt work factor.  Cost 12 is a reasonable balance
// between security and CPU cost (~250 ms on modern hardware).
const bcryptCost = 12

// APIKey represents a long-lived credential issued to a customer.
type APIKey struct {
	ID         string    `json:"id"`
	CustomerID string    `json:"customer_id"`
	Key        string    `json:"key"` // stored as bcrypt hash in DB; plaintext only returned on creation
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at,omitempty"`
	Revoked    bool      `json:"revoked"`
}

// APIKeyManager issues and validates API keys for customer access.
// In production the hash is stored in the database; the in-memory map is
// used for development and testing.
type APIKeyManager struct {
	// keys maps the 8-character ID prefix to the APIKey record.
	keys map[string]*APIKey
}

// NewAPIKeyManager creates an APIKeyManager with an empty in-memory store.
func NewAPIKeyManager() *APIKeyManager {
	return &APIKeyManager{keys: make(map[string]*APIKey)}
}

// GenerateAPIKey generates a new API key in the format:
//
//	prxy_live_<32 random hex chars>
//
// Returns (plainKey, bcryptHash, error). The plainKey is shown to the
// customer exactly once; only the hash should be persisted.
func (m *APIKeyManager) GenerateAPIKey() (plainKey, hash string, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("auth: generate api key random bytes: %w", err)
	}

	plainKey = apiKeyPrefix + hex.EncodeToString(raw)

	hash, err = m.HashKey(plainKey)
	if err != nil {
		return "", "", err
	}

	return plainKey, hash, nil
}

// HashKey returns the bcrypt hash of the given API key.
func (m *APIKeyManager) HashKey(key string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(key), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("auth: hash api key: %w", err)
	}
	return string(h), nil
}

// VerifyKey reports whether the given plaintext key matches the stored hash.
func (m *APIKeyManager) VerifyKey(key, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(key)) == nil
}

// Issue generates and stores a new API key for the customer.
// It returns the APIKey record (with plaintext key for one-time display)
// and the raw key string.
func (m *APIKeyManager) Issue(customerID string, ttl time.Duration) (*APIKey, string, error) {
	plain, hash, err := m.GenerateAPIKey()
	if err != nil {
		return nil, "", err
	}

	// Use first 8 hex chars after prefix as the stable ID.
	id := plain[len(apiKeyPrefix) : len(apiKeyPrefix)+8]

	apiKey := &APIKey{
		ID:         id,
		CustomerID: customerID,
		Key:        hash, // only the hash is persisted
		CreatedAt:  time.Now(),
	}
	if ttl > 0 {
		apiKey.ExpiresAt = time.Now().Add(ttl)
	}

	m.keys[id] = apiKey
	return apiKey, plain, nil
}

// Validate checks that a raw key prefix maps to a non-revoked, non-expired record.
// In production this is backed by a database lookup; the in-memory store is
// used for development.
func (m *APIKeyManager) Validate(rawKey string) (*APIKey, error) {
	if len(rawKey) <= len(apiKeyPrefix)+8 {
		return nil, ErrAPIKeyNotFound
	}
	id := rawKey[len(apiKeyPrefix) : len(apiKeyPrefix)+8]

	apiKey, ok := m.keys[id]
	if !ok {
		return nil, ErrAPIKeyNotFound
	}
	if apiKey.Revoked {
		return nil, errors.New("auth: API key has been revoked")
	}
	if !apiKey.ExpiresAt.IsZero() && time.Now().After(apiKey.ExpiresAt) {
		return nil, errors.New("auth: API key has expired")
	}
	if !m.VerifyKey(rawKey, apiKey.Key) {
		return nil, ErrAPIKeyNotFound
	}
	return apiKey, nil
}

// Revoke marks the key as revoked so it will no longer be accepted.
func (m *APIKeyManager) Revoke(id string) error {
	apiKey, ok := m.keys[id]
	if !ok {
		return ErrAPIKeyNotFound
	}
	apiKey.Revoked = true
	return nil
}
