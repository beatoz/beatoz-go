package vpower

import (
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	"github.com/shopspring/decimal"
)

func (ctrler *VPowerCtrler) ComputeWeight(height, ripeningBlocks, tau int64, totalSupply *uint256.Int) (decimal.Decimal, xerrors.XError) {
	var powChunks []*PowerChunkProto
	for _, val := range ctrler.lastValidators {
		for _, from := range val.Delegators {
			vpow, xerr := ctrler.readVPower(from, val.addr, true)
			if xerr != nil {
				return decimal.Zero, xerr
			}
			powChunks = append(powChunks, vpow.PowerChunks...)
		}
	}

	wvpow := Weight64ByPowerChunk(powChunks, height, ripeningBlocks, int(tau))

	_totalSupply := decimal.NewFromBigInt(totalSupply.ToBig(), 0).Div(decimal.New(1, int32(types.DECIMAL)))
	wa, _ := wvpow.QuoRem(_totalSupply, int32(types.DECIMAL))
	return wa, nil
}
