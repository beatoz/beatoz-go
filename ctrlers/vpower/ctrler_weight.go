package vpower

import (
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	"github.com/shopspring/decimal"
)

func (ctrler *VPowerCtrler) ComputeWeight(
	height, inflationCycle, ripeningBlocks int64, tau int32,
	totalSupply *uint256.Int,
) (*ctrlertypes.Weight, xerrors.XError) {

	//var allPowChunks []*PowerChunkProto
	var benefAddrs []types.Address
	var mapBenefPowChunks = make(map[string]*struct {
		val   bool
		pcs   []*PowerChunkProto
		signW decimal.Decimal
	})

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
		signRate, _ := decimal.NewFromInt(int64(c)).QuoRem(decimal.NewFromInt(inflationCycle), GetDivisionPrecision())
		signRate = decimal.NewFromInt(1).Sub(signRate) // = 1 - missedBlock/inflationCycle

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
					signW decimal.Decimal
				}{
					val:   bytes.Equal(from, val.addr),
					pcs:   vpow.PowerChunks,
					signW: signRate,
				}
			} else {
				b.pcs = append(b.pcs, vpow.PowerChunks...)
			}

			//allPowChunks = append(allPowChunks, vpow.PowerChunks...)
		}
	}

	weightInfo := ctrlertypes.NewWeight()
	for _, addr := range benefAddrs {
		benefPowChunks := mapBenefPowChunks[addr.String()]
		benefW := WaEx64ByPowerChunk(
			benefPowChunks.pcs,
			height, ripeningBlocks, tau, totalSupply)
		weightInfo.Add(addr, benefW, benefPowChunks.signW, benefPowChunks.val)
	}

	return weightInfo, nil
}
