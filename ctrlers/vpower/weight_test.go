package vpower

import (
	"fmt"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/holiman/uint256"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"math/rand"
	"testing"
	"time"
)

var (
	powerRipeningCycle = ctrlertypes.YearSeconds
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
			wi0 := oldWi(vpobj.vpow, vpobj.vdur, powerRipeningCycle, 200, decimal.NewFromBigInt(totalSupply.ToBig(), 0))
			wi0 = wi0.Truncate(GetGuaranteedPrecision())
			wi1 := Wi(vpobj.vpow, vpobj.vdur, powerRipeningCycle, 200, decimal.NewFromBigInt(totalSupply.ToBig(), 0))
			wi1 = wi1.Truncate(GetGuaranteedPrecision())
			require.Equal(t, wi0, wi1, "not equal", "wi0", wi0, "wi1", wi1)

			_wa := wa.Add(wi1)

			require.True(t, _wa.LessThanOrEqual(decimal.NewFromInt(1)),
				fmt.Sprintf("wa: %v, wi_old:%v, wi: %v, new wa: %v", wa, wi0, wi1, _wa))
			wa = _wa
		}
	}
}

func Test_SumWi_Wa_WaEx_WaEx64(t *testing.T) {
	totalSupply := types.ToFons(uint64(350_000_000))
	tau := int32(200) // 0.200
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
			wi := Wi(vpobj.vpow, vpobj.vdur, powerRipeningCycle, tau, decimal.NewFromBigInt(totalSupply.ToBig(), 0))
			w_sumwi = w_sumwi.Add(wi)

			// for computing Wa
			vpows = append(vpows, vpobj.vpow)
			vdurs = append(vdurs, vpobj.vdur)
		}
		w_sumwi = w_sumwi.Truncate(GetGuaranteedPrecision())
		dur0 += time.Since(start)
		//fmt.Println("sum of Wi", w_sumwi)

		// Wa
		start = time.Now()
		w_wa := Wa(vpows, vdurs, powerRipeningCycle, tau, decimal.NewFromBigInt(totalSupply.ToBig(), 0))
		w_wa = w_wa.Truncate(GetGuaranteedPrecision())
		dur1 += time.Since(start)
		//fmt.Println("Wa return", w_wa)

		// WaEx
		start = time.Now()
		w_waex := WaEx(vpows, vdurs, powerRipeningCycle, tau, decimal.NewFromBigInt(totalSupply.ToBig(), 0))
		w_waex = w_waex.Truncate(GetGuaranteedPrecision())
		dur2 += time.Since(start)

		// WaEx64
		start = time.Now()
		w_waex64 := WaEx64(vpows, vdurs, powerRipeningCycle, tau, totalSupply)
		w_waex64 = w_waex64.Truncate(GetGuaranteedPrecision())
		dur3 += time.Since(start)

		require.True(t, w_sumwi.LessThanOrEqual(ctrlertypes.DecimalOne), "SumWi", w_sumwi, "nth", n)
		require.True(t, w_wa.LessThanOrEqual(ctrlertypes.DecimalOne), "Wa", w_wa, "nth", n)
		require.True(t, w_waex.LessThanOrEqual(ctrlertypes.DecimalOne), "WaEx", w_waex, "nth", n)
		require.True(t, w_waex64.LessThanOrEqual(ctrlertypes.DecimalOne), "WaEx64", w_waex64, "nth", n)

		require.True(t, w_sumwi.Equal(w_wa), fmt.Sprintf("SumWi:%v, Wa:%v, nth:%v", w_sumwi, w_wa, n))
		require.True(t, w_wa.Equal(w_waex), fmt.Sprintf("Wa:%v, WaEx:%v, nth:%v", w_wa, w_waex, n))
		require.True(t, w_waex.Equal(w_waex64), fmt.Sprintf("WaEx:%v, WaEx64:%v, nth:%v", w_waex, w_waex64, n))
	}

	fmt.Println("SumWi", dur0/time.Duration(nOp), "Wa", dur1/time.Duration(nOp), "WaEx", dur2/time.Duration(nOp), "WaEx64", dur3/time.Duration(nOp))
}

func Test_WaEx64Pc_Weight64Pc(t *testing.T) {
	totalSupply := types.ToFons(uint64(350_000_000))
	tau := int32(200) // 0.200
	nOp := 1000
	dur0 := time.Duration(0)
	dur1 := time.Duration(0)
	dur2 := time.Duration(0)
	dur3 := time.Duration(0)
	dur4 := time.Duration(0)
	dur5 := time.Duration(0)
	dur6 := time.Duration(0)

	for n := 0; n < nOp; n++ {
		currHeight := powerRipeningCycle
		var vpows, vdurs []int64
		var powChunks []*PowerChunkProto

		// create random power chunks
		maxPow := int64(350_000_000)
		for maxPow > 7000 {
			pow := max(bytes.RandInt64N(1_000_000), 7000)
			height := bytes.RandInt64N(currHeight)*2 + 1
			powChunks = append(powChunks, &PowerChunkProto{
				Power:  pow,
				Height: height,
				TxHash: nil,
			})

			vpows = append(vpows, pow)
			vdurs = append(vdurs, currHeight-height)

			maxPow -= pow
		}

		// Sum Wi
		w_sumwi := decimal.Zero
		start := time.Now()
		for i, pow := range vpows {
			dur := vdurs[i]
			wi := Wi(pow, dur, powerRipeningCycle, tau, decimal.NewFromBigInt(totalSupply.ToBig(), 0))
			w_sumwi = w_sumwi.Add(wi)
		}
		w_sumwi = w_sumwi.Truncate(GetGuaranteedPrecision())
		dur0 += time.Since(start)
		//fmt.Println("sum of Wi", w_sumwi)

		// Wa
		start = time.Now()
		w_wa := Wa(vpows, vdurs, powerRipeningCycle, tau, decimal.NewFromBigInt(totalSupply.ToBig(), 0))
		w_wa = w_wa.Truncate(GetGuaranteedPrecision())
		dur1 += time.Since(start)
		//fmt.Println("Wa return", w_wa)

		// WaEx
		start = time.Now()
		w_waex := WaEx(vpows, vdurs, powerRipeningCycle, tau, decimal.NewFromBigInt(totalSupply.ToBig(), 0))
		w_waex = w_waex.Truncate(GetGuaranteedPrecision())
		dur2 += time.Since(start)

		// WaEx64
		start = time.Now()
		w_waex64 := WaEx64(vpows, vdurs, powerRipeningCycle, tau, totalSupply)
		w_waex64 = w_waex64.Truncate(GetGuaranteedPrecision())
		dur3 += time.Since(start)
		//fmt.Println("WaEx64 return", w_waex64)

		// WaEx64ByPowerChunks
		start = time.Now()
		w_waex64pc := WaEx64ByPowerChunk(powChunks, currHeight, powerRipeningCycle, tau, totalSupply)
		w_waex64pc = w_waex64pc.Truncate(GetGuaranteedPrecision())
		dur4 += time.Since(start)
		//fmt.Println("WaEx64ByPowerChunk return", w_waex64pc)

		// Weight64ByPowerChunks
		start = time.Now()
		w_w64pc := Scaled64PowerChunk(powChunks, currHeight, powerRipeningCycle, tau)
		_totalSupply := decimal.NewFromBigInt(totalSupply.ToBig(), -1*int32(types.DECIMAL))
		w_w64pc, _ = w_w64pc.QuoRem(_totalSupply, GetDivisionPrecision())
		w_w64pc = w_w64pc.Truncate(GetGuaranteedPrecision())
		dur5 += time.Since(start)
		//fmt.Println("Scaled64PowerChunk return", w_w64pc)

		// Weight64ByPowerChunks
		rdx := rand.Intn(len(powChunks))
		start = time.Now()
		w_w64pc_patial0 := Scaled64PowerChunk(powChunks[:rdx], currHeight, powerRipeningCycle, tau)
		w_w64pc_patial1 := Scaled64PowerChunk(powChunks[rdx:], currHeight, powerRipeningCycle, tau)
		w_w64pc_patial := w_w64pc_patial0.Add(w_w64pc_patial1)
		w_w64pc_patial, _ = w_w64pc_patial.QuoRem(_totalSupply, GetDivisionPrecision())
		w_w64pc_patial = w_w64pc_patial.Truncate(GetGuaranteedPrecision())
		dur6 += time.Since(start)
		//fmt.Println("Scaled64PowerChunk return", w_w64pc)

		require.True(t, w_sumwi.LessThanOrEqual(ctrlertypes.DecimalOne), "SumWi", w_sumwi, "nth", n)
		require.True(t, w_wa.LessThanOrEqual(ctrlertypes.DecimalOne), "Wa", w_wa, "nth", n)
		require.True(t, w_waex.LessThanOrEqual(ctrlertypes.DecimalOne), "WaEx", w_waex, "nth", n)
		require.True(t, w_waex64.LessThanOrEqual(ctrlertypes.DecimalOne), "WaEx64", w_waex64, "nth", n)
		require.True(t, w_waex64pc.LessThanOrEqual(ctrlertypes.DecimalOne), "WaEx64ByPowerChunk", w_waex64pc, "nth", n)
		require.True(t, w_w64pc.LessThanOrEqual(ctrlertypes.DecimalOne), "By Scaled64PowerChunk", w_w64pc, "nth", n)
		require.True(t, w_w64pc_patial.LessThanOrEqual(ctrlertypes.DecimalOne), "Weight64ByPowerChunkPartially", w_w64pc_patial, "nth", n)

		// w_sumwi has error, when the height is too small (less than about 500).
		//require.True(t, w_sumwi.Equal(w_wa), fmt.Sprintf("SumWi:%v, Wa:%v, nth:%v", w_sumwi, w_wa, n))
		require.True(t, w_wa.Equal(w_waex), fmt.Sprintf("Wa:%v, WaEx:%v, nth:%v", w_wa, w_waex, n))
		require.True(t, w_waex.Equal(w_waex64), fmt.Sprintf("WaEx:%v, WaEx64:%v, nth:%v", w_waex, w_waex64, n))
		require.True(t, w_waex64.Equal(w_waex64pc), fmt.Sprintf("WaEx64:%v, WaEx64Pc:%v, nth:%v", w_waex64, w_waex64pc, n))
		require.True(t, w_waex64pc.Equal(w_w64pc), fmt.Sprintf("WaEx64Pc:%v, W64Pc:%v, nth:%v", w_waex64pc, w_w64pc, n))
		require.True(t, w_w64pc.Equal(w_w64pc_patial), fmt.Sprintf("W64Pc:%v, W64PcPartial:%v, nth:%v", w_w64pc, w_w64pc_patial, n))
	}

	fmt.Println(
		"SumWi", dur0/time.Duration(nOp),
		"Wa", dur1/time.Duration(nOp),
		"WaEx", dur2/time.Duration(nOp),
		"WaEx64", dur3/time.Duration(nOp),
		"WaEx64Pc", dur4/time.Duration(nOp),
		"W64Pc", dur5/time.Duration(nOp),
		"W64PcParital", dur6/time.Duration(nOp),
	)
}

func Benchmark_SumWi(b *testing.B) {
	totalSupply := types.ToFons(uint64(350_000_000))
	tau := int32(200) // 0.200

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		vpObjs := initTestObjs(int64(100_000_000))
		sumWi := decimal.Zero
		b.StartTimer()

		for _, vpobj := range vpObjs {
			wi := Wi(vpobj.vpow, vpobj.vdur, powerRipeningCycle, tau, decimal.NewFromBigInt(totalSupply.ToBig(), 0))
			sumWi = sumWi.Add(wi)
		}
	}
}

func Benchmark_Wa(b *testing.B) {
	totalSupply := types.ToFons(uint64(350_000_000))
	tau := int32(200)

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		var vpows, vdurs []int64
		vpObjs := initTestObjs(int64(100_000_000))
		b.StartTimer()

		for _, vpobj := range vpObjs {
			vpows = append(vpows, vpobj.vpow)
			vdurs = append(vdurs, vpobj.vdur)
		}
		_ = Wa(vpows, vdurs, powerRipeningCycle, tau, decimal.NewFromBigInt(totalSupply.ToBig(), 0))
	}
}

func Benchmark_WaEx(b *testing.B) {
	totalSupply := types.ToFons(uint64(350_000_000))
	tau := int32(200)

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		var vpows, vdurs []int64
		vpObjs := initTestObjs(int64(100_000_000))
		b.StartTimer()

		for _, vpobj := range vpObjs {
			vpows = append(vpows, vpobj.vpow)
			vdurs = append(vdurs, vpobj.vdur)
		}
		_ = WaEx(vpows, vdurs, powerRipeningCycle, tau, decimal.NewFromBigInt(totalSupply.ToBig(), 0))
	}
}

func Benchmark_WaEx64(b *testing.B) {
	totalSupply := types.ToFons(uint64(350_000_000))
	tau := int32(200)

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		var vpows, vdurs []int64
		vpObjs := initTestObjs(int64(100_000_000))
		b.StartTimer()

		for _, vpobj := range vpObjs {
			vpows = append(vpows, vpobj.vpow)
			vdurs = append(vdurs, vpobj.vdur)
		}
		_ = WaEx64(vpows, vdurs, powerRipeningCycle, tau, totalSupply)
	}
}

func Test_temp(t *testing.T) {
	SetDivisionPrecision(1)
	tau := decimal.New(int64(2), -3)
	fmt.Println("tau", tau)
	tau = tau.Mul(decimal.NewFromInt(2))
	fmt.Println("tau", tau)
	tau, _ = tau.QuoRem(decimal.NewFromInt(2), int32(types.DECIMAL))
	fmt.Println("tau", tau)
}

func Benchmark_Precision_6(b *testing.B) {
	SetDivisionPrecision(3)

	totalSupply := types.ToFons(uint64(123_123_456_789))
	votingPower := int64(123_456_789)
	powerPeriod := powerRipeningCycle / 3

	for i := 0; i < b.N; i++ {
		tau := decimal.New(int64(200), -3)
		keppa := ctrlertypes.DecimalOne.Sub(tau)

		vpPart := tau.Mul(decimal.NewFromInt(powerPeriod)).
			Div(decimal.NewFromInt(powerRipeningCycle)).
			Add(keppa).
			Mul(decimal.NewFromInt(votingPower)) // 106995924.952263
		decTotalSupply := decimal.NewFromBigInt(totalSupply.ToBig(), -1*int32(types.DECIMAL))
		_, _ = vpPart.QuoRem(decTotalSupply, int32(decimal.DivisionPrecision))
		//fmt.Println("Precision 6", "vpPart", vpPart, "w", w)
	}
}

func Benchmark_Precision_16(b *testing.B) {
	SetDivisionPrecision(16)

	totalSupply := types.ToFons(uint64(123_123_456_789))
	votingPower := int64(123_456_789)
	powerPeriod := powerRipeningCycle / 3

	for i := 0; i < b.N; i++ {
		tau := decimal.New(int64(200), -3)
		keppa := ctrlertypes.DecimalOne.Sub(tau)

		vpPart := tau.Mul(decimal.NewFromInt(powerPeriod)).
			Div(decimal.NewFromInt(powerRipeningCycle)).
			Add(keppa).
			Mul(decimal.NewFromInt(votingPower)) // 106995883.8000000041152263
		decTotalSupply := decimal.NewFromBigInt(totalSupply.ToBig(), -1*int32(types.DECIMAL))
		_, _ = vpPart.QuoRem(decTotalSupply, int32(decimal.DivisionPrecision))
		//fmt.Println("Precision 6", "vpPart", vpPart, "w", w)
	}
}

func Benchmark_Decimal_NewExp(b *testing.B) {
	totalSupply := types.ToFons(uint64(123_123_456_789))

	for i := 0; i < b.N; i++ {
		_ = decimal.NewFromBigInt(totalSupply.ToBig(), -1*int32(types.DECIMAL))
	}
}

func Benchmark_Decimal_NewDiv(b *testing.B) {
	totalSupply := types.ToFons(uint64(123_123_456_789))

	for i := 0; i < b.N; i++ {
		_ = decimal.NewFromBigInt(totalSupply.ToBig(), 0).Div(decimal.New(1, int32(types.DECIMAL)))
	}
}

func Test_Precision(t *testing.T) {

	totalSupply := uint256.MustFromDecimal("789123456789123456789123456789")
	votingPower := int64(123_456_789)
	powerPeriod := powerRipeningCycle / 3

	//
	// Precision 6
	SetDivisionPrecision(8)
	fmt.Println("precision           ", GetDivisionPrecision())
	fmt.Println("votingPower         ", decimal.NewFromInt(votingPower))

	tau := decimal.New(int64(200), -3)
	fmt.Println("tau                 ", tau)
	keppa := ctrlertypes.DecimalOne.Sub(tau)
	fmt.Println("keppa               ", keppa)

	vpPart := tau.Mul(decimal.NewFromInt(powerPeriod))
	fmt.Println("vpPart*tau          ", vpPart)
	vpPart, _ = vpPart.QuoRem(decimal.NewFromInt(powerRipeningCycle), GetDivisionPrecision())
	fmt.Println("vpPart/ripeningCycle", vpPart)
	vpPart = vpPart.Add(keppa)
	fmt.Println("vpPart+keppa        ", vpPart)
	vpPart = vpPart.Mul(decimal.NewFromInt(votingPower)) // 106995883.8000000041152263
	fmt.Println("vpPart*vpower       ", vpPart)
	decTotalSupply := decimal.NewFromBigInt(totalSupply.ToBig(), -1*int32(types.DECIMAL))
	fmt.Println("decTotalSupply      ", decTotalSupply)
	q, r := vpPart.QuoRem(decTotalSupply, int32(GetDivisionPrecision()))
	fmt.Println("result", q, "remainder", r)

	fmt.Println("--------------------------")

	//
	// Precision 16 (default)
	SetDivisionPrecision(16)
	fmt.Println("precision           ", GetDivisionPrecision())
	fmt.Println("votingPower         ", decimal.NewFromInt(votingPower))

	tau = decimal.New(int64(200), -3)
	fmt.Println("tau                 ", tau)
	keppa = ctrlertypes.DecimalOne.Sub(tau)
	fmt.Println("keppa               ", keppa)

	vpPart = tau.Mul(decimal.NewFromInt(powerPeriod))
	fmt.Println("vpPart*tau          ", vpPart)
	vpPart, _ = vpPart.QuoRem(decimal.NewFromInt(powerRipeningCycle), GetDivisionPrecision())
	fmt.Println("vpPart/ripeningCycle", vpPart)
	vpPart = vpPart.Add(keppa)
	fmt.Println("vpPart+keppa        ", vpPart)
	vpPart = vpPart.Mul(decimal.NewFromInt(votingPower)) // 106995883.8000000041152263
	fmt.Println("vpPart*vpower       ", vpPart)
	decTotalSupply = decimal.NewFromBigInt(totalSupply.ToBig(), -1*int32(types.DECIMAL))
	fmt.Println("decTotalSupply      ", decTotalSupply)
	q, r = vpPart.QuoRem(decTotalSupply, int32(GetDivisionPrecision()))
	fmt.Println("result", q, "remainder", r)
}
