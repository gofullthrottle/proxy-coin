// Package blockchain provides on-chain interactions with Base L2 smart contracts.
package blockchain

import (
	"context"
	"math/big"
)

// ClaimProof contains the data a node operator needs to claim their PRXY reward
// from the on-chain MerkleDistributor contract.
type ClaimProof struct {
	Index      uint64     `json:"index"`
	WalletAddr string     `json:"wallet_addr"`
	AmountWei  *big.Int   `json:"amount_wei"`
	// Proof is the ordered list of sibling hashes from leaf to root.
	Proof      [][]byte   `json:"proof"`
}

// DistributorClient interacts with the MerkleDistributor smart contract.
type DistributorClient struct {
	client          *Client
	contractAddress string
}

// NewDistributorClient creates a DistributorClient for the contract at the given address.
func NewDistributorClient(client *Client, contractAddress string) *DistributorClient {
	return &DistributorClient{
		client:          client,
		contractAddress: contractAddress,
	}
}

// SetMerkleRoot submits a new Merkle root to the distributor contract,
// opening a new reward epoch for claims.
func (d *DistributorClient) SetMerkleRoot(ctx context.Context, root [32]byte) error {
	// Placeholder — implementation pending go-ethereum + ABI bindings.
	return nil
}

// IsClaimed returns true when the given index has already been claimed.
func (d *DistributorClient) IsClaimed(ctx context.Context, index uint64) (bool, error) {
	// Placeholder — implementation pending go-ethereum + ABI bindings.
	return false, nil
}
