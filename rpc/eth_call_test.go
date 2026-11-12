package rpc

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/stretchr/testify/require"
)

func TestEthServerCallData(t *testing.T) {
	_, fixture := ethTestRPCHandler(t)
	handler, err := newEthRPCHandler(nil)
	require.NoError(t, err)
	fixture.callResult = ctrlertypes.VMCallResult{UsedGas: 21000, ReturnData: []byte{0xde, 0xad, 0xbe, 0xef}}
	from := "0x" + strings.Repeat("ab", 20)
	to := "0x" + strings.Repeat("cd", 20)
	for _, method := range []string{"eth_call", "eth_estimateGas"} {
		t.Run(method, func(t *testing.T) {
			for _, tt := range []struct {
				args string
				want []byte
			}{
				{`{}`, make([]byte, 80)},
				{`{"data":"0x"}`, make([]byte, 80)},
				{`{"data":"","input":"0X"}`, make([]byte, 80)},
				{`{"data":"0xAB","input":"0Xab"}`, append(make([]byte, 80), 0xab)},
				{`{"value":"","gas":"","gasPrice":"","maxFeePerGas":"","maxPriorityFeePerGas":""}`, make([]byte, 80)},
				{`{"value":"0X1","gas":"0X2","gasPrice":"0X3","maxFeePerGas":"0X4","maxPriorityFeePerGas":"0X5"}`, append(append(make([]byte, 71), 1), 0, 0, 0, 0, 0, 0, 0, 2)},
				{`{"from":null,"to":null,"gas":null,"value":null,"data":null,"input":null,"unknown":{"anything":true}}`, make([]byte, 80)},
				{fmt.Sprintf(`{"from":%q,"to":%q}`, "0X"+strings.Repeat("aB", 20), "0X"+strings.Repeat("Cd", 20)), append(append(bytes.Repeat([]byte{0xab}, 20), bytes.Repeat([]byte{0xcd}, 20)...), make([]byte, 40)...)},
				{`{"gas":"0x0"}`, make([]byte, 80)},
				{`{"blockGasLimit":"0xffffffffffffffff"}`, make([]byte, 80)},
				{`{"gas":"0x102030405060708","input":"0xaabb"}`, append(make([]byte, 72), 1, 2, 3, 4, 5, 6, 7, 8, 0xaa, 0xbb)},
				{`{"gas":"0xffffffffffffffff"}`, append(make([]byte, 72), bytes.Repeat([]byte{0xff}, 8)...)},
				{`{"input":"0x1234"}`, append(make([]byte, 80), 0x12, 0x34)},
				{`{"data":"0x1234"}`, append(make([]byte, 80), 0x12, 0x34)},
				{`{"data":"0x1234","input":"0x1234"}`, append(make([]byte, 80), 0x12, 0x34)},
				{
					fmt.Sprintf(`{"from":%q,"to":%q,"value":"0x102","input":"0xaAbB","gas":"0x0","gasPrice":"0x1","maxFeePerGas":"0xff","maxPriorityFeePerGas":"0x2"}`, from, to),
					append(append(append(bytes.Repeat([]byte{0xab}, 20), bytes.Repeat([]byte{0xcd}, 20)...), append(big.NewInt(258).FillBytes(make([]byte, 32)), make([]byte, 8)...)...), 0xaa, 0xbb),
				},
				{`{"value":"0x` + strings.Repeat("f", 64) + `"}`, append(append(make([]byte, 40), bytes.Repeat([]byte{0xff}, 32)...), make([]byte, 8)...)},
			} {
				fixture.queries = nil
				response := ethServerRequest(t, handler, method, `[`+tt.args+`]`)
				require.Nil(t, response.Error, tt.args)
				want := `"0xdeadbeef"`
				if method == "eth_estimateGas" {
					want = `"0x5208"`
				}
				require.JSONEq(t, want, string(response.Result))
				require.Len(t, fixture.queries, 3)
				require.Equal(t, "block_height", fixture.queries[0].Path)
				require.Equal(t, "gov_params", fixture.queries[1].Path)
				require.Equal(t, fixture.appHeight, fixture.queries[1].Height)
				query := fixture.queries[2]
				require.Equal(t, method, query.Path)
				require.Equal(t, fixture.appHeight, query.Height)
				wantData := make([]byte, 116)
				copy(wantData, tt.want[:80])
				header := fixture.blocks[fixture.appHeight].Block.Header
				binary.BigEndian.PutUint64(wantData[80:88], uint64(header.Time.Unix()))
				copy(wantData[88:108], header.ProposerAddress)
				binary.BigEndian.PutUint64(wantData[108:116], 400)
				require.Equal(t, append(wantData, tt.want[80:]...), query.Data)
			}
		})
	}
	fixture.callResult.ReturnData = nil
	response := ethServerRequest(t, handler, "eth_call", `[{}]`)
	require.Nil(t, response.Error)
	require.JSONEq(t, `"0x"`, string(response.Result))
}

func TestEthServerCallBlockContext(t *testing.T) {
	handler, fixture := ethTestRPCHandler(t)
	for height, block := range fixture.blocks {
		block.Block.Time = time.Unix(1_700_000_000+height, 123456789).UTC()
		block.Block.ProposerAddress = bytes.Repeat([]byte{byte(height)}, 20)
	}
	for _, method := range []string{"eth_call", "eth_estimateGas"} {
		for _, tt := range []struct {
			selector string
			height   int64
		}{
			{"", 11}, {`,null`, 11}, {`,"latest"`, 11}, {`,"pending"`, 11}, {`,"safe"`, 11}, {`,"finalized"`, 11},
			{`,"0xb"`, 11}, {`,"earliest"`, 10}, {`,{"blockNumber":"earliest"}`, 10},
			{fmt.Sprintf(`,{"blockHash":%q}`, ethData(fixture.blocks[10].BlockID.Hash)), 10},
		} {
			// The block store is ahead of the committed app state.
			fixture.appHeight = 11
			block := ethServerRequest(t, handler, "eth_getBlockByNumber", fmt.Sprintf(`[%q]`, ethQuantityUint64(uint64(tt.height))))
			require.Nil(t, block.Error)
			var header ethBlock
			require.NoError(t, json.Unmarshal(block.Result, &header))
			fixture.queries = nil
			fixture.blockReads = nil
			response := ethServerRequest(t, handler, method, `[{"from":"0x`+strings.Repeat("ff", 20)+`","input":"0xaabb"}`+tt.selector+`]`)
			require.Nil(t, response.Error, tt.selector)
			require.Empty(t, fixture.blockReads, tt.selector)
			require.Len(t, fixture.queries, 3)
			require.Equal(t, "block_height", fixture.queries[0].Path)
			require.Equal(t, "gov_params", fixture.queries[1].Path)
			require.Equal(t, tt.height, fixture.queries[1].Height)
			require.False(t, fixture.queries[1].Prove)
			query := fixture.queries[2]
			require.Equal(t, method, query.Path)
			require.Equal(t, tt.height, query.Height)
			require.False(t, query.Prove)
			require.Len(t, query.Data, 118)
			require.Equal(t, header.Timestamp, ethQuantityUint64(binary.BigEndian.Uint64(query.Data[80:88])))
			require.Equal(t, header.Miner, ethData(query.Data[88:108]))
			require.Equal(t, header.GasLimit, ethQuantityUint64(binary.BigEndian.Uint64(query.Data[108:116])))
			require.Equal(t, []byte{0xaa, 0xbb}, query.Data[116:])
		}
	}
	// Match the block formatter's padding and truncation as well as normal addresses.
	for _, proposer := range [][]byte{nil, {0xab, 0xcd}, bytes.Repeat([]byte{0xef}, 22)} {
		fixture.blocks[11].Block.ProposerAddress = proposer
		response := ethServerRequest(t, handler, "eth_call", `[{},"latest"]`)
		require.Nil(t, response.Error)
		query := fixture.queries[len(fixture.queries)-1]
		require.Equal(t, ethFixedData(proposer, 20), ethData(query.Data[88:108]))
	}
}

func TestEthServerCallGovernanceError(t *testing.T) {
	handler, fixture := ethTestRPCHandler(t)
	fixture.queryCode = 9
	for _, method := range []string{"eth_call", "eth_estimateGas"} {
		for _, tt := range []struct {
			tag    string
			height int64
		}{
			{"latest", 12}, {"0xa", 10}, {"0xb", 11},
		} {
			t.Run(method+"/"+tt.tag, func(t *testing.T) {
				fixture.queries = nil
				response := ethServerRequest(t, handler, method, fmt.Sprintf(`[{},%q]`, tt.tag))
				require.NotNil(t, response.Error)
				require.Equal(t, -32000, response.Error.Code)
				require.Equal(t, fmt.Sprintf("gov_params at height %d: fixture query (code 9)", tt.height), response.Error.Message)
				require.Empty(t, response.Result)
				require.Len(t, fixture.queries, 2)
				require.Equal(t, "block_height", fixture.queries[0].Path)
				require.Equal(t, "gov_params", fixture.queries[1].Path)
				require.Equal(t, tt.height, fixture.queries[1].Height)
			})
		}
	}
}

func TestEthServerCallHeaderNotFound(t *testing.T) {
	for _, method := range []string{"eth_call", "eth_estimateGas"} {
		for _, tt := range []struct {
			name, selector string
			appHeight      int64
			missingHeight  int64
		}{
			{"missing_number", `,"0xb"`, 12, 11},
			{"missing_latest", `,"latest"`, 12, 12},
			{"below_store_base", `,"0x9"`, 12, 0},
			{"above_store_height", `,"latest"`, 13, 0},
			{"no_committed_block", `,"latest"`, 0, 0},
		} {
			t.Run(method+"/"+tt.name, func(t *testing.T) {
				handler, fixture := ethTestRPCHandler(t)
				fixture.appHeight = tt.appHeight
				delete(fixture.blocks, tt.missingHeight)
				response := ethServerRequest(t, handler, method, `[{} `+tt.selector+`]`)
				require.NotNil(t, response.Error)
				require.Equal(t, -32000, response.Error.Code)
				require.Equal(t, "header not found", response.Error.Message)
				require.Empty(t, fixture.blockReads)
				require.Len(t, fixture.queries, 1)
				require.Equal(t, "block_height", fixture.queries[0].Path)
			})
		}
	}
}

func TestEthServerCallInvalidParams(t *testing.T) {
	_, fixture := ethTestRPCHandler(t)
	handler, err := newEthRPCHandler(nil)
	require.NoError(t, err)
	invalid := []string{
		`{"from":"0x1"}`, `{"to":"bad"}`, `{"data":"0x1"}`, `{"input":"0xgg"}`,
		`{"gas":"0x01","gas":"0x1"}`, `{"from":"0x1","from":null}`, `{"data":"0x1","data":"0x12"}`,
		`{"data":"0x12","input":"0x34"}`, `{"data":"aa"}`, `{"data":"0x+1"}`, `{"data":"0x 1"}`, `{"data":1}`, `{"value":1}`, `{"gas":21000}`, `{"gas":"0x10000000000000000"}`,
	}
	for _, field := range []string{"value", "gas", "gasPrice", "maxFeePerGas", "maxPriorityFeePerGas"} {
		for _, quantity := range []string{"1", "0x", "0x00", "0x01", "-1", "0x+1", "0x-1", "0x1_0", "0xgg", "0x 1", "0x1" + strings.Repeat("0", 64)} {
			invalid = append(invalid, fmt.Sprintf(`{%q:%q}`, field, quantity))
		}
	}
	for _, method := range []string{"eth_call", "eth_estimateGas"} {
		for _, args := range invalid {
			fixture.queries = nil
			response := ethServerRequest(t, handler, method, `[`+args+`]`)
			require.NotNil(t, response.Error, args)
			require.Equal(t, -32602, response.Error.Code, args)
			require.Empty(t, fixture.queries)
			if args == `{"data":"0x12","input":"0x34"}` {
				require.Equal(t, `both "data" and "input" are set and not equal`, response.Error.Message)
			}
		}
	}
}

func TestEthServerCallExecutionErrors(t *testing.T) {
	_, fixture := ethTestRPCHandler(t)
	handler, err := newEthRPCHandler(nil)
	require.NoError(t, err)
	stringType, err := abi.NewType("string", "", nil)
	require.NoError(t, err)
	reasonData, err := (abi.Arguments{{Type: stringType}}).Pack("not allowed")
	require.NoError(t, err)
	revertData := append([]byte{0x08, 0xc3, 0x79, 0xa0}, reasonData...)
	for _, method := range []string{"eth_call", "eth_estimateGas"} {
		for _, tt := range []struct {
			vmErr, message string
			data           []byte
			code           int
		}{
			{"execution reverted", "execution reverted: not allowed", revertData, 3},
			{"execution reverted", "execution reverted", nil, 3},
			{"execution reverted", "execution reverted", []byte{0xde, 0xad}, 3},
			{"out of gas", "out of gas", nil, -32000},
			{"gas required exceeds allowance (21064)", "gas required exceeds allowance (21064)", nil, -32000},
			{"invalid opcode: INVALID", "invalid opcode: INVALID", nil, -32000},
			{"insufficient balance for transfer", "insufficient balance for transfer", nil, -32000},
		} {
			fixture.callResult = ctrlertypes.VMCallResult{UsedGas: 21000, Err: tt.vmErr, ReturnData: tt.data}
			response := ethServerRequest(t, handler, method, `[{}]`)
			require.NotNil(t, response.Error)
			require.Equal(t, tt.code, response.Error.Code)
			require.Equal(t, tt.message, response.Error.Message)
			require.Empty(t, response.Result)
			if tt.code == 3 {
				require.JSONEq(t, fmt.Sprintf("%q", ethData(tt.data)), string(response.Error.Data))
			} else {
				require.Empty(t, response.Error.Data)
			}
		}
		fixture.callValue = []byte(`{"usedGas":`)
		response := ethServerRequest(t, handler, method, `[{}]`)
		require.NotNil(t, response.Error)
		require.Equal(t, -32000, response.Error.Code)
		fixture.callValue = nil
	}
	fixture.callResult = ctrlertypes.VMCallResult{UsedGas: -1}
	response := ethServerRequest(t, handler, "eth_estimateGas", `[{}]`)
	require.NotNil(t, response.Error)
	require.Equal(t, -32000, response.Error.Code)
}
