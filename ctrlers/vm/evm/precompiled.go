package evm

import (
	beatoz_crypto "github.com/beatoz/beatoz-go/types/crypto"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"math/big"
)

func init() {
	vm.PrecompiledContractsHomestead[common.BytesToAddress([]byte{1})] = &beatoz_ecrecover{}
	vm.PrecompiledContractsByzantium[common.BytesToAddress([]byte{1})] = &beatoz_ecrecover{}
	vm.PrecompiledContractsIstanbul[common.BytesToAddress([]byte{1})] = &beatoz_ecrecover{}
	vm.PrecompiledContractsBerlin[common.BytesToAddress([]byte{1})] = &beatoz_ecrecover{}
}

// ECRECOVER implemented as a native contract.
type beatoz_ecrecover struct{}

func (c *beatoz_ecrecover) RequiredGas(input []byte) uint64 {
	return params.EcrecoverGas
}

func (c *beatoz_ecrecover) Run(input []byte) ([]byte, error) {
	const ecRecoverInputLength = 128

	input = common.RightPadBytes(input, ecRecoverInputLength)
	// "input" is (hash, v, r, s), each 32 bytes
	// but for ecrecover we want (r, s, v)

	r := new(big.Int).SetBytes(input[64:96])
	s := new(big.Int).SetBytes(input[96:128])
	v := input[63] // - 27 : it's only for ethereum.

	// tighter sig s values input homestead only apply to tx sigs
	if !allZero(input[32:63]) || !crypto.ValidateSignatureValues(v, r, s, false) {
		return nil, nil
	}
	// We must make sure not to modify the 'input', so placing the 'v' along with
	// the signature needs to be done on a new allocation
	sig := make([]byte, 65)
	copy(sig, input[64:128])
	sig[64] = v
	// v needs to be at the end for libsecp256k1
	publicKey, err := crypto.SigToPub(input[:32], sig)
	if err != nil {
		return nil, err
	}

	return common.LeftPadBytes(beatoz_crypto.Pub2Addr(publicKey), 32), nil
}

func allZero(b []byte) bool {
	for _, b0 := range b {
		if b0 != 0 {
			return false
		}
	}
	return true
}
