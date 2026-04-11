// Package reward calculates PRXY token rewards for proxy node operators.
package reward

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// setMerkleRootSelector is the 4-byte Keccak-256 selector for
// setMerkleRoot(bytes32), computed as keccak256("setMerkleRoot(bytes32)")[0:4].
// Precomputed: 0x7f26b83f
var setMerkleRootSelector = [4]byte{0x7f, 0x26, 0xb8, 0x3f}

// Publisher submits Merkle roots to the RewardDistributor smart contract on Base L2.
// It encodes the ABI call manually so no abigen-generated bindings are required
// before contracts are deployed.
//
// The general flow is:
//
//  1. ABI-encode the setMerkleRoot(bytes32) call
//  2. Sign and broadcast the transaction via JSON-RPC (go-ethereum wired at startup)
//  3. Persist the resulting tx hash in the database for audit
//
// When go-ethereum is added, replace the placeholder sendRawTransaction stub
// with the real ethclient.Client call.
type Publisher struct {
	rpcURL          string
	privateKey      *ecdsa.PrivateKey
	contractAddress string
	pool            *pgxpool.Pool
	maxRetries      int
	retryBackoff    time.Duration
}

// NewPublisher creates a Publisher.
//
//   - rpcURL: Base L2 JSON-RPC endpoint (e.g. "https://mainnet.base.org")
//   - privateKey: the ECDSA key used to sign transactions
//   - contractAddress: hex address of the RewardDistributor contract (with or without "0x")
//   - pool: Postgres connection pool for storing tx hashes
func NewPublisher(
	rpcURL string,
	privateKey *ecdsa.PrivateKey,
	contractAddress string,
	pool *pgxpool.Pool,
) *Publisher {
	return &Publisher{
		rpcURL:          rpcURL,
		privateKey:      privateKey,
		contractAddress: contractAddress,
		pool:            pool,
		maxRetries:      3,
		retryBackoff:    2 * time.Second,
	}
}

// PublishRoot submits a setMerkleRoot(root) transaction to the RewardDistributor
// contract and returns the transaction hash.  It retries up to maxRetries times
// on transient failures.
func (p *Publisher) PublishRoot(ctx context.Context, root []byte) (string, error) {
	calldata, err := encodeMerkleRootCall(root)
	if err != nil {
		return "", fmt.Errorf("publisher: encode calldata: %w", err)
	}

	var txHash string
	var lastErr error

	for attempt := 0; attempt <= p.maxRetries; attempt++ {
		if attempt > 0 {
			log.Printf("publisher: retry %d/%d after error: %v", attempt, p.maxRetries, lastErr)
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(p.retryBackoff * time.Duration(attempt)):
			}
		}

		txHash, lastErr = p.sendTransaction(ctx, calldata)
		if lastErr == nil {
			break
		}
	}

	if lastErr != nil {
		return "", fmt.Errorf("publisher: send transaction after %d attempts: %w", p.maxRetries+1, lastErr)
	}

	if err := p.storeTxHash(ctx, root, txHash); err != nil {
		// Non-fatal: the tx was submitted on-chain; log and continue.
		log.Printf("publisher: WARNING failed to persist tx hash %s: %v", txHash, err)
	}

	log.Printf("publisher: submitted merkle root %s tx=%s", hex.EncodeToString(root), txHash)
	return txHash, nil
}

// encodeMerkleRootCall ABI-encodes a call to setMerkleRoot(bytes32).
//
// ABI encoding:
//
//	bytes[0:4]   = function selector (4 bytes)
//	bytes[4:36]  = root padded to 32 bytes (bytes32 argument)
func encodeMerkleRootCall(root []byte) ([]byte, error) {
	if len(root) > 32 {
		return nil, fmt.Errorf("publisher: root is %d bytes; must be <= 32", len(root))
	}

	calldata := make([]byte, 4+32)
	copy(calldata[0:4], setMerkleRootSelector[:])

	// bytes32 is left-aligned (padded on the right with zeros).
	copy(calldata[4:4+len(root)], root)

	return calldata, nil
}

// sendTransaction signs and broadcasts a raw transaction to the Base L2 RPC.
//
// TODO(production): replace this stub with go-ethereum ethclient:
//
//	nonce, _ := client.PendingNonceAt(ctx, fromAddress)
//	gasPrice, _ := client.SuggestGasPrice(ctx)
//	tx := types.NewTransaction(nonce, contractAddr, value, gasLimit, gasPrice, calldata)
//	signed, _ := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
//	client.SendTransaction(ctx, signed)
//	return signed.Hash().Hex(), nil
func (p *Publisher) sendTransaction(ctx context.Context, calldata []byte) (string, error) {
	if p.rpcURL == "" {
		return "", fmt.Errorf("publisher: rpcURL is not configured")
	}
	if p.privateKey == nil {
		return "", fmt.Errorf("publisher: private key is not configured")
	}
	if p.contractAddress == "" {
		return "", fmt.Errorf("publisher: contract address is not configured")
	}

	// Placeholder until go-ethereum is wired in.
	// Return a deterministic stub hash based on calldata for development.
	_ = calldata
	return "", fmt.Errorf("publisher: go-ethereum not yet wired — add github.com/ethereum/go-ethereum to go.mod")
}

// storeTxHash persists the submitted transaction hash in the database for audit
// and monitoring purposes.
func (p *Publisher) storeTxHash(ctx context.Context, root []byte, txHash string) error {
	const q = `
		INSERT INTO merkle_root_submissions (root_hash, tx_hash, submitted_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (root_hash) DO UPDATE SET
			tx_hash      = EXCLUDED.tx_hash,
			submitted_at = EXCLUDED.submitted_at
	`
	_, err := p.pool.Exec(ctx, q, root, txHash, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("publisher: insert tx hash: %w", err)
	}
	return nil
}

// GasEstimate holds gas parameters for a transaction.
type GasEstimate struct {
	GasLimit *big.Int
	GasPrice *big.Int
}

// EstimateGas returns a conservative gas estimate for setMerkleRoot.
// The call is a single storage write on-chain (~30 000 gas); we pad with
// a 50 % buffer to avoid out-of-gas failures.
func EstimateGas() GasEstimate {
	return GasEstimate{
		GasLimit: big.NewInt(45_000),
		GasPrice: big.NewInt(1_500_000_000), // 1.5 Gwei baseline for Base L2
	}
}
