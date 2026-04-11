// Package blockchain provides on-chain interactions with Base L2 smart contracts.
package blockchain

import (
	"context"
	"math/big"
)

// TokenClient interacts with the PRXY ERC-20 token contract on Base L2.
type TokenClient struct {
	client          *Client
	contractAddress string
}

// NewTokenClient creates a TokenClient for the PRXY contract at the given address.
func NewTokenClient(client *Client, contractAddress string) *TokenClient {
	return &TokenClient{
		client:          client,
		contractAddress: contractAddress,
	}
}

// BalanceOf returns the PRXY token balance (in wei) for the given wallet address.
func (t *TokenClient) BalanceOf(ctx context.Context, walletAddr string) (*big.Int, error) {
	// Placeholder — implementation pending go-ethereum + ABI bindings.
	return big.NewInt(0), nil
}

// TotalSupply returns the current PRXY total supply in wei.
func (t *TokenClient) TotalSupply(ctx context.Context) (*big.Int, error) {
	// Placeholder — implementation pending go-ethereum + ABI bindings.
	return big.NewInt(0), nil
}
