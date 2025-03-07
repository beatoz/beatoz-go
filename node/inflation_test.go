package node

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
	vMaxSupply       = decimal.NewFromBigInt(maxSupply.ToBig(), 0)
	vRemainCap       = decimal.NewFromBigInt((new(uint256.Int).Sub(maxSupply, initSupply)).ToBig(), 0)
	initSupply       = types.ToFons(300_000_000)
	currSupply       *uint256.Int
	vCoDuration      = decimal.RequireFromString("0.002")
	vCoDefault       = decimal.RequireFromString("0.896")
	vCoSupply        = decimal.RequireFromString("0.3")
	twoWeeks         = 1_209_600  // about 2 weeks
	oneYear          = 31_536_000 // about one year
	blockInterval    = 3
	oneYearBlocks    = decimal.NewFromInt(int64(oneYear)).Div(decimal.NewFromInt(int64(blockInterval))).IntPart()
	TotalVotingPower uint64
	VotingPowerObjs  []*VotingPower
	E0               = decimal.RequireFromString("2.71828182845904523536028747135266249775724709369995957496696763")
)

func Wall(maxPotentialPower uint64) decimal.Decimal {
	vsi := decimal.NewFromUint64(maxPotentialPower)
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

func Wi(i int, maxPotentialPower uint64) decimal.Decimal {
	vp := VotingPowerObjs[i]
	return vpWeight(vp.power, vp.duration, maxPotentialPower)
}

func vpWeight(power, duration, maxPotentialPower uint64) decimal.Decimal {
	vd := decimal.NewFromUint64(duration)
	va := decimal.NewFromUint64(power)
	vsi := decimal.NewFromUint64(maxPotentialPower)
	return vCoDuration.Mul(vd).Add(vCoDefault).Mul(va).Div(vsi)
}

func initVPs(reqedPower, maxPotentialPower uint64) {
	TotalVotingPower = 0
	VotingPowerObjs = nil
	rc := rand.IntN(1_000_000)
	for i := 0; i < rc; i++ {
		d := rand.Uint64N(51) + 2
		p := rand.Uint64N(reqedPower-TotalVotingPower) + 1
		TotalVotingPower += p
		VotingPowerObjs = append(VotingPowerObjs, &VotingPower{
			duration: d,
			power:    p,
		})

		if TotalVotingPower >= reqedPower {
			break
		}
	}

	for _, vp := range VotingPowerObjs {
		vp.expectedW = vpWeight(vp.power, vp.duration, maxPotentialPower)
	}
}

func inflationAt(height, lastDeflaHeight int64, deflatedSupply *uint256.Int, maxPotentialPower uint64) decimal.Decimal {
	w_all := Wall(maxPotentialPower)
	wr := w_all.Mul(vCoSupply)
	h := H(height - lastDeflaHeight)

	exp := E0.Pow(wr.Mul(h))

	return vMaxSupply.Sub(vRemainCap.Div(exp))
}

func init() {}

func Test_Wi(t *testing.T) {
	for i := 0; i < 10000; i++ {
		maxPotentialPower, _ := types.FromFons(initSupply)
		reqedPower := rand.Uint64N(maxPotentialPower) + 1
		initVPs(reqedPower, maxPotentialPower)

		for _, vp := range VotingPowerObjs {
			w := vpWeight(vp.power, vp.duration, maxPotentialPower)
			require.Equal(t, vp.expectedW, w)
		}
	}
}

func Test_Wall(t *testing.T) {
	for i := 0; i < 10000; i++ {
		maxPotentialPower, _ := types.FromFons(initSupply)
		reqedPower := rand.Uint64N(maxPotentialPower) + 1
		initVPs(reqedPower, maxPotentialPower)

		wAll0 := decimal.NewFromUint64(0)
		for _, vp := range VotingPowerObjs {
			w := vpWeight(vp.power, vp.duration, maxPotentialPower)
			wAll0 = wAll0.Add(w)
		}

		wAll := Wall(maxPotentialPower)
		require.Equal(t, wAll0.StringFixed(6), wAll.StringFixed(6), "not equal", wAll0.StringFixed(6), wAll.StringFixed(6))
	}
}

func TestInflation(t *testing.T) {
	maxPotentialPower, _ := types.FromFons(initSupply)
	reqedPower := rand.Uint64N(maxPotentialPower) + 1
	initVPs(reqedPower, maxPotentialPower)

	deflatedSupply := initSupply
	lastDeflaHeight := int64(0)

	fmt.Println("W", Wall(maxPotentialPower))

	for i := int64(0); i < 100*oneYearBlocks+1; i += oneYearBlocks {
		vAdjustedSupply := inflationAt(i, lastDeflaHeight, deflatedSupply, maxPotentialPower)

		fmt.Println(types.FromFons(uint256.MustFromBig(vAdjustedSupply.BigInt())))
		maxPotentialPower, _ = types.FromFons(uint256.MustFromBig(vAdjustedSupply.BigInt()))
	}
}

func BenchmarkInflation(b *testing.B) {
	maxPotentialPower, _ := types.FromFons(initSupply)
	reqedPower := rand.Uint64N(maxPotentialPower) + 1
	initVPs(reqedPower, maxPotentialPower)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vAdjustedSupply := inflationAt(int64(i), 0, initSupply, maxPotentialPower)
		b.StopTimer()
		maxPotentialPower, _ = types.FromFons(uint256.MustFromBig(vAdjustedSupply.BigInt()))
		b.StartTimer()
	}
}
