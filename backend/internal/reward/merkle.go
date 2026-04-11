// Package reward calculates PRXY token rewards for proxy node operators.
package reward

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/sha3"
)

// ---------------------------------------------------------------------------
// Keccak-256 helper — matches Solidity's keccak256()
// ---------------------------------------------------------------------------

// keccak256 computes the Ethereum-compatible Keccak-256 hash of data.
// This is NOT SHA-3 (FIPS 202); it is the pre-standard Keccak variant used
// by Solidity and the EVM.
func keccak256(data []byte) []byte {
	h := sha3.NewLegacyKeccak256()
	h.Write(data)
	return h.Sum(nil)
}

// ---------------------------------------------------------------------------
// Leaf encoding — must match the Solidity MerkleDistributor contract
// ---------------------------------------------------------------------------

// HashLeaf computes the leaf hash for a (wallet, cumulativeAmount) pair,
// encoding it to match the on-chain contract's expected ABI layout:
//
//	keccak256(abi.encode(wallet, amount))
//
// abi.encode packs:
//   - wallet as a left-zero-padded 32-byte address
//   - amount as a 32-byte big-endian uint256
//
// The resulting bytes are then Keccak-256 hashed.
func HashLeaf(wallet string, cumulativeAmount *big.Int) []byte {
	// Strip leading "0x" if present.
	addr := wallet
	if len(addr) >= 2 && (addr[:2] == "0x" || addr[:2] == "0X") {
		addr = addr[2:]
	}

	// Decode hex address bytes (20 bytes for an Ethereum address).
	addrBytes, err := hex.DecodeString(addr)
	if err != nil || len(addrBytes) > 32 {
		// Fallback: use raw bytes, right-aligned in 32 bytes.
		addrBytes = []byte(wallet)
	}

	// ABI-encode: left-pad address to 32 bytes.
	var buf [64]byte
	copy(buf[32-len(addrBytes):32], addrBytes)

	// ABI-encode: big-endian uint256 for the amount.
	amtBytes := cumulativeAmount.Bytes()
	if len(amtBytes) > 32 {
		amtBytes = amtBytes[len(amtBytes)-32:] // truncate to 32 bytes (safety)
	}
	copy(buf[64-len(amtBytes):64], amtBytes)

	return keccak256(buf[:])
}

// ---------------------------------------------------------------------------
// MerkleTree
// ---------------------------------------------------------------------------

// MerkleLeaf is a single entry in the reward distribution Merkle tree.
type MerkleLeaf struct {
	// Index is the leaf's position in the sorted leaf array.
	Index      uint64   `json:"index"`
	WalletAddr string   `json:"wallet_addr"`
	AmountWei  *big.Int `json:"amount_wei"`
}

// Hash returns the Keccak-256 leaf hash matching the on-chain contract.
func (l *MerkleLeaf) Hash() []byte {
	return HashLeaf(l.WalletAddr, l.AmountWei)
}

// MerkleTree is an immutable binary Merkle tree built from a set of leaves.
// Nodes at each level are sorted pairs (smaller hash first) to produce a
// deterministic root that matches the OpenZeppelin MerkleProof library.
type MerkleTree struct {
	leaves [][]byte   // ordered leaf hashes (sorted)
	levels [][][]byte // levels[0] = leaves, levels[n] = root
}

// BuildTree constructs a binary Merkle tree from the provided leaf hashes.
// Leaves are sorted lexicographically before building so the root is
// deterministic regardless of insertion order.
func BuildTree(leafHashes [][]byte) *MerkleTree {
	if len(leafHashes) == 0 {
		return &MerkleTree{}
	}

	// Sort leaf hashes for determinism.
	sorted := make([][]byte, len(leafHashes))
	copy(sorted, leafHashes)
	sort.Slice(sorted, func(i, j int) bool {
		return bytes.Compare(sorted[i], sorted[j]) < 0
	})

	t := &MerkleTree{}
	t.levels = append(t.levels, sorted)

	current := sorted
	for len(current) > 1 {
		next := make([][]byte, 0, (len(current)+1)/2)
		for i := 0; i < len(current); i += 2 {
			if i+1 == len(current) {
				// Odd number: duplicate the last node.
				next = append(next, hashPair(current[i], current[i]))
			} else {
				next = append(next, hashPair(current[i], current[i+1]))
			}
		}
		t.levels = append(t.levels, next)
		current = next
	}

	t.leaves = sorted
	return t
}

// hashPair sorts a and b lexicographically, concatenates them, and returns
// the Keccak-256 hash. This matches the OpenZeppelin MerkleProof library's
// commutative hashing scheme.
func hashPair(a, b []byte) []byte {
	if bytes.Compare(a, b) > 0 {
		a, b = b, a
	}
	combined := append(a, b...) //nolint:gocritic // intentional append to new slice
	return keccak256(combined)
}

// Root returns the Merkle root hash, or nil if the tree is empty.
func (t *MerkleTree) Root() []byte {
	if len(t.levels) == 0 {
		return nil
	}
	top := t.levels[len(t.levels)-1]
	if len(top) == 0 {
		return nil
	}
	root := make([]byte, len(top[0]))
	copy(root, top[0])
	return root
}

// GetProof returns the Merkle proof (sibling hashes) for the leaf at the
// given index in the sorted leaf array. The proof can be verified on-chain
// using the OpenZeppelin MerkleProof.verify() function.
func (t *MerkleTree) GetProof(index int) [][]byte {
	if index < 0 || index >= len(t.leaves) {
		return nil
	}

	var proof [][]byte
	idx := index

	for lvl := 0; lvl < len(t.levels)-1; lvl++ {
		level := t.levels[lvl]
		var sibling []byte

		if idx%2 == 0 {
			// Right sibling (duplicate if at end).
			if idx+1 < len(level) {
				sibling = level[idx+1]
			} else {
				sibling = level[idx]
			}
		} else {
			// Left sibling.
			sibling = level[idx-1]
		}

		proof = append(proof, sibling)
		idx /= 2
	}

	return proof
}

// ---------------------------------------------------------------------------
// MerkleRoot — database record
// ---------------------------------------------------------------------------

// MerkleRoot is the stored record for a generated distribution Merkle root.
type MerkleRoot struct {
	ID          int64     `json:"id"`
	Root        []byte    `json:"root"`
	LeafCount   int       `json:"leaf_count"`
	GeneratedAt time.Time `json:"generated_at"`
	Proofs      []MerkleProofEntry
}

// MerkleProofEntry holds the proof data for one reward recipient.
type MerkleProofEntry struct {
	Index            int      `json:"index"`
	WalletAddr       string   `json:"wallet_addr"`
	CumulativeAmount *big.Int `json:"cumulative_amount"`
	Proof            [][]byte `json:"proof"`
}

// ---------------------------------------------------------------------------
// Generator — database-backed Merkle tree construction
// ---------------------------------------------------------------------------

// Generator pulls pending reward data from the database, builds the Merkle
// tree, and stores the root and proofs for use by the claim API.
type Generator struct {
	pool *pgxpool.Pool
}

// NewGenerator creates a Generator backed by the given connection pool.
func NewGenerator(pool *pgxpool.Pool) *Generator {
	return &Generator{pool: pool}
}

// pendingRewardRow represents one row from the claimable rewards query.
type pendingRewardRow struct {
	WalletAddr       string
	CumulativeAmount *big.Int
}

// GenerateMerkleTree queries all nodes with unclaimed rewards, builds the
// Merkle tree, persists the root and per-node proofs, and returns the root
// record. The tree covers cumulative (lifetime) claimable amounts so that
// recipients can claim once and the contract marks them as claimed.
func (g *Generator) GenerateMerkleTree(ctx context.Context) (*MerkleRoot, error) {
	// Fetch all pending (unclaimed) reward aggregates.
	rows, err := g.fetchPendingRewards(ctx)
	if err != nil {
		return nil, fmt.Errorf("merkle: fetch pending rewards: %w", err)
	}
	if len(rows) == 0 {
		return &MerkleRoot{GeneratedAt: time.Now()}, nil
	}

	// Build leaf hashes and track index → row mapping.
	leafHashes := make([][]byte, len(rows))
	for i, row := range rows {
		leafHashes[i] = HashLeaf(row.WalletAddr, row.CumulativeAmount)
	}

	tree := BuildTree(leafHashes)
	root := tree.Root()

	// Persist the Merkle root and proofs.
	mrID, err := g.persistRoot(ctx, root, len(rows))
	if err != nil {
		return nil, fmt.Errorf("merkle: persist root: %w", err)
	}

	// Build sorted leaf lookup for proof generation.
	// BuildTree sorts leaves, so we need to find each row's sorted index.
	sortedHashes := tree.leaves
	indexMap := make(map[string]int, len(sortedHashes)) // hex(hash) → sorted index
	for i, h := range sortedHashes {
		indexMap[hex.EncodeToString(h)] = i
	}

	proofEntries := make([]MerkleProofEntry, 0, len(rows))
	for _, row := range rows {
		leafHash := HashLeaf(row.WalletAddr, row.CumulativeAmount)
		sortedIdx, ok := indexMap[hex.EncodeToString(leafHash)]
		if !ok {
			continue
		}
		proof := tree.GetProof(sortedIdx)

		entry := MerkleProofEntry{
			Index:            sortedIdx,
			WalletAddr:       row.WalletAddr,
			CumulativeAmount: row.CumulativeAmount,
			Proof:            proof,
		}
		proofEntries = append(proofEntries, entry)

		if err := g.persistProof(ctx, mrID, entry); err != nil {
			return nil, fmt.Errorf("merkle: persist proof for %s: %w", row.WalletAddr, err)
		}
	}

	return &MerkleRoot{
		ID:          mrID,
		Root:        root,
		LeafCount:   len(rows),
		GeneratedAt: time.Now(),
		Proofs:      proofEntries,
	}, nil
}

// fetchPendingRewards queries the database for nodes with a positive
// claimable_rewards balance, returning the wallet address and cumulative
// amount in PRXY wei.
func (g *Generator) fetchPendingRewards(ctx context.Context) ([]pendingRewardRow, error) {
	// claimable_rewards is stored as a float (whole PRXY tokens); convert to
	// wei (× 1e18) for the on-chain uint256 representation.
	const q = `
		SELECT
			wallet_address,
			FLOOR(claimable_rewards * 1e18)::bigint AS cumulative_wei
		FROM nodes
		WHERE claimable_rewards > 0
		  AND wallet_address IS NOT NULL
		  AND wallet_address <> ''
		ORDER BY wallet_address
	`

	rows, err := g.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("merkle: query pending rewards: %w", err)
	}
	defer rows.Close()

	var result []pendingRewardRow
	for rows.Next() {
		var walletAddr string
		var cumulativeWei int64
		if err := rows.Scan(&walletAddr, &cumulativeWei); err != nil {
			return nil, fmt.Errorf("merkle: scan pending reward row: %w", err)
		}
		result = append(result, pendingRewardRow{
			WalletAddr:       walletAddr,
			CumulativeAmount: big.NewInt(cumulativeWei),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("merkle: iterate pending reward rows: %w", err)
	}
	return result, nil
}

// persistRoot inserts a new merkle_roots row and returns its generated ID.
func (g *Generator) persistRoot(ctx context.Context, root []byte, leafCount int) (int64, error) {
	const q = `
		INSERT INTO merkle_roots (root_hash, leaf_count, generated_at)
		VALUES ($1, $2, $3)
		RETURNING id
	`
	var id int64
	err := g.pool.QueryRow(ctx, q, root, leafCount, time.Now().UTC()).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("merkle: insert merkle_roots: %w", err)
	}
	return id, nil
}

// persistProof inserts a merkle_proofs row for one recipient.
func (g *Generator) persistProof(ctx context.Context, merkleRootID int64, entry MerkleProofEntry) error {
	// Serialise proof siblings as concatenated 32-byte hashes.
	proofBytes := encodeProof(entry.Proof)
	const q = `
		INSERT INTO merkle_proofs (
			merkle_root_id, leaf_index, wallet_address, cumulative_amount_wei, proof_bytes
		) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (merkle_root_id, wallet_address) DO UPDATE SET
			leaf_index            = EXCLUDED.leaf_index,
			cumulative_amount_wei = EXCLUDED.cumulative_amount_wei,
			proof_bytes           = EXCLUDED.proof_bytes
	`
	_, err := g.pool.Exec(ctx, q,
		merkleRootID,
		entry.Index,
		entry.WalletAddr,
		entry.CumulativeAmount.Bytes(),
		proofBytes,
	)
	if err != nil {
		return fmt.Errorf("merkle: insert proof for %s: %w", entry.WalletAddr, err)
	}
	return nil
}

// encodeProof flattens a [][]byte proof into a single byte slice by
// prepending each element with a 4-byte length, making it easy to decode
// on the way out without a fixed element size assumption.
func encodeProof(proof [][]byte) []byte {
	var buf bytes.Buffer
	for _, p := range proof {
		var lenBuf [4]byte
		binary.BigEndian.PutUint32(lenBuf[:], uint32(len(p)))
		buf.Write(lenBuf[:])
		buf.Write(p)
	}
	return buf.Bytes()
}
