package types_test

import (
	types2 "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	"io"
	"math/rand"
	"testing"
	"time"
)

func TestTrxEncode(t *testing.T) {
	tx0 := &types2.Trx{
		Version:  1,
		Time:     time.Now().UnixNano(),
		Nonce:    rand.Int63(),
		From:     types.RandAddress(),
		To:       types.RandAddress(),
		Amount:   bytes.RandU256Int(),
		Gas:      rand.Int63(),
		GasPrice: uint256.NewInt(rand.Uint64()),
		Type:     types2.TRX_TRANSFER,
	}
	require.Equal(t, types2.TRX_TRANSFER, tx0.GetType())

	bzTx0, err := tx0.Encode()
	require.NoError(t, err)

	tx1 := &types2.Trx{}
	err = tx1.Decode(bzTx0)
	require.NoError(t, err)
	require.Equal(t, types2.TRX_TRANSFER, tx1.GetType())
	require.Equal(t, tx0, tx1)

	bzTx1, err := tx1.Encode()
	require.NoError(t, err)
	require.Equal(t, bzTx0, bzTx1)
}

func TestTrxDecodeWithMaliciousPayload(t *testing.T) {
	// The malicious users can exploit this by submitting a large number of TRX_TRANSFER and TRX_STAKING transactions,
	// each with a large payload but under 1MB in size.
	// In beatoz, when decoding tx, the mempool may be exhausted
	// because it did not check the payload for TRX_TRANSFER and TRX_STAKE, which have no payload.
	tx0 := &types2.Trx{
		Version:  1,
		Time:     time.Now().UnixNano(),
		Nonce:    rand.Int63(),
		From:     types.RandAddress(),
		To:       types.RandAddress(),
		Amount:   bytes.RandU256Int(),
		Gas:      rand.Int63(),
		GasPrice: uint256.NewInt(rand.Uint64()),
		Type:     types2.TRX_TRANSFER,
		Payload: &maliciousPayload{
			dummy: bytes.RandBytes(1024 * 900),
		},
	}
	require.Equal(t, types2.TRX_TRANSFER, tx0.GetType())

	bzTx0, err := tx0.Encode()
	require.NoError(t, err)

	tx1 := &types2.Trx{}
	err = tx1.Decode(bzTx0)
	require.Error(t, err)
}

func TestRLP_TrxPayloadContract(t *testing.T) {
	w := web3.NewWallet([]byte("1"))
	require.NoError(t, w.Unlock([]byte("1")))

	tx0 := &types2.Trx{
		Version:  1,
		Time:     time.Now().UnixNano(),
		Nonce:    rand.Int63(),
		From:     w.Address(),
		To:       types.RandAddress(),
		Amount:   bytes.RandU256Int(),
		Gas:      rand.Int63(),
		GasPrice: uint256.NewInt(rand.Uint64()),
		Type:     types2.TRX_CONTRACT,
		Payload: &types2.TrxPayloadContract{
			Data: bytes.RandHexBytes(10234),
		},
	}
	_, _, err := w.SignTrxRLP(tx0, "trx_test_chain")

	require.NoError(t, err)
	_, _, xerr := types2.VerifyTrxRLP(tx0, "trx_test_chain")
	require.NoError(t, xerr)

	bz0, err := rlp.EncodeToBytes(tx0)
	require.NoError(t, err)

	tx1 := &types2.Trx{}
	err = rlp.DecodeBytes(bz0, tx1)
	require.NoError(t, err)
	_, _, xerr = types2.VerifyTrxRLP(tx1, "trx_test_chain")
	require.NoError(t, xerr)

	require.Equal(t,
		tx0.Payload.(*types2.TrxPayloadContract).Data,
		tx1.Payload.(*types2.TrxPayloadContract).Data)

	bz1, err := rlp.EncodeToBytes(tx1)
	require.Equal(t, bz0, bz1)
}

func TestRLP_TrxPayloadSetDoc(t *testing.T) {
	w := web3.NewWallet([]byte("1"))
	require.NoError(t, w.Unlock([]byte("1")))

	tx0 := &types2.Trx{
		Version:  1,
		Time:     time.Now().UnixNano(),
		Nonce:    rand.Int63(),
		From:     w.Address(),
		To:       types.RandAddress(),
		Amount:   bytes.RandU256Int(),
		Gas:      rand.Int63(),
		GasPrice: uint256.NewInt(rand.Uint64()),
		Type:     types2.TRX_SETDOC,
		Payload: &types2.TrxPayloadSetDoc{
			Name: "test account doc",
			URL:  "https://test.account.doc/1",
		},
	}
	_, _, err := w.SignTrxRLP(tx0, "trx_test_chain")

	require.NoError(t, err)
	_, _, xerr := types2.VerifyTrxRLP(tx0, "trx_test_chain")
	require.NoError(t, xerr)

	bz0, err := rlp.EncodeToBytes(tx0)
	require.NoError(t, err)

	tx1 := &types2.Trx{}
	err = rlp.DecodeBytes(bz0, tx1)
	require.NoError(t, err)
	_, _, xerr = types2.VerifyTrxRLP(tx0, "trx_test_chain")
	require.NoError(t, xerr)

	require.Equal(t,
		tx0.Payload.(*types2.TrxPayloadSetDoc).Name,
		tx1.Payload.(*types2.TrxPayloadSetDoc).Name)

	require.Equal(t,
		tx0.Payload.(*types2.TrxPayloadSetDoc).URL,
		tx1.Payload.(*types2.TrxPayloadSetDoc).URL)

	bz1, err := rlp.EncodeToBytes(tx1)
	require.Equal(t, bz0, bz1)
}

func TestRLP_TrxPayloadProposal(t *testing.T) {
	w := web3.NewWallet([]byte("1"))
	require.NoError(t, w.Unlock([]byte("1")))

	tx0 := &types2.Trx{
		Version:  1,
		Time:     time.Now().UnixNano(),
		Nonce:    rand.Int63(),
		From:     w.Address(),
		To:       types.RandAddress(),
		Amount:   bytes.RandU256Int(),
		Gas:      rand.Int63(),
		GasPrice: uint256.NewInt(rand.Uint64()),
		Type:     types2.TRX_PROPOSAL,
		Payload: &types2.TrxPayloadProposal{
			Message:            "I want to ...",
			StartVotingHeight:  rand.Int63n(10),
			VotingPeriodBlocks: rand.Int63n(100) + 10,
			OptType:            rand.Int31(),
			Options:            [][]byte{bytes.RandBytes(100), bytes.RandBytes(100)},
		},
	}

	// check signature
	_, _, err := w.SignTrxRLP(tx0, "trx_test_chain")
	require.NoError(t, err)
	_, _, xerr := types2.VerifyTrxRLP(tx0, "trx_test_chain")
	require.NoError(t, xerr)

	// check encoding/decoding
	bz0, err := rlp.EncodeToBytes(tx0)
	require.NoError(t, err)

	tx1 := &types2.Trx{}
	err = rlp.DecodeBytes(bz0, tx1)
	require.NoError(t, err)
	_, _, xerr = types2.VerifyTrxRLP(tx0, "trx_test_chain")
	require.NoError(t, xerr)
	require.True(t, tx1.Equal(tx0))

	bz1, err := rlp.EncodeToBytes(tx1)
	require.Equal(t, bz0, bz1)
}

func BenchmarkTrxEncode(b *testing.B) {
	tx0 := &types2.Trx{
		Version:  1,
		Time:     time.Now().UnixNano(),
		Nonce:    rand.Int63(),
		From:     types.RandAddress(),
		To:       types.RandAddress(),
		Amount:   bytes.RandU256Int(),
		Gas:      rand.Int63(),
		GasPrice: uint256.NewInt(rand.Uint64()),
		Payload:  &types2.TrxPayloadAssetTransfer{},
	}
	require.Equal(b, types2.TRX_TRANSFER, tx0.GetType())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := tx0.Encode()
		require.NoError(b, err)
	}
}

func BenchmarkTrxDecode(b *testing.B) {
	tx0 := &types2.Trx{
		Version:  1,
		Time:     time.Now().UnixNano(),
		Nonce:    rand.Int63(),
		From:     types.RandAddress(),
		To:       types.RandAddress(),
		Amount:   bytes.RandU256Int(),
		Gas:      rand.Int63(),
		GasPrice: uint256.NewInt(rand.Uint64()),
		Payload:  &types2.TrxPayloadAssetTransfer{},
	}
	require.Equal(b, types2.TRX_TRANSFER, tx0.GetType())

	bzTx0, err := tx0.Encode()
	require.NoError(b, err)

	tx1 := &types2.Trx{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err = tx1.Decode(bzTx0)
		require.NoError(b, err)
	}
}

type maliciousPayload struct {
	dummy []byte
}

func (tx *maliciousPayload) Type() int32 {
	return types2.TRX_TRANSFER
}

func (tx *maliciousPayload) Equal(_tx types2.ITrxPayload) bool {
	return true
}

func (tx *maliciousPayload) Encode() ([]byte, xerrors.XError) {
	return tx.dummy, nil
}

func (tx *maliciousPayload) Decode(bz []byte) xerrors.XError {
	tx.dummy = bz
	return nil
}

func (tx *maliciousPayload) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, tx.dummy)
}

func (tx *maliciousPayload) DecodeRLP(s *rlp.Stream) error {
	var item []byte
	if err := s.Decode(&item); err != nil {
		return err
	}
	tx.dummy = item
	return nil
}

var _ types2.ITrxPayload = (*maliciousPayload)(nil)
