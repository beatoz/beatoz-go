package types_test

import (
	"github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAdjustMaxGasPerTrx(t *testing.T) {
	_min := types.DefaultGovParams().MinTrxGas()
	_max := types.DefaultGovParams().MaxTrxGas()

	blockCtx := &types.BlockContext{}
	for n := 0; n < 100000; n++ {
		blockCtx.AdjustMaxGasPerTrx(_min, _max)
		adjusted := blockCtx.MaxGasPerTrx()
		require.True(t, _min <= adjusted)
		require.True(t, _max >= adjusted)
		blockCtx.AddTxsCnt(1)
	}
}
