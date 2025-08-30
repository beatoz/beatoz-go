package vpower

import (
	"encoding/hex"
	"sort"

	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/libs/fxnum"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
)

type benefPowChunksW struct {
	pcs   []*PowerChunkProto
	signW fxnum.FxNum
	val   bool
}

func (ctrler *VPowerCtrler) ComputeWeight(
	height, inflationCycle, ripeningBlocks int64, tau int32,
	baseSupply *uint256.Int,
) (ctrlertypes.IWeightResult, xerrors.XError) {

	var arrMapBenefPowerChunksPerVal []map[string]*benefPowChunksW
	var allPowChunks []*PowerChunkProto

	ledger, xerr := ctrler.vpowerState.ImitableLedgerAt(max(height-1, 1))
	if xerr != nil {
		return nil, xerr
	}

	lastValidators := ctrler.CopyLastValidators()
	for _, val := range lastValidators {

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

		mapBenefPowChunks := make(map[string]*benefPowChunksW)
		for _, from := range val.Delegators {
			vpow, xerr := ctrler.readVPower(from, val.addr, true)
			if xerr != nil {
				return nil, xerr
			}

			_mapKey := bytes.HexBytes(from).String()
			b, ok := mapBenefPowChunks[_mapKey]
			if !ok {
				mapBenefPowChunks[_mapKey] = &benefPowChunksW{
					pcs:   vpow.PowerChunks,
					signW: signRate,
					val:   bytes.Equal(from, val.addr),
				}
			} else {
				b.pcs = append(b.pcs, vpow.PowerChunks...)
			}

			allPowChunks = append(allPowChunks, vpow.PowerChunks...)
		}

		arrMapBenefPowerChunksPerVal = append(arrMapBenefPowerChunksPerVal, mapBenefPowChunks)
	}

	var weightInfo *fxnumWeight

	{
		weightInfo = NewWeight()

		supplyInPower, _ := types.AmountToPower(baseSupply)
		fxSupplyPower := fxnum.FromInt(supplyInPower)

		allScaledPower := fxnumScaledPowerChunks(allPowChunks, height, ripeningBlocks, tau)
		allWeight := allScaledPower.Div(fxSupplyPower)

		for i, mapBenefPowerChunks := range arrMapBenefPowerChunksPerVal {
			keys := make([]string, 0, len(mapBenefPowerChunks))
			for k := range mapBenefPowerChunks {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			for j, k := range keys {
				addr, _ := hex.DecodeString(k)
				benefPowChunks := mapBenefPowerChunks[k]
				if i == len(arrMapBenefPowerChunksPerVal)-1 && j == len(keys)-1 {
					benefWeight := allWeight.Sub(weightInfo.sumWeight)
					weightInfo.Add(addr, benefWeight, benefPowChunks.signW, benefPowChunks.val)
				} else {
					benefScaledPower := fxnumScaledPowerChunks(benefPowChunks.pcs, height, ripeningBlocks, tau)
					benefWeight := allWeight.Mul(benefScaledPower).Div(allScaledPower)
					weightInfo.Add(addr, benefWeight, benefPowChunks.signW, benefPowChunks.val)
				}
			}
		}

		weightInfo.sumWeight = allWeight
	}
	return weightInfo, nil
}
