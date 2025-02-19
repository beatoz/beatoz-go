package types_test

import (
	"github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/stretchr/testify/require"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"math/rand/v2"
	"testing"
)

func TestBlockGasLimit(t *testing.T) {
	_max := types.DefaultGovParams().MaxBlockGas()
	_min := types.DefaultGovParams().MinTrxGas() * 10000

	for n := 0; n < 100000; n++ {

		blockCtx := types.NewBlockContext(abcitypes.RequestBeginBlock{}, types.DefaultGovParams(), nil, nil)
		randBlockGasLimit := rand.Uint64N(_max)
		blockCtx.SetBlockGasLimit(randBlockGasLimit)

		randGasUsed := rand.Uint64N(randBlockGasLimit) + 4000
		threshold0 := randBlockGasLimit - randBlockGasLimit/10
		threshold1 := randBlockGasLimit / 10

		adjusted := types.AdjustBlockGasLimit(blockCtx.GetBlockGasLimit(), randGasUsed, _min, _max)

		expected := uint64(0)
		if randGasUsed > threshold0 {
			// expect gas limit increasing.
			expected = randBlockGasLimit + randBlockGasLimit/10
			if expected > _max {
				expected = _max
			}

		} else if randGasUsed <= threshold0 && randGasUsed >= threshold1 {
			// expect gas limit equal
			expected = randBlockGasLimit
		} else if randGasUsed < threshold1 {
			// expect gas limit decreasing.
			expected = randBlockGasLimit - randBlockGasLimit/10
			if expected < _min {
				expected = _min
			}
		}
		require.Equal(t, expected, adjusted, "gasLimitInfo", "n", n, "blockGasLimit", randBlockGasLimit, "gasUsed", randGasUsed, "t0", threshold0, "t1", threshold1, "adjusted", adjusted)
	}
}

func TestUseBlockGas(t *testing.T) {
	initGasLimit := uint64(10000)
	blockCtx := types.NewBlockContext(abcitypes.RequestBeginBlock{}, types.DefaultGovParams(), nil, nil)
	blockCtx.SetBlockGasLimit(initGasLimit)

	sumGasUsed := uint64(0)
	for {
		gas := rand.Uint64N(initGasLimit)
		xerr := blockCtx.UseBlockGas(gas)

		if sumGasUsed+gas > initGasLimit {
			require.ErrorContains(t, xerr, xerrors.ErrInvalidGas.Error())
			break
		} else {
			require.NoError(t, xerr)
		}
		sumGasUsed += gas

	}
}
