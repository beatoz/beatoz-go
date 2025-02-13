package node

import (
	"fmt"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	"math/rand/v2"
	"testing"
	"time"
)

var (
	chainId     = "test-trx-executor-chain"
	govParams   = ctrlertypes.DefaultGovParams()
	acctHandler = &acctHandlerMock{}
)

func Test_commonValidation(t *testing.T) {
	w0 := web3.NewWallet(nil)
	w1 := web3.NewWallet(nil)

	//
	// Invalid nonce
	tx := web3.NewTrxTransfer(w0.Address(), w1.Address(), 1, govParams.MinTrxGas(), govParams.GasPrice(), uint256.NewInt(1000))
	_, _, _ = w0.SignTrxRLP(tx, chainId)
	bztx, _ := tx.Encode()
	txctx, xerr := newTrxCtx(bztx, ctrlertypes.NewBlockContextAs(1, time.Now(), chainId, govParams, acctHandler, nil))
	require.NoError(t, xerr)
	require.ErrorContains(t, commonValidation(txctx), xerrors.ErrInvalidNonce.Error())

	//
	// Insufficient fund
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govParams.MinTrxGas(), govParams.GasPrice(), uint256.NewInt(1001))
	_, _, _ = w0.SignTrxRLP(tx, chainId)
	bztx, _ = tx.Encode()
	txctx, xerr = newTrxCtx(bztx, ctrlertypes.NewBlockContextAs(1, time.Now(), chainId, govParams, acctHandler, nil))
	require.NoError(t, xerr)
	require.ErrorContains(t, commonValidation(txctx), xerrors.ErrInsufficientFund.Error())
}

// test gas pool
func Test_GasPool(t *testing.T) {
	w0 := web3.NewWallet(nil)
	w1 := web3.NewWallet(nil)

	//
	// Too much gas
	tx := web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govParams.MaxTrxGas()+1, govParams.GasPrice(), uint256.NewInt(1000))
	_, _, err := w0.SignTrxRLP(tx, chainId)
	require.NoError(t, err)
	bztx, xerr := tx.Encode()
	require.NoError(t, xerr)
	_, xerr = newTrxCtx(bztx, ctrlertypes.NewBlockContextAs(1, time.Now(), chainId, govParams, acctHandler, nil))
	require.ErrorContains(t, xerr, xerrors.ErrInvalidGas.Error())

	//
	// Too small gas
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govParams.MinTrxGas()-1, govParams.GasPrice(), uint256.NewInt(1000))
	_, _, err = w0.SignTrxRLP(tx, chainId)
	require.NoError(t, err)
	bztx, xerr = tx.Encode()
	require.NoError(t, xerr)
	_, xerr = newTrxCtx(bztx, ctrlertypes.NewBlockContextAs(1, time.Now(), chainId, govParams, acctHandler, nil))
	require.ErrorContains(t, xerr, xerrors.ErrInvalidGas.Error())

	//
	// proper gas (test gas pool)
	bctx0 := ctrlertypes.NewBlockContextAs(1, time.Now(), chainId, govParams, acctHandler, nil)
	require.Equal(t, bctx0.GetTrxGasLimit(), govParams.MaxTrxGas())
	require.Equal(t, bctx0.GetBlockGasLimit(), govParams.MaxBlockGas())

	gasSum := uint64(0)
	for i := 0; i < 100; i++ {
		w0 = web3.NewWallet(nil)
		w1 = web3.NewWallet(nil)

		gas := govParams.MinTrxGas() + rand.Uint64N(1000000)
		amt := uint256.NewInt(rand.Uint64N(1000) + 1000)
		tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, gas, govParams.GasPrice(), amt)
		_, _, err = w0.SignTrxRLP(tx, chainId)
		require.NoError(t, err)
		bztx, xerr = tx.Encode()
		require.NoError(t, xerr)
		txctx, xerr := newTrxCtx(bztx, bctx0)
		require.NoError(t, xerr)

		require.NoError(t, validateTrx(txctx))
		require.NoError(t, runTrx(txctx))

		require.Equal(t, gas, txctx.GasUsed)
		require.Equal(t, gasSum+gas, txctx.BlockContext.GasUsed())
		require.Equal(t, txctx.BlockContext.GetBlockGasLimit()-gasSum-gas, txctx.BlockContext.BlockGasRemained())

		//fmt.Println("gas             ", gas)
		//fmt.Println("trxGasLimit     ", txctx.BlockContext.GetTrxGasLimit())
		//fmt.Println("blockGasLimit   ", txctx.BlockContext.GetBlockGasLimit())
		//fmt.Println("blockGasRemained", txctx.BlockContext.BlockGasRemained())

		gasSum += gas
	}

	bctx1 := ctrlertypes.NewBlockContextAs(2, time.Now(), chainId, govParams, acctHandler, nil)
	bctx1.AdjustTrxGasLimit(bctx0.GetTxsCnt(), govParams.MinTrxGas(), govParams.MaxTrxGas())
	require.NotEqual(t, bctx1.GetTrxGasLimit(), govParams.MaxTrxGas())
	require.Equal(t, bctx1.GetBlockGasLimit(), govParams.MaxBlockGas())

	//fmt.Println("trxGasLimit     ", bctx1.GetTrxGasLimit())
	//fmt.Println("blockGasLimit   ", bctx1.GetBlockGasLimit())
	//fmt.Println("blockGasRemained", bctx1.BlockGasRemained())

	gasSum = uint64(0)
	for i := 0; ; i++ {
		w0 = web3.NewWallet(nil)
		w1 = web3.NewWallet(nil)

		gas := bctx1.GetTrxGasLimit()
		amt := uint256.NewInt(1)
		tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, gas, govParams.GasPrice(), amt)
		_, _, err = w0.SignTrxRLP(tx, chainId)
		require.NoError(t, err)
		bztx, xerr = tx.Encode()
		require.NoError(t, xerr)
		txctx, xerr := newTrxCtx(bztx, bctx1)
		require.NoError(t, xerr)

		require.NoError(t, validateTrx(txctx))
		require.NoError(t, runTrx(txctx))

		require.Equal(t, gas, txctx.GasUsed)
		require.Equal(t, gasSum+gas, txctx.BlockContext.GasUsed())
		require.Equal(t, txctx.BlockContext.GetBlockGasLimit()-gasSum-gas, txctx.BlockContext.BlockGasRemained())

		//fmt.Println("gas             ", gas)
		//fmt.Println("trxGasLimit     ", txctx.BlockContext.GetTrxGasLimit())
		//fmt.Println("blockGasLimit   ", txctx.BlockContext.GetBlockGasLimit())
		//fmt.Println("blockGasRemained", txctx.BlockContext.BlockGasRemained())

		gasSum += gas

		if bctx1.BlockGasRemained() < bctx1.GetTrxGasLimit() {
			fmt.Printf("blockGasRemained(%v) is less than trxGasLimit(%v) at tx[%v]\n",
				bctx1.BlockGasRemained(), bctx1.GetTrxGasLimit(), i)
			break
		}
	}

	// At now, bctx1.BlockGasRemained() is less than bctx1.GetTrxGasLimit()
	// When the tx that has bctx1.GetTrxGasLimit() as gas is executed,
	// it should be fail.

	gas := bctx1.GetTrxGasLimit()
	amt := uint256.NewInt(1)
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, gas, govParams.GasPrice(), amt)
	_, _, err = w0.SignTrxRLP(tx, chainId)
	require.NoError(t, err)
	bztx, xerr = tx.Encode()
	require.NoError(t, xerr)
	txctx, xerr := newTrxCtx(bztx, bctx1)
	require.NoError(t, xerr)

	require.NoError(t, validateTrx(txctx))
	require.Error(t, runTrx(txctx))
}

func newTrxCtx(bztx []byte, bctx *ctrlertypes.BlockContext) (*ctrlertypes.TrxContext, xerrors.XError) {
	return ctrlertypes.NewTrxContext(
		bztx,
		bctx,
		true,
		func(txctx *ctrlertypes.TrxContext) xerrors.XError {
			txctx.TrxAcctHandler = acctHandler
			return nil
		})
}

type acctHandlerMock struct{}

func (a *acctHandlerMock) ValidateTrx(ctx *ctrlertypes.TrxContext) xerrors.XError {
	return nil
}

func (a *acctHandlerMock) ExecuteTrx(ctx *ctrlertypes.TrxContext) xerrors.XError {
	return nil
}

var _ ctrlertypes.IAccountHandler = (*acctHandlerMock)(nil)
var _ ctrlertypes.ITrxHandler = (*acctHandlerMock)(nil)

func (a *acctHandlerMock) FindOrNewAccount(address types.Address, b bool) *ctrlertypes.Account {
	return a.FindAccount(address, b)
}

func (a *acctHandlerMock) FindAccount(address types.Address, b bool) *ctrlertypes.Account {
	acct := ctrlertypes.NewAccount(address)
	acct.AddBalance(govParams.MinTrxFee())
	acct.AddBalance(uint256.NewInt(1_000_000_000_000_000_000))
	return acct
}

func (a *acctHandlerMock) Transfer(address types.Address, address2 types.Address, u *uint256.Int, b bool) xerrors.XError {
	panic("implement me")
}

func (a *acctHandlerMock) Reward(address types.Address, u *uint256.Int, b bool) xerrors.XError {
	panic("implement me")
}

func (a *acctHandlerMock) SimuAcctCtrlerAt(i int64) (ctrlertypes.IAccountHandler, xerrors.XError) {
	panic("implement me")
}
func (a *acctHandlerMock) SetAccount(account *ctrlertypes.Account, b bool) xerrors.XError {
	return nil
}
