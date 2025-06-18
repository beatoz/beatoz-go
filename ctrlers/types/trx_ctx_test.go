package types_test

import (
	"github.com/beatoz/beatoz-go/ctrlers/mocks/acct"
	"github.com/beatoz/beatoz-go/ctrlers/mocks/gov"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

var (
	chainId  = "test_trx_ctx_chain"
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
	_, _, _ = w0.SignTrxRLP(tx, chainId)
	txctx, xerr := newTrxCtx(tx, 1)
	require.ErrorContains(t, xerr, xerrors.ErrInvalidGas.Error())

	//
	// 0 GasPrice
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govMock.MinTrxGas(), uint256.NewInt(0), uint256.NewInt(0))
	_, _, _ = w0.SignTrxRLP(tx, chainId)
	txctx, xerr = newTrxCtx(tx, 1)
	require.ErrorContains(t, xerr, xerrors.ErrInvalidGasPrice.Error())

	//
	// negative GasPrice
	var b [32]byte
	b[0] = 0x80
	neg := uint256.NewInt(0).SetBytes32(b[:])
	require.Negative(t, neg.Sign())
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govMock.MinTrxGas(), neg, uint256.NewInt(0))
	_, _, _ = w0.SignTrxRLP(tx, chainId)
	txctx, xerr = newTrxCtx(tx, 1)
	require.ErrorContains(t, xerr, xerrors.ErrInvalidGasPrice.Error())

	//
	// too much GasPrice
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govMock.MinTrxGas(), uint256.NewInt(10_000_000_001), uint256.NewInt(0))
	_, _, _ = w0.SignTrxRLP(tx, chainId)
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
	_, _, _ = w1.SignTrxRLP(tx, chainId)
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
	tx = web3.NewTrxTransfer(w0.Address(), nil, 0, govMock.MinTrxGas(), govMock.GasPrice(), uint256.NewInt(1000))
	_, _, _ = w0.SignTrxRLP(tx, chainId)
	txctx, xerr = newTrxCtx(tx, 1)
	require.ErrorContains(t, xerr, xerrors.ErrInvalidAddress.Error())

	//
	// To nil address (contract transaction)
	tx = web3.NewTrxContract(w0.Address(), nil, 0, govMock.MinTrxGas(), govMock.GasPrice(), uint256.NewInt(0), bytes.RandBytes(32))
	_, _, _ = w0.SignTrxRLP(tx, chainId)
	txctx, xerr = newTrxCtx(tx, 1)
	require.NoError(t, xerr)
	require.NotNil(t, txctx.Sender)
	require.Equal(t, txctx.Sender.Address, w0.Address())
	require.NotNil(t, txctx.Receiver)
	require.Equal(t, txctx.Receiver.Address, types.ZeroAddress())

	//
	// To Zero Address
	tx = web3.NewTrxTransfer(w0.Address(), types.ZeroAddress(), 0, govMock.MinTrxGas(), govMock.GasPrice(), uint256.NewInt(1000))
	_, _, _ = w0.SignTrxRLP(tx, chainId)
	txctx, xerr = newTrxCtx(tx, 1)
	require.NoError(t, xerr)
	require.NotNil(t, txctx.Sender)
	require.Equal(t, txctx.Sender.Address, w0.Address())
	require.NotNil(t, txctx.Receiver)
	require.Equal(t, txctx.Receiver.Address, types.ZeroAddress())

	//
	// Success
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govMock.MinTrxGas(), govMock.GasPrice(), uint256.NewInt(1000))
	_, _, _ = w0.SignTrxRLP(tx, chainId)
	txctx, xerr = newTrxCtx(tx, 1)
	require.NoError(t, xerr)
}

func newTrxCtx(tx *ctrlertypes.Trx, height int64) (*ctrlertypes.TrxContext, xerrors.XError) {
	bctx := ctrlertypes.TempBlockContext(chainId, height, time.Now(), govMock, acctMock, nil, nil, nil)
	bz, _ := tx.Encode()
	return ctrlertypes.NewTrxContext(bz, bctx, true)
}
