package fxnum

import (
	"github.com/robaho/fixed"
	"github.com/stretchr/testify/require"
	"math/rand"
	"testing"
)

var (
	numCount  = 1000000
	fixedNums []fixed.Fixed
	fxnumNums []FxNum
)

func init() {
	for i := 0; i < numCount; i++ {
		n := rand.Int63n(5000000000)
		fixedNums = append(fixedNums, fixed.NewI(n, 0))
		fxnumNums = append(fxnumNums, FromInt(n))
	}
}

func Benchmark_Add(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = fixedNums[i%numCount].Add(fixedNums[i%numCount])
	}
}

func Benchmark_Add_FxNum(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = fxnumNums[i%numCount].Add(fxnumNums[i%numCount])
	}
}

func Benchmark_Mul(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = fixedNums[i%numCount].Mul(fixedNums[i%numCount])
	}
}

func Benchmark_Mul_FxNum(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = fxnumNums[i%numCount].Mul(fxnumNums[i%numCount])
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
