package stake

import (
	"fmt"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"testing"
)

type votingPowerObj struct {
	vpow int64
	vdur int64
}

func initVotingPowerObjs(maxPow int64) []votingPowerObj {
	var vpObjs []votingPowerObj
	for maxPow > 7000 {
		pow := max(bytes.RandInt64N(1_000_000), 7000)
		v := struct {
			vpow int64
			vdur int64
		}{
			vpow: pow,
			vdur: bytes.RandInt64N(powerRipeningCycle),
		}
		vpObjs = append(vpObjs, v)

		maxPow -= pow
	}
	return vpObjs
}

func initVotingPowerWeights() []*VotingPowerWeight {
	var vpws []*VotingPowerWeight

	for i := 0; i < 21; i++ {
		vpw := &VotingPowerWeight{}

		// self power
		pow := bytes.RandInt64N(1_000_000) + 1_000_000
		vpw.Add(pow, 1)

		// delegating
		for pow > 7000 {
			_pow := max(bytes.RandInt64N(pow/100), 7000) // min amount of delegating

			vpw.Add(pow, _pow)
			pow -= _pow
		}

		vpws = append(vpws, vpw)
	}

	return vpws
}

func Test_VotingPowerWeight(t *testing.T) {
	maxSupply := types.ToFons(uint64(700_000_000))
	initialSupply := types.ToFons(uint64(350_000_000))
	totalSupply := initialSupply.Clone()

	//
	// init []*VotingPowerWeight and []*votingPowerObj
	totalPower := int64(0)
	vpWeights := initVotingPowerWeights()
	var vpObjs []*votingPowerObj
	for _, vpW := range vpWeights {
		for _, rvpw := range vpW.risingPowers {
			vpObjs = append(vpObjs, &votingPowerObj{
				vpow: rvpw.power,
				vdur: rvpw.bondingHeight,
			})
			totalPower += rvpw.power
		}
	}

	fmt.Println("vpWeights:", len(vpWeights), "vpObjs:", len(vpObjs), "totalPower:", totalPower)

	preW := decimal.Zero
	for height := int64(1); height < oneYearSeconds*10; /*10 years*/ height += powerRipeningCycle / 10 {
		W0, W1 := decimal.Zero, decimal.Zero

		// W of all vpWeights at `height`
		for _, vpW := range vpWeights {
			W0 = vpW.Compute(height, decimal.NewFromBigInt(totalSupply.ToBig(), 0), 200).Add(W0)
		}

		// W of all vpObjs at `height`
		for _, vpo := range vpObjs {
			W1 = Wi(vpo.vpow, vpo.vdur, decimal.NewFromBigInt(totalSupply.ToBig(), 0), 200).Add(W1)
			vpo.vdur += powerRipeningCycle / 10
		}

		W0 = W0.Truncate(3)
		W1 = W1.Truncate(3)

		require.True(t, W1.Sub(W0).Abs().LessThanOrEqual(decimal.RequireFromString("0.001")), fmt.Sprintf("height: %v, VotingPowerWeight: %v, votingPowerObj: %v", height, W0, W1))
		added := Sd(height, powerRipeningCycle/52, 1, initialSupply, maxSupply, "0.3", W0, preW)
		_ = totalSupply.Add(totalSupply, added)
		preW = W0

		//fmt.Println("height", height, "W0", W0, "W1", W1, "issued", types.ToBTOZ(added), "total", types.ToBTOZ(totalSupply), "dur0/1", dur0, dur1)
	}

}

func Test_Wi(t *testing.T) {
	totalSupply := types.ToFons(uint64(350_000_000))

	for idx := 0; idx < 1000; idx++ {
		vpObjs := initVotingPowerObjs(int64(350_000_000))
		wa := decimal.NewFromInt(0)

		for _, vpobj := range vpObjs {
			wi := Wi(vpobj.vpow, vpobj.vdur, decimal.NewFromBigInt(totalSupply.ToBig(), 0), 200)

			_wa := wa.Add(wi)

			require.True(t, _wa.LessThanOrEqual(decimal.NewFromInt(1)),
				fmt.Sprintf("wa: %v, wi:%v, new wa: %v", wa, wi, _wa))
			wa = _wa
		}
	}
}

func Test_Wa_SumWi(t *testing.T) {
	totalSupply := types.ToFons(uint64(350_000_000))
	tau := 200 // 0.200

	for n := 0; n < 1000; n++ {
		var vpows, vdurs []int64
		vpObjs := initVotingPowerObjs(int64(350_000_000))
		// Sum Wi
		sumWi0 := decimal.Zero
		for _, vpobj := range vpObjs {
			wi := Wi(vpobj.vpow, vpobj.vdur, decimal.NewFromBigInt(totalSupply.ToBig(), 0), tau)
			sumWi0 = sumWi0.Add(wi)

			// for computing Wa
			vpows = append(vpows, vpobj.vpow)
			vdurs = append(vdurs, vpobj.vdur)
		}
		sumWi0 = sumWi0.Truncate(6)
		//fmt.Println("sum of Wi", sumWi0)

		// Wa
		sumWi1 := Wa(vpows, vdurs, decimal.NewFromBigInt(totalSupply.ToBig(), 0), tau)
		//fmt.Println("Wa return", sumWi1)

		require.True(t, sumWi0.LessThanOrEqual(decimalOne), "SumWi0", sumWi0, "nth", n)
		require.True(t, sumWi1.LessThanOrEqual(decimalOne), "SumWi1", sumWi1, "nth", n)
		errVal := sumWi1.Sub(sumWi0)
		//fmt.Println("error value", errVal)
		require.True(t, errVal.LessThanOrEqual(decimal.RequireFromString("0.000001")), fmt.Sprintf("SumWi0:%v, SumWi1:%v, errVal:%v, nth:%v", sumWi0, sumWi1, errVal, n))
		require.True(t, sumWi0.Equal(sumWi1), fmt.Sprintf("SumWi0:%v, SumWi1:%v, errVal:%v, nth:%v", sumWi0, sumWi1, sumWi1.Sub(sumWi0), n))
	}
}

func Benchmark_SumWi(b *testing.B) {
	totalSupply := types.ToFons(uint64(350_000_000))
	tau := 200 // 0.200

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		vpObjs := initVotingPowerObjs(int64(100_000_000))
		sumWi := decimal.Zero
		b.StartTimer()

		for _, vpobj := range vpObjs {
			wi := Wi(vpobj.vpow, vpobj.vdur, decimal.NewFromBigInt(totalSupply.ToBig(), 0), tau)
			sumWi = sumWi.Add(wi)
		}
	}
}

func Benchmark_Wa(b *testing.B) {
	totalSupply := types.ToFons(uint64(350_000_000))
	tau := 200

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		var vpows, vdurs []int64
		vpObjs := initVotingPowerObjs(int64(100_000_000))
		b.StartTimer()

		for _, vpobj := range vpObjs {
			vpows = append(vpows, vpobj.vpow)
			vdurs = append(vdurs, vpobj.vdur)
		}
		_ = Wa(vpows, vdurs, decimal.NewFromBigInt(totalSupply.ToBig(), 0), tau)
	}
}
