package vpower

import (
	"github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/holiman/uint256"
	"github.com/robaho/fixed"
)

// fixedWeightOfPowerChunks calculates the voting power weight not applied.
// `result = (tau * min({bonding_duration}/ripeningCycle, 1) + keppa) * {sum_of_voting_power} / baseSupply`
func fixedWeightOfPowerChunks(powerChunks []*PowerChunkProto, currHeight, ripeningCycle int64, tau int32, baseSupply *uint256.Int) fixed.Fixed {
	totalPower, _ := types.AmountToPower(baseSupply)
	fixedSupplyPower := fixed.NewI(totalPower, 0)

	fixedScaledPower := fixedScaledPowerChunks(powerChunks, currHeight, ripeningCycle, tau)
	return fixedScaledPower.Div(fixedSupplyPower)
}

func fixedScaledPowerChunks(powerChunks []*PowerChunkProto, currHeight, ripeningCycle int64, tau int32) fixed.Fixed {
	_tau := fixed.NewI(int64(tau), 0).Div(fixed.NewI(1000, 0))
	_keppa := fixed.NewI(1, 0).Sub(_tau)
	_ripeningCycle := fixed.NewI(ripeningCycle, 0)

	maturedPower := int64(0)
	_risingPower := fixed.ZERO

	for _, pc := range powerChunks {
		dur := currHeight - pc.Height
		if dur >= ripeningCycle {
			// mature power
			maturedPower += pc.Power
		} else if dur >= 1 {
			//  (((tau * dur) / ripeningCycle) + keppa) * power_i
			w_riging := _tau.Mul(fixed.NewI(dur, 0)).Div(_ripeningCycle).Add(_keppa).Mul(fixed.NewI(pc.Power, 0))
			_risingPower = _risingPower.Add(w_riging)
		}
		//fmt.Println("fixedScaledPowerChunks", "power", pc.Power, "height", pc.Height, "dur", dur)
	}

	return _risingPower.Add(fixed.NewI(maturedPower, 0))
}

func fixedScaledPowerChunk(pc *PowerChunkProto, currHeight, ripeningCycle int64, tau int32) fixed.Fixed {
	_tau := fixed.NewI(int64(tau), 0).Div(fixed.NewI(1000, 0))
	_keppa := fixed.NewI(1, 0).Sub(_tau)
	_ripeningCycle := fixed.NewI(ripeningCycle, 0)

	dur := currHeight - pc.Height
	if dur >= ripeningCycle {
		return fixed.NewI(pc.Power, 0)
	} else if dur >= 1 {
		//  (((tau * dur) / ripeningCycle) + keppa) * power_i
		return _tau.Mul(fixed.NewI(dur, 0)).Div(_ripeningCycle).Add(_keppa).Mul(fixed.NewI(pc.Power, 0))
	}
	return fixed.ZERO
}
