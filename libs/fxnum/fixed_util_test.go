package fxnum

import (
	"fmt"
	"github.com/robaho/fixed"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"math"
	"runtime"
	"strconv"
	"testing"
	"testing/quick"
)

func Test_FixedPow_TwoRunsEqual1(t *testing.T) {
	cases := []struct {
		base, exp string
	}{
		{"2.0000000", "3.5000000"},
		{"1.2345678", "0.8765432"},
		{"10.0000000", "1.0000000"},
	}
	for _, c := range cases {
		b, _ := fixed.Parse(c.base)
		e, _ := fixed.Parse(c.exp)
		v1, err1 := FixedPow(b, e)
		v2, err2 := FixedPow(b, e)
		if err1 != nil || err2 != nil {
			t.Fatalf("err on %v^%v: %v %v", c.base, c.exp, err1, err2)
		}
		if !v1.Equal(v2) {
			t.Errorf("non-deterministic: %s^%s: %s != %s", c.base, c.exp, v1.String(), v2.String())
		}
	}
}

func Test_FixedPow_TwoRunsEqual2(t *testing.T) {
	const scale = 7
	maxVal := float64(math.MaxInt64) / math.Pow10(scale) // ≈ 9.22e14

	f := func(b, e float64) bool {
		// check b>0, e>=0
		// check out-of-range cases
		if b <= 0 || e < 0 || b > maxVal || e > maxVal {
			return true // skip out-of-range cases
		}
		base := fixed.NewI(int64(b*1e7), scale)
		exp := fixed.NewI(int64(e*1e7), scale)
		v1, _ := FixedPow(base, exp)
		v2, _ := FixedPow(base, exp)
		return v1.Equal(v2)
	}

	if err := quick.Check(f, nil); err != nil {
		t.Fatal(err)
	}
}

func Test_Decimal_Golden(t *testing.T) {
	data := []struct {
		base, exp, out string
	}{
		{"2.0000000", "3.5000000", "11.3137084"},
		{"2.0000000", "2.0000000", "4.0000000"},
		{"1.0000000", "5.6789000", "1.0000000"},
		{"4.0000000", "0.5000000", "2.0000000"},
		{"10.0000000", "-1.0000000", "0.1000000"},
		{"2.0000000", "0.5000000", "1.4142135"},
		{"3.0000000", "1.5000000", "5.1961524"},
		{"1.5000000", "2.5000000", "2.7556759"},
		{"2.7182818", "1.0000000", "2.7182818"},
		{"1.0000001", "100.0000000", "1.0000100"},
	}

	decimal.DivisionPrecision = 7
	for _, rec := range data {
		//
		// computed by decimal.Decimal
		b, err := decimal.NewFromString(rec.base)
		require.NoError(t, err)
		e, err := decimal.NewFromString(rec.exp)
		require.NoError(t, err)
		want, err := decimal.NewFromString(rec.out)
		require.NoError(t, err)

		got := b.Pow(e).Truncate(7)
		//fmt.Println("shopspring/decimal", "base", b, "exponent", e, "result", r.Truncate(7), "want", rec.out)
		require.True(t, got.Equal(want), "mismatch on %v^%v: got %v want %v", rec.base, rec.exp, got, want)
	}
}

func Test_Fixed_Golden(t *testing.T) {
	data := []struct {
		base, exp, out string
	}{
		/*
			OS:         Ubuntu 22.04.5 LTS
			Arch:       amd64 (x86_64)
			Go Version: go version go1.23.3 linux/amd64
			fxnum setting: lnTerms=32, expTerms=32
			FixedPow(fixed.NewI(2,0), fixed.NewI(35,1)) → 11.313694

			"11.3137085" computed by `shopspring/decimal` in iMac
		*/
		{"2.0000000", "3.5000000", "11.313694"},
		{"2.0000000", "2.0000000", "3.9999970"},
		{"1.0000000", "5.6789000", "1.0000000"},
		{"4.0000000", "0.5000000", "1.9999996"},
		{"10.0000000", "-1.0000000", "0.1000001"},
		{"2.0000000", "0.5000000", "1.4142133"},
		{"3.0000000", "1.5000000", "5.1961519"},
		{"1.5000000", "2.5000000", "2.7556766"},
		{"2.7182818", "1.0000000", "2.7182818"},
		{"1.0000001", "100.0000000", "1.0000000"},
	}
	for _, rec := range data {
		b, err := fixed.Parse(rec.base)
		require.NoError(t, err)
		e, err := fixed.Parse(rec.exp)
		require.NoError(t, err)
		want, err := fixed.Parse(rec.out)
		require.NoError(t, err)
		got, err := FixedPow(b, e)
		require.NoError(t, err)
		//fmt.Println("robaho/fixed", "base", b, "exponent", e, "result", got, "want", want)
		require.True(t, got.Equal(want), "mismatch on %v^%v: got %v want %v", rec.base, rec.exp, got, rec.out)
	}

}

func NonDetPow(x, y float64) float64 {
	// x^y = exp(y * ln(x)) using float64
	return math.Exp(y * math.Log(x))
}

func TestNonDetPow(t *testing.T) {
	/*
		- Model: iMac, Retina 5K, 27-inch, 2020.
		- CPU: 3.8 GHz 8코어 Intel Core i7
		- OS: macOS Sequoia 15.4.1(24E263)

		NonDetPow(1.2345678, 0.8765432) = 1.202864764209711
		NonDetPow(2.7182818, 1.6180339) = 5.043165110348398
		NonDetPow(10.0000000, 0.1234567) = 1.328791067482019
		NonDetPow(5.0000000, 2.5000000) = 55.901699437494734
	*/

	inputs := []struct {
		x, y          float64
		expected      string
		fixedExpected string
	}{
		{1.2345678, 0.8765432, "1.202864764209711", "1.2028646"},
		{2.7182818, 1.6180339, "5.043165110348398", "5.0431653"},
		{10.0000000, 0.1234567, "1.328791067482019", "1.328791"},
		{5.0000000, 2.5000000, "55.901699437494734", "55.9016866"},
	}

	fmt.Printf("System: %s, %s, %s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	cpuInfo, _ := cpu.Info()
	if cpuInfo != nil {
		for _, info := range cpuInfo {
			fmt.Printf("%v, %v Cores\n", info.ModelName, info.Cores)
		}

	}

	for _, in := range inputs {
		r := NonDetPow(in.x, in.y)
		if strconv.FormatFloat(r, 'f', 15, 64) != in.expected {
			fmt.Printf("float64 - NonDetPow(%.7f, %.7f) = %.15f, other's result: %s\n", in.x, in.y, r, in.expected)
		}

		fixedX := fixed.NewI(int64(in.x*1e7), 7)
		fixedY := fixed.NewI(int64(in.y*1e7), 7)
		fixedR, err := FixedPow(fixedX, fixedY)
		require.NoError(t, err)
		require.Equal(t, in.fixedExpected, fixedR.String(), fmt.Sprintf("FixedPow(%v, %v) = %v\n", fixedX, fixedY, fixedR))
	}
}
