package fxnum

import (
	"github.com/robaho/fixed"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"math/rand"
	"testing"
)

var (
	numCount  = 100
	decNums   []decimal.Decimal
	fixedNums []fixed.Fixed
	fixedExps []fixed.Fixed
	decExps   []decimal.Decimal
	fxnumNums []FxNum
	fxnumExps []FxNum
)

func init() {
	for i := 0; i < numCount; i++ {
		n := rand.Int63n(5000000000)
		decNums = append(decNums, decimal.NewFromInt(n))
		fixedNums = append(fixedNums, fixed.NewI(n, 0))
		fxnumNums = append(fxnumNums, FromInt(n))

		n = rand.Int63n(1000)
		decExps = append(decExps, decimal.New(n, -3))
		fixedExps = append(fixedExps, fixed.NewI(n, 3))
		fxnumExps = append(fxnumExps, New(n, 3))
	}
}

func Benchmark_FxNum_Add(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = fxnumNums[i%numCount].Add(fxnumNums[(i+1)%numCount])
	}
}
func Benchmark_Fixed_Add(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = fixedNums[i%numCount].Add(fixedNums[(i+1)%numCount])
	}
}
func Benchmark_Decimal_Add(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = decNums[i%numCount].Add(decNums[(i+1)%numCount])
	}
}

func Benchmark_FxNum_Mul(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = fxnumNums[i%numCount].Mul(fxnumNums[(i+1)%numCount])
	}
}
func Benchmark_Fixed_Mul(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = fixedNums[i%numCount].Mul(fixedNums[(i+1)%numCount])
	}
}
func Benchmark_Decimal_Mul(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = decNums[i%numCount].Mul(decNums[(i+1)%numCount])
	}
}

func Benchmark_FxNum_Pow(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = fxnumNums[i%numCount].Pow(fxnumExps[i%numCount])
	}
}
func Benchmark_Fixed_Pow(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = FixedPow(fixedNums[i%numCount], fixedExps[i%numCount])
	}
}

func Benchmark_Decimal_Pow(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = decNums[i%numCount].Pow(decExps[i%numCount])
	}
}

func Benchmark_FixedToDecimalByInt(b *testing.B) {
	src := fixed.NewI(91234567898, 6)
	for i := 0; i < b.N; i++ {
		_, _ = FixedToDecimalByInt(src)
	}
}

func Benchmark_FixedToDecimalByString(b *testing.B) {
	src := fixed.NewI(91234567898, 6)
	for i := 0; i < b.N; i++ {
		_, _ = FixedToDecimalByString(src)
	}
}

func Test_FixedToDecimalByInt(t *testing.T) {
	src := fixed.NewI(6789891234567898, 7)
	expected := "678989123.4567898"

	dst, err := FixedToDecimalByString(src)
	require.NoError(t, err)
	require.Equal(t, expected, src.String())
	require.Equal(t, expected, dst.String())
}
