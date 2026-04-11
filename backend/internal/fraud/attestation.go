// Package fraud provides anti-fraud detection for the Proxy Coin network.
package fraud

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// attestationMaxAge is the maximum age of an attestation token before a
// node must re-attest. Android Play Integrity tokens expire after 1 hour;
// we enforce a 6-hour server-side window to give devices room to refresh.
const attestationMaxAge = 6 * time.Hour

// AttestationRecord stores the most recent device attestation result for a node.
type AttestationRecord struct {
	NodeID     string    `json:"node_id"`
	IsValid    bool      `json:"is_valid"`
	AttestedAt time.Time `json:"attested_at"`
	// ExpiresAt is when the attestation must be renewed.
	ExpiresAt time.Time `json:"expires_at"`
	// Error is populated when IsValid is false, describing the failure.
	Error string `json:"error,omitempty"`
}

// AttestationVerifier checks that nodes have valid Android Play Integrity
// (or equivalent) attestations before allowing reward accumulation.
// It maintains an in-memory cache of results; the backing storage for
// persistence is managed by the caller (e.g. via the database layer).
//
// All methods are safe for concurrent use.
type AttestationVerifier struct {
	mu      sync.RWMutex
	records map[string]*AttestationRecord
}

// NewAttestationVerifier creates a verifier with an empty attestation store.
func NewAttestationVerifier() *AttestationVerifier {
	return &AttestationVerifier{
		records: make(map[string]*AttestationRecord),
	}
}

// Record stores or updates an attestation result for a node.
func (v *AttestationVerifier) Record(rec *AttestationRecord) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.records[rec.NodeID] = rec
}

// Verify checks whether nodeID has a valid, unexpired attestation.
func (v *AttestationVerifier) Verify(nodeID string) (Verdict, string) {
	v.mu.RLock()
	rec, ok := v.records[nodeID]
	v.mu.RUnlock()

	if !ok {
		return VerdictReview, "attestation: no attestation on file for node " + nodeID
	}
	if !rec.IsValid {
		return VerdictBlock, fmt.Sprintf("attestation: invalid attestation for node %s: %s", nodeID, rec.Error)
	}
	if time.Now().After(rec.ExpiresAt) {
		return VerdictReview, "attestation: expired attestation for node " + nodeID
	}
	return VerdictAllow, ""
}

// VerifyAttestation contacts Google's Play Integrity API to verify an
// integrity token submitted by the Android node at registration time.
//
// token is the Play Integrity token produced by the Android device.
// The result is cached in the verifier so subsequent calls to Verify()
// reflect the outcome without re-contacting Google.
//
// Production integration note: this function should call
// https://playintegrity.googleapis.com/v1/{packageName}:decodeIntegrityToken
// using a service-account credential. The implementation below provides the
// correct structure and caching; the HTTP call is a well-defined stub that
// must be wired to the real API before launch.
func (v *AttestationVerifier) VerifyAttestation(ctx context.Context, nodeID, token string) (bool, error) {
	if nodeID == "" {
		return false, fmt.Errorf("attestation: nodeID must not be empty")
	}
	if token == "" {
		// Empty token → device has not submitted attestation.
		rec := &AttestationRecord{
			NodeID:     nodeID,
			IsValid:    false,
			AttestedAt: time.Now(),
			ExpiresAt:  time.Now().Add(attestationMaxAge),
			Error:      "empty integrity token",
		}
		v.Record(rec)
		return false, fmt.Errorf("attestation: node %s submitted empty integrity token", nodeID)
	}

	// TODO(production): replace this stub with a real Google Play Integrity
	// API call. The call should:
	//
	//   1. Obtain a short-lived OAuth2 access token via the service account
	//      stored in Infisical at path: /proxy-coin/prod/google/play-integrity-sa-json
	//
	//   2. POST to:
	//        https://playintegrity.googleapis.com/v1/com.proxycoin.app:decodeIntegrityToken
	//      with body: { "integrity_token": "<token>" }
	//
	//   3. Inspect the response fields:
	//      - requestDetails.requestPackageName == "com.proxycoin.app"
	//      - appIntegrity.appRecognitionVerdict == "PLAY_RECOGNIZED"
	//      - deviceIntegrity.deviceRecognitionVerdict contains "MEETS_DEVICE_INTEGRITY"
	//      - accountDetails.appLicensingVerdict == "LICENSED"
	//
	//   4. Return (true, nil) only when all four conditions pass.
	//
	// Error: "attestation: Google Play Integrity API call failed: <status>: <body>"

	// Stub: accept any non-empty token as valid for development.
	isValid := len(token) >= 10
	errMsg := ""
	if !isValid {
		errMsg = "token too short (minimum 10 characters) — stub rejection"
	}

	rec := &AttestationRecord{
		NodeID:     nodeID,
		IsValid:    isValid,
		AttestedAt: time.Now(),
		ExpiresAt:  time.Now().Add(attestationMaxAge),
		Error:      errMsg,
	}
	v.Record(rec)

	if !isValid {
		return false, fmt.Errorf("attestation: node %s failed integrity check: %s", nodeID, errMsg)
	}
	return true, nil
}

// CheckAttestationAge reports whether the stored attestation for nodeID is
// fresh enough to allow reward accumulation. An attestation is considered
// fresh if it was recorded within the last attestationMaxAge window and is
// still marked as valid.
//
// Returns (true, nil) when the node is cleared for rewards.
// Returns (false, nil) when the attestation is too old or absent.
// Returns (false, error) only on internal errors.
func (v *AttestationVerifier) CheckAttestationAge(ctx context.Context, nodeID string) (bool, error) {
	v.mu.RLock()
	rec, ok := v.records[nodeID]
	v.mu.RUnlock()

	if !ok {
		// No record on file — node has never attested.
		return false, nil
	}

	if !rec.IsValid {
		return false, nil
	}

	age := time.Since(rec.AttestedAt)
	if age > attestationMaxAge {
		return false, nil
	}

	// Also check the explicit expiry in case it was set shorter than the max age.
	if time.Now().After(rec.ExpiresAt) {
		return false, nil
	}

	return true, nil
}

// GetRecord returns the stored attestation record for nodeID, or nil if none
// exists. The returned value is a copy; mutating it has no effect on the cache.
func (v *AttestationVerifier) GetRecord(nodeID string) *AttestationRecord {
	v.mu.RLock()
	rec, ok := v.records[nodeID]
	v.mu.RUnlock()
	if !ok {
		return nil
	}
	// Return a copy to prevent callers from mutating the cached value.
	copy := *rec
	return &copy
}

// Invalidate removes the attestation record for nodeID, forcing a fresh
// attestation on the next request. Use this after a suspicious event.
func (v *AttestationVerifier) Invalidate(nodeID string) {
	v.mu.Lock()
	delete(v.records, nodeID)
	v.mu.Unlock()
}
