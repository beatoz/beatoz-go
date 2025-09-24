package types_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/crypto"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	ctrtypes "github.com/beatoz/beatoz-go/ctrlers/types"
)

func TestSignerV0_Recover(t *testing.T) {
	senderWal := web3.NewWallet([]byte("1"))
	require.NoError(t, senderWal.Unlock([]byte("1")))
	payerWal := web3.NewWallet([]byte("1"))
	require.NoError(t, payerWal.Unlock([]byte("1")))

	chainId := "0xabcd"
	tx := &ctrtypes.Trx{
		Version:  1,
		Time:     time.Now().UnixNano(),
		Nonce:    rand.Int63(),
		From:     senderWal.Address(),
		To:       types.RandAddress(),
		Amount:   bytes.RandU256Int(),
		Gas:      rand.Int63(),
		GasPrice: uint256.NewInt(rand.Uint64()),
		Type:     ctrtypes.TRX_TRANSFER,
		Payer:    payerWal.Address(),
	}

	// for Sender
	_, preimg0, err := senderWal.SignTrxRLP(tx, chainId)
	require.NoError(t, err)

	addr0, pubKey0, err := ctrtypes.VerifyTrxRLP(tx, chainId)
	require.NoError(t, err)
	require.Equal(t, senderWal.Address(), addr0)
	require.Equal(t, senderWal.GetPubKey(), pubKey0)

	// test for signer
	signer := ctrtypes.NewSignerV0(chainId)
	preimag1, err := signer.GetPreimageSender(tx)
	require.NoError(t, err)
	require.Equal(t, preimg0, preimag1)

	addr1, pubKey1, err := signer.VerifySender(tx)
	require.NoError(t, err)
	require.Equal(t, addr0, addr1)
	require.Equal(t, pubKey0, pubKey1)

	// for Payer
	_, preimg0, err = payerWal.SignPayerTrxRLP(tx, chainId)
	require.NoError(t, err)

	addr0, pubKey0, err = ctrtypes.VerifyPayerTrxRLP(tx, chainId)
	require.NoError(t, err)
	require.Equal(t, payerWal.Address(), addr0)
	require.Equal(t, payerWal.GetPubKey(), pubKey0)

	// test for signer
	preimag1, err = signer.GetPreimagePayer(tx)
	require.NoError(t, err)
	require.Equal(t, preimg0, preimag1)

	addr1, pubKey1, err = signer.VerifyPayer(tx)
	require.NoError(t, err)
	require.Equal(t, addr0, addr1)
	require.Equal(t, pubKey0, pubKey1)
}

func TestSignerV1_Recover(t *testing.T) {
	senderPrvBz, senderPubBz0 := crypto.NewKeypairBytes()
	senderWal := web3.ImportKey(senderPrvBz, []byte("1"))
	require.NoError(t, senderWal.Unlock([]byte("1")))

	payerPrvBz, payerPubBz0 := crypto.NewKeypairBytes()
	payerWal := web3.ImportKey(payerPrvBz, []byte("1"))
	require.NoError(t, payerWal.Unlock([]byte("1")))

	chainId := "0xabcd"
	tx := &ctrtypes.Trx{
		Version:  1,
		Time:     time.Now().UnixNano(),
		Nonce:    rand.Int63(),
		From:     senderWal.Address(),
		To:       types.RandAddress(),
		Amount:   bytes.RandU256Int(),
		Gas:      rand.Int63(),
		GasPrice: uint256.NewInt(rand.Uint64()),
		Type:     ctrtypes.TRX_TRANSFER,
		Payer:    payerWal.Address(),
	}

	//
	signer := ctrtypes.NewSignerV1(chainId)

	// for Sender
	_, err := signer.SignSender(tx, senderPrvBz)
	require.NoError(t, err)

	addr, pubBytes1, err := signer.VerifySender(tx)
	require.NoError(t, err)
	require.Equal(t, senderWal.Address(), addr)
	require.Equal(t, senderPubBz0, pubBytes1)

	// for Payer
	_, err = signer.SignPayer(tx, payerPrvBz)
	require.NoError(t, err)

	addr, pubBytes1, err = signer.VerifyPayer(tx)
	require.NoError(t, err)
	require.Equal(t, payerWal.Address(), addr)
	require.Equal(t, payerPubBz0, pubBytes1)

	// fail expected
	tx.PayerSig[64] -= 27
	addr, pubBytes1, err = signer.VerifyPayer(tx)
	require.Error(t, err)
}
