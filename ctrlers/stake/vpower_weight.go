package stake

import (
	"github.com/beatoz/beatoz-go/types"
	"github.com/shopspring/decimal"
)

type powerWeightObj struct {
	power         int64
	bondingHeight int64
	weight        decimal.Decimal
}

type VotingPowerWeight struct {
	SumPower     int64
	risingPowers []powerWeightObj
}

func (pw *VotingPowerWeight) Add(power, height int64) {
	pw.risingPowers = append(pw.risingPowers, powerWeightObj{
		power:         power,
		bondingHeight: height,
		weight:        decimal.Zero,
	})
}

func (pw *VotingPowerWeight) Compute(height int64, totalSupply decimal.Decimal, tau int) decimal.Decimal {
	risingWeight := decimal.Zero
	_risings := pw.risingPowers[:0]

	for _, p := range pw.risingPowers {
		dur := height - p.bondingHeight
		if dur >= powerRipeningCycle {
			pw.SumPower += p.power
		} else {
			p.weight = Wi(p.power, dur, totalSupply, tau)
			risingWeight = risingWeight.Add(p.weight)
			_risings = append(_risings, p)
		}
	}
	pw.risingPowers = _risings

	return Wi(pw.SumPower, powerRipeningCycle, totalSupply, tau).Add(risingWeight)
}

// Wa calculates the total voting power weight of all validators and delegators.
// The result may differ from the sum of Wi due to floating-point error.
func Wa(vpows, vpdurs []int64, totalSupply decimal.Decimal, tau int) decimal.Decimal {
	sumPow := decimal.Zero
	sumTmW := decimal.Zero
	for i, vpow := range vpows {
		vpAmt := decimal.New(vpow, int32(types.DECIMAL))
		sumPow = sumPow.Add(vpAmt)

		tmW := vpAmt
		if vpdurs[i] < powerRipeningCycle {
			tmW = decimal.NewFromInt(vpdurs[i]).Mul(vpAmt).Div(decimal.NewFromInt(powerRipeningCycle))
		}
		sumTmW = sumTmW.Add(tmW)
	}

	_tau := decimal.New(int64(tau), -3)
	_keppa := decimalOne.Sub(_tau)

	// Use `QuoRem` instead of `Div`.
	// Because `Div` does round up, the sum of `Wi` can be greater than `1`.
	q, _ := _tau.Mul(sumTmW).Add(_keppa.Mul(sumPow)).QuoRem(totalSupply, 16)
	return q
}

// Wi calculates the voting power weight `W_i` of an validator and delegator like the below.
// `W_i = (tau * min(StakeDurationInSecond/InflationCycle, 1) + keppa) * Stake_i / S_i`
func Wi(vpow, vdur int64, totalSupply decimal.Decimal, tau int) decimal.Decimal {
	decDur := decimalOne
	if vdur < powerRipeningCycle {
		decDur = decimal.NewFromInt(vdur).Div(decimal.NewFromInt(powerRipeningCycle))
	}
	decV := decimal.New(vpow, 18)
	decTau := decimal.New(int64(tau), -3)
	decKeppa := decimalOne.Sub(decTau)

	// Use `QuoRem` instead of `Div`.
	// Because `Div` does round up, the sum of `Wi` can be greater than `1`.
	q, _ := decTau.Mul(decDur).Add(decKeppa).Mul(decV).QuoRem(totalSupply, 18)
	return q
}

// H returns the normalized block time corresponding to the given block height.
// It calculates how far along the blockchain is relative to a predefined reference period.
// For example, if the reference period is one year, a return value of 1.0 indicates that
// exactly one reference period has elapsed.
func H(height, blockIntvSec int64) decimal.Decimal {
	return decimal.NewFromInt(height).Mul(decimal.NewFromInt(blockIntvSec)).Div(decimal.NewFromInt(oneYearSeconds))
}
