package test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/beatoz/beatoz-sdk-go/vm"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/cryptobyte"
	"golang.org/x/crypto/cryptobyte/asn1"
)

func TestP256Verifier(t *testing.T) {
	bzweb3 := randBeatozWeb3()
	sender := randCommonWallet()
	require.NoError(t, sender.Unlock(defaultRpcNode.Pass), string(defaultRpcNode.Pass))
	require.NoError(t, sender.SyncAccount(bzweb3))

	// deploy P256Verifier
	contract, err := vm.NewEVMContract("./abi_p256_verifier.json")
	require.NoError(t, err)

	ret, err := contract.ExecCommit("", nil,
		sender, sender.GetNonce(), contractGas, defGasPrice, uint256.NewInt(0), bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, ret.CheckTx.Code, ret.CheckTx.Log)
	require.Equal(t, xerrors.ErrCodeSuccess, ret.DeliverTx.Code, ret.DeliverTx.Log)
	require.NotNil(t, contract.GetAddress())

	address := contract.GetAddress()
	fmt.Println("P256Verifier contract address", address)

	// P256 Signature
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	msgH := sha256.Sum256([]byte("hello world"))
	sigDER, err := privKey.Sign(rand.Reader, msgH[:], nil)
	require.NoError(t, err)
	r, s, err := parseDERSignature(sigDER)
	require.NoError(t, err)

	// call P256Verifier.verify
	//   1. success
	require.NoError(t, sender.SyncAccount(bzweb3))
	ret, err = contract.ExecCommit(
		"verify",
		[]interface{}{
			msgH,
			[32]byte(common.LeftPadBytes(r.Bytes(), 32)),
			[32]byte(common.LeftPadBytes(s.Bytes(), 32)),
			[32]byte(common.LeftPadBytes(privKey.PublicKey.X.Bytes(), 32)),
			[32]byte(common.LeftPadBytes(privKey.PublicKey.Y.Bytes(), 32)),
		},
		sender,
		sender.GetNonce(),
		contractGas,
		defGasPrice,
		uint256.NewInt(0),
		bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, ret.CheckTx.Code, ret.CheckTx.Log)
	require.Equal(t, xerrors.ErrCodeSuccess, ret.DeliverTx.Code, ret.DeliverTx.Log)

	//   1. wrong message
	wrongMsgH := sha256.Sum256([]byte("wrong message"))
	require.NoError(t, sender.SyncAccount(bzweb3))
	ret, err = contract.ExecCommit(
		"verify",
		[]interface{}{
			wrongMsgH,
			[32]byte(common.LeftPadBytes(r.Bytes(), 32)),
			[32]byte(common.LeftPadBytes(s.Bytes(), 32)),
			[32]byte(common.LeftPadBytes(privKey.PublicKey.X.Bytes(), 32)),
			[32]byte(common.LeftPadBytes(privKey.PublicKey.Y.Bytes(), 32)),
		},
		sender,
		sender.GetNonce(),
		contractGas,
		defGasPrice,
		uint256.NewInt(0),
		bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, ret.CheckTx.Code, ret.CheckTx.Log)
	require.NotEqual(t, xerrors.ErrCodeSuccess, ret.DeliverTx.Code, ret.DeliverTx.Log)
	require.Contains(t, ret.DeliverTx.Log, "Verification failed")
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
