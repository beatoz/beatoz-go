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

func TestGovParamsValidateBasic(t *testing.T) {
	require.NoError(t, DefaultGovParams().ValidateBasic())
}

func TestGovParamsValidateBasic_InvalidValues(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*GovParamsProto)
	}{
		{
			name: "negative_max_validator_count",
			mutate: func(v *GovParamsProto) {
				v.MaxValidatorCnt = -1
			},
		},
		{
			name: "zero_inflation_cycle_blocks",
			mutate: func(v *GovParamsProto) {
				v.InflationCycleBlocks = 0
			},
		},
		{
			name: "negative_inflation_cycle_blocks",
			mutate: func(v *GovParamsProto) {
				v.InflationCycleBlocks = -1
			},
		},
		{
			name: "negative_block_gas_limit",
			mutate: func(v *GovParamsProto) {
				v.BlockGasLimit = -1
			},
		},
		{
			name: "tx_fee_reward_rate_over_100",
			mutate: func(v *GovParamsProto) {
				v.TxFeeRewardRate = 200
			},
		},
		{
			name: "slash_rate_over_100",
			mutate: func(v *GovParamsProto) {
				v.SlashRate = 200
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := DefaultGovParams()
			params.SetValue(tt.mutate)
			require.Error(t, params.ValidateBasic())
		})
	}
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

func TestGovParamsUnmarshalError(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		errField string
	}{
		{
			name:     "gas price type",
			input:    `{"gasPrice": 1}`,
			errField: "gasPrice",
		},
		{
			name:     "gas price value",
			input:    `{"gasPrice": "not-a-number"}`,
			errField: "gasPrice",
		},
		{
			name:     "total supply type",
			input:    `{"maxTotalSupply": 1}`,
			errField: "maxTotalSupply",
		},
		{
			name:     "total supply value",
			input:    `{"maxTotalSupply": "not-a-number"}`,
			errField: "maxTotalSupply",
		},
		{
			name:     "dead address type",
			input:    `{"deadAddress": 1}`,
			errField: "deadAddress",
		},
		{
			name:     "dead address value",
			input:    `{"deadAddress": "not-hex"}`,
			errField: "deadAddress",
		},
		{
			name:     "reward address type",
			input:    `{"rewardPoolAddress": 1}`,
			errField: "rewardPoolAddress",
		},
		{
			name:     "reward address value",
			input:    `{"rewardPoolAddress": "not-hex"}`,
			errField: "rewardPoolAddress",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := &GovParams{}
			err := jsonx.Unmarshal([]byte(tt.input), params)
			require.Error(t, err)
			require.ErrorContains(t, err, tt.errField)
		})
	}
}
