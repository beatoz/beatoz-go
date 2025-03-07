package test

import (
	"fmt"
	"github.com/beatoz/beatoz-go/types"
	"github.com/holiman/uint256"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"math/rand/v2"
	"testing"
)

type VotingPower struct {
	duration  uint64
	power     uint64
	expectedW decimal.Decimal
}

var (
	maxSupply        = types.ToFons(700_000_000)
	initSupply       = types.ToFons(300_000_000)
	adjustedSupply   = initSupply
	currSupply       *uint256.Int
	vCoDuration      = decimal.RequireFromString("0.002")
	vCoDefault       = decimal.RequireFromString("0.896")
	vCoSupply        = decimal.RequireFromString("0.3")
	twoWeeks         = 1_209_600  // about 2 weeks
	oneYear          = 31_536_000 // about one year
	blockInterval    = 3
	oneYearBlocks    = decimal.NewFromInt(int64(oneYear)).Div(decimal.NewFromInt(int64(blockInterval))).IntPart()
	TotalVotingPower uint64
	VotingPowerObjs  []VotingPower
)

func Wall(totalSupplyPower uint64) decimal.Decimal {
	vsi := decimal.NewFromUint64(totalSupplyPower)
	vpsum := decimal.NewFromUint64(0)
	for _, vp := range VotingPowerObjs {
		vd := decimal.NewFromUint64(vp.duration)
		va := decimal.NewFromUint64(vp.power)
		vpsum = vCoDuration.Mul(vd).Add(vCoDefault).Mul(va).Add(vpsum)
	}
	return vpsum.Div(vsi)
}

func H(n int64) decimal.Decimal {
	return decimal.NewFromInt(int64(blockInterval) * n).Div(decimal.NewFromInt(int64(oneYear)))
}

func Wi(i int, totalSupplyPower uint64) decimal.Decimal {
	vp := VotingPowerObjs[i]
	return vpWeight(vp.power, vp.duration, totalSupplyPower)
}

func vpWeight(power, duration, totalSupplyPower uint64) decimal.Decimal {
	vd := decimal.NewFromUint64(duration)
	va := decimal.NewFromUint64(power)
	vsi := decimal.NewFromUint64(totalSupplyPower)
	return vCoDuration.Mul(vd).Add(vCoDefault).Mul(va).Div(vsi)
}

func init() {}
func initVPs(totalSupplyPower uint64) {
	TotalVotingPower = 0
	VotingPowerObjs = nil
	rc := rand.IntN(1_000_000)
	for i := 0; i < rc; i++ {
		d := rand.Uint64N(51) + 2
		p := rand.Uint64N(totalSupplyPower-TotalVotingPower) + 1
		TotalVotingPower += p
		VotingPowerObjs = append(VotingPowerObjs, VotingPower{
			duration:  d,
			power:     p,
			expectedW: vpWeight(p, d, totalSupplyPower),
		})

		if TotalVotingPower >= totalSupplyPower {
			break
		}
	}
}

func Test_Wi(t *testing.T) {
	for i := 0; i < 10000; i++ {
		_mx, _ := types.FromFons(maxSupply)
		supplyPower := rand.Uint64N(_mx) + 1
		initVPs(supplyPower)
		for _, vp := range VotingPowerObjs {
			w := vpWeight(vp.power, vp.duration, supplyPower)
			require.Equal(t, vp.expectedW, w)
		}
	}
}

func Test_Wall(t *testing.T) {
	for i := 0; i < 10000; i++ {
		_mx, _ := types.FromFons(maxSupply)
		supplyPower := rand.Uint64N(_mx) + 1
		initVPs(supplyPower)

		wAll0 := decimal.NewFromUint64(0)
		for _, vp := range VotingPowerObjs {
			w := vpWeight(vp.power, vp.duration, supplyPower)
			wAll0 = wAll0.Add(w)
		}

		wAll := Wall(supplyPower)
		require.Equal(t, wAll0.StringFixed(6), wAll.StringFixed(6), "not equal", wAll0.StringFixed(6), wAll.StringFixed(6))
	}
}

func TestInflation(t *testing.T) {
	sn, _ := types.FromFons(initSupply)
	initVPs(sn)

	lastAdjustHeight := int64(0)
	adjustedSupply = initSupply

	e0 := decimal.RequireFromString("2.71828182845904523536028747135266249775724709369995957496696763")

	for i := int64(0); i < 100*oneYearBlocks+1; i += oneYearBlocks {
		w_all := Wall(sn)
		wr := w_all.Mul(vCoSupply)
		//fmt.Println("r", vCoSupply.StringFixed(2))
		//fmt.Println("w", w_all.StringFixed(2))
		//fmt.Println("wr", wr.StringFixed(2))

		h := H(i - lastAdjustHeight)

		e := e0.Pow(wr.Mul(h).Neg())

		_rsupply := new(uint256.Int).Sub(maxSupply, adjustedSupply)
		v_supply := decimal.NewFromBigInt(maxSupply.ToBig(), 0).Sub(decimal.NewFromBigInt(_rsupply.ToBig(), 0).Mul(e))
		sn, _ = types.FromFons(uint256.MustFromBig(v_supply.BigInt()))

		fmt.Println(sn)
	}

}
