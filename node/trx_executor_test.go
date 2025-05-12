package node

import (
	"github.com/beatoz/beatoz-go/ctrlers/mocks"
	"github.com/beatoz/beatoz-go/ctrlers/mocks/acct"
	"github.com/beatoz/beatoz-go/ctrlers/mocks/gov"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmtypes "github.com/tendermint/tendermint/proto/tendermint/types"
	"math/rand/v2"
	"testing"
	"time"
)

var (
	chainId  = "test-trx-executor-chain"
	govMock  = gov.NewGovHandlerMock(ctrlertypes.DefaultGovParams())
	acctMock = acct.NewAccountHandlerMock(1000)
	balance  = uint64(10_000_000_000_000_000_000)
)

func init() {
	acctMock.Iterate(func(idx int, w *web3.Wallet) bool {
		_ = w.GetAccount().AddBalance(govMock.MinTrxFee())
		_ = w.GetAccount().AddBalance(uint256.NewInt(balance))
		return true
	})
}

func Test_commonValidation(t *testing.T) {
	w0 := acctMock.RandWallet() // web3.NewWallet(nil)
	w1 := web3.NewWallet(nil)

	//
	// Invalid nonce
	tx := web3.NewTrxTransfer(w0.Address(), w1.Address(), 1, govMock.MinTrxGas(), govMock.GasPrice(), uint256.NewInt(balance))
	_, _, err := w0.SignTrxRLP(tx, chainId)
	require.NoError(t, err)

	txctx, xerr := mocks.MakeTrxCtxWithTrx(tx, chainId, 1, time.Now(), true, govMock, acctMock, nil, nil, nil)
	require.NoError(t, xerr)
	xerr = commonValidation(txctx)
	require.ErrorContains(t, xerr, xerrors.ErrInvalidNonce.Error(), xerr)

	//
	// Insufficient fund
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govMock.MinTrxGas(), govMock.GasPrice(), uint256.NewInt(balance+1))
	_, _, err = w0.SignTrxRLP(tx, chainId)
	require.NoError(t, err)

	txctx, xerr = mocks.MakeTrxCtxWithTrx(tx, chainId, 1, time.Now(), true, govMock, acctMock, nil, nil, nil)
	require.NoError(t, xerr)
	require.ErrorContains(t, commonValidation(txctx), xerrors.ErrInsufficientFund.Error())
}

func Test_BlockGasLimit(t *testing.T) {
	w0 := acctMock.RandWallet() //web3.NewWallet(nil)
	w1 := web3.NewWallet(nil)

	blockGasLimit := int64(5_000_000)
	blockGasUsed := int64(0)
	upper := blockGasLimit - (blockGasLimit / 10)
	//lower := blockGasLimit / 100

	bctx := ctrlertypes.NewBlockContext(
		abcitypes.RequestBeginBlock{Header: tmtypes.Header{Height: 1}},
		govMock,
		acctMock,
		nil, nil, nil)
	bctx.SetBlockGasLimit(blockGasLimit)
	require.Equal(t, blockGasLimit, bctx.GetBlockGasLimit())
	require.Equal(t, blockGasUsed, bctx.GetBlockGasUsed())

	for {
		rnGas := rand.Int64N(100_000) + govMock.MinTrxGas()
		tx := web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, rnGas, govMock.GasPrice(), uint256.NewInt(1))
		_, _, xerr := w0.SignTrxRLP(tx, chainId)
		require.NoError(t, xerr)

		txctx, xerr := mocks.MakeTrxCtxWithTrx(tx, chainId, 1, time.Now(), true, govMock, acctMock, nil, nil, nil)
		require.NoError(t, xerr)

		require.NoError(t, runTrx(txctx, bctx))
		require.Equal(t, rnGas, txctx.GasUsed)

		blockGasUsed += rnGas

		require.Equal(t, blockGasLimit, bctx.GetBlockGasLimit())
		require.Equal(t, blockGasUsed, bctx.GetBlockGasUsed())

		if blockGasUsed > upper {
			break
		}
	}

	expected := blockGasLimit + blockGasLimit/10 // increasing by 10%
	adjusted := ctrlertypes.AdjustBlockGasLimit(bctx.GetBlockGasLimit(), bctx.GetBlockGasUsed(), govMock.MinTrxGas(), govMock.MaxBlockGas())
	require.Equal(t, expected, adjusted)

	blockGasLimit = adjusted
	blockGasUsed = int64(0)
	lower := blockGasLimit / 100

	bctx = ctrlertypes.NewBlockContext(
		abcitypes.RequestBeginBlock{Header: tmtypes.Header{Height: 1}},
		govMock,
		acctMock,
		nil, nil, nil)
	bctx.SetBlockGasLimit(blockGasLimit)
	require.Equal(t, blockGasLimit, bctx.GetBlockGasLimit())
	require.Equal(t, blockGasUsed, bctx.GetBlockGasUsed())

	for {
		rnGas := govMock.MinTrxGas()
		if blockGasUsed+rnGas >= lower {
			break
		}
		tx := web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, rnGas, govMock.GasPrice(), uint256.NewInt(1))
		_, _, xerr := w0.SignTrxRLP(tx, chainId)
		require.NoError(t, xerr)

		txctx, xerr := mocks.MakeTrxCtxWithTrx(tx, chainId, 1, time.Now(), true, govMock, acctMock, nil, nil, nil)
		require.NoError(t, xerr)

		require.NoError(t, runTrx(txctx, bctx))
		require.Equal(t, rnGas, txctx.GasUsed)

		blockGasUsed += rnGas

		require.Equal(t, blockGasLimit, bctx.GetBlockGasLimit())
		require.Equal(t, blockGasUsed, bctx.GetBlockGasUsed())
	}

	expected = blockGasLimit - blockGasLimit/100 // increasing by 10%
	adjusted = ctrlertypes.AdjustBlockGasLimit(bctx.GetBlockGasLimit(), bctx.GetBlockGasUsed(), govMock.MinTrxGas(), govMock.MaxBlockGas())
	require.Equal(t, expected, adjusted)
}

//type acctHandlerMock struct{}
//
//func (a *acctHandlerMock) ValidateTrx(ctx *ctrlertypes.TrxContext) xerrors.XError {
//	return nil
//}
//
//func (a *acctHandlerMock) ExecuteTrx(ctx *ctrlertypes.TrxContext) xerrors.XError {
//	_ = ctx.Sender.AddBalance(ctx.Tx.Amount)
//	_ = ctx.Receiver.SubBalance(ctx.Tx.Amount)
//	return nil
//}
//
//var _ ctrlertypes.IAccountHandler = (*acctHandlerMock)(nil)
//var _ ctrlertypes.ITrxHandler = (*acctHandlerMock)(nil)
//
//func (a *acctHandlerMock) FindOrNewAccount(address types.Address, b bool) *ctrlertypes.Account {
//	return a.FindAccount(address, b)
//}
//
//func (a *acctHandlerMock) FindAccount(address types.Address, b bool) *ctrlertypes.Account {
//	acct := ctrlertypes.NewAccount(address)
//	acct.AddBalance(govMock.MinTrxFee())
//	acct.AddBalance(uint256.NewInt(balance))
//	return acct
//}
//
//func (a *acctHandlerMock) Transfer(address types.Address, address2 types.Address, u *uint256.Int, b bool) xerrors.XError {
//	panic("implement me")
//}
//
//func (a *acctHandlerMock) Reward(address types.Address, u *uint256.Int, b bool) xerrors.XError {
//	panic("implement me")
//}
//
//func (a *acctHandlerMock) SimuAcctCtrlerAt(i int64) (ctrlertypes.IAccountHandler, xerrors.XError) {
//	panic("implement me")
//}
//func (a *acctHandlerMock) SetAccount(account *ctrlertypes.Account, b bool) xerrors.XError {
//	return nil
//}
