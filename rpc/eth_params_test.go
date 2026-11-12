package rpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"testing"

	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/stretchr/testify/require"
)

func TestParseEthStorageSlot(t *testing.T) {
	for _, slot := range []string{"0x", "0X", "0x0", "0x1", "0x001", "0XAbC", "0xAbC", "0x" + strings.Repeat("F", 64), "0x" + strings.Repeat("0", 64)} {
		value, err := parseEthStorageSlot(slot)
		require.NoError(t, err, slot)
		require.Len(t, value, 32)
		digits := strings.TrimPrefix(strings.TrimPrefix(slot, "0x"), "0X")
		want := new(big.Int)
		if digits != "" {
			_, ok := want.SetString(digits, 16)
			require.True(t, ok)
		}
		require.Zero(t, want.Cmp(new(big.Int).SetBytes(value)))
	}
	value, err := parseEthStorageSlot("0x1")
	require.NoError(t, err)
	require.Equal(t, append(bytes.Repeat([]byte{0}, 31), 1), value)
	for _, slot := range []string{"", "0", "1", "001", "10", "zz", "0x+1", "0x-1", "0x1_0", "0xgg", "0x 1", "0x" + strings.Repeat("0", 65), "0x" + strings.Repeat("f", 66)} {
		_, err := parseEthStorageSlot(slot)
		require.Error(t, err, slot)
		rpcErr, ok := err.(*ethRPCError)
		require.True(t, ok)
		require.Equal(t, -32602, rpcErr.ErrorCode())
	}
}

func TestEthServerBlockNumberOrHash(t *testing.T) {
	handler, fixture := ethTestRPCHandler(t)
	address := `"0x` + strings.Repeat("aB", 20) + `"`
	fixture.accounts[string(bytes.Repeat([]byte{0xab}, 20))] = []byte(`{"nonce":"42","balance":"256","code":"ABCD"}`)
	fixture.storageValue = append(make([]byte, 31), 0xab)
	fixture.callResult = ctrlertypes.VMCallResult{UsedGas: 21000, ReturnData: []byte{0xde, 0xad}}
	for _, tt := range []struct{ method, params, path, want string }{
		{"eth_getBalance", address, "account", `"0x100"`},
		{"eth_getTransactionCount", address, "account", `"0x2a"`},
		{"eth_getCode", address, "account", `"0xabcd"`},
		{"eth_getStorageAt", address + `,"0x1"`, "eth_getStorageAt", `"0x` + strings.Repeat("00", 31) + `ab"`},
		{"eth_call", `{"to":` + address + `,"input":"0x1234"}`, "eth_call", `"0xdead"`},
		{"eth_estimateGas", `{"to":` + address + `,"input":"0x1234"}`, "eth_estimateGas", `"0x5208"`},
	} {
		t.Run(tt.method, func(t *testing.T) {
			for _, height := range []int64{10, 11, 12} {
				number := ethQuantityUint64(uint64(height))
				block := ethServerRequest(t, handler, "eth_getBlockByNumber", fmt.Sprintf(`[%q]`, number))
				require.Nil(t, block.Error)
				var header ethBlock
				require.NoError(t, json.Unmarshal(block.Result, &header))
				byNumber := ethServerRequest(t, handler, tt.method, fmt.Sprintf(`[%s,%q]`, tt.params, number))
				require.Nil(t, byNumber.Error)
				require.JSONEq(t, tt.want, string(byNumber.Result))
				query := fixture.queries[len(fixture.queries)-1]
				selectors := []string{
					fmt.Sprintf(`{"blockNumber":%q}`, number),
					fmt.Sprintf(`{"blockNumber":%q,"unknown":true}`, number),
					fmt.Sprintf(`{"blockHash":%q}`, header.Hash),
					fmt.Sprintf(`{"blockHash":%q,"blockNumber":null}`, header.Hash),
					fmt.Sprintf(`{"blockNumber":"earliest","blockNumber":null,"blockHash":%q}`, header.Hash),
					fmt.Sprintf(`{"blockHash":%q,"requireCanonical":true}`, header.Hash),
					fmt.Sprintf(`{"blockHash":%q,"requireCanonical":false}`, header.Hash),
					fmt.Sprintf(`%q`, header.Hash),
				}
				if height == 10 {
					selectors = append(selectors, `"earliest"`, `"\u0065arliest"`, `{"blockNumber":"earliest"}`,
						`{"BLOCKNUMBER":"earliest"}`, `{"block\u004eumber":"earliest"}`,
						`{"blockNumber":"0x0","BLOCKNUMBER":"earliest"}`)
				}
				if height == 12 {
					for _, tag := range []string{"latest", "pending", "safe", "finalized"} {
						selectors = append(selectors, fmt.Sprintf(`{"blockNumber":%q}`, tag))
					}
				}
				for _, selector := range selectors {
					response := ethServerRequest(t, handler, tt.method, `[`+tt.params+`,`+selector+`]`)
					require.Nil(t, response.Error, selector)
					require.JSONEq(t, string(byNumber.Result), string(response.Result))
					got := fixture.queries[len(fixture.queries)-1]
					require.Equal(t, tt.path, got.Path)
					require.Equal(t, query.Data, got.Data)
					if (strings.Contains(selector, "latest") || strings.Contains(selector, "pending") || strings.Contains(selector, "safe") || strings.Contains(selector, "finalized")) && tt.method != "eth_call" && tt.method != "eth_estimateGas" {
						require.Zero(t, got.Height)
					} else {
						require.Equal(t, height, got.Height)
					}
				}
			}
			hash := ethData(fixture.blocks[12].BlockID.Hash)
			for _, selector := range []string{
				fmt.Sprintf(`{"blockNumber":"0xc","blockHash":%q}`, hash),
				`{"blockNumber":"bad"}`, `{"blockNumber":"Earliest"}`, `{"blockNumber":"\u0065arliest"}`, `{"blockHash":"0x1"}`, `{"blockHash":12}`,
				fmt.Sprintf(`{"blockHash":%q,"requireCanonical":"true"}`, hash),
			} {
				fixture.queries = nil
				response := ethServerRequest(t, handler, tt.method, `[`+tt.params+`,`+selector+`]`)
				require.NotNil(t, response.Error, selector)
				require.Equal(t, -32602, response.Error.Code)
				require.Empty(t, fixture.queries)
			}
			unknown := "0x" + strings.Repeat("ff", 32)
			for _, selector := range []string{
				fmt.Sprintf(`%q`, unknown), fmt.Sprintf(`{"blockHash":%q}`, unknown),
				fmt.Sprintf(`{"blockHash":%q,"requireCanonical":true}`, unknown),
			} {
				response := ethServerRequest(t, handler, tt.method, `[`+tt.params+`,`+selector+`]`)
				require.NotNil(t, response.Error)
				require.Equal(t, -32000, response.Error.Code)
				require.Equal(t, "header for hash not found", response.Error.Message)
			}
			fixture.appHeight = 11
			for _, selector := range []string{`{"blockNumber":"0xc"}`, fmt.Sprintf(`%q`, hash), fmt.Sprintf(`{"blockHash":%q,"requireCanonical":true}`, hash)} {
				fixture.queries = nil
				response := ethServerRequest(t, handler, tt.method, `[`+tt.params+`,`+selector+`]`)
				require.NotNil(t, response.Error)
				require.Equal(t, -32000, response.Error.Code)
				require.Equal(t, "header not found", response.Error.Message)
				require.Len(t, fixture.queries, 1)
				require.Equal(t, "block_height", fixture.queries[0].Path)
			}
			fixture.appHeight = 12
		})
	}
}

func TestEthServerParameterDecoding(t *testing.T) {
	handler, _ := ethTestRPCHandler(t)
	for _, tt := range []struct {
		method         string
		valid, invalid []string
	}{
		{"eth_getBalance", []string{"0x" + strings.Repeat("aB", 20), "0X" + strings.Repeat("aB", 20)}, []string{
			"", "0x", strings.Repeat("ab", 20), "0x" + strings.Repeat("ab", 19), "0x" + strings.Repeat("ab", 21), "0x" + strings.Repeat("gg", 20),
		}},
		{"eth_getBlockByHash", []string{"0x" + strings.Repeat("0b", 32), "0X" + strings.Repeat("0B", 32)}, []string{
			"", "0x", "0xab", strings.Repeat("ab", 32), "0x" + strings.Repeat("ab", 31), "0x" + strings.Repeat("ab", 33), "0x" + strings.Repeat("zz", 32),
		}},
		{"eth_getBlockByNumber", []string{"latest", "pending", "safe", "finalized", "earliest", "0x0", "0xb", "0Xb", "0x7fffffffffffffff"}, []string{
			"", "0", "123", "LATEST", "Earliest", "EARLIEST", "unknown", "0x", "0x01", "0x00", "0xg", "0x1g", "0x-1", "0x+1", "-0x1", "0x1_0", " 0x1", "0x1 ", "0x8000000000000000", "0xffffffffffffffff", "0x10000000000000000",
		}},
	} {
		for _, input := range tt.valid {
			response := ethServerRequest(t, handler, tt.method, fmt.Sprintf(`[%q]`, input))
			require.Nil(t, response.Error, "%s: %s", tt.method, input)
		}
		for _, input := range tt.invalid {
			response := ethServerRequest(t, handler, tt.method, fmt.Sprintf(`[%q]`, input))
			require.NotNil(t, response.Error, "%s: %s", tt.method, input)
			require.Equal(t, -32602, response.Error.Code)
		}
	}
}

func TestEthServerEscapedParameterStrings(t *testing.T) {
	handler, _ := ethTestRPCHandler(t)
	for _, tt := range []struct{ method, params string }{
		{"eth_getBalance", `["\u0030x` + strings.Repeat("ab", 20) + `"]`},
		{"eth_getBlockByHash", `["\u0030x` + strings.Repeat("0b", 32) + `"]`},
		{"eth_getBlockByNumber", `["\u0030xb"]`},
		{"eth_getBlockByNumber", `["\u0065arliest"]`},
		{"eth_feeHistory", `["\u0030x2","latest"]`},
		{"eth_feeHistory", `[2,"\u006catest"]`},
		{"eth_call", `[{"from":"\u0030x` + strings.Repeat("ab", 20) + `"}]`},
		{"eth_call", `[{"gas":"\u0030x1"}]`},
		{"eth_call", `[{"value":"\u0030x1"}]`},
		{"eth_call", `[{"input":"\u0030x12"}]`},
	} {
		response := ethServerRequest(t, handler, tt.method, tt.params)
		require.NotNil(t, response.Error, tt.params)
		require.Equal(t, -32602, response.Error.Code)
	}
	response := ethServerRequest(t, handler, "eth_call", `[{},"\u0030xb"]`)
	require.Nil(t, response.Error)
	response = ethServerRequest(t, handler, "eth_getStorageAt", `["0x`+strings.Repeat("ab", 20)+`","\u0030x1"]`)
	require.Nil(t, response.Error)
}
