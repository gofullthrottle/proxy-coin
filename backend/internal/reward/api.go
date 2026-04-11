// Package reward calculates PRXY token rewards for proxy node operators.
package reward

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
)

// RewardAPI exposes HTTP endpoints for reward proof retrieval and history.
type RewardAPI struct {
	generator *Generator
	ledger    *Ledger
}

// NewRewardAPI creates a RewardAPI backed by the given Generator and Ledger.
func NewRewardAPI(generator *Generator, ledger *Ledger) *RewardAPI {
	return &RewardAPI{
		generator: generator,
		ledger:    ledger,
	}
}

// RegisterRoutes mounts reward endpoints on the given mux.
func (a *RewardAPI) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/rewards/proof", a.HandleGetProof)
	mux.HandleFunc("/v1/rewards/history", a.HandleGetHistory)
}

// ---------------------------------------------------------------------------
// GET /v1/rewards/proof
// ---------------------------------------------------------------------------

// ProofResponse is the JSON payload returned by HandleGetProof.
type ProofResponse struct {
	WalletAddress    string   `json:"wallet_address"`
	LeafIndex        int      `json:"leaf_index"`
	CumulativeAmount string   `json:"cumulative_amount_wei"` // decimal string to avoid JS precision loss
	Proof            []string `json:"proof"`                // hex-encoded sibling hashes
	MerkleRoot       string   `json:"merkle_root"`
	MerkleRootID     int64    `json:"merkle_root_id"`
}

// HandleGetProof handles GET /v1/rewards/proof?wallet=0x...
// It returns the current Merkle proof for the given wallet address.
// If no pending rewards exist, it returns a 404.
func (a *RewardAPI) HandleGetProof(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	wallet := strings.TrimSpace(r.URL.Query().Get("wallet"))
	if wallet == "" {
		http.Error(w, `{"error":"wallet query parameter is required"}`, http.StatusBadRequest)
		return
	}

	// Generate (or re-use cached) Merkle tree.
	mr, err := a.generator.GenerateMerkleTree(r.Context())
	if err != nil {
		http.Error(w, `{"error":"failed to generate merkle tree"}`, http.StatusInternalServerError)
		return
	}

	if len(mr.Root) == 0 || len(mr.Proofs) == 0 {
		http.Error(w, `{"error":"no pending rewards"}`, http.StatusNotFound)
		return
	}

	// Find the proof entry for the requested wallet (case-insensitive).
	walletLower := strings.ToLower(wallet)
	var found *MerkleProofEntry
	for i := range mr.Proofs {
		if strings.ToLower(mr.Proofs[i].WalletAddr) == walletLower {
			found = &mr.Proofs[i]
			break
		}
	}

	if found == nil {
		http.Error(w, `{"error":"no pending rewards for wallet"}`, http.StatusNotFound)
		return
	}

	// Encode proof siblings as hex strings.
	proofHex := make([]string, len(found.Proof))
	for i, p := range found.Proof {
		proofHex[i] = "0x" + hex.EncodeToString(p)
	}

	resp := ProofResponse{
		WalletAddress:    found.WalletAddr,
		LeafIndex:        found.Index,
		CumulativeAmount: found.CumulativeAmount.String(),
		Proof:            proofHex,
		MerkleRoot:       "0x" + hex.EncodeToString(mr.Root),
		MerkleRootID:     mr.ID,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ---------------------------------------------------------------------------
// GET /v1/rewards/history
// ---------------------------------------------------------------------------

// HistoryResponse is the JSON payload returned by HandleGetHistory.
type HistoryResponse struct {
	NodeID   string          `json:"node_id"`
	Summary  *EarningsSummary `json:"summary"`
	Records  []EarningRecord  `json:"records"`
}

// HandleGetHistory handles GET /v1/rewards/history?node_id=...&from=...&to=...
// It returns the earnings history for a node within an optional date range.
func (a *RewardAPI) HandleGetHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query()
	nodeID := strings.TrimSpace(q.Get("node_id"))
	if nodeID == "" {
		http.Error(w, `{"error":"node_id query parameter is required"}`, http.StatusBadRequest)
		return
	}

	from := q.Get("from")
	to := q.Get("to")

	records, err := a.ledger.GetEarnings(r.Context(), nodeID, from, to)
	if err != nil {
		http.Error(w, `{"error":"failed to fetch earnings history"}`, http.StatusInternalServerError)
		return
	}

	summary, err := a.ledger.GetSummary(r.Context(), nodeID)
	if err != nil {
		// Non-fatal: return records without summary.
		summary = &EarningsSummary{}
	}

	if records == nil {
		records = []EarningRecord{}
	}

	resp := HistoryResponse{
		NodeID:  nodeID,
		Summary: summary,
		Records: records,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
