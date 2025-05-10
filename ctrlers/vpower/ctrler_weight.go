package vpower

import (
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	"github.com/shopspring/decimal"
)

func (ctrler *VPowerCtrler) ComputeWeight(
	height, ripeningBlocks int64, tau int32,
	totalSupply *uint256.Int) (decimal.Decimal, []decimal.Decimal, []types.Address, xerrors.XError) {

	var allPowChunks []*PowerChunkProto
	var benefs []types.Address
	var mapPowChunks = make(map[string][]*PowerChunkProto)
	for _, val := range ctrler.lastValidators {
		for _, from := range val.Delegators {
			vpow, xerr := ctrler.readVPower(from, val.addr, true)
			if xerr != nil {
				return decimal.Zero, nil, nil, xerr
			}

			_mapKey := bytes.HexBytes(from).String()
			pcs, ok := mapPowChunks[_mapKey]
			if !ok {
				benefs = append(benefs, from)
				mapPowChunks[_mapKey] = vpow.PowerChunks
			} else {
				mapPowChunks[_mapKey] = append(pcs, vpow.PowerChunks...)
			}

			allPowChunks = append(allPowChunks, vpow.PowerChunks...)
		}
	}

	benefWeights := make([]decimal.Decimal, len(benefs))
	for i, addr := range benefs {
		benefWeights[i] = WaEx64ByPowerChunk(
			mapPowChunks[addr.String()],
			height, ripeningBlocks, tau, totalSupply)
	}
	wvpow := Weight64ByPowerChunk(allPowChunks, height, ripeningBlocks, tau)

	_totalSupply := decimal.NewFromBigInt(totalSupply.ToBig(), 0).Div(decimal.New(1, int32(types.DECIMAL)))
	wa, _ := wvpow.QuoRem(_totalSupply, int32(types.DECIMAL))
	// wa = wa.Truncate(6)
	return wa, benefWeights, benefs, nil
}
