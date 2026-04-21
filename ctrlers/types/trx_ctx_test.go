package types_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/beatoz/beatoz-go/ctrlers/mocks/acct"
	"github.com/beatoz/beatoz-go/ctrlers/mocks/gov"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/merkle"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

var (
	chainId  = uint256.MustFromHex("0xabc")
	govMock  = gov.NewGovHandlerMock(ctrlertypes.DefaultGovParams())
	acctMock = acct.NewAcctHandlerMock(1000)
)

func init() {
	acctMock.Iterate(func(idx int, w *web3.Wallet) bool {
		w.GetAccount().SetBalance(uint256.NewInt(1_000_000_000))
		return true
	})
}

func Test_NewTrxContext(t *testing.T) {
	w0 := acctMock.RandWallet() //web3.NewWallet(nil)
	w1 := web3.NewWallet(nil)

	//
	// Small Gas
	tx := web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govMock.MinTrxGas()-1, govMock.GasPrice(), uint256.NewInt(0))
	_, _, _ = w0.SignTrxRLP(tx, chainId.Hex())
	txctx, xerr := newTrxCtx(tx, 1)
	require.ErrorContains(t, xerr, xerrors.ErrInvalidGas.Error())

	//
	// 0 GasPrice
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govMock.MinTrxGas(), uint256.NewInt(0), uint256.NewInt(0))
	_, _, _ = w0.SignTrxRLP(tx, chainId.Hex())
	txctx, xerr = newTrxCtx(tx, 1)
	require.ErrorContains(t, xerr, xerrors.ErrInvalidGasPrice.Error())

	//
	// negative GasPrice
	var b [32]byte
	b[0] = 0x80
	neg := uint256.NewInt(0).SetBytes32(b[:])
	require.Negative(t, neg.Sign())
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govMock.MinTrxGas(), neg, uint256.NewInt(0))
	_, _, _ = w0.SignTrxRLP(tx, chainId.Hex())
	txctx, xerr = newTrxCtx(tx, 1)
	require.ErrorContains(t, xerr, xerrors.ErrInvalidGasPrice.Error())

	//
	// too much GasPrice
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govMock.MinTrxGas(), uint256.NewInt(10_000_000_001), uint256.NewInt(0))
	_, _, _ = w0.SignTrxRLP(tx, chainId.Hex())
	txctx, xerr = newTrxCtx(tx, 1)
	require.ErrorContains(t, xerr, xerrors.ErrInvalidGasPrice.Error())

	//
	// Wrong Signature - no signature
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govMock.MinTrxGas(), govMock.GasPrice(), uint256.NewInt(0))
	txctx, xerr = newTrxCtx(tx, 1)
	require.ErrorContains(t, xerr, xerrors.ErrInvalidTrxSig.Error())

	//
	// Wrong Signature - other's signature
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govMock.MinTrxGas(), govMock.GasPrice(), uint256.NewInt(0))
	_, _, _ = w1.SignTrxRLP(tx, chainId.Hex())
	txctx, xerr = newTrxCtx(tx, 1)
	require.ErrorContains(t, xerr, xerrors.ErrInvalidTrxSig.Error())

	//
	// Wrong Signature - wrong chainId
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govMock.MinTrxGas(), govMock.GasPrice(), uint256.NewInt(0))
	_, _, _ = w0.SignTrxRLP(tx, "tx_executor_test_chain_wrong")
	txctx, xerr = newTrxCtx(tx, 1)
	require.ErrorContains(t, xerr, xerrors.ErrInvalidTrxSig.Error())

	//
	// To nil address (not contract transaction)
	// todo: move this case to trx_test.go
	tx = web3.NewTrxTransfer(w0.Address(), nil, 0, govMock.MinTrxGas(), govMock.GasPrice(), uint256.NewInt(1000))
	_, _, _ = w0.SignTrxRLP(tx, chainId.Hex())
	txctx, xerr = newTrxCtx(tx, 1)
	require.ErrorContains(t, xerr, xerrors.ErrInvalidAddress.Error())

	//
	// To nil address (contract transaction)
	tx = web3.NewTrxContract(w0.Address(), nil, 0, govMock.MinTrxGas(), govMock.GasPrice(), uint256.NewInt(0), bytes.RandBytes(32))
	_, _, _ = w0.SignTrxRLP(tx, chainId.Hex())
	txctx, xerr = newTrxCtx(tx, 1)
	require.NoError(t, xerr)
	require.NotNil(t, txctx.Sender)
	require.Equal(t, txctx.Sender.Address, w0.Address())
	require.NotNil(t, txctx.Receiver)
	require.Equal(t, txctx.Receiver.Address, types.ZeroAddress())
	require.NotNil(t, txctx.Payer)
	require.Equal(t, txctx.Payer.Address, w0.Address())
	//
	// To Zero Address
	tx = web3.NewTrxTransfer(w0.Address(), types.ZeroAddress(), 0, govMock.MinTrxGas(), govMock.GasPrice(), uint256.NewInt(1000))
	_, _, _ = w0.SignTrxRLP(tx, chainId.Hex())
	txctx, xerr = newTrxCtx(tx, 1)
	require.NoError(t, xerr)
	require.NotNil(t, txctx.Sender)
	require.Equal(t, txctx.Sender.Address, w0.Address())
	require.NotNil(t, txctx.Receiver)
	require.Equal(t, txctx.Receiver.Address, types.ZeroAddress())
	require.NotNil(t, txctx.Payer)
	require.Equal(t, txctx.Payer.Address, w0.Address())

	//
	// Success
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govMock.MinTrxGas(), govMock.GasPrice(), uint256.NewInt(1000))
	_, _, _ = w0.SignTrxRLP(tx, chainId.Hex())
	txctx, xerr = newTrxCtx(tx, 1)
	require.NoError(t, xerr)
	require.NotNil(t, txctx.Sender)
	require.Equal(t, txctx.Sender.Address, w0.Address())
	require.NotNil(t, txctx.Receiver)
	require.Equal(t, txctx.Receiver.Address, w1.Address())
	require.NotNil(t, txctx.Payer)
	require.EqualValues(t, txctx.Sender.Address, txctx.Payer.Address)

	//
	// Payer: not found payer account
	payer := web3.NewWallet(nil)
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govMock.MinTrxGas(), govMock.GasPrice(), uint256.NewInt(1000))
	_, _, err := w0.SignTrxRLP(tx, chainId.Hex())
	require.NoError(t, err)
	_, _, err = payer.SignPayerTrxRLP(tx, chainId.Hex())
	require.NoError(t, err)
	txctx, xerr = newTrxCtx(tx, 1)
	require.ErrorContains(t, xerr, xerrors.ErrNotFoundAccount.Error())

	//
	// Payer: not Sender
	payer = acctMock.RandWallet()
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govMock.MinTrxGas(), govMock.GasPrice(), uint256.NewInt(1000))
	_, _, err = w0.SignTrxRLP(tx, chainId.Hex())
	require.NoError(t, err)
	_, _, err = payer.SignPayerTrxRLP(tx, chainId.Hex())
	require.NoError(t, err)
	txctx, xerr = newTrxCtx(tx, 1)
	require.NoError(t, xerr)
	require.NotNil(t, txctx.Payer)
	require.Equal(t, payer.Address(), txctx.Payer.Address)
}

func newTrxCtx(tx *ctrlertypes.Trx, height int64) (*ctrlertypes.TrxContext, xerrors.XError) {
	bctx := ctrlertypes.TempBlockContext(chainId.Hex(), height, time.Now(), govMock, acctMock, nil, nil, nil)
	bz, _ := tx.Encode()
	return ctrlertypes.NewTrxContext(bz, bctx, true)
}

//
// test code for `EventRoot()` and `EventRootEx()`
//

func Test_TrxContext_EventRoot(t *testing.T) {

	txctx := &ctrlertypes.TrxContext{}
	txctx.Events = append(txctx.Events, abcitypes.Event{
		Type: "tx",
		Attributes: []abcitypes.EventAttribute{
			{Key: []byte(ctrlertypes.EVENT_ATTR_TXTYPE), Value: []byte("TestTransfer"), Index: true},
			{Key: []byte(ctrlertypes.EVENT_ATTR_TXSENDER), Value: []byte("123456"), Index: true},
			{Key: []byte(ctrlertypes.EVENT_ATTR_TXRECVER), Value: []byte("abcdef"), Index: true},
			{Key: []byte(ctrlertypes.EVENT_ATTR_AMOUNT), Value: []byte("1111"), Index: false},
		},
	})
	tree, root := ctrlertypes.EventRoot(txctx)
	require.NotNil(t, tree)
	require.NotNil(t, root)

	targetIdx := 1
	targetData := append([]byte("tx"), append([]byte(ctrlertypes.EVENT_ATTR_TXSENDER), []byte("123456")...)...)

	_, siblings, err := tree.Proof(targetIdx)
	require.NoError(t, err)

	err = merkle.VerifyProof(targetIdx, targetData, siblings, root)
	require.NoError(t, err)

	// wrong index
	err = merkle.VerifyProof(targetIdx+1, targetData, siblings, root)
	require.Error(t, err)

	// wrong data
	err = merkle.VerifyProof(targetIdx, []byte("wrong data"), siblings, root)
	require.Error(t, err)

	// other siblings
	_, otherSiblings, err := tree.Proof(targetIdx + 1)
	require.NoError(t, err)
	err = merkle.VerifyProof(targetIdx, targetData, otherSiblings, root)
	require.Error(t, err)
}

func Test_TrxContext_EventRootEx(t *testing.T) {
	// empty events
	txctx := &ctrlertypes.TrxContext{}
	tree, root := ctrlertypes.EventRootEx(txctx)
	require.Nil(t, tree)
	require.Nil(t, root)

	// two events
	txctx.Events = append(txctx.Events, abcitypes.Event{
		Type: "tx",
		Attributes: []abcitypes.EventAttribute{
			{Key: []byte(ctrlertypes.EVENT_ATTR_TXTYPE), Value: []byte("TestTransfer"), Index: true},
			{Key: []byte(ctrlertypes.EVENT_ATTR_TXSENDER), Value: []byte("sender1"), Index: true},
			{Key: []byte(ctrlertypes.EVENT_ATTR_TXRECVER), Value: []byte("recver1"), Index: true},
			{Key: []byte(ctrlertypes.EVENT_ATTR_AMOUNT), Value: []byte("1000"), Index: false},
		},
	})
	txctx.Events = append(txctx.Events, abcitypes.Event{
		Type: "tx",
		Attributes: []abcitypes.EventAttribute{
			{Key: []byte(ctrlertypes.EVENT_ATTR_TXTYPE), Value: []byte("TestTransfer"), Index: true},
			{Key: []byte(ctrlertypes.EVENT_ATTR_TXSENDER), Value: []byte("sender2"), Index: true},
			{Key: []byte(ctrlertypes.EVENT_ATTR_TXRECVER), Value: []byte("recver2"), Index: true},
			{Key: []byte(ctrlertypes.EVENT_ATTR_AMOUNT), Value: []byte("2000"), Index: false},
		},
	})

	tree, root = ctrlertypes.EventRootEx(txctx)
	require.NotNil(t, tree)
	require.NotNil(t, root)

	// Verify proof for the first event root (index 0) in the top-level tree.
	// The top-level tree leaves are already hashed, so use preHashed=true for VerifyProof.
	leafHash, siblings, err := tree.Proof(0)
	require.NoError(t, err)
	err = merkle.VerifyProof(0, leafHash, siblings, root, true)
	require.NoError(t, err)

	// Verify proof for the second event root (index 1)
	leafHash, siblings, err = tree.Proof(1)
	require.NoError(t, err)
	err = merkle.VerifyProof(1, leafHash, siblings, root, true)
	require.NoError(t, err)

	// wrong data should fail
	err = merkle.VerifyProof(0, []byte("wrong"), siblings, root, true)
	require.Error(t, err)

	// Verify that per-event merkle root matches the leaf in the top-level tree.
	// Reconstruct the first event's merkle root manually.
	evt0 := txctx.Events[0]
	var leaves [][]byte
	for _, attr := range evt0.Attributes {
		leaves = append(leaves, attr.Value)
	}
	evt0Tree := merkle.NewMerkleTree(merkle.WithRawLeaves(leaves))
	evt0Root := evt0Tree.Root()

	leaf0, _, err := tree.Proof(0)
	require.NoError(t, err)
	require.Equal(t, evt0Root, leaf0)

	// Verify a specific attribute in the per-event merkle tree.
	// Events[0].Attributes[1] (TXSENDER, "sender1")
	targetIdx := 1
	targetData := []byte("sender1")
	_, evt0Siblings, err := evt0Tree.Proof(targetIdx)
	require.NoError(t, err)
	err = merkle.VerifyProof(targetIdx, targetData, evt0Siblings, evt0Root)
	require.NoError(t, err)

	// wrong attribute data should fail
	err = merkle.VerifyProof(targetIdx, []byte("wrong"), evt0Siblings, evt0Root)
	require.Error(t, err)

	// Events[1].Attributes[2] (TXRECVER, "recver2")
	evt1 := txctx.Events[1]
	var leaves1 [][]byte
	for _, attr := range evt1.Attributes {
		leaves1 = append(leaves1, attr.Value)
	}
	evt1Tree := merkle.NewMerkleTree(merkle.WithRawLeaves(leaves1))
	evt1Root := evt1Tree.Root()

	targetIdx = 2
	targetData = []byte("recver2")
	_, evt1Siblings, err := evt1Tree.Proof(targetIdx)
	require.NoError(t, err)
	err = merkle.VerifyProof(targetIdx, targetData, evt1Siblings, evt1Root)
	require.NoError(t, err)
}

func newBenchTrxContext(eventCount, attrCount int) *ctrlertypes.TrxContext {
	txctx := &ctrlertypes.TrxContext{}
	for i := 0; i < eventCount; i++ {
		var attrs []abcitypes.EventAttribute
		for j := 0; j < attrCount; j++ {
			attrs = append(attrs, abcitypes.EventAttribute{
				Key:   []byte(fmt.Sprintf("key_%d_%d", i, j)),
				Value: []byte(fmt.Sprintf("value_%d_%d", i, j)),
				Index: true,
			})
		}
		txctx.Events = append(txctx.Events, abcitypes.Event{
			Type:       fmt.Sprintf("event_%d", i),
			Attributes: attrs,
		})
	}
	return txctx
}

func Benchmark_EventRoot(b *testing.B) {
	for _, bc := range []struct {
		name       string
		eventCount int
		attrCount  int
	}{
		{"10events_4attrs", 10, 4},
		{"100events_4attrs", 100, 4},
		{"100events_10attrs", 100, 10},
	} {
		txctx := newBenchTrxContext(bc.eventCount, bc.attrCount)
		b.Run(bc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				ctrlertypes.EventRoot(txctx)
			}
		})
	}
}

func Benchmark_EventRootEx(b *testing.B) {
	for _, bc := range []struct {
		name       string
		eventCount int
		attrCount  int
	}{
		{"10events_4attrs", 10, 4},
		{"100events_4attrs", 100, 4},
		{"100events_10attrs", 100, 10},
	} {
		txctx := newBenchTrxContext(bc.eventCount, bc.attrCount)
		b.Run(bc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				ctrlertypes.EventRootEx(txctx)
			}
		})
	}
}
