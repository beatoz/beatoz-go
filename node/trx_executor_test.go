package node

import (
	"fmt"
	"math/rand/v2"
	"testing"
	"time"

	btzcfg "github.com/beatoz/beatoz-go/cmd/config"
	"github.com/beatoz/beatoz-go/ctrlers/account"
	"github.com/beatoz/beatoz-go/ctrlers/mocks"
	"github.com/beatoz/beatoz-go/ctrlers/mocks/acct"
	"github.com/beatoz/beatoz-go/ctrlers/mocks/gov"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
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

type failAfterExecuteAcctHandler struct {
	ctrlertypes.IAccountHandler
}

func (handler *failAfterExecuteAcctHandler) ExecuteTrx(ctx *ctrlertypes.TrxContext) xerrors.XError {
	if xerr := handler.IAccountHandler.ExecuteTrx(ctx); xerr != nil {
		return xerr
	}
	return xerrors.ErrInvalidTrx.Wrapf("forced account execute failure")
}

func (handler *failAfterExecuteAcctHandler) CacheHandlerContext(exec bool) (ctrlertypes.IAccountHandler, func() xerrors.XError) {
	cached, writeCache := handler.IAccountHandler.(ctrlertypes.ICacheableAccountHandler).CacheHandlerContext(exec)
	return &failAfterExecuteAcctHandler{IAccountHandler: cached}, writeCache
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

func TestTxCacheSequence(t *testing.T) {
	localConfig := btzcfg.DefaultConfig(chainId.Hex())
	localConfig.SetRoot(t.TempDir())

	acctCtrler, err := account.NewAcctCtrler(localConfig, log.NewNopLogger())
	require.NoError(t, err)
	defer func() {
		if acctCtrler != nil {
			require.NoError(t, acctCtrler.Close())
		}
	}()

	sender := web3.NewWallet(nil)
	receiver := web3.NewWallet(nil)
	initialBalance := uint256.NewInt(balance)
	sender.GetAccount().SetBalance(initialBalance)
	receiver.GetAccount().SetBalance(uint256.NewInt(0))
	require.NoError(t, acctCtrler.SetAccount(sender.GetAccount(), true))
	require.NoError(t, acctCtrler.SetAccount(receiver.GetAccount(), true))
	_, _, xerr := acctCtrler.Commit()
	require.NoError(t, xerr)

	bctx := ctrlertypes.TempBlockContext(localConfig.ChainIdHex(), 2, time.Now(), govMock, acctCtrler, nil, nil, nil)
	txe := NewTrxExecutor(log.NewNopLogger())

	gas := govMock.MinTrxGas()
	fee := types.GasToFee(gas, govMock.GasPrice())
	successAmt := uint256.NewInt(1000)
	rollbackAmt := uint256.NewInt(2000)

	tx1 := web3.NewTrxTransfer(sender.Address(), receiver.Address(), 0, gas, govMock.GasPrice(), successAmt)
	_, _, err = sender.SignTrxRLP(tx1, localConfig.ChainIdHex())
	require.NoError(t, err)
	txctx1, xerr := mocks.MakeTrxCtxWithTrxBctx(tx1, bctx, true)
	require.NoError(t, xerr)

	tx2 := web3.NewTrxTransfer(sender.Address(), receiver.Address(), 1, gas, govMock.GasPrice(), rollbackAmt)
	_, _, err = sender.SignTrxRLP(tx2, localConfig.ChainIdHex())
	require.NoError(t, err)
	txctx2, xerr := mocks.MakeTrxCtxWithTrxBctx(tx2, bctx, true)
	require.NoError(t, xerr)

	require.NoError(t, txe.ExecuteSync(txctx1))

	afterTx1Sender := acctCtrler.FindAccount(sender.Address(), true)
	require.NotNil(t, afterTx1Sender)
	require.Equal(t, int64(1), afterTx1Sender.GetNonce())
	require.Equal(t, new(uint256.Int).Sub(initialBalance, new(uint256.Int).Add(successAmt, fee)).Dec(), afterTx1Sender.GetBalance().Dec())
	afterTx1Receiver := acctCtrler.FindAccount(receiver.Address(), true)
	require.NotNil(t, afterTx1Receiver)
	require.Equal(t, successAmt.Dec(), afterTx1Receiver.GetBalance().Dec())

	bctx.AcctHandler = &failAfterExecuteAcctHandler{IAccountHandler: acctCtrler}
	xerr = txe.ExecuteSync(txctx2)
	require.Error(t, xerr)
	require.True(t, xerr.Contains(xerrors.ErrInvalidTrx))

	twoFees := new(uint256.Int).Mul(fee, uint256.NewInt(2))
	expectedSenderBalance := new(uint256.Int).Sub(initialBalance, new(uint256.Int).Add(successAmt, twoFees))
	expectedReceiverBalance := successAmt

	afterTx2Sender := acctCtrler.FindAccount(sender.Address(), true)
	require.NotNil(t, afterTx2Sender)
	require.Equal(t, int64(2), afterTx2Sender.GetNonce())
	require.Equal(t, expectedSenderBalance.Dec(), afterTx2Sender.GetBalance().Dec())
	afterTx2Receiver := acctCtrler.FindAccount(receiver.Address(), true)
	require.NotNil(t, afterTx2Receiver)
	require.Equal(t, expectedReceiverBalance.Dec(), afterTx2Receiver.GetBalance().Dec())

	_, _, xerr = acctCtrler.Commit()
	require.NoError(t, xerr)
	require.NoError(t, acctCtrler.Close())
	acctCtrler = nil

	reopened, err := account.NewAcctCtrler(localConfig, log.NewNopLogger())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, reopened.Close())
	}()

	committedSender := reopened.FindAccount(sender.Address(), true)
	require.NotNil(t, committedSender)
	require.Equal(t, int64(2), committedSender.GetNonce())
	require.Equal(t, expectedSenderBalance.Dec(), committedSender.GetBalance().Dec())
	committedReceiver := reopened.FindAccount(receiver.Address(), true)
	require.NotNil(t, committedReceiver)
	require.Equal(t, expectedReceiverBalance.Dec(), committedReceiver.GetBalance().Dec())
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
