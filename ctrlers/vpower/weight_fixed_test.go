package vpower

import (
	"fmt"
	"github.com/robaho/fixed"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"math/rand"
	"testing"
)

var (
	powerChunks []*PowerChunkProto
)

func init() {
	SetDivisionPrecision(7)
	preparePowerChunks()
}

func preparePowerChunks() {
	n := 21 * 1000 // 21 validators, and 1000 delegators per a validator.
	powerChunks = make([]*PowerChunkProto, n)
	for i := 0; i < n; i++ {
		powerChunks[i] = &PowerChunkProto{
			Power:  rand.Int63n(int64(fixed.MAX) / int64(n)),
			Height: rand.Int63n(powerRipeningCycle),
		}
	}
}

func Test_Scaled64PowerChunkFixed(t *testing.T) {
	for i := 0; i < 10; i++ {
		preparePowerChunks()
		w0 := Scaled64PowerChunks(powerChunks, powerRipeningCycle*3/2, powerRipeningCycle, int32(200))
		w1 := FixedWeightedPowerChunks(powerChunks, powerRipeningCycle*3/2, powerRipeningCycle, int32(200))

		diff, err := decimal.NewFromString(w1.String())
		require.NoError(t, err)
		diff = diff.Sub(w0)
		fmt.Println("w0", w0, "w1", w1, "diff", diff.Abs())
	}
}

func Test_Fixed_Decimal(t *testing.T) {
	preparePowerChunks()
	fmt.Println("powerChunks size", len(powerChunks))

	wa_pow := Scaled64PowerChunks(powerChunks, powerRipeningCycle*3/2, powerRipeningCycle, int32(200))
	fmt.Println("wa_pow", wa_pow)

	sum_wi_pow := decimal.Zero
	for _, pc := range powerChunks {
		wi_pow := Scaled64PowerChunk(pc, powerRipeningCycle*3/2, powerRipeningCycle, int32(200))
		sum_wi_pow = sum_wi_pow.Add(wi_pow)
	}
	fmt.Println("sum_wi_pow", sum_wi_pow)
	require.Equal(t, wa_pow.String(), sum_wi_pow.String())

	sum_pow_rate := decimal.Zero
	for _, pc := range powerChunks {
		wi_pow := Scaled64PowerChunk(pc, powerRipeningCycle*3/2, powerRipeningCycle, int32(200))
		sum_pow_rate = sum_pow_rate.Add(wi_pow.Div(wa_pow))
	}
	fmt.Println("sum_pow_rate", sum_pow_rate, "diff", decimal.NewFromInt(1).Sub(sum_pow_rate).Abs())

}

func Test_Fixed_Sum(t *testing.T) {
	preparePowerChunks()
	fmt.Println("powerChunks size", len(powerChunks))

	wa_pow := FixedWeightedPowerChunks(powerChunks, powerRipeningCycle*3/2, powerRipeningCycle, int32(200))
	fmt.Println("wa_pow", wa_pow)

	sum_wi_pow := fixed.ZERO
	for _, pc := range powerChunks {
		wi_pow := FixedWeightedPowerChunk(pc, powerRipeningCycle*3/2, powerRipeningCycle, int32(200))
		sum_wi_pow = sum_wi_pow.Add(wi_pow)
	}
	fmt.Println("sum_wi_pow", sum_wi_pow)
	require.Equal(t, wa_pow.String(), sum_wi_pow.String())

	sum_pow_rate := fixed.ZERO
	for _, pc := range powerChunks {
		wi_pow := FixedWeightedPowerChunk(pc, powerRipeningCycle*3/2, powerRipeningCycle, int32(200))
		sum_pow_rate = sum_pow_rate.Add(wi_pow.Div(wa_pow))
	}
	fmt.Println("sum_pow_rate", sum_pow_rate, "diff", fixed.NewI(1, 0).Sub(sum_pow_rate).Abs())
}

func Benchmark_Scaled64PowerChunkFixed(b *testing.B) {
	for i := 0; i < b.N; i++ {
		FixedWeightedPowerChunks(powerChunks, powerRipeningCycle*3/2, powerRipeningCycle, int32(200))
	}
}

func Benchmark_Scaled64PowerChunk(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Scaled64PowerChunks(powerChunks, powerRipeningCycle*3/2, powerRipeningCycle, int32(200))
	}
}
