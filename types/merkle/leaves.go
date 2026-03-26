package merkle

type IMerkleProvable interface {
	Root() []byte
	Proof(int) ([]byte, [][]byte, error)
	VerifyProof(int, [][]byte) error
}

type ILeaves interface {
	Leaf(int) []byte
	Leaves() [][]byte
	LeavesLen() int
}
