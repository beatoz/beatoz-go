package types

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestProtoCodec(t *testing.T) {
	params0 := DefaultGovParams()
	bz, err := params0.Encode()
	require.NoError(t, err)

	params1, err := DecodeGovParams(bz)
	require.NoError(t, err)

	require.Equal(t, params0, params1)

}

func TestJsonCodec(t *testing.T) {
	params0 := DefaultGovParams()
	bz, err := json.Marshal(params0)
	require.NoError(t, err)

	params1 := &GovParams{}
	err = json.Unmarshal(bz, params1)
	require.NoError(t, err)

	require.Equal(t, params0, params1)
}

func TestNewGovParams(t *testing.T) {
	govParams := newGovParamsWith(1)
	require.EqualValues(t, 2*7*24*60*60, govParams.lazyUnstakingBlocks)  // 2weeks
	require.EqualValues(t, 1*24*60*60, govParams.lazyApplyingBlocks)     // 1day
	require.EqualValues(t, 1*24*60*60, govParams.minVotingPeriodBlocks)  // 1day
	require.EqualValues(t, 30*24*60*60, govParams.maxVotingPeriodBlocks) // 30day

}
