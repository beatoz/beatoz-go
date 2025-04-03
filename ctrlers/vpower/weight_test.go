package vpower

import (
	"fmt"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"testing"
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

	for idx := 0; idx < 1000; idx++ {
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
func Test_Wa_SumWi(t *testing.T) {
	totalSupply := types.ToFons(uint64(350_000_000))
	tau := 200 // 0.200

	for n := 0; n < 1000; n++ {
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
		sumWi1 := Wa(vpows, vdurs, powerRipeningCycle, decimal.NewFromBigInt(totalSupply.ToBig(), 0), tau).Truncate(6)
		//fmt.Println("Wa return", sumWi1)

		require.True(t, sumWi0.LessThanOrEqual(decimalOne), "SumWi0", sumWi0, "nth", n)
		require.True(t, sumWi1.LessThanOrEqual(decimalOne), "SumWi1", sumWi1, "nth", n)
		errVal := sumWi1.Sub(sumWi0)
		//fmt.Println("error value", errVal)
		require.True(t, errVal.LessThanOrEqual(decimal.RequireFromString("0.000001")), fmt.Sprintf("SumWi0:%v, SumWi1:%v, errVal:%v, nth:%v", sumWi0, sumWi1, errVal, n))
		require.True(t, sumWi0.Equal(sumWi1), fmt.Sprintf("SumWi0:%v, SumWi1:%v, errVal:%v, nth:%v", sumWi0, sumWi1, errVal, n))
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
