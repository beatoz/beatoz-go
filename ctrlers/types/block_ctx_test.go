package types_test

import (
	govmock "github.com/beatoz/beatoz-go/ctrlers/mocks/gov"
	"github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/libs"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/stretchr/testify/require"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"math/rand/v2"
	"testing"
)

func TestBlockGasLimit(t *testing.T) {
	_max := types.DefaultGovParams().MaxBlockGasLimit()
	_min := types.DefaultGovParams().MinBlockGasLimit()

	for n := 0; n < 1000; n++ {

		blockCtx := types.NewBlockContext(abcitypes.RequestBeginBlock{}, govmock.NewGovHandlerMock(types.DefaultGovParams()), nil, nil, nil, nil)
		randBlockGasLimit := rand.Int64N(_max)
		blockCtx.SetBlockGasLimit(randBlockGasLimit)

		randGasUsed := rand.Int64N(randBlockGasLimit)
		threshold0 := randBlockGasLimit - randBlockGasLimit/10
		threshold1 := randBlockGasLimit / 100

		adjusted := types.AdjustBlockGasLimit(randBlockGasLimit, randGasUsed, _min, _max)

		expected := int64(0)
		if randGasUsed > threshold0 {
			// expect gas limit increasing.
			expected = randBlockGasLimit + randBlockGasLimit/10
			if expected > _max {
				expected = _max
			}

		} else if randGasUsed == 0 || randGasUsed <= threshold0 && randGasUsed >= threshold1 {
			// expect gas limit equal
			expected = randBlockGasLimit
		} else if randGasUsed < threshold1 {
			// expect gas limit decreasing.
			expected = randBlockGasLimit - randBlockGasLimit/100
			if expected < _min {
				expected = _min
			}
		}
		expected = libs.MaxInt64(expected, _min)

		require.Equal(t, expected, adjusted, "gasLimitInfo", "n", n, "blockGasLimit", randBlockGasLimit, "gasUsed", randGasUsed, "t0", threshold0, "t1", threshold1, "adjusted", adjusted)
	}
}

func TestUseBlockGas(t *testing.T) {
	initGasLimit := int64(10000)
	blockCtx := types.NewBlockContext(abcitypes.RequestBeginBlock{}, govmock.NewGovHandlerMock(types.DefaultGovParams()), nil, nil, nil, nil)
	blockCtx.SetBlockGasLimit(initGasLimit)

	sumGasUsed := int64(0)
	for {
		gas := rand.Int64N(initGasLimit)
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
