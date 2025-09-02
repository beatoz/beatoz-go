package vpower

import (
	"github.com/beatoz/beatoz-go/types"
	"github.com/holiman/uint256"
	"github.com/shopspring/decimal"
)

// decimalWeightOfPowerChunks calculates the voting power weight.
// `result = (tau * min({bonding_duration}/ripeningCycle, 1) + keppa) * {sum_of_voting_power} / baseSupply`
func decimalWeightOfPowerChunks(powerChunks []*PowerChunkProto, currHeight, ripeningCycle int64, tau int32, baseSupply *uint256.Int) decimal.Decimal {
	decBaseSupply := decimal.NewFromBigInt(baseSupply.ToBig(), -1*int32(types.DECIMAL))
	decScaledPower := decimalScaledPowerChunks(powerChunks, currHeight, ripeningCycle, tau)
	q, _ := decScaledPower.QuoRem(decBaseSupply, getDivisionPrecision())
	return q
}

// decimalScaledPowerChunks calculates the voting power weight not applied to the total supply.
// `result = (tau * min({bonding_duration}/ripeningCycle, 1) + keppa) * {sum_of_voting_power}`
func decimalScaledPowerChunks(powerChunks []*PowerChunkProto, currHeight, ripeningCycle int64, tau int32) decimal.Decimal {
	_tau := decimal.New(int64(tau), -3)
	_keppa := DecimalOne.Sub(_tau)
	_ripeningCycle := decimal.NewFromInt(ripeningCycle)

	_maturedPower := int64(0)
	risingPower := decimal.Zero

	for _, pc := range powerChunks {
		dur := currHeight - pc.Height
		if dur >= ripeningCycle {
			// mature power
			_maturedPower += pc.Power
		} else if dur >= 1 {
			//  (((tau * dur) / ripeningCycle) + keppa) * power_i
			decW, _ := _tau.Mul(decimal.NewFromInt(dur)).QuoRem(_ripeningCycle, getDivisionPrecision())
			decW = decW.Add(_keppa).Mul(decimal.NewFromInt(pc.Power))
			risingPower = risingPower.Add(decW) // risingPower += decW
		}
	}

	decWeightedPower := risingPower.Add(decimal.NewFromInt(_maturedPower))
	return decWeightedPower
}

func decimalScaledPowerChunk(pc *PowerChunkProto, currHeight, ripeningCycle int64, tau int32) decimal.Decimal {
	_tau := decimal.New(int64(tau), -3)
	_keppa := DecimalOne.Sub(_tau)
	_ripeningCycle := decimal.NewFromInt(ripeningCycle)

	dur := currHeight - pc.Height
	if dur >= ripeningCycle {
		// mature power
		return decimal.NewFromInt(pc.Power)
	} else if dur >= 1 {
		//  (((tau * dur) / ripeningCycle) + keppa) * power_i
		ret, _ := _tau.Mul(decimal.NewFromInt(dur)).QuoRem(_ripeningCycle, getDivisionPrecision())
		return ret.Add(_keppa).Mul(decimal.NewFromInt(pc.Power))
	}
	return decimal.Zero
}

// Wa calculates the total voting power weight of all validators and delegators.
// The result may differ from the sum of Wi due to floating-point error.
func Wa(pows, vpdurs []int64, ripeningCycle int64, tau int32, baseSupply decimal.Decimal) decimal.Decimal {
	sumTmV := decimal.Zero
	sumPowAmt := decimal.Zero
	_tau := decimal.New(int64(tau), -3) // tau is permil
	_keppa := DecimalOne.Sub(_tau)      // keppa = 1 - tau

	for i, pow := range pows {
		if vpdurs[i] <= 0 {
			continue
		}
		vpAmt := decimal.New(pow, int32(types.DECIMAL))
		sumPowAmt = sumPowAmt.Add(vpAmt)

		tmV := vpAmt
		if vpdurs[i] < ripeningCycle {
			tmV = decimal.NewFromInt(vpdurs[i]).Mul(vpAmt)
			tmV, _ = tmV.QuoRem(decimal.NewFromInt(ripeningCycle), getDivisionPrecision())
		}
		sumTmV = sumTmV.Add(tmV)
	}

	// Use `QuoRem` instead of `Div`.
	// Because `Div` does round up, the sum of `Wi` can be greater than `1`.
	q, _ := _tau.Mul(sumTmV).Add(_keppa.Mul(sumPowAmt)).QuoRem(baseSupply, getDivisionPrecision())
	return q
}

func oldWi(pow, vdur, ripeningCycle int64, tau int32, baseSupply decimal.Decimal) decimal.Decimal {
	if vdur <= 0 {
		return decimal.Zero
	}

	decDur := DecimalOne
	if vdur < ripeningCycle {
		decDur, _ = decimal.NewFromInt(vdur).QuoRem(decimal.NewFromInt(ripeningCycle), getDivisionPrecision())
	}
	decV := decimal.New(pow, int32(types.DECIMAL))
	decTau := decimal.New(int64(tau), -3) // tau is permil
	decKeppa := DecimalOne.Sub(decTau)

	// Use `QuoRem` instead of `Div`.
	// Because `Div` does round up, the sum of `Wi` can be greater than `1`.
	q, _ := decTau.Mul(decDur).Add(decKeppa).Mul(decV).QuoRem(baseSupply, getDivisionPrecision())
	return q
}

// Wi calculates the voting power weight `W_i` of a validator and delegator like the below.
// `W_i = (tau * min(StakeDurationInSecond/RipeningCycle, 1) + keppa) * Stake_i / S_i`
func Wi(pow, vdur, ripeningCycle int64, tau int32, baseSupply decimal.Decimal) decimal.Decimal {
	if vdur <= 0 {
		return decimal.Zero
	}

	decTau := decimal.New(int64(tau), -3) // tau is permil
	decKeppa := DecimalOne.Sub(decTau)
	decCo := DecimalOne
	if vdur < ripeningCycle {
		decTm, _ := decTau.Mul(decimal.NewFromInt(vdur)).QuoRem(decimal.NewFromInt(ripeningCycle), getDivisionPrecision())
		decCo = decTm.Add(decKeppa)
	}

	decV := decimal.New(pow, int32(types.DECIMAL)) // supply amount unit => pow * 10^18

	// Use `QuoRem` instead of `Div`.
	// Because `Div` does round up, the sum of `Wi` can be greater than `1`.
	q, _ := decCo.Mul(decV).QuoRem(baseSupply, getDivisionPrecision())
	return q
}

func WaEx(pows, vpdurs []int64, ripeningCycle int64, tau int32, baseSupply decimal.Decimal) decimal.Decimal {
	_tau := decimal.New(int64(tau), -3)
	_keppa := DecimalOne.Sub(_tau)

	_maturedPower := int64(0)
	weightedPower := decimal.Zero

	for i, pow := range pows {
		if vpdurs[i] >= ripeningCycle {
			// mature power
			_maturedPower += pow
		} else if vpdurs[i] >= 1 {
			decDur, _ := decimal.NewFromInt(vpdurs[i]).QuoRem(decimal.NewFromInt(ripeningCycle), getDivisionPrecision()) // dur = vpdur / ripeningCycle
			decCo := _tau.Mul(decDur).Add(_keppa)                                                                        // tau * (vpdur / ripeningCycle) + keppa
			decV := decimal.NewFromInt(pow)
			weightedPower = weightedPower.Add(decCo.Mul(decV))
		}
	}

	weightedPower = weightedPower.Mul(decimal.New(1, int32(types.DECIMAL)))
	decPowerAmt := weightedPower.Add(decimal.New(_maturedPower, int32(types.DECIMAL)))
	q, _ := decPowerAmt.QuoRem(baseSupply, getDivisionPrecision())
	return q
}

func WaEx64(pows, durs []int64, ripeningCycle int64, tau int32, baseSupply *uint256.Int) decimal.Decimal {
	_tau := decimal.New(int64(tau), -3)
	_keppa := DecimalOne.Sub(_tau)

	_maturedPower := int64(0)
	risingPower := decimal.Zero

	for i, pow := range pows {
		if durs[i] >= ripeningCycle {
			// mature power
			_maturedPower += pow
		} else if durs[i] >= 1 {
			//  (((tau * dur) / ripeningCycle) + keppa) * power_i
			decW, _ := _tau.Mul(decimal.NewFromInt(durs[i])).QuoRem(decimal.NewFromInt(ripeningCycle), getDivisionPrecision())
			decW = decW.Add(_keppa).Mul(decimal.NewFromInt(pow))
			risingPower = risingPower.Add(decW) // risingPower += decW
		}
	}

	decWeightedPower := risingPower.Add(decimal.NewFromInt(_maturedPower))
	decBaseSupply := decimal.NewFromBigInt(baseSupply.ToBig(), -1*int32(types.DECIMAL))

	q, _ := decWeightedPower.QuoRem(decBaseSupply, getDivisionPrecision())
	return q
}

var decimalPrecision = int32(decimal.DivisionPrecision)

func init() {
	setDivisionPrecision(16)
}

func setDivisionPrecision(precision int32) {
	decimalPrecision = max(3, precision/3)
	decimal.DivisionPrecision = int(precision)
}

func getPrecision() int32 {
	return decimalPrecision
}

func getDivisionPrecision() int32 {
	return int32(decimal.DivisionPrecision)
}
