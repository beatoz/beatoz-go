package vpower

import (
	"fmt"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

var (
	powerRipeningCycle = oneYearSeconds
)

type testPowObj struct {
	vpow int64
	vdur int64
}

func initTestObjs(maxPow int64) []testPowObj {
	var vpObjs []testPowObj
	for maxPow > 7000 {
		pow := max(bytes.RandInt64N(1_000_000), 7000)
		v := struct {
			vpow int64
			vdur int64
		}{
			vpow: pow,
			vdur: bytes.RandInt64N(powerRipeningCycle * 2),
		}
		vpObjs = append(vpObjs, v)

		maxPow -= pow
	}
	return vpObjs
}

// Test_Wi tests that the sum of Wi is not greater than `1` for any value of the voting power.
func Test_Wi(t *testing.T) {
	totalSupply := types.ToFons(uint64(700_000_000))

	nOp := 1000
	for idx := 0; idx < nOp; idx++ {
		vpObjs := initTestObjs(int64(700_000_000))
		wa := decimal.NewFromInt(0)

		for _, vpobj := range vpObjs {
			wi0 := oldWi(vpobj.vpow, vpobj.vdur, powerRipeningCycle, decimal.NewFromBigInt(totalSupply.ToBig(), 0), 200)
			wi1 := Wi(vpobj.vpow, vpobj.vdur, powerRipeningCycle, decimal.NewFromBigInt(totalSupply.ToBig(), 0), 200)
			require.Equal(t, wi0, wi1)

			_wa := wa.Add(wi1)

			require.True(t, _wa.LessThanOrEqual(decimal.NewFromInt(1)),
				fmt.Sprintf("wa: %v, wi_old:%v, wi: %v, new wa: %v", wa, wi0, wi1, _wa))
			wa = _wa
		}
	}
}

func Test_SumWi_Wa_WaEx_WaEx64(t *testing.T) {
	totalSupply := types.ToFons(uint64(350_000_000))
	tau := 200 // 0.200
	nOp := 1000
	dur0 := time.Duration(0)
	dur1 := time.Duration(0)
	dur2 := time.Duration(0)
	dur3 := time.Duration(0)
	for n := 0; n < nOp; n++ {
		var vpows, vdurs []int64
		vpObjs := initTestObjs(int64(350_000_000))

		// Sum Wi
		w_sumwi := decimal.Zero
		start := time.Now()
		for _, vpobj := range vpObjs {
			wi := Wi(vpobj.vpow, vpobj.vdur, powerRipeningCycle, decimal.NewFromBigInt(totalSupply.ToBig(), 0), tau)
			w_sumwi = w_sumwi.Add(wi)

			// for computing Wa
			vpows = append(vpows, vpobj.vpow)
			vdurs = append(vdurs, vpobj.vdur)
		}
		w_sumwi = w_sumwi.Truncate(6)
		dur0 += time.Since(start)
		//fmt.Println("sum of Wi", Wa0)

		// Wa
		start = time.Now()
		w_wa := Wa(vpows, vdurs, powerRipeningCycle, decimal.NewFromBigInt(totalSupply.ToBig(), 0), tau)
		w_wa = w_wa.Truncate(6)
		dur1 += time.Since(start)
		//fmt.Println("Wa return", w_wa)

		// WaEx
		start = time.Now()
		w_waex := WaEx(vpows, vdurs, powerRipeningCycle, decimal.NewFromBigInt(totalSupply.ToBig(), 0), tau)
		w_waex = w_waex.Truncate(6)
		dur2 += time.Since(start)

		// WaEx64
		start = time.Now()
		w_waex64 := WaEx64(vpows, vdurs, powerRipeningCycle, totalSupply, tau)
		w_waex64 = w_waex64.Truncate(6)
		dur3 += time.Since(start)

		require.True(t, w_sumwi.LessThanOrEqual(decimalOne), "SumWi", w_sumwi, "nth", n)
		require.True(t, w_wa.LessThanOrEqual(decimalOne), "Wa", w_wa, "nth", n)
		require.True(t, w_waex.LessThanOrEqual(decimalOne), "WaEx", w_waex, "nth", n)
		require.True(t, w_waex64.LessThanOrEqual(decimalOne), "WaEx64", w_waex64, "nth", n)

		require.True(t, w_sumwi.Equal(w_wa), fmt.Sprintf("SumWi:%v, Wa:%v, nth:%v", w_sumwi, w_wa, n))
		require.True(t, w_wa.Equal(w_waex), fmt.Sprintf("Wa:%v, WaEx:%v, nth:%v", w_wa, w_waex, n))
		require.True(t, w_waex.Equal(w_waex64), fmt.Sprintf("WaEx:%v, WaEx64:%v, nth:%v", w_waex, w_waex64, n))
	}

	fmt.Println("SumWi", dur0/time.Duration(nOp), "Wa", dur1/time.Duration(nOp), "WaEx", dur2/time.Duration(nOp), "WaEx64", dur3/time.Duration(nOp))
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
			wi := Wi(vpobj.vpow, vpobj.vdur, powerRipeningCycle, decimal.NewFromBigInt(totalSupply.ToBig(), 0), tau)
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
		_ = Wa(vpows, vdurs, powerRipeningCycle, decimal.NewFromBigInt(totalSupply.ToBig(), 0), tau)
	}
}

func Benchmark_WaEx(b *testing.B) {
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
		_ = WaEx(vpows, vdurs, powerRipeningCycle, decimal.NewFromBigInt(totalSupply.ToBig(), 0), tau)
	}
}

func Benchmark_WaEx64(b *testing.B) {
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
		_ = WaEx64(vpows, vdurs, powerRipeningCycle, totalSupply, tau)
	}
}
