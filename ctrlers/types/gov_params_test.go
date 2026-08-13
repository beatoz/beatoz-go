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
	negativeMaxValidatorCount := DefaultGovParams()
	negativeMaxValidatorCount.SetValue(func(v *GovParamsProto) {
		v.MaxValidatorCnt = -1
	})
	zeroInflationCycleBlocks := DefaultGovParams()
	zeroInflationCycleBlocks.SetValue(func(v *GovParamsProto) {
		v.InflationCycleBlocks = 0
	})
	negativeInflationCycleBlocks := DefaultGovParams()
	negativeInflationCycleBlocks.SetValue(func(v *GovParamsProto) {
		v.InflationCycleBlocks = -1
	})
	negativeBlockGasLimit := DefaultGovParams()
	negativeBlockGasLimit.SetValue(func(v *GovParamsProto) {
		v.BlockGasLimit = -1
	})
	invalidTxFeeRewardRate := DefaultGovParams()
	invalidTxFeeRewardRate.SetValue(func(v *GovParamsProto) {
		v.TxFeeRewardRate = 200
	})
	invalidSlashRate := DefaultGovParams()
	invalidSlashRate.SetValue(func(v *GovParamsProto) {
		v.SlashRate = 200
	})

	for _, test := range []struct {
		name   string
		params *GovParams
	}{
		{
			name:   "negative_max_validator_count",
			params: negativeMaxValidatorCount,
		},
		{
			name:   "zero_inflation_cycle_blocks",
			params: zeroInflationCycleBlocks,
		},
		{
			name:   "negative_inflation_cycle_blocks",
			params: negativeInflationCycleBlocks,
		},
		{
			name:   "negative_block_gas_limit",
			params: negativeBlockGasLimit,
		},
		{
			name:   "tx_fee_reward_rate_over_100",
			params: invalidTxFeeRewardRate,
		},
		{
			name:   "slash_rate_over_100",
			params: invalidSlashRate,
		},
	} {
		require.Error(t, test.params.ValidateBasic(), "case=%s", test.name)
	}
}

func TestValidateCurrentSupply(t *testing.T) {
	for _, test := range []struct {
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
	} {
		params := DefaultGovParams()
		params.SetValue(func(v *GovParamsProto) {
			v.XMaxTotalSupply = uint256.NewInt(test.maxSupply).Bytes()
		})

		xerr := params.ValidateCurrentSupply(uint256.NewInt(test.currentSupply))
		if test.wantErr {
			require.Error(t, xerr, "case=%s", test.name)
			require.Contains(t, xerr.Error(), "maxTotalSupply", "case=%s", test.name)
			continue
		}
		require.NoError(t, xerr, "case=%s", test.name)
	}
}

func TestGovParamsUnmarshalError(t *testing.T) {
	for _, test := range []struct {
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
	} {
		params := &GovParams{}
		err := jsonx.Unmarshal([]byte(test.input), params)
		require.Error(t, err, "case=%s", test.name)
		require.ErrorContains(t, err, test.errField, "case=%s", test.name)
	}
}
