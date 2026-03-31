package merkle

import (
	"crypto/sha256"
	"fmt"
	"math/bits"
)

// MerkleTree is an array-based complete binary tree.
// nodes is 1-indexed: nodes[1] = root, children of nodes[i] = nodes[2i], nodes[2i+1].
// Leaves occupy indices [leafCount, 2*leafCount-1].
type MerkleTree struct {
	nodes     [][]byte
	leafCount int
}

type OptFunc func() ([][]byte, bool)

func WithILeaves(leaves ILeaves) OptFunc {
	return func() ([][]byte, bool) {
		return leaves.Leaves(), false
	}
}

func WithRawLeaves(leaves [][]byte) OptFunc {
	return func() ([][]byte, bool) {
		return leaves, false
	}
}

func WithHashedLeaves(leaves [][]byte) OptFunc {
	return func() ([][]byte, bool) {
		return leaves, true
	}
}

func NewMerkleTree(opt OptFunc) *MerkleTree {
	leaves, preHashed := opt()
	return newMerkleTree(leaves, preHashed)
}

func newMerkleTree(leaves [][]byte, preHashed bool) *MerkleTree {
	leafCount := nextPowerOf2(len(leaves))
	tree := &MerkleTree{
		nodes:     make([][]byte, leafCount*2), // 1-indexed, nodes[0] is unused
		leafCount: leafCount,
	}

	// populate leaves
	for i, leaf := range leaves {
		if preHashed {
			tree.nodes[leafCount+i] = leaf
		} else {
			h := sha256.Sum256(leaf)
			tree.nodes[leafCount+i] = h[:]
		}
	}
	// remaining leaf slots are nil

	// build internal nodes from bottom up
	for i := leafCount - 1; i >= 1; i-- {
		left := tree.nodes[2*i]
		right := tree.nodes[2*i+1]
		tree.nodes[i] = hashPair(left, right)
	}

	return tree
}

// Root returns the merkle root hash.
func (t *MerkleTree) Root() []byte {
	return t.nodes[1]
}

// Proof returns the sibling hashes needed to verify the leaf at the given index.
// The proof is ordered from leaf level to root level.
func (t *MerkleTree) Proof(index int) ([]byte, [][]byte, error) {
	if index < 0 || index >= t.leafCount {
		return nil, nil, fmt.Errorf("index %d out of range [0, %d)", index, t.leafCount)
	}

	var proof [][]byte
	nodeIdx := t.leafCount + index
	for nodeIdx > 1 {
		// sibling is the XOR toggle of the last bit
		siblingIdx := nodeIdx ^ 1
		proof = append(proof, t.nodes[siblingIdx])
		nodeIdx /= 2 // move to parent
	}
	return t.nodes[t.leafCount+index], proof, nil
}

// VerifyProof verifies that data at the given index is part of the tree with the given root.
// If preHashed is true, data is used as-is; otherwise it is hashed first.
func VerifyProof(index int, data []byte, siblings [][]byte, root []byte, preHashed ...bool) error {
	var leafHash []byte
	if len(preHashed) > 0 && preHashed[0] {
		leafHash = data
	} else {
		h := sha256.Sum256(data)
		leafHash = h[:]
	}

	current := leafHash
	nodeIdx := index
	for _, sibling := range siblings {
		if nodeIdx%2 == 0 { // current is left child
			current = hashPair(current, sibling)
		} else { // current is right child
			current = hashPair(sibling, current)
		}
		nodeIdx /= 2
	}

	if len(current) != len(root) {
		return fmt.Errorf("length mismatch; expected %d, got %d", len(current), len(root))
	}
	for i := range current {
		if current[i] != root[i] {
			return fmt.Errorf("hash mismatch; expected %x, got %x", current, root)
		}
	}
	return nil
}

func hashPair(left, right []byte) []byte {
	if left == nil && right == nil {
		return nil
	}
	var buf []byte
	if left != nil {
		buf = append(buf, left...)
	}
	if right != nil {
		buf = append(buf, right...)
	}
	h := sha256.Sum256(buf)
	return h[:]
}

func nextPowerOf2(n int) int {
	if n <= 1 {
		return 1
	}
	return 1 << bits.Len(uint(n-1))
}
