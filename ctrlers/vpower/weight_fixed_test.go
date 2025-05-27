package vpower

import (
	"fmt"
	"github.com/beatoz/beatoz-go/ctrlers/types"
	types2 "github.com/beatoz/beatoz-go/types"
	"github.com/robaho/fixed"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"math/rand"
	"testing"
)

var (
	totalSupply        = types2.ToFons(700_000_000)
	powerChunks        []*PowerChunkProto
	powerRipeningCycle = types.YearSeconds
)

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

func Test_ScaledPowerChunk_Fixed_Decimal(t *testing.T) {
	for i := 0; i < 10; i++ {
		preparePowerChunks()
		w0 := decimalScaledPowerChunks(powerChunks, powerRipeningCycle*3/2, powerRipeningCycle, int32(200))
		w1 := fixedScaledPowerChunks(powerChunks, powerRipeningCycle*3/2, powerRipeningCycle, int32(200))

		diff, err := decimal.NewFromString(w1.String())
		require.NoError(t, err)
		diff = diff.Sub(w0)
		fmt.Println("w0", w0, "w1", w1, "diff", diff.Abs())
	}
}

func Test_ScaledPowerChunk_Decimal(t *testing.T) {
	preparePowerChunks()
	fmt.Println("powerChunks size", len(powerChunks))

	wa_pow := decimalScaledPowerChunks(powerChunks, powerRipeningCycle*3/2, powerRipeningCycle, int32(200))
	fmt.Println("wa_pow", wa_pow)

	sum_wi_pow := decimal.Zero
	for _, pc := range powerChunks {
		wi_pow := decimalScaledPowerChunk(pc, powerRipeningCycle*3/2, powerRipeningCycle, int32(200))
		sum_wi_pow = sum_wi_pow.Add(wi_pow)
	}
	fmt.Println("sum_wi_pow", sum_wi_pow)
	require.Equal(t, wa_pow.String(), sum_wi_pow.String())

	sum_pow_rate := decimal.Zero
	for _, pc := range powerChunks {
		wi_pow := decimalScaledPowerChunk(pc, powerRipeningCycle*3/2, powerRipeningCycle, int32(200))
		sum_pow_rate = sum_pow_rate.Add(wi_pow.Div(wa_pow))
	}
	fmt.Println("sum_pow_rate", sum_pow_rate, "diff", decimal.NewFromInt(1).Sub(sum_pow_rate).Abs())

}

func Test_ScaledPowerChunk_Fixed(t *testing.T) {
	preparePowerChunks()
	fmt.Println("powerChunks size", len(powerChunks))

	wa_pow := fixedScaledPowerChunks(powerChunks, powerRipeningCycle*3/2, powerRipeningCycle, int32(200))
	fmt.Println("wa_pow", wa_pow)

	sum_wi_pow := fixed.ZERO
	for _, pc := range powerChunks {
		wi_pow := fixedScaledPowerChunk(pc, powerRipeningCycle*3/2, powerRipeningCycle, int32(200))
		sum_wi_pow = sum_wi_pow.Add(wi_pow)
	}
	fmt.Println("sum_wi_pow", sum_wi_pow)
	require.Equal(t, wa_pow.String(), sum_wi_pow.String())

	sum_pow_rate := fixed.ZERO
	for _, pc := range powerChunks {
		wi_pow := fixedScaledPowerChunk(pc, powerRipeningCycle*3/2, powerRipeningCycle, int32(200))
		sum_pow_rate = sum_pow_rate.Add(wi_pow.Div(wa_pow))
	}
	fmt.Println("sum_pow_rate", sum_pow_rate, "diff", fixed.NewI(1, 0).Sub(sum_pow_rate))
}
