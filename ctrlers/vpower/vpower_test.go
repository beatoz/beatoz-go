package vpower

import (
	"fmt"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/rand"
	"math"
	"testing"
)

func initVPowers() []*VPower {
	var vpows []*VPower

	for i := 0; i < 21; i++ {
		vpow := &VPower{}

		// self power
		pow := bytes.RandInt64N(1_000_000) + 1_000_000
		vpow.Add(pow, 1)

		// delegating
		for pow > 7000 {
			_pow := max(bytes.RandInt64N(pow/100), 7000) // min amount of delegating

			vpow.Add(pow, _pow)
			pow -= _pow
		}

		vpows = append(vpows, vpow)
	}

	return vpows
}

func Test_VPower_AddSub(t *testing.T) {
	from := rand.Bytes(20)
	to := rand.Bytes(20)

	var pows []int64
	totalPower := int64(0)
	vpow := &VPower{
		From: from,
		To:   to,
	}

	for i := 0; i < 10000; i++ {
		pow := rand.Int63n(7_000_000)
		vpow.Add(pow, int64(i+1))

		pows = append(pows, pow)
		totalPower += pow
	}
	require.Equal(t, int64(0), vpow.MaturePower())
	require.Equal(t, totalPower, vpow.RisingPower())
	require.Equal(t, totalPower, vpow.TotalPower())

	_ = vpow.Compute(powerRipeningCycle+1, types.ToFons(math.MaxUint64), 200)
	require.Equal(t, pows[0], vpow.MaturePower())
	require.Equal(t, totalPower-pows[0], vpow.RisingPower())
	require.Equal(t, totalPower, vpow.TotalPower())

	_ = vpow.Compute(powerRipeningCycle+2, types.ToFons(math.MaxUint64), 200)
	require.Equal(t, pows[0]+pows[1], vpow.MaturePower())
	require.Equal(t, totalPower-pows[0]-pows[1], vpow.RisingPower())
	require.Equal(t, totalPower, vpow.TotalPower())

	sumReducedPower := int64(0)
	originTotalPower := vpow.TotalPower()

	for {
		maturePower0 := vpow.MaturePower()
		risingPower0 := vpow.RisingPower()
		totalPower0 := vpow.TotalPower()
		require.Equal(t, maturePower0+risingPower0, totalPower0)

		// the request value may be greater than `totalPower0`.
		reducedPower := vpow.Sub(rand.Int63n(totalPower0) + totalPower0/3)
		sumReducedPower += reducedPower

		//fmt.Println(maturePower0, risingPower0, totalPower0, _subPow, subPow)

		if reducedPower >= maturePower0 {
			require.Equal(t, int64(0), vpow.MaturePower(), "subPow", reducedPower, vpow)
			require.Equal(t, maturePower0+risingPower0-reducedPower, vpow.RisingPower(), "subPow", reducedPower, vpow)
			require.Equal(t, totalPower0-reducedPower, vpow.TotalPower(), "subPow", reducedPower, vpow)
		} else {
			require.Equal(t, maturePower0-reducedPower, vpow.MaturePower(), "subPow", reducedPower, vpow)
			require.Equal(t, risingPower0, vpow.RisingPower(), "subPow", reducedPower, vpow)
			require.Equal(t, totalPower0-reducedPower, vpow.TotalPower(), "subPow", reducedPower, vpow)
		}

		require.GreaterOrEqual(t, vpow.MaturePower(), int64(0))
		require.GreaterOrEqual(t, vpow.RisingPower(), int64(0))
		require.GreaterOrEqual(t, vpow.TotalPower(), int64(0))

		if vpow.TotalPower() == 0 {
			break
		}
	}
	require.Equal(t, int64(0), vpow.MaturePower())
	require.Equal(t, int64(0), vpow.RisingPower())
	require.Equal(t, int64(0), vpow.TotalPower())
	require.Equal(t, originTotalPower, sumReducedPower)
}

func Test_VPower_Weight(t *testing.T) {
	maxSupply := types.ToFons(uint64(700_000_000))
	initialSupply := types.ToFons(uint64(350_000_000))
	totalSupply := initialSupply.Clone()

	//
	// init []*VPower and []*vpTestObj
	totalPower := int64(0)
	vpWeights := initVPowers()
	var vpObjs []*vpTestObj
	for _, vpW := range vpWeights {
		for _, rvpw := range vpW.risingWeights {
			vpObjs = append(vpObjs, &vpTestObj{
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
			W0 = vpW.Compute(height, totalSupply, 200).Add(W0)
		}

		// W of all vpObjs at `height`
		for _, vpo := range vpObjs {
			W1 = Wi(vpo.vpow, vpo.vdur, decimal.NewFromBigInt(totalSupply.ToBig(), 0), 200).Add(W1)
			vpo.vdur += powerRipeningCycle / 10
		}

		W0 = W0.Truncate(3)
		W1 = W1.Truncate(3)

		require.True(t, W1.Sub(W0).Abs().LessThanOrEqual(decimal.RequireFromString("0.001")), fmt.Sprintf("height: %v, VPower: %v, vpTestObj: %v", height, W0, W1))
		added := Sd(height, powerRipeningCycle/52, 1, initialSupply, maxSupply, "0.3", W0, preW)
		_ = totalSupply.Add(totalSupply, added)
		preW = W0

		//fmt.Println("height", height, "W0", W0, "W1", W1, "issued", types.ToBTOZ(added), "total", types.ToBTOZ(totalSupply), "dur0/1", dur0, dur1)
	}

}

type vpTestObj struct {
	vpow int64
	vdur int64
}

func initTestObjs(maxPow int64) []vpTestObj {
	var vpObjs []vpTestObj
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

func Test_Wi(t *testing.T) {
	totalSupply := types.ToFons(uint64(350_000_000))

	for idx := 0; idx < 1000; idx++ {
		vpObjs := initTestObjs(int64(350_000_000))
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
		vpObjs := initTestObjs(int64(350_000_000))
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
		vpObjs := initTestObjs(int64(100_000_000))
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
		vpObjs := initTestObjs(int64(100_000_000))
		b.StartTimer()

		for _, vpobj := range vpObjs {
			vpows = append(vpows, vpobj.vpow)
			vdurs = append(vdurs, vpobj.vdur)
		}
		_ = Wa(vpows, vdurs, decimal.NewFromBigInt(totalSupply.ToBig(), 0), tau)
	}
}
