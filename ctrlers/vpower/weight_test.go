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

// Test_Wa_SumWi tests whether the difference between the result of Wa and the sum of Wi is the same to 6 decimal places.
func Test_Wa_vs_SumWi(t *testing.T) {
	totalSupply := types.ToFons(uint64(350_000_000))
	tau := 200 // 0.200

	nOp := 1000
	for n := 0; n < nOp; n++ {
		var vpows, vdurs []int64
		vpObjs := initTestObjs(int64(350_000_000))
		// Sum Wi
		sumWi0 := decimal.Zero
		for _, vpobj := range vpObjs {
			wi := Wi(vpobj.vpow, vpobj.vdur, powerRipeningCycle, decimal.NewFromBigInt(totalSupply.ToBig(), 0), tau)
			sumWi0 = sumWi0.Add(wi)

			// for computing Wa
			vpows = append(vpows, vpobj.vpow)
			vdurs = append(vdurs, vpobj.vdur)
		}
		sumWi0 = sumWi0.Truncate(6)
		//fmt.Println("sum of Wi", sumWi0)

		// Wa
		sumWi1 := Wa(vpows, vdurs, powerRipeningCycle, decimal.NewFromBigInt(totalSupply.ToBig(), 0), tau)
		sumWi1 = sumWi1.Truncate(6)
		//fmt.Println("Wa return", sumWi1)

		require.True(t, sumWi0.LessThanOrEqual(decimalOne), "SumWi0", sumWi0, "nth", n)
		require.True(t, sumWi1.LessThanOrEqual(decimalOne), "SumWi1", sumWi1, "nth", n)
		errVal := sumWi1.Sub(sumWi0)
		//fmt.Println("error value", errVal)
		require.True(t, errVal.LessThanOrEqual(decimal.RequireFromString("0.000001")), fmt.Sprintf("SumWi0:%v, SumWi1:%v, errVal:%v, nth:%v", sumWi0, sumWi1, errVal, n))
		require.True(t, sumWi0.Equal(sumWi1), fmt.Sprintf("SumWi0:%v, SumWi1:%v, errVal:%v, nth:%v", sumWi0, sumWi1, errVal, n))
	}
}

// Test_Wa_SumWi tests whether the difference between the result of Wa and the sum of Wi is the same to 6 decimal places.
func Test_Wa_vs_SumWi_vs_WaWeighted(t *testing.T) {
	totalSupply := types.ToFons(uint64(350_000_000))
	tau := 200 // 0.200
	nOp := 1000
	dur0 := time.Duration(0)
	dur1 := time.Duration(0)
	dur2 := time.Duration(0)
	for n := 0; n < nOp; n++ {
		var vpows, vdurs []int64
		vpObjs := initTestObjs(int64(350_000_000))

		// Sum Wi
		sumWi0 := decimal.Zero
		start := time.Now()
		for _, vpobj := range vpObjs {
			wi := Wi(vpobj.vpow, vpobj.vdur, powerRipeningCycle, decimal.NewFromBigInt(totalSupply.ToBig(), 0), tau)
			sumWi0 = sumWi0.Add(wi)

			// for computing Wa
			vpows = append(vpows, vpobj.vpow)
			vdurs = append(vdurs, vpobj.vdur)
		}
		sumWi0 = sumWi0.Truncate(6)
		dur0 += time.Since(start)
		//fmt.Println("sum of Wi", sumWi0)

		// Wa
		start = time.Now()
		sumWi1 := Wa(vpows, vdurs, powerRipeningCycle, decimal.NewFromBigInt(totalSupply.ToBig(), 0), tau)
		sumWi1 = sumWi1.Truncate(6)
		dur1 += time.Since(start)
		//fmt.Println("Wa return", sumWi1)

		// WaWeighted
		start = time.Now()
		sumWi2 := WaWeighted(vpows, vdurs, powerRipeningCycle, decimal.NewFromBigInt(totalSupply.ToBig(), 0), tau)
		sumWi2 = sumWi2.Truncate(6)
		dur2 += time.Since(start)

		require.True(t, sumWi0.LessThanOrEqual(decimalOne), "SumWi0", sumWi0, "nth", n)
		require.True(t, sumWi1.LessThanOrEqual(decimalOne), "SumWi1", sumWi1, "nth", n)
		require.True(t, sumWi2.LessThanOrEqual(decimalOne), "sumWi2", sumWi2, "nth", n)

		require.True(t, sumWi0.Equal(sumWi1), fmt.Sprintf("SumWi0:%v, SumWi1:%v, nth:%v", sumWi0, sumWi1, n))
		require.True(t, sumWi1.Equal(sumWi2), fmt.Sprintf("SumWi1:%v, SumWi2:%v, nth:%v", sumWi1, sumWi2, n))
	}

	fmt.Println("SumWi", dur0/time.Duration(nOp), "Wa", dur1/time.Duration(nOp), "WaWeighted", dur2/time.Duration(nOp))
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

func Benchmark_WaWeighted(b *testing.B) {
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
		_ = WaWeighted(vpows, vdurs, powerRipeningCycle, decimal.NewFromBigInt(totalSupply.ToBig(), 0), tau)
	}
}
