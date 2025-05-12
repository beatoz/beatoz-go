package stake

import (
	"github.com/beatoz/beatoz-go/ctrlers/stake"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

type StakeHandlerMock struct {
	ValCnt     int
	Delegatees []*stake.Delegatee
}

func NewStakeHandlerMock(valCnt int, delegatees []*stake.Delegatee) *StakeHandlerMock {
	return &StakeHandlerMock{
		ValCnt:     valCnt,
		Delegatees: delegatees,
	}
}

func (s *StakeHandlerMock) Validators() ([]*abcitypes.Validator, int64) {
	totalPower := int64(0)
	vals := make([]*abcitypes.Validator, s.ValCnt)
	for i := 0; i < s.ValCnt; i++ {
		vals[i] = &abcitypes.Validator{
			Address: s.Delegatees[i].Addr,
			Power:   s.Delegatees[i].TotalPower,
		}
		totalPower += s.Delegatees[i].TotalPower
	}
	return vals, totalPower
}

func (s *StakeHandlerMock) IsValidator(addr types.Address) bool {
	for i := 0; i < s.ValCnt; i++ {
		if bytes.Compare(addr, s.Delegatees[i].Addr) == 0 {
			return true
		}
	}
	return false
}

func (s *StakeHandlerMock) GetTotalAmount() *uint256.Int {
	return ctrlertypes.PowerToAmount(s.GetTotalPower())
}

func (s *StakeHandlerMock) GetTotalPower() int64 {
	sum := int64(0)
	for _, v := range s.Delegatees {
		sum += v.TotalPower
	}
	return sum
}

func (s *StakeHandlerMock) TotalPowerOf(addr types.Address) int64 {
	for _, v := range s.Delegatees {
		if bytes.Compare(addr, v.Addr) == 0 {
			return v.TotalPower
		}
	}
	return int64(0)
}

func (s *StakeHandlerMock) SelfPowerOf(addr types.Address) int64 {
	return 0
}

func (s *StakeHandlerMock) DelegatedPowerOf(addr types.Address) int64 {
	return 0
}

func (s *StakeHandlerMock) PickAddress(i int) types.Address {
	return s.Delegatees[i].Addr
}

func (s *StakeHandlerMock) ComputeWeight(height, ripeningBlocks int64, tau int32, totalSupply *uint256.Int) (*ctrlertypes.Weight, xerrors.XError) {
	return nil, nil
}

func (s *StakeHandlerMock) BeginBlock(context *ctrlertypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	//TODO implement me
	panic("implement me")
}

func (s *StakeHandlerMock) EndBlock(context *ctrlertypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	//TODO implement me
	panic("implement me")
}

func (s *StakeHandlerMock) ValidateTrx(context *ctrlertypes.TrxContext) xerrors.XError {
	//TODO implement me
	panic("implement me")
}

func (s *StakeHandlerMock) ExecuteTrx(context *ctrlertypes.TrxContext) xerrors.XError {
	//TODO implement me
	panic("implement me")
}

var _ ctrlertypes.IStakeHandler = (*StakeHandlerMock)(nil)

//type acctHelperMock struct {
//	acctMap map[ctrlertypes.AcctKey]*ctrlertypes.Account
//}
//
//func (a *acctHelperMock) FindOrNewAccount(addr types.Address, exec bool) *ctrlertypes.Account {
//	acctKey := ctrlertypes.ToAcctKey(addr)
//	if acct, ok := a.acctMap[acctKey]; ok {
//		return acct
//	} else {
//		acct = ctrlertypes.NewAccount(addr)
//		acct.AddBalance(uint256.NewInt(100000))
//		a.acctMap[acctKey] = acct
//		return acct
//	}
//}
//
//func (a *acctHelperMock) FindAccount(addr types.Address, exec bool) *ctrlertypes.Account {
//	return a.FindOrNewAccount(addr, exec)
//}
//
//func (a *acctHelperMock) Transfer(address types.Address, address2 types.Address, u *uint256.Int, b bool) xerrors.XError {
//	panic("implement me")
//}
//
//func (a *acctHelperMock) Reward(address types.Address, u *uint256.Int, b bool) xerrors.XError {
//	panic("implement me")
//}
//
//func (a *acctHelperMock) ImmutableAcctCtrlerAt(i int64) (ctrlertypes.IAccountHandler, xerrors.XError) {
//	panic("implement me")
//}
//
//func (a *acctHelperMock) SimuAcctCtrlerAt(i int64) (ctrlertypes.IAccountHandler, xerrors.XError) {
//	panic("implement me")
//}
//
//func (a *acctHelperMock) SetAccount(account *ctrlertypes.Account, b bool) xerrors.XError {
//	panic("implement me")
//}
//
//var _ ctrlertypes.IAccountHandler = (*acctHelperMock)(nil)
