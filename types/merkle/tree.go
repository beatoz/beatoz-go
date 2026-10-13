package merkle

import (
	"crypto/sha256"
	"fmt"
	"math/bits"
)

const (
	LEAF_PREFIX  byte = 0x00
	INNER_PREFIX byte = 0x01

	// SHA-256 of empty input (padding) and of 0x00 (a nil or empty leaf).
	EMPTY_HASH = "\xe3\xb0\xc4\x42\x98\xfc\x1c\x14\x9a\xfb\xf4\xc8\x99\x6f\xb9\x24\x27\xae\x41\xe4\x64\x9b\x93\x4c\xa4\x95\x99\x1b\x78\x52\xb8\x55"
	NIL_HASH   = "\x6e\x34\x0b\x9c\xff\xb3\x7a\x98\x9c\xa5\x44\xe6\xbb\x78\x0a\x2c\x78\x90\x1d\x3f\xb3\x37\x38\x76\x85\x11\xa3\x06\x17\xaf\xa0\x1d"
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

// NewMerkleTree uses legacy hashing unless the optional btip48 argument is true.
// BTIP48 treats every input as raw leaf data, even with WithHashedLeaves.
func NewMerkleTree(opt OptFunc, btip48 ...bool) *MerkleTree {
	leaves, preHashed := opt()
	// The variadic argument is a slice: omitted or false selects legacy hashing.
	// Check its length before accessing [0]; only an explicit true enables BTIP48.
	if len(btip48) > 0 && btip48[0] {
		return newMerkleTreeBTIP48(leaves)
	}
	return newMerkleTree(leaves, preHashed)
}

func newMerkleTreeBTIP48(leaves [][]byte) *MerkleTree {
	leafCount := nextPowerOf2(len(leaves))
	tree := &MerkleTree{
		nodes:     make([][]byte, leafCount*2),
		leafCount: leafCount,
	}

	for i, leaf := range leaves {
		tree.nodes[leafCount+i] = leafHashBTIP48(leaf)
	}

	for i := len(leaves); i < leafCount; i++ {
		tree.nodes[leafCount+i] = []byte(EMPTY_HASH)
	}

	for i := leafCount - 1; i >= 1; i-- {
		tree.nodes[i] = innerHashBTIP48(tree.nodes[2*i], tree.nodes[2*i+1])
	}

	return tree
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

func leafHashBTIP48(data []byte) []byte {
	if len(data) == 0 {
		return []byte(NIL_HASH)
	}

	buf := make([]byte, len(data)+1)
	buf[0] = LEAF_PREFIX
	copy(buf[1:], data)

	h := sha256.Sum256(buf)
	return h[:]
}

func innerHashBTIP48(left, right []byte) []byte {
	buf := make([]byte, 1, 1+len(left)+len(right))
	buf[0] = INNER_PREFIX
	buf = append(buf, left...)
	buf = append(buf, right...)

	h := sha256.Sum256(buf)
	return h[:]
}

func nextPowerOf2(n int) int {
	if n <= 1 {
		return 1
	}
	return 1 << bits.Len(uint(n-1))
}
