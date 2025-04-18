package vpower

import (
	"github.com/beatoz/beatoz-go/types"
	"github.com/shopspring/decimal"
)

// Wa calculates the total voting power weight of all validators and delegators.
// The result may differ from the sum of Wi due to floating-point error.
func Wa(pows, vpdurs []int64, ripeningCycle int64, totalSupply decimal.Decimal, tau int) decimal.Decimal {
	sumTmW := decimal.Zero
	sumPowAmt := decimal.Zero
	_tau := decimal.New(int64(tau), -3)
	_keppa := decimalOne.Sub(_tau)

	for i, pow := range pows {
		vpAmt := decimal.New(pow, int32(types.DECIMAL))
		sumPowAmt = sumPowAmt.Add(vpAmt)

		tmW := vpAmt
		if vpdurs[i] < ripeningCycle {
			tmW = decimal.NewFromInt(vpdurs[i]).Mul(vpAmt).Div(decimal.NewFromInt(ripeningCycle))
		}
		sumTmW = sumTmW.Add(tmW)
	}

	// Use `QuoRem` instead of `Div`.
	// Because `Div` does round up, the sum of `Wi` can be greater than `1`.
	q, _ := _tau.Mul(sumTmW).Add(_keppa.Mul(sumPowAmt)).QuoRem(totalSupply, int32(types.DECIMAL))
	return q
}

func WaWeighted(pows, vpdurs []int64, ripeningCycle int64, totalSupply decimal.Decimal, tau int) decimal.Decimal {
	_tau := decimal.New(int64(tau), -3)
	_keppa := decimalOne.Sub(_tau)

	_maturedPower := int64(0)
	weightedPower := decimal.Zero

	for i, pow := range pows {
		if vpdurs[i] >= ripeningCycle {
			// mature power
			_maturedPower += pow
		} else {
			decDur := decimal.NewFromInt(vpdurs[i]).Div(decimal.NewFromInt(ripeningCycle))
			decCo := _tau.Mul(decDur).Add(_keppa)
			decV := decimal.NewFromInt(pow)
			weightedPower = weightedPower.Add(decCo.Mul(decV))
		}
	}

	weightedPower = weightedPower.Mul(decimal.New(1, int32(types.DECIMAL)))
	decPowerAmt := weightedPower.Add(decimal.New(_maturedPower, int32(types.DECIMAL)))
	q, _ := decPowerAmt.QuoRem(totalSupply, int32(types.DECIMAL))
	return q
}

func WaWeightedEx(powChunks []*PowerChunk, height int64, ripeningCycle int64, totalSupply decimal.Decimal, tau int) decimal.Decimal {
	_tau := decimal.New(int64(tau), -3)
	_keppa := decimalOne.Sub(_tau)

	_maturedPower := int64(0)
	weightedPower := decimal.Zero

	for _, pc := range powChunks {
		dur := height - pc.Height
		if dur >= ripeningCycle {
			// mature power
			_maturedPower += pc.Power
		} else {
			decDur := decimal.NewFromInt(dur).Div(decimal.NewFromInt(ripeningCycle))
			decCo := _tau.Mul(decDur).Add(_keppa)
			decV := decimal.NewFromInt(pc.Power)
			weightedPower = weightedPower.Add(decCo.Mul(decV))
		}
	}

	weightedPower = weightedPower.Mul(decimal.New(1, int32(types.DECIMAL)))
	decPowerAmt := weightedPower.Add(decimal.New(_maturedPower, int32(types.DECIMAL)))
	q, _ := decPowerAmt.QuoRem(totalSupply, int32(types.DECIMAL))
	return q
}

func WaWeightedEx2(powChunks []*PowerChunk, maturePower, height int64, ripeningCycle int64, totalSupply decimal.Decimal, tau int) decimal.Decimal {
	_tau := decimal.New(int64(tau), -3)
	_keppa := decimalOne.Sub(_tau)

	weightedPower := decimal.Zero

	for _, pc := range powChunks {
		dur := height - pc.Height
		decDur := decimal.NewFromInt(dur).Div(decimal.NewFromInt(ripeningCycle))
		decCo := _tau.Mul(decDur).Add(_keppa)
		decV := decimal.NewFromInt(pc.Power)
		weightedPower = weightedPower.Add(decCo.Mul(decV))
	}

	weightedPower = weightedPower.Mul(decimal.New(1, int32(types.DECIMAL)))
	decPowerAmt := weightedPower.Add(decimal.New(maturePower, int32(types.DECIMAL)))
	q, _ := decPowerAmt.QuoRem(totalSupply, int32(types.DECIMAL))
	return q
}

// Wi calculates the voting power weight `W_i` of a validator and delegator like the below.
// `W_i = (tau * min(StakeDurationInSecond/RipeningCycle, 1) + keppa) * Stake_i / S_i`
func Wi(pow, vdur, ripeningCycle int64, totalSupply decimal.Decimal, tau int) decimal.Decimal {
	if vdur == 0 {
		return decimal.Zero
	}
	decCo := decimalOne
	if vdur < ripeningCycle {
		decDur := decimal.NewFromInt(vdur).Div(decimal.NewFromInt(ripeningCycle))
		decTau := decimal.New(int64(tau), -3) // tau is permil
		decKeppa := decimalOne.Sub(decTau)
		decCo = decTau.Mul(decDur).Add(decKeppa)
	}

	decV := decimal.New(pow, int32(types.DECIMAL)) // supply amount unit => pow * 10^18

	// Use `QuoRem` instead of `Div`.
	// Because `Div` does round up, the sum of `Wi` can be greater than `1`.
	q, _ := decCo.Mul(decV).QuoRem(totalSupply, int32(types.DECIMAL))
	return q
}

func oldWi(pow, vdur, ripeningCycle int64, totalSupply decimal.Decimal, tau int) decimal.Decimal {
	decDur := decimalOne
	if vdur < ripeningCycle {
		decDur = decimal.NewFromInt(vdur).Div(decimal.NewFromInt(ripeningCycle))
	}
	decV := decimal.New(pow, int32(types.DECIMAL))
	decTau := decimal.New(int64(tau), -3) // tau is permil
	decKeppa := decimalOne.Sub(decTau)

	// Use `QuoRem` instead of `Div`.
	// Because `Div` does round up, the sum of `Wi` can be greater than `1`.
	q, _ := decTau.Mul(decDur).Add(decKeppa).Mul(decV).QuoRem(totalSupply, int32(types.DECIMAL))
	return q
}

// H returns the normalized block time corresponding to the given block height.
// It calculates how far along the blockchain is relative to a predefined reference period.
// For example, if the reference period is one year, a return value of 1.0 indicates that
// exactly one reference period has elapsed.
func H(height, blockIntvSec int64) decimal.Decimal {
	return decimal.NewFromInt(height).Mul(decimal.NewFromInt(blockIntvSec)).Div(decimal.NewFromInt(oneYearSeconds))
}
