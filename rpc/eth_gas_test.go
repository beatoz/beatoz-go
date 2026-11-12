package rpc

import (
	"encoding/json"
	"fmt"
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
		{"earliest height", 4, "0xa", 10, 20, 10, 1},
		{"empty store", 4, "latest", 0, 0, 0, 0},
		{"zero count", 0, "latest", 1, 20, 0, 0},
		{"cap", math.MaxUint64, "latest", 1, 2000, 977, 1024},
		{"max height", 4, "latest", 1, math.MaxInt64, math.MaxInt64 - 3, 4},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var number ethBlockNumberParam
			require.NoError(t, json.Unmarshal([]byte(`"`+tt.tag+`"`), &number))
			oldest, count := ethFeeHistoryRange(tt.count, number, coretypes.SyncInfo{EarliestBlockHeight: tt.earliest, LatestBlockHeight: tt.latest})
			require.Equal(t, tt.oldest, oldest)
			require.Equal(t, tt.wantCount, count)
		})
	}
}

func TestFormatEthFeeHistory(t *testing.T) {
	blocks := []ethFeeHistoryBlock{{gasUsed: 0, gasLimit: 100}, {gasUsed: 50, gasLimit: 100}, {gasUsed: 150, gasLimit: 200}}
	for _, percentiles := range [][]float64{nil, {}, {25, 75}, {0, 50.5, 100}} {
		result, err := formatEthFeeHistory(16, blocks, percentiles)
		require.NoError(t, err)
		require.Equal(t, "0x10", result.OldestBlock)
		require.Equal(t, []string{"0x0", "0x0", "0x0", "0x0"}, result.BaseFeePerGas)
		require.Equal(t, []float64{0, 0.5, 0.75}, result.GasUsedRatio)
		encoded, err := json.Marshal(result)
		require.NoError(t, err)
		if len(percentiles) == 0 {
			require.Nil(t, result.Reward)
			require.NotContains(t, string(encoded), `"reward"`)
		} else {
			require.NotNil(t, result.Reward)
			require.Len(t, result.Reward, 3)
			for _, row := range result.Reward {
				require.Len(t, row, len(percentiles))
				for _, reward := range row {
					require.Equal(t, "0x0", reward)
				}
			}
			require.Contains(t, string(encoded), `"reward"`)
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
	require.NotContains(t, string(encoded), `"reward"`)
	_, err = formatEthFeeHistory(1, []ethFeeHistoryBlock{{gasLimit: 0}}, nil)
	require.Error(t, err)
}

func TestEthFeeHistoryInvalidPercentiles(t *testing.T) {
	var newest ethBlockNumberParam
	require.NoError(t, json.Unmarshal([]byte(`"latest"`), &newest))
	// No backend is installed; invalid input must be rejected before any lookup.
	for _, percentiles := range [][]float64{{-1}, {101}, {50, 25}, {math.NaN()}, {math.Inf(1)}} {
		result, err := ethFeeHistory(nil, 1, newest, percentiles)
		require.Nil(t, result)
		require.Error(t, err)
		rpcErr, ok := err.(*ethRPCError)
		require.True(t, ok)
		require.Equal(t, -32602, rpcErr.ErrorCode())
	}
}

func TestEthServerFeeHistoryParameterTypes(t *testing.T) {
	handler, fixture := ethTestRPCHandler(t)
	for _, count := range []string{`"0x4"`, `4`, ` 4 `, `"4"`, `"0004"`, `"0x04"`, `"0X4"`, `"0x400"`, `1025`, `"0xffffffffffffffff"`, `18446744073709551615`} {
		response := ethServerRequest(t, handler, "eth_feeHistory", `[`+count+`,"latest"]`)
		require.Nil(t, response.Error, count)
		require.JSONEq(t, `{"oldestBlock":"0xa","baseFeePerGas":["0x0","0x0","0x0","0x0"],"gasUsedRatio":[0.25,0.5,0.75]}`, string(response.Result))
	}
	for _, count := range []string{`""`, `"0x0"`, `0`, `"0"`, `"00"`} {
		response := ethServerRequest(t, handler, "eth_feeHistory", `[`+count+`,"latest"]`)
		require.Nil(t, response.Error, count)
		require.JSONEq(t, `{"oldestBlock":"0x0","baseFeePerGas":["0x0"],"gasUsedRatio":[]}`, string(response.Result))
	}
	for _, count := range []string{`null`, `true`, `{}`, `[]`, `-1`, `4.0`, `1e3`, `"0x"`, `"0x-1"`, `"0x+1"`, `"0xg"`, `"0x1_0"`, `"0x10000000000000000"`, `18446744073709551616`} {
		response := ethServerRequest(t, handler, "eth_feeHistory", `[`+count+`,"latest"]`)
		require.NotNil(t, response.Error, count)
		require.Equal(t, -32602, response.Error.Code)
	}
	for _, params := range []string{`[]`, `[1]`, `[1,null]`, `[1,"Earliest"]`, `[1,"EARLIEST"]`} {
		response := ethServerRequest(t, handler, "eth_feeHistory", params)
		require.NotNil(t, response.Error, params)
		require.Equal(t, -32602, response.Error.Code)
	}
	for _, newest := range []string{`"earliest"`, ` "earliest" `} {
		response := ethServerRequest(t, handler, "eth_feeHistory", `[4,`+newest+`]`)
		require.Nil(t, response.Error)
		require.JSONEq(t, `{"oldestBlock":"0xa","baseFeePerGas":["0x0","0x0"],"gasUsedRatio":[0.25]}`, string(response.Result))
	}
	for _, newest := range []string{`"0x0"`, `"0X0"`} {
		for _, suffix := range []string{"", ",null", ",[]", ",[50]"} {
			fixture.queries, fixture.resultReads = nil, nil
			response := ethServerRequest(t, handler, "eth_feeHistory", `[4,`+newest+suffix+`]`)
			require.NotNil(t, response.Error)
			require.Equal(t, -32000, response.Error.Code)
			require.Equal(t, "header not found", response.Error.Message)
			require.Empty(t, response.Result)
			require.Empty(t, fixture.resultReads)
			require.Empty(t, fixture.queries)
		}
	}
	response := ethServerRequest(t, handler, "eth_feeHistory", `[1,"latest",[null,50,100]]`)
	require.Nil(t, response.Error)
	require.JSONEq(t, `{"oldestBlock":"0xc","baseFeePerGas":["0x0","0x0"],"gasUsedRatio":[0.75],"reward":[["0x0","0x0","0x0"]]}`, string(response.Result))
	response = ethServerRequest(t, handler, "eth_feeHistory", `[1,"latest",[50,null]]`)
	require.NotNil(t, response.Error)
	require.Equal(t, -32602, response.Error.Code)
}

func TestEthServerFeeHistoryMissingBlocks(t *testing.T) {
	handler, fixture := ethTestRPCHandler(t)
	for _, newest := range []int64{0, fixture.baseHeight - 1, 13, math.MaxInt64} {
		for _, count := range []int{0, 2, 4, 1025} {
			fixture.queries, fixture.resultReads = nil, nil
			response := ethServerRequest(t, handler, "eth_feeHistory", fmt.Sprintf(`[%d,%q]`, count, ethQuantityUint64(uint64(newest))))
			require.NotNil(t, response.Error)
			require.Equal(t, -32000, response.Error.Code)
			if newest < fixture.baseHeight {
				require.Equal(t, "header not found", response.Error.Message)
			} else {
				require.Equal(t, fmt.Sprintf("request beyond head block: requested %d, head 12", newest), response.Error.Message)
			}
			if newest == 0 {
				require.Empty(t, fixture.queries)
			} else {
				require.Len(t, fixture.queries, 1)
				require.Equal(t, "block_height", fixture.queries[0].Path)
			}
			require.Empty(t, response.Result)
			require.Empty(t, fixture.resultReads)
		}
	}
}

func TestEthServerFeeHistoryExistingHeights(t *testing.T) {
	handler, _ := ethTestRPCHandler(t)
	for _, tt := range []struct{ newest, want string }{
		{"0xa", `{"oldestBlock":"0xa","baseFeePerGas":["0x0","0x0"],"gasUsedRatio":[0.25]}`},
		{"0xb", `{"oldestBlock":"0xa","baseFeePerGas":["0x0","0x0","0x0"],"gasUsedRatio":[0.25,0.5]}`},
		{"0xc", `{"oldestBlock":"0xa","baseFeePerGas":["0x0","0x0","0x0","0x0"],"gasUsedRatio":[0.25,0.5,0.75]}`},
	} {
		response := ethServerRequest(t, handler, "eth_feeHistory", fmt.Sprintf(`[4,%q]`, tt.newest))
		require.Nil(t, response.Error)
		require.JSONEq(t, tt.want, string(response.Result))
	}
}
