// Package reward calculates PRXY token rewards for proxy node operators.
package reward

import (
	"encoding/hex"
	"log"
)

// Distributor orchestrates the end-to-end reward settlement process:
// aggregate usage → calculate rewards → build Merkle tree → submit root on-chain.
type Distributor struct {
	calculator *Calculator
}

// NewDistributor creates a Distributor using the given Calculator.
func NewDistributor(calculator *Calculator) *Distributor {
	return &Distributor{calculator: calculator}
}

// Distribute takes a slice of NodeRewards, builds the Merkle tree from their
// leaf hashes, and returns the tree for on-chain submission.
func (d *Distributor) Distribute(rewards []NodeReward) (*MerkleTree, error) {
	leafHashes := make([][]byte, len(rewards))
	for i, r := range rewards {
		leafHashes[i] = HashLeaf(r.WalletAddr, r.AmountWei)
	}

	tree := BuildTree(leafHashes)
	rootHex := ""
	if r := tree.Root(); r != nil {
		rootHex = hex.EncodeToString(r)
	}
	log.Printf("reward: built Merkle tree root=%s leaves=%d", rootHex, len(rewards))

	// On-chain submission will be wired via pkg/blockchain once contracts are deployed.
	return tree, nil
}
