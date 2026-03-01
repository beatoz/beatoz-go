package types_test

import (
	"math/rand/v2"
	"testing"
	"time"

	govmock "github.com/beatoz/beatoz-go/ctrlers/mocks/gov"
	"github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/stretchr/testify/require"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

func TestUseBlockGas(t *testing.T) {
	gasLimit := int64(10000)
	blockCtx := types.NewBlockContext(abcitypes.RequestBeginBlock{}, govmock.NewGovHandlerMock(types.DefaultGovParams()), nil, nil, nil, nil)
	blockCtx.SetBlockGasLimit(gasLimit)

	sumGasUsed := int64(0)
	for {
		gas := rand.Int64N(gasLimit)
		xerr := blockCtx.UseBlockGas(gas)

		if sumGasUsed+gas > gasLimit {
			require.ErrorContains(t, xerr, xerrors.ErrInvalidGas.Error())
			break
		} else {
			require.NoError(t, xerr)
		}
		sumGasUsed += gas

	}
}

// TestExpectNextBlockContext_DoubleScalingBug demonstrates a bug in ExpectNextBlockContext()
// where the interval is double-scaled when it is passed in as a Duration.
// This was reported in the security audit BEA-07.
func TestExpectNextBlockContext_DoubleScalingBug(t *testing.T) {
	gparams := types.DefaultGovParams()
	bctx := types.NewBlockContext(abcitypes.RequestBeginBlock{}, govmock.NewGovHandlerMock(gparams), nil, nil, nil, nil)
	start := time.Unix(1_000_000_000, 0) // seconds precision
	bctx.SetBlockInfo(abcitypes.RequestBeginBlock{Header: bctx.BlockInfo().Header})
	bctx.SetByzantine(nil)
	bctx.SetChainID("test-chain")
	bctx.SetHeight(123)
	// overwrite time in header
	bi := bctx.BlockInfo()
	bi.Header.Time = start
	bctx.SetBlockInfo(bi)

	// interval is 7 seconds
	interval := time.Duration(int64(7)) * time.Second

	// caller passes interval already scaled to Duration in seconds
	next := types.ExpectNextBlockContext(bctx, interval)

	// due to bug, it will scale interval*time.Second again and add it to the next block time
	gotDelta := next.BlockInfo().Header.Time.Sub(start)
	require.Equal(t, interval, gotDelta, "double-scaled duration expected here")

	// Demonstrate the correct interval that should be have been added  vs the got timestamp
	expectedNext := start.Add(interval)
	gotNext := next.BlockInfo().Header.Time
	require.Equal(t, expectedNext, gotNext, "expected next block time (single-scaled): %v, got (double-scaled): %v", expectedNext, gotNext)

	// log for clarity in test output
	//t.Logf("Start=%v Expected=%v Actual=%v ExpectedDelta=%v ActualDelta=%v", start, expectedNext, gotNext, interval, gotDelta)
}
