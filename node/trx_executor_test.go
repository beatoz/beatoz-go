package node

import (
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

var (
	chainId   = "test-trx-executor-chain"
	govParams = ctrlertypes.DefaultGovParams()
)

func Test_commonValidation(t *testing.T) {
	w0 := web3.NewWallet(nil)
	w1 := web3.NewWallet(nil)

	//
	// Invalid nonce
	tx := web3.NewTrxTransfer(w0.Address(), w1.Address(), 1, govParams.MinTrxGas(), govParams.GasPrice(), uint256.NewInt(1000))
	_, _, _ = w0.SignTrxRLP(tx, chainId)
	bztx, _ := tx.Encode()
	txctx, xerr := newTrxCtx(bztx, 1)
	require.NoError(t, xerr)
	require.ErrorContains(t, commonValidation(txctx), xerrors.ErrInvalidNonce.Error())

	//
	// Insufficient fund
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govParams.MinTrxGas(), govParams.GasPrice(), uint256.NewInt(1001))
	_, _, _ = w0.SignTrxRLP(tx, chainId)
	bztx, _ = tx.Encode()
	txctx, xerr = newTrxCtx(bztx, 1)
	require.NoError(t, xerr)
	require.ErrorContains(t, commonValidation(txctx), xerrors.ErrInsufficientFund.Error())
}

func newTrxCtx(bztx []byte, height int64) (*ctrlertypes.TrxContext, xerrors.XError) {
	return ctrlertypes.NewTrxContext(bztx, height, time.Now().UnixMilli(), true, func(_txctx *ctrlertypes.TrxContext) xerrors.XError {
		_txctx.GovHandler = govParams
		_txctx.AcctHandler = &acctHandlerMock{}
		_txctx.ChainID = chainId
		return nil
	})
}

type acctHandlerMock struct{}

var _ ctrlertypes.IAccountHandler = (*acctHandlerMock)(nil)

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

func (a *acctHandlerMock) SimuAcctCtrlerAt(i int64) (ctrlertypes.IAccountHandler, xerrors.XError) {
	panic("implement me")
}
func (a *acctHandlerMock) SetAccount(account *ctrlertypes.Account, b bool) xerrors.XError {
	panic("implement me")
}
