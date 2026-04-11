// Package blockchain provides on-chain interactions with Base L2 smart contracts.
package blockchain

import "context"

// Client wraps an Ethereum JSON-RPC connection to Base L2.
// Full implementation uses go-ethereum once the dependency is added.
type Client struct {
	rpcURL string
	// ethClient will hold *ethclient.Client once go-ethereum is imported.
	ethClient interface{}
}

// NewClient creates a Client connected to the given Base L2 RPC endpoint.
func NewClient(rpcURL string) (*Client, error) {
	return &Client{rpcURL: rpcURL}, nil
}

// BlockNumber returns the current chain head block number.
func (c *Client) BlockNumber(ctx context.Context) (uint64, error) {
	// Placeholder — implementation pending go-ethereum dependency.
	return 0, nil
}

// Close releases the underlying RPC connection.
func (c *Client) Close() {
	// Placeholder — implementation pending go-ethereum dependency.
}
