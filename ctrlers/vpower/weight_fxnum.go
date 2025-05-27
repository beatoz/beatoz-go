package vpower

import (
	"github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/libs/fxnum"
	"github.com/holiman/uint256"
)

func FxNumWeightOfPowerChunks(powerChunks []*PowerChunkProto, currHeight, ripeningCycle int64, tau int32, totalSupply *uint256.Int) fxnum.FxNum {
	return fxnumWeightOfPowerChunks(powerChunks, currHeight, ripeningCycle, tau, totalSupply)
}

// fxnumWeightOfPowerChunks calculates the voting power weight not applied.
// `result = (tau * min({bonding_duration}/ripeningCycle, 1) + keppa) * {sum_of_voting_power} / totalSupply`
func fxnumWeightOfPowerChunks(powerChunks []*PowerChunkProto, currHeight, ripeningCycle int64, tau int32, totalSupply *uint256.Int) fxnum.FxNum {
	totalPower, _ := types.AmountToPower(totalSupply)
	fxSupplyPower := fxnum.FromInt(totalPower)

	fxScaledPower := fxnumScaledPowerChunks(powerChunks, currHeight, ripeningCycle, tau)
	return fxScaledPower.Div(fxSupplyPower)
}

func fxnumScaledPowerChunks(powerChunks []*PowerChunkProto, currHeight, ripeningCycle int64, tau int32) fxnum.FxNum {
	_tau := fxnum.Permil(int(tau))
	_keppa := fxnum.ONE.Sub(_tau)
	_ripeningCycle := fxnum.FromInt(ripeningCycle)

	maturedPower := int64(0)
	_risingPower := fxnum.ZERO

	for _, pc := range powerChunks {
		dur := currHeight - pc.Height
		if dur >= ripeningCycle {
			// mature power
			maturedPower += pc.Power
		} else {
			//  (((tau * dur) / ripeningCycle) + keppa) * power_i
			w_riging := _tau.Mul(fxnum.FromInt(dur)).Div(_ripeningCycle).Add(_keppa).Mul(fxnum.FromInt(pc.Power))
			_risingPower = _risingPower.Add(w_riging)
		}
		//fmt.Println("fxnumScaledPowerChunks", "power", pc.Power, "height", pc.Height, "dur", dur)
	}

	return _risingPower.Add(fxnum.FromInt(maturedPower))
}

func fxnumScaledPowerChunk(pc *PowerChunkProto, currHeight, ripeningCycle int64, tau int32) fxnum.FxNum {
	_tau := fxnum.Permil(int(tau))
	_keppa := fxnum.ONE.Sub(_tau)
	_ripeningCycle := fxnum.FromInt(ripeningCycle)

	dur := currHeight - pc.Height
	if dur >= ripeningCycle {
		return fxnum.FromInt(pc.Power)
	} else {
		//  (((tau * dur) / ripeningCycle) + keppa) * power_i
		return _tau.Mul(fxnum.FromInt(dur)).Div(_ripeningCycle).Add(_keppa).Mul(fxnum.FromInt(pc.Power))
	}
}
