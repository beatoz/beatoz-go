package rpc

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
)

func TestParseEthGasParams(t *testing.T) {
	params, err := parseEthGasParams([]byte(`{"blockGasLimit":"120000000","gasPrice":"48000000000","minTrxGas":"5000"}`))
	require.NoError(t, err)
	require.Equal(t, uint64(120000000), params.blockGasLimit)
	require.Equal(t, "48000000000", params.gasPrice.String())
	for _, value := range []string{
		`null`, `{}`, `{"blockGasLimit":120000000,"gasPrice":"1"}`,
		`{"blockGasLimit":"0","gasPrice":"1"}`, `{"blockGasLimit":"-1","gasPrice":"1"}`,
		`{"blockGasLimit":"18446744073709551616","gasPrice":"1"}`,
		`{"blockGasLimit":"1","gasPrice":"-1"}`, `{"blockGasLimit":"1","gasPrice":"+1"}`,
		`{"blockGasLimit":"1","gasPrice":"0x1"}`, `{"blockGasLimit":"1","gasPrice":null}`,
	} {
		_, err := parseEthGasParams([]byte(value))
		require.Error(t, err, value)
	}
}

func TestParseEthBlockCount(t *testing.T) {
	for _, tt := range []struct {
		input string
		want  uint64
	}{
		{`"0x0"`, 0}, {`0`, 0}, {`"0x4"`, 4}, {`4`, 4}, {` 4 `, 4}, {`"0xA"`, 10},
		{`"0x400"`, 1024}, {`1025`, 1024}, {`"0xffffffffffffffffffff"`, 1024}, {`18446744073709551616`, 1024},
	} {
		got, err := parseEthBlockCount(json.RawMessage(tt.input))
		require.NoError(t, err, tt.input)
		require.Equal(t, tt.want, got, tt.input)
	}
	for _, input := range []string{
		``, `null`, `true`, `{}`, `[]`, `-1`, `4.0`, `1e3`, `"4"`, `"0x"`, `"0X4"`, `"0x04"`, `"0x-1"`, `"0x+1"`, `"0xg"`, `"0x1_0"`, `04`, `+4`,
	} {
		_, err := parseEthBlockCount(json.RawMessage(input))
		require.Error(t, err, input)
	}
}

func TestEthFeeHistoryRange(t *testing.T) {
	for _, tt := range []struct {
		name             string
		count            uint64
		tag              string
		earliest, latest int64
		oldest           int64
		wantCount        uint64
	}{
		{"latest", 4, "latest", 10, 20, 17, 4},
		{"pending", 4, "pending", 10, 20, 17, 4},
		{"safe", 4, "safe", 10, 20, 17, 4},
		{"finalized", 4, "finalized", 10, 20, 17, 4},
		{"explicit", 4, "0x10", 10, 20, 13, 4},
		{"earliest", 4, "earliest", 10, 20, 10, 1},
		{"pruned start", 20, "latest", 10, 20, 10, 11},
		{"future end", 4, "0x100", 10, 20, 17, 4},
		{"pruned end", 4, "0x9", 10, 20, 0, 0},
		{"zero height", 4, "0x0", 1, 20, 0, 0},
		{"empty store", 4, "latest", 0, 0, 0, 0},
		{"zero count", 0, "latest", 1, 20, 0, 0},
		{"cap", math.MaxUint64, "latest", 1, 2000, 977, 1024},
		{"max height", 4, "latest", 1, math.MaxInt64, math.MaxInt64 - 3, 4},
	} {
		t.Run(tt.name, func(t *testing.T) {
			height, earliest, err := parseEthBlockTag(tt.tag)
			require.NoError(t, err)
			oldest, count := ethFeeHistoryRange(tt.count, height, earliest, coretypes.SyncInfo{EarliestBlockHeight: tt.earliest, LatestBlockHeight: tt.latest})
			require.Equal(t, tt.oldest, oldest)
			require.Equal(t, tt.wantCount, count)
		})
	}
}

func TestFormatEthFeeHistory(t *testing.T) {
	blocks := []ethFeeHistoryBlock{{gasUsed: 0, gasLimit: 100}, {gasUsed: 50, gasLimit: 100}, {gasUsed: 150, gasLimit: 200}}
	for _, percentiles := range [][]float64{nil, {}, {0, 50.5, 100}} {
		result, err := formatEthFeeHistory(16, blocks, percentiles)
		require.NoError(t, err)
		require.Equal(t, "0x10", result.OldestBlock)
		require.Equal(t, []string{"0x0", "0x0", "0x0", "0x0"}, result.BaseFeePerGas)
		require.Equal(t, []float64{0, 0.5, 0.75}, result.GasUsedRatio)
		encoded, err := json.Marshal(result)
		require.NoError(t, err)
		if percentiles == nil {
			require.Nil(t, result.Reward)
			require.NotContains(t, string(encoded), `"reward"`)
		} else {
			require.NotNil(t, result.Reward)
			require.Len(t, *result.Reward, 3)
			for _, row := range *result.Reward {
				require.Len(t, row, len(percentiles))
				for _, reward := range row {
					require.Equal(t, "0x0", reward)
				}
			}
			if len(percentiles) == 0 {
				require.Contains(t, string(encoded), `"reward":[[],[],[]]`)
			}
		}
	}
	result, err := formatEthFeeHistory(0, nil, nil)
	require.NoError(t, err)
	encoded, err := json.Marshal(result)
	require.NoError(t, err)
	require.JSONEq(t, `{"oldestBlock":"0x0","baseFeePerGas":["0x0"],"gasUsedRatio":[]}`, string(encoded))
	result, err = formatEthFeeHistory(0, nil, []float64{50})
	require.NoError(t, err)
	encoded, err = json.Marshal(result)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"reward":[]`)
	_, err = formatEthFeeHistory(1, []ethFeeHistoryBlock{{gasLimit: 0}}, nil)
	require.Error(t, err)
	for _, percentiles := range [][]float64{{-1}, {101}, {50, 25}, {math.NaN()}, {math.Inf(1)}} {
		_, err := formatEthFeeHistory(1, blocks, percentiles)
		require.Error(t, err)
	}
}
