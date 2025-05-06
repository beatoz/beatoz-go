package mocks

//
//import (
//	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
//	"github.com/beatoz/beatoz-go/ctrlers/vpower"
//	"github.com/beatoz/beatoz-go/types"
//	"github.com/beatoz/beatoz-go/types/bytes"
//	"github.com/beatoz/beatoz-go/types/xerrors"
//	"github.com/holiman/uint256"
//	abcitypes "github.com/tendermint/tendermint/abci/types"
//)
//
//type VPowerHandlerMock struct {
//	valCnt     int
//	delegatees []*vpower.Delegatee
//}
//
//func (s *VPowerHandlerMock) Validators() ([]*abcitypes.Validator, int64) {
//	totalPower := int64(0)
//	vals := make([]*abcitypes.Validator, s.valCnt)
//	for i := 0; i < s.valCnt; i++ {
//		vals[i] = &abcitypes.Validator{
//			Address: s.delegatees[i].Address(),
//			Power:   s.delegatees[i].SumPower,
//		}
//		totalPower += s.delegatees[i].SumPower
//	}
//	return vals, totalPower
//}
//
//func (s *VPowerHandlerMock) IsValidator(addr types.Address) bool {
//	for i := 0; i < s.valCnt; i++ {
//		if bytes.Compare(addr, s.delegatees[i].Address()) == 0 {
//			return true
//		}
//	}
//	return false
//}
//
//func (s *VPowerHandlerMock) GetTotalAmount() *uint256.Int {
//	return ctrlertypes.PowerToAmount(s.GetTotalPower())
//}
//
//func (s *VPowerHandlerMock) GetTotalPower() int64 {
//	sum := int64(0)
//	for _, v := range s.delegatees {
//		sum += v.SumPower
//	}
//	return sum
//}
//
//func (s *VPowerHandlerMock) TotalPowerOf(addr types.Address) int64 {
//	for _, v := range s.delegatees {
//		if bytes.Compare(addr, v.Address()) == 0 {
//			return v.SumPower
//		}
//	}
//	return int64(0)
//}
//
//func (s *VPowerHandlerMock) SelfPowerOf(addr types.Address) int64 {
//	return 0
//}
//
//func (s *VPowerHandlerMock) DelegatedPowerOf(addr types.Address) int64 {
//	return 0
//}
//
//func (s *VPowerHandlerMock) PickAddress(i int) types.Address {
//	return s.delegatees[i].Address()
//}
//
//var _ ctrlertypes.IStakeHandler = (*VPowerHandlerMock)(nil)
//
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
