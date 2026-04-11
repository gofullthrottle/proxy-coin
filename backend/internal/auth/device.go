// Package auth handles authentication for node operators and API customers.
package auth

import (
	"errors"
	"time"
)

// DeviceToken is a short-lived credential issued to an Android node on
// successful attestation, used to authenticate WebSocket connections.
type DeviceToken struct {
	NodeID    string    `json:"node_id"`
	Token     string    `json:"token"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// DeviceAuthenticator issues and verifies per-device tokens for Android nodes.
type DeviceAuthenticator struct {
	jwtManager *JWTManager
}

// NewDeviceAuthenticator creates a DeviceAuthenticator backed by the given JWTManager.
func NewDeviceAuthenticator(jwtManager *JWTManager) *DeviceAuthenticator {
	return &DeviceAuthenticator{jwtManager: jwtManager}
}

// IssueDeviceToken creates a short-lived JWT for an Android node after attestation.
func (d *DeviceAuthenticator) IssueDeviceToken(nodeID string) (*DeviceToken, error) {
	tokenStr, err := d.jwtManager.Issue(nodeID, "node")
	if err != nil {
		return nil, err
	}
	return &DeviceToken{
		NodeID:    nodeID,
		Token:     tokenStr,
		IssuedAt:  time.Now(),
		ExpiresAt: time.Now().Add(d.jwtManager.ttl),
	}, nil
}

// VerifyDeviceToken validates a device token and returns the node ID on success.
func (d *DeviceAuthenticator) VerifyDeviceToken(token string) (string, error) {
	claims, err := d.jwtManager.Verify(token)
	if err != nil {
		return "", err
	}
	if claims.Role != "node" {
		return "", errors.New("auth: token role is not 'node'")
	}
	return claims.Subject, nil
}
