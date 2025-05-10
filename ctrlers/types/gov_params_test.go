package types

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	tmjson "github.com/tendermint/tendermint/libs/json"
	"google.golang.org/protobuf/proto"
	"testing"
)

func TestProtoCodec(t *testing.T) {
	params0 := DefaultGovParams()
	bz, err := params0.Encode()
	require.NoError(t, err)

	params1 := emptyGovParams()

	err = params1.Decode(nil, bz)
	require.NoError(t, err)

	require.True(t, proto.Equal(&params0._v, &params1._v))

}

func TestJsonCodec(t *testing.T) {
	params0 := DefaultGovParams()
	bz, err := json.Marshal(params0)
	require.NoError(t, err)
	fmt.Println("json", string(bz))

	bz1, err := tmjson.Marshal(params0)
	require.NoError(t, err)
	fmt.Println("tmjson", string(bz1))

	params1 := emptyGovParams()
	err = json.Unmarshal(bz, params1)
	require.NoError(t, err)

	require.True(t, proto.Equal(&params0._v, &params1._v))
}
