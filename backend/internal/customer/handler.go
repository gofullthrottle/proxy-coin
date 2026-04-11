// Package customer handles customer-facing REST API operations.
package customer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gofullthrottle/proxy-coin/backend/internal/auth"
	"github.com/gofullthrottle/proxy-coin/backend/internal/proxy"
)

// ---------------------------------------------------------------------------
// Shared types
// ---------------------------------------------------------------------------

// PricingTier enumerates the available service tiers for customers.
type PricingTier = string

const (
	TierFree       PricingTier = "free"
	TierStarter    PricingTier = "starter"
	TierPro        PricingTier = "pro"
	TierEnterprise PricingTier = "enterprise"
)

// Customer represents a registered API customer (business or developer).
type Customer struct {
	ID        string      `json:"id"`
	Email     string      `json:"email"`
	Tier      PricingTier `json:"tier"`
	CreatedAt time.Time   `json:"created_at"`
	APIKeyID  string      `json:"api_key_id,omitempty"`
}

// CreateCustomerRequest is the payload for creating a new customer.
type CreateCustomerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Tier     PricingTier `json:"tier,omitempty"`
}

// UsageSummary is a per-customer bandwidth consumption report.
type UsageSummary struct {
	CustomerID  string    `json:"customer_id"`
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
	BytesUsed   int64     `json:"bytes_used"`
	Requests    int64     `json:"requests"`
}

// ---------------------------------------------------------------------------
// Handler
// ---------------------------------------------------------------------------

// Handler exposes HTTP endpoints for customer registration and management.
type Handler struct {
	service    *Service
	jwtManager *auth.JWTManager
	apiKeyMgr  *auth.APIKeyManager
	proxyH     *proxy.Handler
	usageTracker *UsageTracker
}

// NewHandler creates a Handler backed by the given Service and dependencies.
func NewHandler(
	service *Service,
	jwtManager *auth.JWTManager,
	apiKeyMgr *auth.APIKeyManager,
	usageTracker *UsageTracker,
) *Handler {
	return &Handler{
		service:      service,
		jwtManager:   jwtManager,
		apiKeyMgr:    apiKeyMgr,
		usageTracker: usageTracker,
	}
}

// NewHandlerLegacy creates a Handler with only a Service, for backward compatibility.
func NewHandlerLegacy(service *Service) *Handler {
	return &Handler{service: service}
}

// SetProxyHandler wires the proxy.Handler for request forwarding.
func (h *Handler) SetProxyHandler(ph *proxy.Handler) {
	h.proxyH = ph
}

// RegisterRoutes mounts customer and auth endpoints on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Auth endpoints
	mux.HandleFunc("/v1/auth/register", h.HandleRegister)
	mux.HandleFunc("/v1/auth/login", h.HandleLogin)
	mux.HandleFunc("/v1/auth/refresh", h.HandleRefreshToken)
	mux.HandleFunc("/v1/auth/apikey", h.HandleGetAPIKey)
	mux.HandleFunc("/v1/auth/apikey/rotate", h.HandleRotateAPIKey)

	// Customer management (legacy)
	mux.HandleFunc("/v1/customers", h.handleCustomers)
	mux.HandleFunc("/v1/customers/", h.handleCustomerByID)

	// Proxy endpoints
	mux.HandleFunc("/v1/proxy", h.HandleProxyRequest)
	mux.HandleFunc("/v1/proxy/batch", h.HandleBatchProxy)

	// Usage
	mux.HandleFunc("/v1/usage", h.handleUsage)
}

// ---------------------------------------------------------------------------
// Auth handlers
// ---------------------------------------------------------------------------

// HandleRegister handles POST /v1/auth/register.
func (h *Handler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email and password are required"})
		return
	}

	customer, err := h.service.CreateCustomer(r.Context(), req.Email, req.Password)
	if err != nil {
		if strings.Contains(err.Error(), "already registered") {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "email already registered"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "registration failed"})
		return
	}

	accessToken, refreshToken, err := h.issueTokenPair(customer.ID, "customer", customer.Email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to issue token"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"customer":      customer,
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}

// HandleLogin handles POST /v1/auth/login.
func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	customer, err := h.service.AuthenticateCustomer(r.Context(), req.Email, req.Password)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	accessToken, refreshToken, err := h.issueTokenPair(customer.ID, "customer", customer.Email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to issue token"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"customer":      customer,
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}

// HandleRefreshToken handles POST /v1/auth/refresh.
func (h *Handler) HandleRefreshToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if h.jwtManager == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "jwt manager not configured"})
		return
	}

	newAccess, newRefresh, err := h.jwtManager.RefreshToken(req.RefreshToken)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid refresh token"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"access_token":  newAccess,
		"refresh_token": newRefresh,
	})
}

// HandleGetAPIKey handles GET /v1/auth/apikey.
// Returns the current API key ID for the authenticated customer.
func (h *Handler) HandleGetAPIKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	customerID, err := h.requireCustomerAuth(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return
	}

	if h.apiKeyMgr == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "api key manager not configured"})
		return
	}

	// Issue a new key (idempotent: rotate the existing one).
	apiKey, plain, err := h.apiKeyMgr.Issue(customerID, 0)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate api key"})
		return
	}

	// Persist the hash.
	if err := h.service.SaveAPIKey(r.Context(), customerID, apiKey.ID, apiKey.Key); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save api key"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"api_key_id":  apiKey.ID,
		"api_key":     plain, // returned once; customer must store this
		"created_at":  apiKey.CreatedAt,
	})
}

// HandleRotateAPIKey handles POST /v1/auth/apikey/rotate.
// Revokes the current key and issues a fresh one.
func (h *Handler) HandleRotateAPIKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	customerID, err := h.requireCustomerAuth(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return
	}

	if h.apiKeyMgr == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "api key manager not configured"})
		return
	}

	apiKey, plain, err := h.apiKeyMgr.Issue(customerID, 0)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to rotate api key"})
		return
	}

	if err := h.service.SaveAPIKey(r.Context(), customerID, apiKey.ID, apiKey.Key); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save rotated api key"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"api_key_id": apiKey.ID,
		"api_key":    plain,
		"created_at": apiKey.CreatedAt,
	})
}

// ---------------------------------------------------------------------------
// Legacy customer CRUD
// ---------------------------------------------------------------------------

func (h *Handler) handleCustomers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.createCustomer(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleCustomerByID(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getCustomer(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) createCustomer(w http.ResponseWriter, r *http.Request) {
	var req CreateCustomerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	customer, err := h.service.CreateCustomer(r.Context(), req.Email, req.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(customer)
}

func (h *Handler) getCustomer(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/v1/customers/")
	if id == "" {
		http.Error(w, "customer id required", http.StatusBadRequest)
		return
	}

	customer, err := h.service.GetCustomer(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(customer)
}

// ---------------------------------------------------------------------------
// Proxy handlers
// ---------------------------------------------------------------------------

// HandleProxyRequest handles POST /v1/proxy.
// Authenticates the customer via API key, then forwards to proxy.Handler.
func (h *Handler) HandleProxyRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	customerID, err := h.requireAPIKeyAuth(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "valid API key required"})
		return
	}

	var req proxy.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	req.CustomerID = customerID

	if h.proxyH == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "proxy not available"})
		return
	}

	resp, err := h.proxyH.HandleRequest(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	// Record usage (best-effort).
	if h.usageTracker != nil {
		_ = h.usageTracker.RecordUsage(r.Context(), customerID, 1, resp.BytesTransferred, 0)
	}

	writeJSON(w, http.StatusOK, resp)
}

// HandleBatchProxy handles POST /v1/proxy/batch.
// Executes multiple proxy requests concurrently and returns an array of responses.
func (h *Handler) HandleBatchProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	customerID, err := h.requireAPIKeyAuth(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "valid API key required"})
		return
	}

	var reqs []proxy.Request
	if err := json.NewDecoder(r.Body).Decode(&reqs); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if len(reqs) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "requests array is empty"})
		return
	}
	if len(reqs) > 50 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "batch size exceeds maximum of 50"})
		return
	}

	if h.proxyH == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "proxy not available"})
		return
	}

	type result struct {
		index int
		resp  *proxy.Response
		err   error
	}

	ch := make(chan result, len(reqs))
	for i := range reqs {
		i := i
		req := reqs[i]
		req.CustomerID = customerID
		go func() {
			resp, err := h.proxyH.HandleRequest(r.Context(), req)
			ch <- result{index: i, resp: resp, err: err}
		}()
	}

	responses := make([]interface{}, len(reqs))
	var totalBytes int64
	for range reqs {
		res := <-ch
		if res.err != nil {
			responses[res.index] = map[string]string{"error": res.err.Error()}
		} else {
			responses[res.index] = res.resp
			totalBytes += res.resp.BytesTransferred
		}
	}

	// Record aggregated usage.
	if h.usageTracker != nil {
		_ = h.usageTracker.RecordUsage(r.Context(), customerID, int64(len(reqs)), totalBytes, 0)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"results": responses})
}

// ---------------------------------------------------------------------------
// Usage handler
// ---------------------------------------------------------------------------

func (h *Handler) handleUsage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	customerID, err := h.requireCustomerAuth(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return
	}

	if h.usageTracker == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "usage tracking not configured"})
		return
	}

	summary, err := h.usageTracker.GetUsage(r.Context(), customerID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to fetch usage"})
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// issueTokenPair generates a new access + refresh token pair for the given subject.
func (h *Handler) issueTokenPair(customerID, role, email string) (accessToken, refreshToken string, err error) {
	if h.jwtManager == nil {
		return "", "", fmt.Errorf("jwt manager not configured")
	}
	accessToken, err = h.jwtManager.GenerateToken(customerID, role, email)
	if err != nil {
		return "", "", fmt.Errorf("generate access token: %w", err)
	}
	refreshToken, err = h.jwtManager.GenerateRefreshToken(customerID)
	if err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}
	return accessToken, refreshToken, nil
}

// requireCustomerAuth extracts and validates the Bearer JWT from the request,
// returning the customer ID on success.
func (h *Handler) requireCustomerAuth(r *http.Request) (string, error) {
	if h.jwtManager == nil {
		return "", fmt.Errorf("jwt manager not configured")
	}
	bearerToken := r.Header.Get("Authorization")
	bearerToken = strings.TrimPrefix(bearerToken, "Bearer ")
	if bearerToken == "" {
		return "", fmt.Errorf("missing bearer token")
	}
	claims, err := h.jwtManager.ValidateToken(bearerToken)
	if err != nil {
		return "", err
	}
	return claims.Subject, nil
}

// requireAPIKeyAuth extracts the API key from the Authorization header
// (Authorization: Bearer prxy_live_...) and validates it, returning the
// customer ID on success.
func (h *Handler) requireAPIKeyAuth(r *http.Request) (string, error) {
	if h.apiKeyMgr == nil {
		return "", fmt.Errorf("api key manager not configured")
	}
	apiKey := r.Header.Get("Authorization")
	apiKey = strings.TrimPrefix(apiKey, "Bearer ")
	if apiKey == "" {
		// Also accept X-API-Key header.
		apiKey = r.Header.Get("X-API-Key")
	}
	if apiKey == "" {
		return "", fmt.Errorf("missing api key")
	}

	key, err := h.apiKeyMgr.Validate(apiKey)
	if err != nil {
		return "", err
	}
	return key.CustomerID, nil
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
