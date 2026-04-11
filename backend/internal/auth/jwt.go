// Package auth handles authentication for node operators and API customers.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrInvalidToken is returned when a JWT cannot be verified or has expired.
var ErrInvalidToken = errors.New("auth: invalid or expired token")

// jwtHeader is the fixed, pre-encoded header for all tokens issued by this manager.
// {"alg":"HS256","typ":"JWT"}
const jwtHeaderEncoded = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"

// Claims holds the payload embedded in a Proxy Coin JWT.
type Claims struct {
	Subject   string    `json:"sub"`
	Role      string    `json:"role"`
	Email     string    `json:"email,omitempty"`
	TokenType string    `json:"token_type,omitempty"` // "access" or "refresh"
	IssuedAt  time.Time `json:"iat"`
	ExpiresAt time.Time `json:"exp"`
}

// rawClaims is the JSON-serialisable form used during encoding/decoding.
type rawClaims struct {
	Sub       string `json:"sub"`
	Role      string `json:"role,omitempty"`
	Email     string `json:"email,omitempty"`
	TokenType string `json:"token_type,omitempty"`
	Iat       int64  `json:"iat"`
	Exp       int64  `json:"exp"`
}

// JWTManager issues and verifies JSON Web Tokens for API access.
// It uses HMAC-SHA256 (HS256) signing — no external library required.
type JWTManager struct {
	secret        []byte
	ttl           time.Duration
	refreshExpiry time.Duration
}

// NewJWTManager creates a JWTManager with the given signing secret and TTLs.
//
//   - secret: HMAC-SHA256 key (loaded from config/secrets; minimum 32 bytes recommended)
//   - ttl: access token lifetime (e.g. 15 * time.Minute)
//   - refreshExpiry: refresh token lifetime (e.g. 7 * 24 * time.Hour)
func NewJWTManager(secret string, ttl, refreshExpiry time.Duration) *JWTManager {
	return &JWTManager{
		secret:        []byte(secret),
		ttl:           ttl,
		refreshExpiry: refreshExpiry,
	}
}

// NewJWTManagerFromBytes creates a JWTManager from a raw byte slice secret.
// This constructor supports the original API used by device.go and other callers
// that obtain the secret as []byte from config rather than a string.
func NewJWTManagerFromBytes(secret []byte, ttl time.Duration) *JWTManager {
	return &JWTManager{
		secret:        secret,
		ttl:           ttl,
		refreshExpiry: 7 * 24 * time.Hour,
	}
}

// Issue creates a signed access JWT for the given subject and role.
// This is the original API kept for backward compatibility with device.go.
func (m *JWTManager) Issue(subject, role string) (string, error) {
	return m.GenerateToken(subject, role, "")
}

// Verify parses and validates a JWT string, returning its Claims.
// This is the original API kept for backward compatibility with device.go.
func (m *JWTManager) Verify(token string) (*Claims, error) {
	return m.ValidateToken(token)
}

// GenerateToken creates a signed access JWT for the given customer.
func (m *JWTManager) GenerateToken(customerID, role, email string) (string, error) {
	now := time.Now()
	rc := rawClaims{
		Sub:       customerID,
		Role:      role,
		Email:     email,
		TokenType: "access",
		Iat:       now.Unix(),
		Exp:       now.Add(m.ttl).Unix(),
	}
	return m.sign(rc)
}

// GenerateRefreshToken creates a long-lived refresh JWT.
// Refresh tokens carry only the subject; they cannot be used for API access directly.
func (m *JWTManager) GenerateRefreshToken(customerID string) (string, error) {
	now := time.Now()
	rc := rawClaims{
		Sub:       customerID,
		TokenType: "refresh",
		Iat:       now.Unix(),
		Exp:       now.Add(m.refreshExpiry).Unix(),
	}
	return m.sign(rc)
}

// ValidateToken parses and validates a JWT string, returning its Claims.
// Returns ErrInvalidToken on any structural, signature, or expiry failure.
func (m *JWTManager) ValidateToken(tokenStr string) (*Claims, error) {
	parts := strings.SplitN(tokenStr, ".", 3)
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	// Verify signature: HMAC-SHA256(header.payload, secret)
	message := parts[0] + "." + parts[1]
	expectedSig := m.computeSignature(message)
	if !hmac.Equal([]byte(expectedSig), []byte(parts[2])) {
		return nil, ErrInvalidToken
	}

	// Decode payload.
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}

	var rc rawClaims
	if err := json.Unmarshal(payloadJSON, &rc); err != nil {
		return nil, ErrInvalidToken
	}

	if time.Now().Unix() > rc.Exp {
		return nil, ErrInvalidToken
	}

	return &Claims{
		Subject:   rc.Sub,
		Role:      rc.Role,
		Email:     rc.Email,
		TokenType: rc.TokenType,
		IssuedAt:  time.Unix(rc.Iat, 0),
		ExpiresAt: time.Unix(rc.Exp, 0),
	}, nil
}

// RefreshToken validates a refresh token and issues a new access + refresh token pair.
// Returns (newAccessToken, newRefreshToken, error).
func (m *JWTManager) RefreshToken(refreshTokenStr string) (string, string, error) {
	claims, err := m.ValidateToken(refreshTokenStr)
	if err != nil {
		return "", "", fmt.Errorf("auth: refresh token invalid: %w", err)
	}
	if claims.TokenType != "refresh" {
		return "", "", fmt.Errorf("auth: provided token is not a refresh token")
	}

	newAccess, err := m.GenerateToken(claims.Subject, claims.Role, claims.Email)
	if err != nil {
		return "", "", fmt.Errorf("auth: generate new access token: %w", err)
	}

	newRefresh, err := m.GenerateRefreshToken(claims.Subject)
	if err != nil {
		return "", "", fmt.Errorf("auth: generate new refresh token: %w", err)
	}

	return newAccess, newRefresh, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// sign encodes the claims as a compact JWT and returns the signed token string.
func (m *JWTManager) sign(rc rawClaims) (string, error) {
	payloadJSON, err := json.Marshal(rc)
	if err != nil {
		return "", fmt.Errorf("auth: marshal claims: %w", err)
	}

	payloadEncoded := base64.RawURLEncoding.EncodeToString(payloadJSON)
	message := jwtHeaderEncoded + "." + payloadEncoded
	signature := m.computeSignature(message)

	return message + "." + signature, nil
}

// computeSignature returns the base64url-encoded HMAC-SHA256 of the message.
func (m *JWTManager) computeSignature(message string) string {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(message))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
