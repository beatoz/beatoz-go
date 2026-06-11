package types

import (
	"github.com/beatoz/beatoz-go/libs/jsonx"
	"github.com/holiman/uint256"
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

func TestValidateCurrentSupply(t *testing.T) {
	tests := []struct {
		name          string
		maxSupply     uint64
		currentSupply uint64
		wantErr       bool
	}{
		{
			name:          "max_supply_less_than_current_supply",
			maxSupply:     99,
			currentSupply: 100,
			wantErr:       true,
		},
		{
			name:          "max_supply_equal_to_current_supply",
			maxSupply:     100,
			currentSupply: 100,
		},
		{
			name:          "max_supply_greater_than_current_supply",
			maxSupply:     101,
			currentSupply: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := DefaultGovParams()
			params.SetValue(func(v *GovParamsProto) {
				v.XMaxTotalSupply = uint256.NewInt(tt.maxSupply).Bytes()
			})

			xerr := params.ValidateCurrentSupply(uint256.NewInt(tt.currentSupply))
			if tt.wantErr {
				require.Error(t, xerr)
				require.Contains(t, xerr.Error(), "maxTotalSupply")
				return
			}
			require.NoError(t, xerr)
		})
	}
}
