package gov

import (
	"github.com/beatoz/beatoz-go/ctrlers/stake"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"time"
)

type stakeHandlerMock struct {
	valCnt     int
	delegatees []*stake.Delegatee
}

func (s *stakeHandlerMock) Validators() ([]*abcitypes.Validator, int64) {
	totalPower := int64(0)
	vals := make([]*abcitypes.Validator, s.valCnt)
	for i := 0; i < s.valCnt; i++ {
		vals[i] = &abcitypes.Validator{
			Address: s.delegatees[i].Addr,
			Power:   s.delegatees[i].TotalPower,
		}
		totalPower += s.delegatees[i].TotalPower
	}
	return vals, totalPower
}

func (s *stakeHandlerMock) IsValidator(addr types.Address) bool {
	for i := 0; i < s.valCnt; i++ {
		if bytes.Compare(addr, s.delegatees[i].Addr) == 0 {
			return true
		}
	}
	return false
}

func (s *stakeHandlerMock) GetTotalAmount() *uint256.Int {
	return ctrlertypes.PowerToAmount(s.GetTotalPower())
}

func (s *stakeHandlerMock) GetTotalPower() int64 {
	sum := int64(0)
	for _, v := range s.delegatees {
		sum += v.TotalPower
	}
	return sum
}

func (s *stakeHandlerMock) TotalPowerOf(addr types.Address) int64 {
	for _, v := range s.delegatees {
		if bytes.Compare(addr, v.Addr) == 0 {
			return v.TotalPower
		}
	}
	return int64(0)
}

func (s *stakeHandlerMock) SelfPowerOf(addr types.Address) int64 {
	return 0
}

func (s *stakeHandlerMock) DelegatedPowerOf(addr types.Address) int64 {
	return 0
}

func (s *stakeHandlerMock) PickAddress(i int) types.Address {
	return s.delegatees[i].Addr
}

var _ ctrlertypes.IStakeHandler = (*stakeHandlerMock)(nil)

type acctHelperMock struct {
	acctMap map[ctrlertypes.AcctKey]*ctrlertypes.Account
}

func (a *acctHelperMock) FindOrNewAccount(addr types.Address, exec bool) *ctrlertypes.Account {
	acctKey := ctrlertypes.ToAcctKey(addr)
	if acct, ok := a.acctMap[acctKey]; ok {
		return acct
	} else {
		acct = ctrlertypes.NewAccount(addr)
		acct.AddBalance(uint256.NewInt(100000))
		a.acctMap[acctKey] = acct
		return acct
	}
}

func (a *acctHelperMock) FindAccount(addr types.Address, exec bool) *ctrlertypes.Account {
	return a.FindOrNewAccount(addr, exec)
}

func (a *acctHelperMock) Transfer(address types.Address, address2 types.Address, u *uint256.Int, b bool) xerrors.XError {
	//TODO implement me
	panic("implement me")
}

func (a *acctHelperMock) Reward(address types.Address, u *uint256.Int, b bool) xerrors.XError {
	//TODO implement me
	panic("implement me")
}

func (a *acctHelperMock) ImmutableAcctCtrlerAt(i int64) (ctrlertypes.IAccountHandler, xerrors.XError) {
	//TODO implement me
	panic("implement me")
}

func (a *acctHelperMock) SimuAcctCtrlerAt(i int64) (ctrlertypes.IAccountHandler, xerrors.XError) {
	//TODO implement me
	panic("implement me")
}

func (a *acctHelperMock) SetAccount(account *ctrlertypes.Account, b bool) xerrors.XError {
	//TODO implement me
	panic("implement me")
}

var _ ctrlertypes.IAccountHandler = (*acctHelperMock)(nil)

func makeTrxCtx(tx *ctrlertypes.Trx, height int64, exec bool) *ctrlertypes.TrxContext {
	txbz, _ := tx.Encode()
	txctx, xerr := ctrlertypes.NewTrxContext(txbz, height, time.Now().Unix(), exec, func(_txctx *ctrlertypes.TrxContext) xerrors.XError {
		_tx := _txctx.Tx
		// find sender account
		acct := acctHelper.FindAccount(_tx.From, _txctx.Exec)
		if acct == nil {
			return xerrors.ErrNotFoundAccount
		}
		_txctx.Sender = acct
		_txctx.GovHandler = govCtrler
		_txctx.AcctHandler = acctHelper
		_txctx.StakeHandler = stakeHelper
		return nil
	})
	if xerr != nil {
		panic(xerr)
	}

	return txctx
}

func runCase(c *Case) xerrors.XError {
	return runTrx(c.txctx)
}

func runTrx(ctx *ctrlertypes.TrxContext) xerrors.XError {
	if xerr := govCtrler.ValidateTrx(ctx); xerr != nil {
		return xerr
	}
	if xerr := govCtrler.ExecuteTrx(ctx); xerr != nil {
		return xerr
	}
	return nil
}
