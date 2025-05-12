package vpower

import (
	"encoding/hex"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/ctrlers/vpower"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/holiman/uint256"
	"github.com/shopspring/decimal"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"math/rand"
	"sort"
)

type VPowerHandlerMock struct {
	delegatees []*vpower.Delegatee
	validators []*vpower.Delegatee
	mapVPowers map[string]*vpower.VPower
	totalPower int64 // key is from_address + to_address
}

func NewVPowerHandlerMock(valWals []*web3.Wallet) *VPowerHandlerMock {
	valsCnt := len(valWals)
	delegatees := make([]*vpower.Delegatee, valsCnt)
	mapVPowers := make(map[string]*vpower.VPower)

	sumPower := int64(0)
	for i, w := range valWals {
		dgtee := vpower.NewDelegatee(w.GetPubKey())
		delegatees[i] = dgtee

		pow := rand.Int63n(1_000_000) + 100_000
		vpow := vpower.NewVPower(w.Address(), w.GetPubKey()) // self bonding
		vpow.AddPowerWithTxHash(pow, 1, bytes.ZeroBytes(32))
		mapVPowers[w.Address().String()] = vpow

		dgtee.AddPower(w.Address(), pow)
		dgtee.AddDelegator(w.Address())

		sumPower += pow
	}
	sort.Slice(delegatees, func(i, j int) bool {
		return delegatees[i].SumPower > delegatees[j].SumPower
	})

	return &VPowerHandlerMock{
		delegatees: delegatees,
		validators: delegatees,
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
		totalPower += mock.delegatees[i].SumPower
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
	return ctrlertypes.PowerToAmount(mock.GetTotalPower())
}

func (mock *VPowerHandlerMock) GetTotalPower() int64 {
	sum := int64(0)
	for _, v := range mock.delegatees {
		sum += v.SumPower
	}
	return sum
}

func (mock *VPowerHandlerMock) TotalPowerOf(addr types.Address) int64 {
	for _, v := range mock.delegatees {
		if bytes.Compare(addr, v.Address()) == 0 {
			return v.SumPower
		}
	}
	return int64(0)
}

func (mock *VPowerHandlerMock) SelfPowerOf(addr types.Address) int64 {
	return 0
}

func (mock *VPowerHandlerMock) DelegatedPowerOf(addr types.Address) int64 {
	return 0
}

func (mock *VPowerHandlerMock) PickAddress(i int) types.Address {
	return mock.delegatees[i].Address()
}

func (mock *VPowerHandlerMock) ComputeWeight(height, ripeningBlocks int64, tau int32, totalSupply *uint256.Int) (*ctrlertypes.Weight, xerrors.XError) {
	mapWeightObjs := make(map[string]*struct {
		isval bool
		w     decimal.Decimal
	})

	for k, vpow := range mock.mapVPowers {
		_w := vpower.WaEx64ByPowerChunk(vpow.PowerChunks, height, ripeningBlocks, tau, totalSupply)
		wobj, ok := mapWeightObjs[k]
		if !ok {
			wobj = &struct {
				isval bool
				w     decimal.Decimal
			}{
				isval: true,
				w:     _w,
			}
		}
		wobj.w = wobj.w.Add(_w)
		mapWeightObjs[k] = wobj
	}

	weightInfo := ctrlertypes.NewWeight()
	for k, wo := range mapWeightObjs {
		addr, _ := hex.DecodeString(k)
		weightInfo.Add(addr, wo.w, wo.isval)
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
	//TODO implement me
	panic("implement me")
}

func (mock *VPowerHandlerMock) EndBlock(context *ctrlertypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	//TODO implement me
	panic("implement me")
}

var _ ctrlertypes.IVPowerHandler = (*VPowerHandlerMock)(nil)
