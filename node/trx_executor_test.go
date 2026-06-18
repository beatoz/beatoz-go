package node

import (
	"fmt"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/beatoz/beatoz-go/ctrlers/mocks"
	"github.com/beatoz/beatoz-go/ctrlers/mocks/acct"
	"github.com/beatoz/beatoz-go/ctrlers/mocks/gov"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmtypes "github.com/tendermint/tendermint/proto/tendermint/types"
)

var (
	chainId  = uint256.MustFromHex("0xabc")
	govMock  = gov.NewGovHandlerMock(ctrlertypes.DefaultGovParams())
	acctMock = acct.NewAcctHandlerMock(1000)
	balance  = uint64(10_000_000_000_000_000_000)
)

func init() {
	ctrlertypes.InitSigner(chainId)
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
	// Exceed the block gas limit
	blockGasLimit := int64(10_000)
	bctx := ctrlertypes.NewBlockContext(
		abcitypes.RequestBeginBlock{Header: tmtypes.Header{ChainID: chainId.Hex(), Height: 1}},
		govMock,
		acctMock,
		nil, nil, nil)
	bctx.SetBlockGasLimit(blockGasLimit)
	require.Equal(t, blockGasLimit, bctx.GetBlockGasLimit())
	// expected success
	gas := govMock.MinTrxGas()
	tx := web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, gas, govMock.GasPrice(), uint256.NewInt(1))
	_, _, xerr := w0.SignTrxRLP(tx, chainId.Hex())
	require.NoError(t, xerr)
	txctx, xerr := mocks.MakeTrxCtxWithTrxBctx(tx, bctx, true)
	require.NoError(t, xerr)
	require.NoError(t, commonValidation(txctx))
	// expected failure
	gas = govMock.MinTrxGas() * 3
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, gas, govMock.GasPrice(), uint256.NewInt(1))
	_, _, xerr = w0.SignTrxRLP(tx, chainId.Hex())
	require.NoError(t, xerr)
	txctx, xerr = mocks.MakeTrxCtxWithTrxBctx(tx, bctx, true)
	require.NoError(t, xerr)
	require.ErrorContains(t, commonValidation(txctx), xerrors.ErrInvalidGas.Error(), xerr)

	//
	// Invalid nonce
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 1, govMock.MinTrxGas(), govMock.GasPrice(), uint256.NewInt(balance))
	_, _, err := w0.SignTrxRLP(tx, chainId.Hex())
	require.NoError(t, err)
	txctx, xerr = mocks.MakeTrxCtxWithTrx(tx, chainId.Hex(), 1, time.Now(), true, govMock, acctMock, nil, nil, nil)
	require.NoError(t, xerr)
	require.ErrorContains(t, commonValidation(txctx), xerrors.ErrInvalidNonce.Error(), xerr)

	//
	// Insufficient fund
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govMock.MinTrxGas(), govMock.GasPrice(), uint256.NewInt(balance+1))
	_, _, err = w0.SignTrxRLP(tx, chainId.Hex())
	require.NoError(t, err)
	txctx, xerr = mocks.MakeTrxCtxWithTrx(tx, chainId.Hex(), 1, time.Now(), true, govMock, acctMock, nil, nil, nil)
	require.NoError(t, xerr)
	require.ErrorContains(t, commonValidation(txctx), xerrors.ErrInsufficientFund.Error())
}

func Test_Gas_FailedTx(t *testing.T) {
	w0 := acctMock.RandWallet() //web3.NewWallet(nil)
	w1 := web3.NewWallet(nil)

	blockGasLimit := int64(5_000_000)
	bctx := ctrlertypes.NewBlockContext(
		abcitypes.RequestBeginBlock{Header: tmtypes.Header{ChainID: chainId.Hex(), Height: 1}},
		govMock,
		acctMock,
		nil, nil, nil)
	bctx.SetBlockGasLimit(blockGasLimit)
	require.Equal(t, blockGasLimit, bctx.GetBlockGasLimit())
	require.Equal(t, int64(0), bctx.GetBlockGasUsed())

	balance0 := w0.GetBalance()
	gas := govMock.MinTrxGas()

	// TEST FOR FAILED TX
	// make tx to be failed; wrong balance
	tx := web3.NewTrxTransfer(w0.Address(), w1.Address(), w0.GetNonce(), gas, govMock.GasPrice(), new(uint256.Int).Add(w0.GetBalance(), uint256.NewInt(1)))
	_, _, xerr := w0.SignTrxRLP(tx, chainId.Hex())
	require.NoError(t, xerr)

	//
	// test in CheckTx; expected gas is not used
	txctx, xerr := mocks.MakeTrxCtxWithTrxBctx(tx, bctx, false)
	require.NoError(t, xerr)
	// Do not call validateTrx to avoid checking balance,
	// require.NoError(t, validateTrx(txctx))
	require.Error(t, runTrx(txctx))
	require.Equal(t, int64(0), txctx.GasUsed)
	require.Equal(t, int64(0), bctx.GetBlockGasUsed())
	require.Equal(t, balance0.Dec(), w0.GetBalance().Dec())

	// test in DeliverTx; expected gas is used
	txctx, xerr = mocks.MakeTrxCtxWithTrxBctx(tx, bctx, true)
	require.NoError(t, xerr)
	// Do not call validateTrx to avoid checking balance,
	// require.NoError(t, validateTrx(txctx))
	require.Error(t, runTrx(txctx))
	require.Equal(t, gas, txctx.GasUsed)
	require.Equal(t, gas, bctx.GetBlockGasUsed())
	fee := types.GasToFee(txctx.GasUsed, txctx.Tx.GasPrice)
	require.Equal(t, new(uint256.Int).Sub(balance0, fee).Dec(), w0.GetBalance().Dec())

	// TEST FOR SUCCEED TX
	// test in CheckTx; expected gas is used
	usedGas0 := bctx.GetBlockGasUsed()
	balance0 = w0.GetBalance()
	amt := uint256.NewInt(1)
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), w0.GetNonce(), gas, govMock.GasPrice(), amt)
	_, _, xerr = w0.SignTrxRLP(tx, chainId.Hex())
	require.NoError(t, xerr)
	txctx, xerr = mocks.MakeTrxCtxWithTrxBctx(tx, bctx, false)
	require.NoError(t, xerr)
	require.NoError(t, validateTrx(txctx))
	require.NoError(t, runTrx(txctx))
	require.Equal(t, gas, txctx.GasUsed)
	require.Equal(t, usedGas0+gas, bctx.GetBlockGasUsed())
	fee = types.GasToFee(txctx.GasUsed, txctx.Tx.GasPrice)
	require.Equal(t, new(uint256.Int).Sub(balance0, new(uint256.Int).Add(fee, amt)).Dec(), w0.GetBalance().Dec())

	// test in DeliverTx; expected gas is used
	usedGas0 = bctx.GetBlockGasUsed()
	balance0 = w0.GetBalance()
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), w0.GetNonce(), gas, govMock.GasPrice(), amt)
	_, _, xerr = w0.SignTrxRLP(tx, chainId.Hex())
	require.NoError(t, xerr)
	txctx, xerr = mocks.MakeTrxCtxWithTrxBctx(tx, bctx, true)
	require.NoError(t, xerr)
	require.NoError(t, validateTrx(txctx))
	require.NoError(t, runTrx(txctx))
	require.Equal(t, gas, txctx.GasUsed)
	require.Equal(t, usedGas0+gas, bctx.GetBlockGasUsed())
	fee = types.GasToFee(txctx.GasUsed, txctx.Tx.GasPrice)
	require.Equal(t, new(uint256.Int).Sub(balance0, new(uint256.Int).Add(fee, amt)).Dec(), w0.GetBalance().Dec())
}

//func Test_Gas_FailedTx_EVM(t *testing.T) {
//
//}

//func Test_AdjustBlockGasLimit(t *testing.T) {
//	w0 := acctMock.RandWallet() //web3.NewWallet(nil)
//	w1 := web3.NewWallet(nil)
//
//	blockGasLimit := int64(5_000_000)
//	blockGasUsed := int64(0)
//	upper := blockGasLimit - (blockGasLimit / 10)
//	//lower := blockGasLimit / 100
//
//	bctx := ctrlertypes.NewBlockContext(
//		abcitypes.RequestBeginBlock{Header: tmtypes.Header{ChainID: chainId.Hex(), Height: 1}},
//		govMock,
//		acctMock,
//		nil, nil, nil)
//	bctx.SetBlockGasLimit(blockGasLimit)
//	require.Equal(t, blockGasLimit, bctx.GetBlockGasLimit())
//	require.Equal(t, blockGasUsed, bctx.GetBlockGasUsed())
//
//	nonce := w0.GetNonce()
//	for {
//		rnGas := rand.Int64N(100_000) + govMock.MinTrxGas()
//		tx := web3.NewTrxTransfer(w0.Address(), w1.Address(), nonce, rnGas, govMock.GasPrice(), uint256.NewInt(1))
//		_, _, xerr := w0.SignTrxRLP(tx, chainId.Hex())
//		require.NoError(t, xerr)
//
//		txctx, xerr := mocks.MakeTrxCtxWithTrxBctx(tx, bctx, true)
//		require.NoError(t, xerr)
//
//		require.NoError(t, validateTrx(txctx))
//		require.NoError(t, runTrx(txctx))
//		require.Equal(t, rnGas, txctx.GasUsed)
//
//		nonce++
//
//		blockGasUsed += rnGas
//
//		require.Equal(t, blockGasLimit, bctx.GetBlockGasLimit())
//		require.Equal(t, blockGasUsed, bctx.GetBlockGasUsed())
//
//		if blockGasUsed > upper {
//			break
//		}
//	}
//
//	expected := blockGasLimit + blockGasLimit/10 // increasing by 10%
//	adjusted := ctrlertypes.AdjustBlockGasLimit(bctx.GetBlockGasLimit(), bctx.GetBlockGasUsed(), govMock.MinTrxGas(), govMock.BlockGasLimit())
//	require.Equal(t, expected, adjusted)
//
//	blockGasLimit = adjusted
//	blockGasUsed = int64(0)
//	lower := blockGasLimit / 100
//
//	bctx = ctrlertypes.NewBlockContext(
//		abcitypes.RequestBeginBlock{Header: tmtypes.Header{ChainID: chainId.Hex(), Height: 1}},
//		govMock,
//		acctMock,
//		nil, nil, nil)
//	bctx.SetBlockGasLimit(blockGasLimit)
//	require.Equal(t, blockGasLimit, bctx.GetBlockGasLimit())
//	require.Equal(t, blockGasUsed, bctx.GetBlockGasUsed())
//
//	for {
//		rnGas := govMock.MinTrxGas()
//		if blockGasUsed+rnGas >= lower {
//			break
//		}
//		tx := web3.NewTrxTransfer(w0.Address(), w1.Address(), nonce, rnGas, govMock.GasPrice(), uint256.NewInt(1))
//		_, _, xerr := w0.SignTrxRLP(tx, chainId.Hex())
//		require.NoError(t, xerr)
//
//		txctx, xerr := mocks.MakeTrxCtxWithTrxBctx(tx, bctx, true)
//		require.NoError(t, xerr)
//
//		require.NoError(t, validateTrx(txctx))
//		require.NoError(t, runTrx(txctx))
//		require.Equal(t, rnGas, txctx.GasUsed)
//
//		nonce++
//
//		blockGasUsed += rnGas
//
//		require.Equal(t, blockGasLimit, bctx.GetBlockGasLimit())
//		require.Equal(t, blockGasUsed, bctx.GetBlockGasUsed())
//	}
//
//	expected = blockGasLimit - blockGasLimit/100 // increasing by 10%
//	adjusted = ctrlertypes.AdjustBlockGasLimit(bctx.GetBlockGasLimit(), bctx.GetBlockGasUsed(), govMock.MinTrxGas(), govMock.BlockGasLimit())
//	require.Equal(t, expected, adjusted)
//}

func Test_Payer(t *testing.T) {
	sender := acctMock.RandWallet()
	amt := uint256.NewInt(rand.Uint64N(sender.GetBalance().Uint64()/2) + 10)
	fmt.Println("sender", sender.Address(), "balance", sender.GetBalance(), "transfer", amt, "fee", govMock.MinTrxFee())

	//
	// Insufficient fund
	payer := web3.NewWallet(nil)
	acctMock.AddWallet(payer) // payer has no balance
	tx := web3.NewTrxTransfer(sender.Address(), types.RandAddress(), sender.GetNonce(), govMock.MinTrxGas(), govMock.GasPrice(), amt)
	_, _, err := sender.SignTrxRLP(tx, chainId.Hex())
	require.NoError(t, err)
	_, _, err = payer.SignPayerTrxRLP(tx, chainId.Hex())
	require.NoError(t, err)
	txctx, xerr := mocks.MakeTrxCtxWithTrx(tx, chainId.Hex(), 1, time.Now(), true, govMock, acctMock, nil, nil, nil)
	require.NoError(t, xerr)
	require.ErrorContains(t, validateTrx(txctx), xerrors.ErrInsufficientFund.Error())

	//
	// Sufficient fund
	_ = payer.GetAccount().AddBalance(uint256.NewInt(balance))
	expectedPayerBalance := payer.GetBalance().Clone()
	_ = expectedPayerBalance.Sub(expectedPayerBalance, govMock.MinTrxFee())
	expectedSenderBalance := sender.GetBalance().Clone()
	_ = expectedSenderBalance.Sub(expectedSenderBalance, amt)

	txctx, xerr = mocks.MakeTrxCtxWithTrx(tx, chainId.Hex(), 1, time.Now(), true, govMock, acctMock, nil, nil, nil)
	require.NoError(t, xerr)
	require.NoError(t, validateTrx(txctx))
	require.NoError(t, runTrx(txctx))

	actualPayer := acctMock.FindAccount(payer.Address(), true)
	require.Equal(t, expectedPayerBalance.Dec(), actualPayer.GetBalance().Dec())
	actualSender := acctMock.FindAccount(sender.Address(), true)
	require.Equal(t, expectedSenderBalance.Dec(), actualSender.GetBalance().Dec())

	//
	// No Payer: sender should pay tx fee
	amt = uint256.NewInt(rand.Uint64N(sender.GetBalance().Uint64()/2) + 10)
	fmt.Println("sender", sender.Address(), "balance", sender.GetBalance(), "transfer", amt, "fee", govMock.MinTrxFee())

	expectedSenderBalance = sender.GetBalance().Clone()
	_ = expectedSenderBalance.Sub(expectedSenderBalance, amt)
	_ = expectedSenderBalance.Sub(expectedSenderBalance, govMock.MinTrxFee()) // pay tx fee

	tx = web3.NewTrxTransfer(sender.Address(), types.RandAddress(), sender.GetNonce(), govMock.MinTrxGas(), govMock.GasPrice(), amt)
	_, _, err = sender.SignTrxRLP(tx, chainId.Hex())
	require.NoError(t, err)
	txctx, xerr = mocks.MakeTrxCtxWithTrx(tx, chainId.Hex(), 1, time.Now(), true, govMock, acctMock, nil, nil, nil)
	require.NoError(t, xerr)
	require.EqualValues(t, txctx.Sender.Address, txctx.Payer.Address)
	require.NoError(t, validateTrx(txctx))
	require.NoError(t, runTrx(txctx))

	actualSender = acctMock.FindAccount(sender.Address(), true)
	require.Equal(t, expectedSenderBalance.Dec(), actualSender.GetBalance().Dec())
}

func Test_RunTrx_Rollback(t *testing.T) {
	t.Run("failed_non_evm", func(t *testing.T) {
		sender := web3.NewWallet(nil)
		receiver := web3.NewWallet(nil)
		gas := govMock.MinTrxGas()
		amt := uint256.NewInt(100)
		senderBalance := uint256.NewInt(balance)

		acctHandler := newFailingAcctHandler(
			accountWithBalance(sender.Address(), senderBalance),
			accountWithBalance(receiver.Address(), uint256.NewInt(0)),
		)
		acctHandler.executeErr = xerrors.ErrInvalidTrx
		bctx := mocks.InitBlockCtxWith(chainId.Hex(), 1, govMock, acctHandler, nil, nil, nil)
		bctx.SetBlockGasLimit(gas * 10)

		tx := web3.NewTrxTransfer(sender.Address(), receiver.Address(), 0, gas, govMock.GasPrice(), amt)
		_, _, err := sender.SignTrxRLP(tx, chainId.Hex())
		require.NoError(t, err)
		txctx, xerr := mocks.MakeTrxCtxWithTrxBctx(tx, bctx, true)
		require.NoError(t, xerr)

		xerr = runTrx(txctx)
		require.Error(t, xerr)
		require.Equal(t, 1, acctHandler.SnapshotCount())
		require.Equal(t, 1, acctHandler.RevertCount())
		require.Equal(t, gas, txctx.GasUsed)
		require.Equal(t, gas, bctx.GetBlockGasUsed())

		fee := types.GasToFee(gas, govMock.GasPrice())
		expectedSenderBalance := new(uint256.Int).Sub(senderBalance, fee)
		actualSender := acctHandler.FindAccount(sender.Address(), true)
		require.Equal(t, expectedSenderBalance.Dec(), actualSender.GetBalance().Dec())
		require.Equal(t, int64(1), actualSender.GetNonce())

		actualReceiver := acctHandler.FindAccount(receiver.Address(), true)
		require.Equal(t, "0", actualReceiver.GetBalance().Dec())
	})

	t.Run("rollback_panic", func(t *testing.T) {
		sender := web3.NewWallet(nil)
		receiver := web3.NewWallet(nil)
		gas := govMock.MinTrxGas()

		acctHandler := newFailingAcctHandler(
			accountWithBalance(sender.Address(), uint256.NewInt(balance)),
			accountWithBalance(receiver.Address(), uint256.NewInt(0)),
		)
		acctHandler.executeErr = xerrors.ErrInvalidTrx
		acctHandler.revertErr = xerrors.ErrInvalidSnapshot
		bctx := mocks.InitBlockCtxWith(chainId.Hex(), 1, govMock, acctHandler, nil, nil, nil)
		bctx.SetBlockGasLimit(gas * 10)

		tx := web3.NewTrxTransfer(sender.Address(), receiver.Address(), 0, gas, govMock.GasPrice(), uint256.NewInt(100))
		_, _, err := sender.SignTrxRLP(tx, chainId.Hex())
		require.NoError(t, err)
		txctx, xerr := mocks.MakeTrxCtxWithTrxBctx(tx, bctx, true)
		require.NoError(t, xerr)

		require.Panics(t, func() {
			_ = runTrx(txctx)
		})
		require.Equal(t, int64(0), bctx.GetBlockGasUsed())
	})

	t.Run("evm_skip", func(t *testing.T) {
		sender := web3.NewWallet(nil)
		receiver := web3.NewWallet(nil)
		gas := govMock.MinTrxGas()
		receiverAcct := accountWithBalance(receiver.Address(), uint256.NewInt(0))
		receiverAcct.SetCode([]byte{0x01})

		acctHandler := newFailingAcctHandler(
			accountWithBalance(sender.Address(), uint256.NewInt(balance)),
			receiverAcct,
		)
		evmHandler := &errEVMHandler{xerr: xerrors.ErrInvalidTrx}
		bctx := mocks.InitBlockCtxWith(chainId.Hex(), 1, govMock, acctHandler, evmHandler, nil, nil)
		bctx.SetBlockGasLimit(gas * 10)

		tx := web3.NewTrxTransfer(sender.Address(), receiver.Address(), 0, gas, govMock.GasPrice(), uint256.NewInt(100))
		_, _, err := sender.SignTrxRLP(tx, chainId.Hex())
		require.NoError(t, err)
		txctx, xerr := mocks.MakeTrxCtxWithTrxBctx(tx, bctx, false)
		require.NoError(t, xerr)

		require.Error(t, runTrx(txctx))
		require.Equal(t, 0, acctHandler.SnapshotCount())
		require.Equal(t, 1, evmHandler.executeCount)
		require.Equal(t, int64(0), bctx.GetBlockGasUsed())
	})
}

type failingAcctHandler struct {
	*acct.AcctHandlerMock
	executeErr xerrors.XError
	revertErr  xerrors.XError
}

func newFailingAcctHandler(accounts ...*ctrlertypes.Account) *failingAcctHandler {
	ret := &failingAcctHandler{
		AcctHandlerMock: acct.NewAcctHandlerMock(0),
	}
	for _, acct := range accounts {
		ret.AddAccount(acct.Clone())
	}
	return ret
}

func accountWithBalance(addr types.Address, balance *uint256.Int) *ctrlertypes.Account {
	acct := ctrlertypes.NewAccount(addr)
	acct.SetBalance(balance)
	return acct
}

func (handler *failingAcctHandler) RevertToSnapshot(snap v1.Snapshot, exec bool) xerrors.XError {
	if handler.revertErr != nil {
		return handler.revertErr
	}
	return handler.AcctHandlerMock.RevertToSnapshot(snap, exec)
}

func (handler *failingAcctHandler) ExecuteTrx(ctx *ctrlertypes.TrxContext) xerrors.XError {
	if xerr := ctx.Sender.SubBalance(ctx.Tx.Amount); xerr != nil {
		return xerr
	}
	if xerr := handler.SetAccount(ctx.Sender, ctx.Exec); xerr != nil {
		return xerr
	}
	if handler.executeErr != nil {
		return handler.executeErr
	}
	if xerr := ctx.Receiver.AddBalance(ctx.Tx.Amount); xerr != nil {
		return xerr
	}
	return handler.SetAccount(ctx.Receiver, ctx.Exec)
}

type errEVMHandler struct {
	ctrlertypes.IEVMHandler
	xerr         xerrors.XError
	executeCount int
}

func (handler *errEVMHandler) ExecuteTrx(*ctrlertypes.TrxContext) xerrors.XError {
	handler.executeCount++
	return handler.xerr
}
