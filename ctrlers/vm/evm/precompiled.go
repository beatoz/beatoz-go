package evm

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

func init() {
	// P256(secp256r1) signature verification
	vm.PrecompiledContractsHomestead[common.BytesToAddress([]byte{0x1, 0x00})] = &beatoz_p256Verify{}
	vm.PrecompiledContractsByzantium[common.BytesToAddress([]byte{0x1, 0x00})] = &beatoz_p256Verify{}
	vm.PrecompiledContractsIstanbul[common.BytesToAddress([]byte{0x1, 0x00})] = &beatoz_p256Verify{}
	vm.PrecompiledContractsBerlin[common.BytesToAddress([]byte{0x1, 0x00})] = &beatoz_p256Verify{}
}

const (
	P256VerifyGas         = 6900
	P256VerifyInputLength = 160
)

var (
	true32Byte = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
)

// P256VERIFY (secp256r1 signature verification)
// implemented as a native contract
type beatoz_p256Verify struct{}

// RequiredGas returns the gas required to execute the precompiled contract
func (c *beatoz_p256Verify) RequiredGas(input []byte) uint64 {
	return P256VerifyGas
}

// Run executes the precompiled contract with given 160 bytes of param, returning the output and the used gas
func (c *beatoz_p256Verify) Run(input []byte) ([]byte, error) {
	if len(input) != P256VerifyInputLength {
		return nil, errors.New("p256:invalid input length")
	}

	// Extract hash, r, s, x, y from the input.
	hash := input[0:32]
	r, s := new(big.Int).SetBytes(input[32:64]), new(big.Int).SetBytes(input[64:96])
	x, y := new(big.Int).SetBytes(input[96:128]), new(big.Int).SetBytes(input[128:160])

	// Verify the signature.
	if x == nil || y == nil || !elliptic.P256().IsOnCurve(x, y) {
		return nil, errors.New("p256:invalid public key")
	}
	pk := &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}
	if ecdsa.Verify(pk, hash, r, s) {
		return true32Byte, nil
	}

	return nil, errors.New("p256: failed verification")
}

func (c *beatoz_p256Verify) Name() string {
	return "P256VERIFY"
}
