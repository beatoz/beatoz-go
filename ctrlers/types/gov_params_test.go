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
