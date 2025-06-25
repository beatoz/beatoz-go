package types

import (
	"github.com/beatoz/beatoz-go/libs/jsonx"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"reflect"
	"testing"
)

func Test_ProtoCodec(t *testing.T) {
	params0 := DefaultGovParams()
	bz, err := params0.Encode()
	require.NoError(t, err)

	params1 := &GovParams{}

	err = params1.Decode(nil, bz)
	require.NoError(t, err)

	require.True(t, proto.Equal(&params0._v, &params1._v))

}

func Test_JsonCodec(t *testing.T) {
	govParams := DefaultGovParams()
	jz, err := jsonx.MarshalIndent(govParams, "", "  ")
	require.NoError(t, err)

	govParams2 := &GovParams{}
	err = jsonx.Unmarshal(jz, govParams2)
	require.NoError(t, err)

	require.True(t, reflect.DeepEqual(govParams, govParams2))

	jz2, err := jsonx.MarshalIndent(govParams2, "", "  ")
	require.NoError(t, err)
	require.Equal(t, jz, jz2)
}
