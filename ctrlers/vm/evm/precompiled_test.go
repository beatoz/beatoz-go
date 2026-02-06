package evm

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"

	"golang.org/x/crypto/cryptobyte"
	"golang.org/x/crypto/cryptobyte/asn1"
)

func TestP256Verify(t *testing.T) {
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	msgH := sha256.Sum256([]byte("hello world"))

	// Sign the hash
	sigDER, err := privKey.Sign(rand.Reader, msgH[:], nil)
	require.NoError(t, err)

	// Parse DER signature to get R and S
	r, s, err := parseDERSignature(sigDER)
	require.NoError(t, err)

	contractInput := make([]byte, P256VerifyInputLength)
	copy(contractInput[0:32], msgH[:])
	copy(contractInput[32:64], common.LeftPadBytes(r.Bytes(), 32))
	copy(contractInput[64:96], common.LeftPadBytes(s.Bytes(), 32))
	copy(contractInput[96:128], common.LeftPadBytes(privKey.PublicKey.X.Bytes(), 32))
	copy(contractInput[128:160], common.LeftPadBytes(privKey.PublicKey.Y.Bytes(), 32))

	// ecr := &beatoz_p256Verify{}
	ecr := vm.PrecompiledContractsHomestead[common.BytesToAddress([]byte{0x1, 0x00})]

	ret, err := ecr.Run(contractInput)
	require.NoError(t, err)
	require.Equal(t, true32Byte, ret)

	// invalid length
	ret, err = ecr.Run(contractInput[:159])
	require.NotEqual(t, true32Byte, ret)
	require.ErrorContains(t, err, "invalid input length")

	// wrong pubKey
	wrongKey := privKey.PublicKey.X.Bytes()
	wrongKey[0] = 0xff
	copy(contractInput[0:32], msgH[:])
	copy(contractInput[32:64], common.LeftPadBytes(r.Bytes(), 32))
	copy(contractInput[64:96], common.LeftPadBytes(s.Bytes(), 32))
	copy(contractInput[96:128], common.LeftPadBytes(wrongKey, 32))
	copy(contractInput[128:160], common.LeftPadBytes(privKey.PublicKey.Y.Bytes(), 32))
	ret, err = ecr.Run(contractInput)
	require.NotEqual(t, true32Byte, ret)
	require.ErrorContains(t, err, "p256:invalid public key")

	// wrong message
	wrongMsgH := sha256.Sum256([]byte("wrong msg"))
	copy(contractInput[0:32], wrongMsgH[:])
	copy(contractInput[32:64], common.LeftPadBytes(r.Bytes(), 32))
	copy(contractInput[64:96], common.LeftPadBytes(s.Bytes(), 32))
	copy(contractInput[96:128], common.LeftPadBytes(privKey.PublicKey.X.Bytes(), 32))
	copy(contractInput[128:160], common.LeftPadBytes(privKey.PublicKey.Y.Bytes(), 32))

	ret, err = ecr.Run(contractInput)
	require.NotEqual(t, true32Byte, ret)
}

func parseDERSignature(sigDER []byte) (*big.Int, *big.Int, error) {
	var (
		r, s  = new(big.Int), new(big.Int)
		inner cryptobyte.String
	)
	input := cryptobyte.String(sigDER)
	if !input.ReadASN1(&inner, asn1.SEQUENCE) ||
		!input.Empty() ||
		!inner.ReadASN1Integer(r) ||
		!inner.ReadASN1Integer(s) ||
		!inner.Empty() {
		return nil, nil, errors.New("invalid DER signature")
	}
	return r, s, nil
}
