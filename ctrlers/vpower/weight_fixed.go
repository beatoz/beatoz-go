package vpower

import (
	"github.com/beatoz/beatoz-go/libs/fixedutil"
	"github.com/robaho/fixed"
)

func FixedWeightedPowerChunks(powerChunks []*PowerChunkProto, currHeight, ripeningCycle int64, tau int32) fixed.Fixed {
	_tau := fixed.NewI(int64(tau), 0).Div(fixedutil.PermilBase)
	_keppa := fixed.NewI(1, 0).Sub(_tau)
	_ripeningCycle := fixed.NewI(ripeningCycle, 0)

	maturedPower := int64(0)
	_risingPower := fixed.ZERO

	for _, pc := range powerChunks {
		dur := currHeight - pc.Height
		if dur >= ripeningCycle {
			// mature power
			maturedPower += pc.Power
		} else {
			//  (((tau * dur) / ripeningCycle) + keppa) * power_i
			w_riging := _tau.Mul(fixed.NewI(dur, 0)).Div(_ripeningCycle).Add(_keppa).Mul(fixed.NewI(pc.Power, 0))
			_risingPower = _risingPower.Add(w_riging)
		}
		//fmt.Println("Scaled64PowerChunks", "power", pc.Power, "height", pc.Height, "dur", dur)
	}

	return _risingPower.Add(fixed.NewI(maturedPower, 0))
}

func FixedWeightedPowerChunk(pc *PowerChunkProto, currHeight, ripeningCycle int64, tau int32) fixed.Fixed {
	_tau := fixed.NewI(int64(tau), 0).Div(fixedutil.PermilBase)
	_keppa := fixed.NewI(1, 0).Sub(_tau)
	_ripeningCycle := fixed.NewI(ripeningCycle, 0)

	dur := currHeight - pc.Height
	if dur >= ripeningCycle {
		return fixed.NewI(pc.Power, 0)
	} else {
		//  (((tau * dur) / ripeningCycle) + keppa) * power_i
		return _tau.Mul(fixed.NewI(dur, 0)).Div(_ripeningCycle).Add(_keppa).Mul(fixed.NewI(pc.Power, 0))
	}
}
