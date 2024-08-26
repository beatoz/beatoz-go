package node

import (
	"fmt"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/libs/web3"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type caseObj struct {
	name     string
	txctx    *ctrlertypes.TrxContext
	expected xerrors.XError
}

var (
	cases     []*caseObj
	govParams = ctrlertypes.DefaultGovParams()
)

func Test_DestZeroAddr(t *testing.T) {
	w0 := web3.NewWallet(nil)

	tx0 := web3.NewTrxTransfer(w0.Address(), types.ZeroAddress(), 0, uint64(99_999), govParams.GasPrice(), uint256.NewInt(0))
	_, _, _ = w0.SignTrxRLP(tx0, "tx_executor_test_chain")
	txctx0 := makeTrxCtx(tx0, 1)
	require.NotNil(t, txctx0.Sender)
	require.Equal(t, txctx0.Sender.Address, w0.Address())
	require.NotNil(t, txctx0.Receiver)
	require.Equal(t, txctx0.Receiver.Address, types.ZeroAddress())
}

func Test_commonValidation(t *testing.T) {
	for i, c := range cases {
		xerr := testCommonValidation(c.txctx)
		if c.expected != nil {
			require.Error(t, xerr, fmt.Sprintf("case #%d", i))
			require.Equal(t, c.expected.Code(), xerr.Code(), fmt.Sprintf("case[%d] %s", i, c.name))
		} else { // success
			require.NoError(t, xerr, fmt.Sprintf("case[%d] %s", i, c.name))
		}
	}
}

func init() {
	w0 := web3.NewWallet(nil)
	w1 := web3.NewWallet(nil)

	//
	// Small Gas
	tx := web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, uint64(99_999), govParams.GasPrice(), uint256.NewInt(0))
	_, _, _ = w0.SignTrxRLP(tx, "tx_executor_test_chain")
	txctx := makeTrxCtx(tx, 1)
	cases = append(cases, &caseObj{"Small Gas", txctx, xerrors.ErrInvalidGas})

	//
	// Wrong GasPrice
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govParams.MinTrxGas(), uint256.NewInt(10_000_000_001), uint256.NewInt(0))
	_, _, _ = w0.SignTrxRLP(tx, "tx_executor_test_chain")
	txctx = makeTrxCtx(tx, 1)
	cases = append(cases, &caseObj{"Wrong GasPrice", txctx, xerrors.ErrInvalidGasPrice})

	//
	// Wrong Signature - sign with proto encoding
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govParams.MinTrxGas(), govParams.GasPrice(), uint256.NewInt(0))
	_, _, _ = w0.SignTrxProto(tx, "tx_executor_test_chain")
	txctx = makeTrxCtx(tx, 1)
	cases = append(cases, &caseObj{"Wrong Signature - sign with proto encoding", txctx, xerrors.ErrInvalidTrxSig})

	//
	// Wrong Signature - no signature
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govParams.MinTrxGas(), govParams.GasPrice(), uint256.NewInt(0))
	txctx = makeTrxCtx(tx, 1)
	cases = append(cases, &caseObj{"Wrong Signature - no signature", txctx, xerrors.ErrInvalidTrxSig})

	//
	// Wrong Signature - other's signature
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govParams.MinTrxGas(), govParams.GasPrice(), uint256.NewInt(0))
	_, _, _ = w1.SignTrxRLP(tx, "tx_executor_test_chain")
	txctx = makeTrxCtx(tx, 1)
	cases = append(cases, &caseObj{"Wrong Signature - other's signature", txctx, xerrors.ErrInvalidTrxSig})

	//
	// Wrong Signature - wrong chainId
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govParams.MinTrxGas(), govParams.GasPrice(), uint256.NewInt(0))
	_, _, _ = w0.SignTrxRLP(tx, "tx_executor_test_chain_wrong")
	txctx = makeTrxCtx(tx, 1)
	cases = append(cases, &caseObj{"Wrong Signature - wrong chainId", txctx, xerrors.ErrInvalidTrxSig})

	//
	// Invalid nonce
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 1, govParams.MinTrxGas(), govParams.GasPrice(), uint256.NewInt(1000))
	_, _, _ = w0.SignTrxRLP(tx, "tx_executor_test_chain")
	txctx = makeTrxCtx(tx, 1)
	cases = append(cases, &caseObj{"Invalid nonce", txctx, xerrors.ErrInvalidNonce})

	//
	// Insufficient fund
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govParams.MinTrxGas(), govParams.GasPrice(), uint256.NewInt(1001))
	_, _, _ = w0.SignTrxRLP(tx, "tx_executor_test_chain")
	txctx = makeTrxCtx(tx, 1)
	cases = append(cases, &caseObj{"Insufficient fund", txctx, xerrors.ErrInsufficientFund})

	//
	// To nil address
	tx = web3.NewTrxTransfer(w0.Address(), nil, 0, govParams.MinTrxGas(), govParams.GasPrice(), uint256.NewInt(1000))
	_, _, _ = w0.SignTrxRLP(tx, "tx_executor_test_chain")
	txctx = makeTrxCtx(tx, 1)
	cases = append(cases, &caseObj{"To nil address", txctx, xerrors.ErrInvalidAddress})

	//
	// To Zero Address
	tx = web3.NewTrxTransfer(w0.Address(), types.ZeroAddress(), 0, govParams.MinTrxGas(), govParams.GasPrice(), uint256.NewInt(1000))
	_, _, _ = w0.SignTrxRLP(tx, "tx_executor_test_chain")
	txctx = makeTrxCtx(tx, 1)
	cases = append(cases, &caseObj{"To Zero Address", txctx, nil})

	//
	// Success
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govParams.MinTrxGas(), govParams.GasPrice(), uint256.NewInt(1000))
	_, _, _ = w0.SignTrxRLP(tx, "tx_executor_test_chain")
	txctx = makeTrxCtx(tx, 1)
	cases = append(cases, &caseObj{"Success", txctx, nil})
}

func makeTrxCtx(tx *ctrlertypes.Trx, height int64) *ctrlertypes.TrxContext {
	bz, _ := tx.Encode()
	txctx, _ := ctrlertypes.NewTrxContext(bz, height, time.Now().UnixMilli(), true, func(_txctx *ctrlertypes.TrxContext) xerrors.XError {
		_txctx.GovHandler = govParams
		_txctx.AcctHandler = &acctHandlerMock{}
		_txctx.TrxAcctHandler = &acctHandlerMock{}
		_txctx.ChainID = "tx_executor_test_chain"
		return nil
	})
	return txctx
}

func testCommonValidation(ctx *ctrlertypes.TrxContext) xerrors.XError {
	if xerr := commonValidation0(ctx); xerr != nil {
		return xerr
	}
	return commonValidation1(ctx)
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
	//TODO implement me
	panic("implement me")
}

func (a *acctHandlerMock) Reward(address types.Address, u *uint256.Int, b bool) xerrors.XError {
	//TODO implement me
	panic("implement me")
}

func (a *acctHandlerMock) ImmutableAcctCtrlerAt(i int64) (ctrlertypes.IAccountHandler, xerrors.XError) {
	//TODO implement me
	panic("implement me")
}

func (a *acctHandlerMock) SetAccountCommittable(account *ctrlertypes.Account, b bool) xerrors.XError {
	//TODO implement me
	panic("implement me")
}

var _ ctrlertypes.ITrxHandler = (*acctHandlerMock)(nil)
var _ ctrlertypes.IAccountHandler = (*acctHandlerMock)(nil)
