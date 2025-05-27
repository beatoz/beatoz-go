package vpower

import (
	"testing"
)

func init() {
	preparePowerChunks()
}

func Benchmark_ScaledPowerChunk_Fixed(b *testing.B) {
	for i := 0; i < b.N; i++ {
		fixedScaledPowerChunks(powerChunks, powerRipeningCycle*3/2, powerRipeningCycle, int32(200))
	}
}

func Benchmark_ScaledPowerChunk_FxNum(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fxnumScaledPowerChunks(powerChunks, powerRipeningCycle*3/2, powerRipeningCycle, int32(200))
	}
}

func Benchmark_ScaledPowerChunk_Decimal(b *testing.B) {
	for i := 0; i < b.N; i++ {
		decimalScaledPowerChunks(powerChunks, powerRipeningCycle*3/2, powerRipeningCycle, int32(200))
	}
}

func Benchmark_WeightOfPowerChunks_Fixed(b *testing.B) {
	for i := 0; i < b.N; i++ {
		fixedWeightOfPowerChunks(powerChunks, powerRipeningCycle*3/2, powerRipeningCycle, int32(200), totalSupply)
	}
}

func Benchmark_WeightOfPowerChunks_FxNum(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fxnumWeightOfPowerChunks(powerChunks, powerRipeningCycle*3/2, powerRipeningCycle, int32(200), totalSupply)
	}
}

func Benchmark_WeightOfPowerChunks_Decimal(b *testing.B) {
	for i := 0; i < b.N; i++ {
		decimalWeightOfPowerChunks(powerChunks, powerRipeningCycle*3/2, powerRipeningCycle, int32(200), totalSupply)
	}
}
