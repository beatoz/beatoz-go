package vpower

//
//import (
//	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
//	"github.com/beatoz/beatoz-go/ctrlers/vpower"
//	"github.com/beatoz/beatoz-go/types"
//	"github.com/beatoz/beatoz-go/types/bytes"
//	"github.com/beatoz/beatoz-go/types/crypto"
//	"github.com/beatoz/beatoz-go/types/xerrors"
//	"github.com/holiman/uint256"
//	"github.com/shopspring/decimal"
//	abcitypes "github.com/tendermint/tendermint/abci/types"
//	"math/rand"
//)
//
//type VPowerHandlerMock struct {
//	valCnt     int
//	validators []*vpower.Delegatee
//	vpows      map[string]*vpower.VPower
//	totalPower int64 // key is from_address + to_address
//}
//
//func NewVPowerHandlerMock(valCnt int) *VPowerHandlerMock {
//	validators := make([]*vpower.Delegatee, valCnt)
//	for i := 0; i < valCnt; i++ {
//		_, pub := crypto.NewKeypairBytes()
//		validators[i] = vpower.NewDelegatee(pub)
//
//		vpowCnt := rand.Intn(10000)
//		for j := 0; j < vpowCnt; j++ {
//			vpow := vpower.NewVPower(types.RandAddress(), pub)
//			vpow.PowerChunks
//		}
//	}
//}
//
//func (s *VPowerHandlerMock) ComputeWeight(height, ripeningBlocks, tau int64, totalSupply *uint256.Int) (decimal.Decimal, xerrors.XError) {
//	var powChunks []*vpower.PowerChunkProto
//	for _, val := range s.delegatees {
//		for _, from := range val.Delegators {
//			vpow, xerr := ctrler.readVPower(from, val.addr, true)
//			if xerr != nil {
//				return decimal.Zero, xerr
//			}
//			powChunks = append(powChunks, vpow.PowerChunks...)
//		}
//	}
//
//	wvpow := Weight64ByPowerChunk(powChunks, height, ripeningBlocks, int(tau))
//
//	_totalSupply := decimal.NewFromBigInt(totalSupply.ToBig(), 0).Div(decimal.New(1, int32(types.DECIMAL)))
//	wa, _ := wvpow.QuoRem(_totalSupply, int32(types.DECIMAL))
//	return wa, nil
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
//var _ ctrlertypes.IVPowerHandler = (*VPowerHandlerMock)(nil)
