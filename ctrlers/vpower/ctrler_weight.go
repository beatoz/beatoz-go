package vpower

import (
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
)

func (ctrler *VPowerCtrler) ComputeWeight(
	height, ripeningBlocks int64, tau int32,
	totalSupply *uint256.Int,
) (*ctrlertypes.Weight, xerrors.XError) {

	//var allPowChunks []*PowerChunkProto
	var benefAddrs []types.Address
	var mapBenefPowChunks = make(map[string]*struct {
		val bool
		pcs []*PowerChunkProto
	})

	// todo: Compute weights for only validators who has signed during the last inflation cycle.
	//  and reward them based on their signing ratio during the last inflation
	for _, val := range ctrler.lastValidators {
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
					val bool
					pcs []*PowerChunkProto
				}{
					val: bytes.Equal(from, val.addr),
					pcs: vpow.PowerChunks,
				}
			} else {
				b.pcs = append(b.pcs, vpow.PowerChunks...)
			}

			//allPowChunks = append(allPowChunks, vpow.PowerChunks...)
		}
	}

	weightInfo := ctrlertypes.NewWeight()
	for _, addr := range benefAddrs {
		benefW := WaEx64ByPowerChunk(
			mapBenefPowChunks[addr.String()].pcs,
			height, ripeningBlocks, tau, totalSupply)

		weightInfo.Add(addr, benefW.Truncate(6), mapBenefPowChunks[addr.String()].val)
	}

	//totalSupplyPower := decimal.NewFromBigInt(totalSupply.ToBig(), 0).Div(decimal.New(1, int32(types.DECIMAL)))
	//vpowW := Scaled64PowerChunk(allPowChunks, height, ripeningBlocks, tau)
	//vpowW, _ = vpowW.QuoRem(totalSupplyPower, int32(types.DECIMAL))
	//// weightInfo.SumWeight is equal to vpowW
	//if vpowW.Equal(weightInfo.SumWeight()) == false {
	//	panic(fmt.Errorf("vpowW(%v) != weightInfo.SumWeight(%v)", vpowW, weightInfo.SumWeight()))
	//}

	return weightInfo, nil
}
