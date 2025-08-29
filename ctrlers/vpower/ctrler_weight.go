package vpower

import (
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/libs/fxnum"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
)

func (ctrler *VPowerCtrler) ComputeWeight(
	height, inflationCycle, ripeningBlocks int64, tau int32,
	baseSupply *uint256.Int,
) (ctrlertypes.IWeightResult, xerrors.XError) {

	//var allPowChunks []*PowerChunkProto
	var benefAddrs []types.Address
	var mapBenefPowChunks = make(map[string]*struct {
		val   bool
		pcs   []*PowerChunkProto
		signW fxnum.FxNum
	})
	var allPowChunks []*PowerChunkProto

	ledger, xerr := ctrler.vpowerState.ImitableLedgerAt(max(height-1, 1))
	if xerr != nil {
		return nil, xerr
	}

	for _, val := range ctrler.lastValidators {

		// NOTE: Consider caching the missed block count.
		item, xerr := ledger.Get(v1.LedgerKeyMissedBlockCount(val.addr))
		if xerr != nil && !xerr.Contains(xerrors.ErrNotFoundResult) {
			return nil, xerr
		}
		c := BlockCount(0)
		if item != nil {
			ptr, _ := item.(*BlockCount)
			c = *ptr
		}
		signRate, _ := fxnum.FromInt(int64(c)).QuoRem(fxnum.FromInt(inflationCycle), fxnum.GetDivisionPrecision())
		signRate = fxnum.FromInt(1).Sub(signRate) // = 1 - missedBlock/inflationCycle

		for _, from := range val.Delegators {
			vpow, xerr := ctrler.readVPower(from, val.addr, true)
			if xerr != nil {
				return nil, xerr
			}

			_mapKey := bytes.HexBytes(from).String()
			b, ok := mapBenefPowChunks[_mapKey]
			if !ok {
				benefAddrs = append(benefAddrs, from)
				mapBenefPowChunks[_mapKey] = &struct {
					val   bool
					pcs   []*PowerChunkProto
					signW fxnum.FxNum
				}{
					val:   bytes.Equal(from, val.addr),
					pcs:   vpow.PowerChunks,
					signW: signRate,
				}
			} else {
				b.pcs = append(b.pcs, vpow.PowerChunks...)
			}

			allPowChunks = append(allPowChunks, vpow.PowerChunks...)
		}
	}

	var weightInfo *fxnumWeight

	{
		weightInfo = NewWeight()

		supplyInPower, _ := types.AmountToPower(baseSupply)
		fxSupplyPower := fxnum.FromInt(supplyInPower)

		allScaledPower := fxnumScaledPowerChunks(allPowChunks, height, ripeningBlocks, tau)
		allWeight := allScaledPower.Div(fxSupplyPower)

		for i, addr := range benefAddrs {
			benefPowChunks := mapBenefPowChunks[addr.String()]
			if i == len(benefAddrs)-1 {
				benefWeight := allWeight.Sub(weightInfo.sumWeight)
				weightInfo.Add(addr, benefWeight, benefPowChunks.signW, benefPowChunks.val)
			} else {
				benefScaledPower := fxnumScaledPowerChunks(benefPowChunks.pcs, height, ripeningBlocks, tau)
				benefWeight := allWeight.Mul(benefScaledPower).Div(allScaledPower)
				weightInfo.Add(addr, benefWeight, benefPowChunks.signW, benefPowChunks.val)
			}
		}
		weightInfo.sumWeight = allWeight
	}
	return weightInfo, nil
}
