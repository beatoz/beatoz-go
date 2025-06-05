package vpower

import (
	"encoding/hex"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/ctrlers/vpower"
	"github.com/beatoz/beatoz-go/libs/fxnum"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/holiman/uint256"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"math/rand"
	"sort"
)

type VPowerHandlerMock struct {
	ValCnt     int
	Delegatees []*vpower.Delegatee
	validators []*vpower.Delegatee
	mapVPowers map[string]*vpower.VPower
	totalPower int64 // key is from_address + to_address
}

func NewVPowerHandlerMock(dgteeWals []*web3.Wallet, valCnt int) *VPowerHandlerMock {
	valsCnt := valCnt
	delegatees := make([]*vpower.Delegatee, len(dgteeWals))
	mapVPowers := make(map[string]*vpower.VPower)

	sumPower := int64(0)
	for i, w := range dgteeWals {
		dgtee := vpower.NewDelegatee(w.GetPubKey())
		delegatees[i] = dgtee

		pow := rand.Int63n(1_000_000) + 1_000_000
		vpow := vpower.NewVPower(w.Address(), w.Address()) // self bonding
		vpow.AddPowerWithTxHash(pow, 1, bytes.ZeroBytes(32))

		mapVPowers[w.Address().String()+dgtee.Address().String()] = vpow
		dgtee.AddPower(w.Address(), pow)
		dgtee.AddDelegator(w.Address())

		sumPower += pow
	}
	sort.Slice(delegatees, func(i, j int) bool {
		return delegatees[i].SumPower > delegatees[j].SumPower
	})

	return &VPowerHandlerMock{
		ValCnt:     valsCnt,
		Delegatees: delegatees,
		validators: delegatees[:valCnt],
		mapVPowers: mapVPowers,
		totalPower: sumPower,
	}
}

func NewVPowerHandlerMockWithPower(dgteeWals []*web3.Wallet, valCnt int, powerPerVal int64) *VPowerHandlerMock {
	valsCnt := valCnt
	delegatees := make([]*vpower.Delegatee, len(dgteeWals))
	mapVPowers := make(map[string]*vpower.VPower)

	sumPower := int64(0)
	for i, w := range dgteeWals {
		dgtee := vpower.NewDelegatee(w.GetPubKey())
		delegatees[i] = dgtee

		pow := powerPerVal
		vpow := vpower.NewVPower(w.Address(), w.Address()) // self bonding
		vpow.AddPowerWithTxHash(pow, 1, bytes.ZeroBytes(32))

		mapVPowers[w.Address().String()+dgtee.Address().String()] = vpow
		dgtee.AddPower(w.Address(), pow)
		dgtee.AddDelegator(w.Address())

		sumPower += pow
	}
	sort.Slice(delegatees, func(i, j int) bool {
		return delegatees[i].SumPower > delegatees[j].SumPower
	})

	return &VPowerHandlerMock{
		ValCnt:     valsCnt,
		Delegatees: delegatees,
		validators: delegatees[:valCnt],
		mapVPowers: mapVPowers,
		totalPower: sumPower,
	}
}

func (mock *VPowerHandlerMock) Validators() ([]*abcitypes.Validator, int64) {
	totalPower := int64(0)
	vals := make([]*abcitypes.Validator, len(mock.validators))
	for i, v := range mock.validators {
		vals[i] = &abcitypes.Validator{
			Address: v.Address(),
			Power:   v.SumPower,
		}
		totalPower += mock.Delegatees[i].SumPower
	}
	return vals, totalPower
}

func (mock *VPowerHandlerMock) IsValidator(addr types.Address) bool {
	for _, v := range mock.validators {
		if bytes.Compare(v.Address(), addr) == 0 {
			return true
		}
	}
	return false
}

func (mock *VPowerHandlerMock) GetTotalAmount() *uint256.Int {
	return types.PowerToAmount(mock.GetTotalPower())
}

func (mock *VPowerHandlerMock) GetTotalPower() int64 {
	sum := int64(0)
	for _, v := range mock.Delegatees {
		sum += v.SumPower
	}
	return sum
}

func (mock *VPowerHandlerMock) TotalPowerOf(addr types.Address) int64 {
	for _, v := range mock.Delegatees {
		if bytes.Compare(addr, v.Address()) == 0 {
			return v.SumPower
		}
	}
	return int64(0)
}

func (mock *VPowerHandlerMock) SelfPowerOf(addr types.Address) int64 {
	panic("implement me")
}

func (mock *VPowerHandlerMock) DelegatedPowerOf(addr types.Address) int64 {
	panic("implement me")
}

func (mock *VPowerHandlerMock) PickAddress(i int) types.Address {
	return mock.Delegatees[i].Address()
}

func (mock *VPowerHandlerMock) ComputeWeight(height, inflationCycle, ripeningBlocks int64, tau int32, totalSupply *uint256.Int) (ctrlertypes.IWeightResult, xerrors.XError) {
	mapWeightObjs := make(map[string]*struct {
		isval bool
		w     fxnum.FxNum
	})

	for k, vpow := range mock.mapVPowers {
		fromAddrStr := k[:40]
		toAddrStr := k[40:]
		toAddr, _ := hex.DecodeString(toAddrStr)
		isVal := false
		for _, val := range mock.validators {
			if bytes.Equal(val.Address(), toAddr) {
				isVal = true
				break
			}
		}
		if !isVal {
			// toAddr is not validator
			continue
		}
		_w := vpower.FxNumWeightOfPowerChunks(vpow.PowerChunks, height, ripeningBlocks, tau, totalSupply)
		wobj, ok := mapWeightObjs[fromAddrStr]
		if !ok {
			wobj = &struct {
				isval bool
				w     fxnum.FxNum
			}{
				isval: fromAddrStr == toAddrStr, // reach at here when toAddrStr is validator
				w:     _w,
			}
		} else {
			wobj.w = wobj.w.Add(_w)
		}
		mapWeightObjs[fromAddrStr] = wobj
	}

	weightInfo := vpower.NewWeight()
	for k, wo := range mapWeightObjs {
		addr, _ := hex.DecodeString(k)
		weightInfo.Add(addr, wo.w, fxnum.FromInt(1), wo.isval)
	}

	return weightInfo, nil
}

func (mock *VPowerHandlerMock) ValidateTrx(context *ctrlertypes.TrxContext) xerrors.XError {
	//TODO implement me
	panic("implement me")
}

func (mock *VPowerHandlerMock) ExecuteTrx(context *ctrlertypes.TrxContext) xerrors.XError {
	//TODO implement me
	panic("implement me")
}

func (mock *VPowerHandlerMock) BeginBlock(context *ctrlertypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	panic("implement me")
}

func (mock *VPowerHandlerMock) EndBlock(context *ctrlertypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	panic("implement me")
}

func (mock *VPowerHandlerMock) Commit() ([]byte, int64, xerrors.XError) {
	//TODO implement me
	panic("implement me")
}

var _ ctrlertypes.IVPowerHandler = (*VPowerHandlerMock)(nil)
