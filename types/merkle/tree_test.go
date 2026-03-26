package merkle

import (
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func hash(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

// TestNewMerkleTree_RawData tests tree construction with non-32-byte data (auto hashing).
func TestNewMerkleTree_RawData(t *testing.T) {
	leaves := [][]byte{
		[]byte("alice"),
		[]byte("bob"),
		[]byte("charlie"),
		[]byte("dave"),
	}
	tree := NewMerkleTree(WithRawLeaves(leaves))
	require.NotNil(t, tree.Root(), "root should not be nil")
	require.Equal(t, sha256.Size, len(tree.Root()), fmt.Sprintf("root should be %d bytes, got %d", sha256.Size, len(tree.Root())))
}

// TestNewMerkleTree_PreHashed tests tree construction with pre-hashed 32-byte leaves (no rehashing).
func TestNewMerkleTree_PreHashed(t *testing.T) {
	hashedLeaves := [][]byte{
		hash([]byte("alice")),
		hash([]byte("bob")),
		hash([]byte("charlie")),
		hash([]byte("dave")),
	}
	tree := NewMerkleTree(WithHashedLeaves(hashedLeaves))

	// Manually compute expected root
	n1 := hashPair(hashedLeaves[0], hashedLeaves[1])
	n2 := hashPair(hashedLeaves[2], hashedLeaves[3])
	expectedRoot := hashPair(n1, n2)

	require.Equal(t, expectedRoot, tree.Root(), fmt.Sprintf("root mismatch:\n  got:  %x\n  want: %x", tree.Root(), expectedRoot))
}

// TestNewMerkleTree_RawAndPreHashed ensures raw data tree and pre-hashed tree produce the same root.
func TestNewMerkleTree_RawAndPreHashed(t *testing.T) {
	leaves := [][]byte{
		[]byte("alice"),
		[]byte("bob"),
		[]byte("charlie"),
		[]byte("dave"),
	}
	hashedLeaves := make([][]byte, len(leaves))
	for i, l := range leaves {
		hashedLeaves[i] = hash(l)
	}

	rawTree := NewMerkleTree(WithRawLeaves(leaves))
	hashedTree := NewMerkleTree(WithHashedLeaves(hashedLeaves))

	require.Equal(t, rawTree.Root(), hashedTree.Root(), fmt.Sprintf("root should be identical:\n  raw:    %x\n  hashed: %x", rawTree.Root(), hashedTree.Root()))
}

// TestNewMerkleTree_PaddingToPowerOf2 tests that non-power-of-2 leaves are padded.
func TestNewMerkleTree_PaddingToPowerOf2(t *testing.T) {
	hashedLeaves := [][]byte{
		hash([]byte("a")),
		hash([]byte("b")),
		hash([]byte("c")),
	}
	tree := NewMerkleTree(WithHashedLeaves(hashedLeaves))
	require.Equal(t, 4, tree.leafCount)
	// 4th leaf should be nil (padding)
	require.Nil(t, tree.nodes[tree.leafCount+3], "padding leaf should be nil")
	require.NotNil(t, tree.Root(), "root should not be nil")
}

// TestNewMerkleTree_SingleLeaf tests tree with a single leaf.
// With leafCount=1, nodes=[unused, leaf]. No internal node computation,
// so root = leaf itself.
func TestNewMerkleTree_SingleLeaf(t *testing.T) {
	leaf := []byte("only")
	tree := NewMerkleTree(WithRawLeaves([][]byte{leaf}))
	require.Equal(t, 1, tree.leafCount, "leafCount should be 1")
	require.Equal(t, tree.Root(), hash(leaf), "root should equal the hash(single leaf)")

	hashedLeaf := hash([]byte("only"))
	tree = NewMerkleTree(WithHashedLeaves([][]byte{hashedLeaf}))
	require.Equal(t, 1, tree.leafCount, "leafCount should be 1")
	require.Equal(t, tree.Root(), hashedLeaf, "root should equal the single 32B hashed leaf")
}

// TestProof_Valid tests that proof generation and verification work for every leaf.
func TestProof_Valid(t *testing.T) {
	rawLeaves := [][]byte{
		[]byte("tx1"),
		[]byte("tx2"),
		[]byte("tx3"),
		[]byte("tx4"),
	}
	tree := NewMerkleTree(WithRawLeaves(rawLeaves))

	for i, data := range rawLeaves {
		_, proof, err := tree.Proof(i)
		require.NoError(t, err)
		require.NoError(t, VerifyProof(i, data, proof, tree.Root()))
	}
}

// TestProof_PreHashed tests proof with pre-hashed leaves.
func TestProof_PreHashed(t *testing.T) {
	hashedLeaves := [][]byte{
		hash([]byte("tx1")),
		hash([]byte("tx2")),
		hash([]byte("tx3")),
		hash([]byte("tx4")),
	}
	tree := NewMerkleTree(WithHashedLeaves(hashedLeaves))

	for i, data := range hashedLeaves {
		_, proof, err := tree.Proof(i)
		require.NoError(t, err)
		require.NoError(t, VerifyProof(i, data, proof, tree.Root(), true))
	}
}

// TestProof_OutOfRange tests that Proof returns error for invalid indices.
func TestProof_OutOfRange(t *testing.T) {
	tree := NewMerkleTree(WithHashedLeaves([][]byte{hash([]byte("a")), hash([]byte("b"))}))
	_, _, err := tree.Proof(-1)
	require.Error(t, err, "expected error for negative index")
	_, _, err = tree.Proof(2)
	require.Error(t, err, "expected error for out of range index")
}

// TestVerify_WrongData tests that verification fails with incorrect data.
func TestVerify_WrongData(t *testing.T) {
	leaves := [][]byte{[]byte("tx1"), []byte("tx2"), []byte("tx3"), []byte("tx4")}
	tree := NewMerkleTree(WithRawLeaves(leaves))
	_, proof, err := tree.Proof(0)
	require.NoError(t, err)
	require.Error(t, VerifyProof(0, []byte("fake"), proof, tree.Root()), "VerifyMerkleProof should fail with wrong data")
}

// TestVerify_WrongIndex tests that verification fails with incorrect index.
func TestVerify_WrongIndex(t *testing.T) {
	leaves := [][]byte{[]byte("tx1"), []byte("tx2"), []byte("tx3"), []byte("tx4")}
	tree := NewMerkleTree(WithRawLeaves(leaves))
	_, proof, err := tree.Proof(0)
	require.NoError(t, err)

	// Use correct data but wrong index
	require.Error(t, VerifyProof(1, []byte("tx1"), proof, tree.Root()), "VerifyMerkleProof should fail with wrong index")
}

// TestVerify_WrongRoot tests that verification fails with a different root.
func TestVerify_WrongRoot(t *testing.T) {
	leaves := [][]byte{[]byte("tx1"), []byte("tx2"), []byte("tx3"), []byte("tx4")}
	tree := NewMerkleTree(WithRawLeaves(leaves))
	_, proof, err := tree.Proof(0)
	require.NoError(t, err)

	fakeRoot := hash([]byte("fake root"))
	require.Error(t, VerifyProof(0, []byte("tx1"), proof, fakeRoot), "VerifyMerkleProof should fail with wrong root")
}

// TestVerify_TamperedProof tests that verification fails with a modified proof element.
func TestVerify_TamperedProof(t *testing.T) {
	leaves := [][]byte{[]byte("tx1"), []byte("tx2"), []byte("tx3"), []byte("tx4")}
	tree := NewMerkleTree(WithRawLeaves(leaves))
	_, proof, err := tree.Proof(0)
	require.NoError(t, err)

	// Tamper with the first sibling in the proof
	tampered := make([][]byte, len(proof))
	copy(tampered, proof)
	tampered[0] = hash([]byte("tampered"))

	require.Error(t, VerifyProof(0, []byte("tx1"), tampered, tree.Root()), "VerifyMerkleProof should fail with tampered proof")
}

// TestSecurity_InternalNodeAsLeaf tests that even though an attacker can build a shorter tree
// with the same root (no domain separation), verification with index+original data prevents forgery.
func TestSecurity_InternalNodeAsLeaf(t *testing.T) {
	hashedLeaves := [][]byte{
		hash([]byte("A")),
		hash([]byte("B")),
		hash([]byte("C")),
		hash([]byte("D")),
	}
	tree := NewMerkleTree(WithHashedLeaves(hashedLeaves))
	root := tree.Root()

	// Attacker builds a shorter tree using internal nodes as leaves
	n1 := hashPair(hashedLeaves[0], hashedLeaves[1])
	n2 := hashPair(hashedLeaves[2], hashedLeaves[3])
	forgedTree := NewMerkleTree(WithHashedLeaves([][]byte{n1, n2}))

	// Without domain separation, the roots ARE equal (known property)
	require.Equal(t, forgedTree.Root(), root, "expected same root without domain separation")

	// But: attacker cannot forge a valid proof for original leaf data.
	// Attacker tries to prove leaf[0] (H("A")) exists in the forged tree.
	_, forgedProof, err := forgedTree.Proof(0)
	require.NoError(t, err)
	require.Error(t, VerifyProof(0, hashedLeaves[0], forgedProof, root, true), "VULNERABLE: forged proof accepted for original leaf data")
}

// TestSecurity_ConcatenatedLeavesAsLeaf tests that concatenating two leaves
// into one cannot forge a valid proof.
func TestSecurity_ConcatenatedLeavesAsLeaf(t *testing.T) {
	leaves := [][]byte{[]byte("tx1"), []byte("tx2"), []byte("tx3"), []byte("tx4")}
	tree := NewMerkleTree(WithRawLeaves(leaves))
	root := tree.Root()

	// Attacker concatenates tx1 and tx2 into a single leaf
	concat := append([]byte("tx1"), []byte("tx2")...)
	forgedLeaves := [][]byte{concat, []byte("tx3"), []byte("tx4")}
	forgedTree := NewMerkleTree(WithRawLeaves(forgedLeaves))

	require.NotEqual(t, forgedTree.Root(), root, "VULNERABLE: concatenated leaves produce the same root")
}

// TestSecurity_ForgedProofWithInternalNode tests that an attacker cannot
// construct a valid proof by submitting an internal node as data.
func TestSecurity_ForgedProofWithInternalNode(t *testing.T) {
	hashedLeaves := [][]byte{
		hash([]byte("A")),
		hash([]byte("B")),
		hash([]byte("C")),
		hash([]byte("D")),
	}
	tree := NewMerkleTree(WithHashedLeaves(hashedLeaves))
	root := tree.Root()

	// Attacker knows internal node N1 = H(A||B) and tries to verify it as leaf at index 0
	n1 := hashPair(hashedLeaves[0], hashedLeaves[1])
	_, proof, err := tree.Proof(0)
	require.NoError(t, err)
	require.Error(t, VerifyProof(0, n1, proof, root, true), "VULNERABLE: internal node accepted as valid leaf data")
}

// TestSecurity_ShorterTreeSameRoot verifies that even though a shorter tree can have the same root
// (no domain separation), an attacker cannot use the shorter tree's proof to verify original leaves.
func TestSecurity_ShorterTreeSameRoot(t *testing.T) {
	hashedLeaves := [][]byte{
		hash([]byte("A")),
		hash([]byte("B")),
		hash([]byte("C")),
		hash([]byte("D")),
	}
	tree4 := NewMerkleTree(WithHashedLeaves(hashedLeaves))
	root := tree4.Root()

	// Shorter tree with internal nodes as leaves — same root (known property)
	n1 := hashPair(hashedLeaves[0], hashedLeaves[1])
	n2 := hashPair(hashedLeaves[2], hashedLeaves[3])
	tree2 := NewMerkleTree(WithHashedLeaves([][]byte{n1, n2}))
	require.Equal(t, tree2.Root(), root, "expected same root without domain separation")

	// Attacker tries to prove original leaves using the shorter tree's proofs
	for i, data := range hashedLeaves {
		_, forgedProof, err := tree2.Proof(i % tree2.leafCount)
		require.NoError(t, err)
		require.Error(t, VerifyProof(i%tree2.leafCount, data, forgedProof, root, true), fmt.Errorf("VULNERABLE: shorter tree proof accepted for original hashedLeaf[%d]", i))
	}
}

// TestSubtreeComposition tests that subtree roots can be composed into a parent tree
// without double hashing, and verification still works end-to-end.
func TestSubtreeComposition(t *testing.T) {
	// Build two subtrees from raw data
	sub1Leaves := [][]byte{[]byte("a"), []byte("b")}
	sub2Leaves := [][]byte{[]byte("c"), []byte("d")}
	sub1 := NewMerkleTree(WithRawLeaves(sub1Leaves))
	sub2 := NewMerkleTree(WithRawLeaves(sub2Leaves))

	// Compose parent tree from subtree roots (already 32 bytes, no rehashing)
	parentTree := NewMerkleTree(WithHashedLeaves([][]byte{sub1.Root(), sub2.Root()}))
	require.NotNil(t, parentTree.Root(), "parent tree should not be nil")

	// VerifyMerkleProof subtree root proof in parent tree
	_, proof, err := parentTree.Proof(0)
	require.NoError(t, err)
	require.NoError(t, VerifyProof(0, sub1.Root(), proof, parentTree.Root(), true), "subtree root should be verifiable in parent tree")
}

func TestNextPowerOf2(t *testing.T) {
	tests := []struct {
		input, expected int
	}{
		{0, 1}, {1, 1}, {2, 2}, {3, 4}, {4, 4}, {5, 8}, {7, 8}, {8, 8}, {9, 16},
	}
	for _, tc := range tests {
		got := nextPowerOf2(tc.input)
		require.Equal(t, tc.expected, got, fmt.Sprintf("nextPowerOf2(%d) = %d, want %d", tc.input, got, tc.expected))
	}
}
