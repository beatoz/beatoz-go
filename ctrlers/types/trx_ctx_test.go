package types_test

import (
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
	govParams = ctrlertypes.DefaultGovParams()
	chainId   = "test_trx_ctx_chain"
)

func Test_NewTrxContext(t *testing.T) {
	w0 := web3.NewWallet(nil)
	w1 := web3.NewWallet(nil)

	//
	// Small Gas
	tx := web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govParams.MinTrxGas()-1, govParams.GasPrice(), uint256.NewInt(0))
	_, _, _ = w0.SignTrxRLP(tx, chainId)
	txctx, xerr := newTrxCtx(tx, 1)
	require.ErrorContains(t, xerr, xerrors.ErrInvalidGas.Error())

	//
	// 0 GasPrice
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govParams.MinTrxGas(), uint256.NewInt(0), uint256.NewInt(0))
	_, _, _ = w0.SignTrxRLP(tx, chainId)
	txctx, xerr = newTrxCtx(tx, 1)
	require.ErrorContains(t, xerr, xerrors.ErrInvalidGasPrice.Error())

	//
	// negative GasPrice
	var b [32]byte
	b[0] = 0x80
	neg := uint256.NewInt(0).SetBytes32(b[:])
	require.Negative(t, neg.Sign())
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govParams.MinTrxGas(), neg, uint256.NewInt(0))
	_, _, _ = w0.SignTrxRLP(tx, chainId)
	txctx, xerr = newTrxCtx(tx, 1)
	require.ErrorContains(t, xerr, xerrors.ErrInvalidGasPrice.Error())

	//
	// too much GasPrice
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govParams.MinTrxGas(), uint256.NewInt(10_000_000_001), uint256.NewInt(0))
	_, _, _ = w0.SignTrxRLP(tx, chainId)
	txctx, xerr = newTrxCtx(tx, 1)
	require.ErrorContains(t, xerr, xerrors.ErrInvalidGasPrice.Error())

	//
	// Wrong Signature - sign with proto encoding
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govParams.MinTrxGas(), govParams.GasPrice(), uint256.NewInt(0))
	_, _, _ = w0.SignTrxProto(tx, chainId)
	txctx, xerr = newTrxCtx(tx, 1)
	require.ErrorContains(t, xerr, xerrors.ErrInvalidTrxSig.Error())

	//
	// Wrong Signature - no signature
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govParams.MinTrxGas(), govParams.GasPrice(), uint256.NewInt(0))
	txctx, xerr = newTrxCtx(tx, 1)
	require.ErrorContains(t, xerr, xerrors.ErrInvalidTrxSig.Error())

	//
	// Wrong Signature - other's signature
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govParams.MinTrxGas(), govParams.GasPrice(), uint256.NewInt(0))
	_, _, _ = w1.SignTrxRLP(tx, chainId)
	txctx, xerr = newTrxCtx(tx, 1)
	require.ErrorContains(t, xerr, xerrors.ErrInvalidTrxSig.Error())

	//
	// Wrong Signature - wrong chainId
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govParams.MinTrxGas(), govParams.GasPrice(), uint256.NewInt(0))
	_, _, _ = w0.SignTrxRLP(tx, "tx_executor_test_chain_wrong")
	txctx, xerr = newTrxCtx(tx, 1)
	require.ErrorContains(t, xerr, xerrors.ErrInvalidTrxSig.Error())

	//
	// To nil address (not contract transaction)
	tx = web3.NewTrxTransfer(w0.Address(), nil, 0, govParams.MinTrxGas(), govParams.GasPrice(), uint256.NewInt(1000))
	_, _, _ = w0.SignTrxRLP(tx, chainId)
	txctx, xerr = newTrxCtx(tx, 1)
	require.ErrorContains(t, xerr, xerrors.ErrInvalidAddress.Error())

	//
	// To nil address (contract transaction)
	tx = web3.NewTrxContract(w0.Address(), nil, 0, govParams.MinTrxGas(), govParams.GasPrice(), uint256.NewInt(0), bytes.RandBytes(32))
	_, _, _ = w0.SignTrxRLP(tx, chainId)
	txctx, xerr = newTrxCtx(tx, 1)
	require.NoError(t, xerr)
	require.NotNil(t, txctx.Sender)
	require.Equal(t, txctx.Sender.Address, w0.Address())
	require.NotNil(t, txctx.Receiver)
	require.Equal(t, txctx.Receiver.Address, types.ZeroAddress())

	//
	// To Zero Address
	tx = web3.NewTrxTransfer(w0.Address(), types.ZeroAddress(), 0, govParams.MinTrxGas(), govParams.GasPrice(), uint256.NewInt(1000))
	_, _, _ = w0.SignTrxRLP(tx, chainId)
	txctx, xerr = newTrxCtx(tx, 1)
	require.NoError(t, xerr)
	require.NotNil(t, txctx.Sender)
	require.Equal(t, txctx.Sender.Address, w0.Address())
	require.NotNil(t, txctx.Receiver)
	require.Equal(t, txctx.Receiver.Address, types.ZeroAddress())

	//
	// Success
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govParams.MinTrxGas(), govParams.GasPrice(), uint256.NewInt(1000))
	_, _, _ = w0.SignTrxRLP(tx, chainId)
	txctx, xerr = newTrxCtx(tx, 1)
	require.NoError(t, xerr)
}

func newTrxCtx(tx *ctrlertypes.Trx, height int64) (*ctrlertypes.TrxContext, xerrors.XError) {
	bz, _ := tx.Encode()
	return ctrlertypes.NewTrxContext(bz, height, time.Now().UnixMilli(), true, func(_txctx *ctrlertypes.TrxContext) xerrors.XError {
		_txctx.GovParams = govParams
		_txctx.AcctHandler = &acctHandlerMock{}
		_txctx.ChainID = chainId
		return nil
	})
}

type acctHandlerMock struct{}

func (a *acctHandlerMock) ValidateTrx(context *ctrlertypes.TrxContext) xerrors.XError {
	return nil
}

func (a *acctHandlerMock) ExecuteTrx(context *ctrlertypes.TrxContext) xerrors.XError {
	return nil
}

func (a *acctHandlerMock) FindOrNewAccount(address types.Address, b bool) *ctrlertypes.Account {
	return a.FindAccount(address, b)
}

func (a *acctHandlerMock) FindAccount(address types.Address, b bool) *ctrlertypes.Account {
	acct := ctrlertypes.NewAccount(address)
	acct.AddBalance(govParams.MinTrxFee())
	acct.AddBalance(uint256.NewInt(1000))
	return acct
}

func (a *acctHandlerMock) Transfer(address types.Address, address2 types.Address, u *uint256.Int, b bool) xerrors.XError {
	panic("implement me")
}

func (a *acctHandlerMock) Reward(address types.Address, u *uint256.Int, b bool) xerrors.XError {
	panic("implement me")
}

func (a *acctHandlerMock) ImmutableAcctCtrlerAt(i int64) (ctrlertypes.IAccountHandler, xerrors.XError) {
	panic("implement me")
}
func (a *acctHandlerMock) SimuAcctCtrlerAt(i int64) (ctrlertypes.IAccountHandler, xerrors.XError) {
	panic("implement me")
}
func (a *acctHandlerMock) SetAccount(account *ctrlertypes.Account, b bool) xerrors.XError {
	panic("implement me")
}
