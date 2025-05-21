package vpower

import (
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	"github.com/holiman/uint256"
	"github.com/shopspring/decimal"
)

const (
	DivisionPrecision = 6
	ResultPrecision   = 3
)

func init() {
	decimal.DivisionPrecision = DivisionPrecision
}

// Wa calculates the total voting power weight of all validators and delegators.
// The result may differ from the sum of Wi due to floating-point error.
func Wa(pows, vpdurs []int64, ripeningCycle int64, tau int32, totalSupply decimal.Decimal) decimal.Decimal {
	sumTmV := decimal.Zero
	sumPowAmt := decimal.Zero
	_tau := decimal.New(int64(tau), -3)        // tau is permil
	_keppa := ctrlertypes.DecimalOne.Sub(_tau) // keppa = 1 - tau

	for i, pow := range pows {
		vpAmt := decimal.New(pow, int32(types.DECIMAL))
		sumPowAmt = sumPowAmt.Add(vpAmt)

		tmV := vpAmt
		if vpdurs[i] < ripeningCycle {
			tmV = decimal.NewFromInt(vpdurs[i]).Mul(vpAmt).Div(decimal.NewFromInt(ripeningCycle))
		}
		sumTmV = sumTmV.Add(tmV)
	}

	// Use `QuoRem` instead of `Div`.
	// Because `Div` does round up, the sum of `Wi` can be greater than `1`.
	q, _ := _tau.Mul(sumTmV).Add(_keppa.Mul(sumPowAmt)).QuoRem(totalSupply, DivisionPrecision)
	return q
}

func oldWi(pow, vdur, ripeningCycle int64, tau int32, totalSupply decimal.Decimal) decimal.Decimal {
	decDur := ctrlertypes.DecimalOne
	if vdur < ripeningCycle {
		decDur = decimal.NewFromInt(vdur).Div(decimal.NewFromInt(ripeningCycle))
	}
	decV := decimal.New(pow, int32(types.DECIMAL))
	decTau := decimal.New(int64(tau), -3) // tau is permil
	decKeppa := ctrlertypes.DecimalOne.Sub(decTau)

	// Use `QuoRem` instead of `Div`.
	// Because `Div` does round up, the sum of `Wi` can be greater than `1`.
	q, _ := decTau.Mul(decDur).Add(decKeppa).Mul(decV).QuoRem(totalSupply, DivisionPrecision)
	return q
}

// Wi calculates the voting power weight `W_i` of a validator and delegator like the below.
// `W_i = (tau * min(StakeDurationInSecond/RipeningCycle, 1) + keppa) * Stake_i / S_i`
func Wi(pow, vdur, ripeningCycle int64, tau int32, totalSupply decimal.Decimal) decimal.Decimal {
	if vdur == 0 {
		return decimal.Zero
	}

	decTau := decimal.New(int64(tau), -3) // tau is permil
	decKeppa := ctrlertypes.DecimalOne.Sub(decTau)
	decCo := ctrlertypes.DecimalOne
	if vdur < ripeningCycle {
		decTm := decTau.Mul(decimal.NewFromInt(vdur)).Div(decimal.NewFromInt(ripeningCycle))
		decCo = decTm.Add(decKeppa)
	}

	decV := decimal.New(pow, int32(types.DECIMAL)) // supply amount unit => pow * 10^18

	// Use `QuoRem` instead of `Div`.
	// Because `Div` does round up, the sum of `Wi` can be greater than `1`.
	q, _ := decCo.Mul(decV).QuoRem(totalSupply, DivisionPrecision)
	return q
}

func WaEx(pows, vpdurs []int64, ripeningCycle int64, tau int32, totalSupply decimal.Decimal) decimal.Decimal {
	_tau := decimal.New(int64(tau), -3)
	_keppa := ctrlertypes.DecimalOne.Sub(_tau)

	_maturedPower := int64(0)
	weightedPower := decimal.Zero

	for i, pow := range pows {
		if vpdurs[i] >= ripeningCycle {
			// mature power
			_maturedPower += pow
		} else {
			decDur := decimal.NewFromInt(vpdurs[i]).Div(decimal.NewFromInt(ripeningCycle)) // dur = vpdur / ripeningCycle
			decCo := _tau.Mul(decDur).Add(_keppa)                                          // tau * (vpdur / ripeningCycle) + keppa
			decV := decimal.NewFromInt(pow)
			weightedPower = weightedPower.Add(decCo.Mul(decV))
		}
	}

	weightedPower = weightedPower.Mul(decimal.New(1, int32(types.DECIMAL)))
	decPowerAmt := weightedPower.Add(decimal.New(_maturedPower, int32(types.DECIMAL)))
	q, _ := decPowerAmt.QuoRem(totalSupply, DivisionPrecision)
	return q
}

func WaEx64(pows, durs []int64, ripeningCycle int64, tau int32, totalSupply *uint256.Int) decimal.Decimal {
	_tau := decimal.New(int64(tau), -3)
	_keppa := ctrlertypes.DecimalOne.Sub(_tau)

	_maturedPower := int64(0)
	risingPower := decimal.Zero

	for i, pow := range pows {
		if durs[i] >= ripeningCycle {
			// mature power
			_maturedPower += pow
		} else {
			//  (((tau * dur) / ripeningCycle) + keppa) * power_i
			decW := _tau.Mul(decimal.NewFromInt(durs[i])).Div(decimal.NewFromInt(ripeningCycle)).Add(_keppa).Mul(decimal.NewFromInt(pow))
			risingPower = risingPower.Add(decW) // risingPower += decW
		}
	}

	decWightedPower := risingPower.Add(decimal.NewFromInt(_maturedPower))
	decTotalSupply := decimal.NewFromBigInt(totalSupply.ToBig(), 0).Div(decimal.New(1, int32(types.DECIMAL)))

	q, _ := decWightedPower.QuoRem(decTotalSupply, DivisionPrecision)
	return q
}

// WaEx64ByPowerChunk calculates the voting power weight not applied.
// `result = (tau * min({bonding_duration}/ripeningCycle, 1) + keppa) * {sum_of_voting_power} / totalSupply`
func WaEx64ByPowerChunk(powerChunks []*PowerChunkProto, currHeight, ripeningCycle int64, tau int32, totalSupply *uint256.Int) decimal.Decimal {
	_tau := decimal.New(int64(tau), -3)
	_keppa := ctrlertypes.DecimalOne.Sub(_tau)

	_maturedPower := int64(0)
	risingPower := decimal.Zero

	for _, pc := range powerChunks {
		dur := currHeight - pc.Height
		if dur >= ripeningCycle {
			// mature power
			_maturedPower += pc.Power
		} else {
			//  (((tau * dur) / ripeningCycle) + keppa) * power_i
			decW := _tau.Mul(decimal.NewFromInt(dur)).Div(decimal.NewFromInt(ripeningCycle)).Add(_keppa).Mul(decimal.NewFromInt(pc.Power))
			risingPower = risingPower.Add(decW) // risingPower += decW
		}
		//fmt.Println("WaEx64ByPowerChunk", "currHeight", currHeight, "pc.Height", pc.Height, "dur", dur, "Power", pc.Power, "txHash", bytes.HexBytes(pc.TxHash))
	}

	decWightedPower := risingPower.Add(decimal.NewFromInt(_maturedPower))
	decTotalSupply := decimal.NewFromBigInt(totalSupply.ToBig(), 0).Div(decimal.New(1, int32(types.DECIMAL)))

	q, _ := decWightedPower.QuoRem(decTotalSupply, DivisionPrecision)
	return q
}

// Scaled64PowerChunk calculates the voting power weight not applied to the total supply.
// `result = (tau * min({bonding_duration}/ripeningCycle, 1) + keppa) * {sum_of_voting_power}`
func Scaled64PowerChunk(powerChunks []*PowerChunkProto, currHeight, ripeningCycle int64, tau int32) decimal.Decimal {
	_tau := decimal.New(int64(tau), -3)
	_keppa := ctrlertypes.DecimalOne.Sub(_tau)

	_maturedPower := int64(0)
	risingPower := decimal.Zero

	for _, pc := range powerChunks {
		dur := currHeight - pc.Height
		if dur >= ripeningCycle {
			// mature power
			_maturedPower += pc.Power
		} else {
			//  (((tau * dur) / ripeningCycle) + keppa) * power_i
			decW := _tau.Mul(decimal.NewFromInt(dur)).Div(decimal.NewFromInt(ripeningCycle)).Add(_keppa).Mul(decimal.NewFromInt(pc.Power))
			risingPower = risingPower.Add(decW) // risingPower += decW
		}
		//fmt.Println("Scaled64PowerChunk", "power", pc.Power, "height", pc.Height, "dur", dur)
	}

	decWightedPower := risingPower.Add(decimal.NewFromInt(_maturedPower))
	return decWightedPower
}
