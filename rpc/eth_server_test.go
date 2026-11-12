package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

type ethServerTestResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	} `json:"error"`
}

func ethServerPost(t *testing.T, handler http.Handler, body string) []byte {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)
	return recorder.Body.Bytes()
}

func ethServerRequest(t *testing.T, handler http.Handler, method, params string) ethServerTestResponse {
	t.Helper()
	body := ethServerPost(t, handler, `{"jsonrpc":"2.0","id":1,"method":"`+method+`","params":`+params+`}`)
	var response ethServerTestResponse
	require.NoError(t, json.Unmarshal(body, &response))
	require.Equal(t, "2.0", response.JSONRPC)
	require.Equal(t, "1", string(response.ID))
	return response
}

func TestEthServerMethods(t *testing.T) {
	ethTestRPCHandler(t)
	handler, err := newEthRPCHandler(nil)
	require.NoError(t, err)
	clientVersion, err := ethClientVersion(ethTMContext())
	require.NoError(t, err)
	versionJSON, err := json.Marshal(clientVersion)
	require.NoError(t, err)
	fullBlock := ethServerRequest(t, handler, "eth_getBlockByNumber", `["latest",true]`)
	require.Nil(t, fullBlock.Error)
	var block struct {
		Transactions []json.RawMessage `json:"transactions"`
	}
	require.NoError(t, json.Unmarshal(fullBlock.Result, &block))
	require.Len(t, block.Transactions, 1)
	address := `"0x` + strings.Repeat("ab", 20) + `"`
	for _, tt := range []struct {
		method, params, want string
	}{
		{"eth_chainId", `[]`, `"0xbea701"`},
		{"eth_blockNumber", `[]`, `"0xc"`},
		{"eth_syncing", `[]`, `false`},
		{"eth_accounts", `[]`, `[]`},
		{"eth_getBlockByNumber", `["0xc"]`, ""},
		{"eth_getBlockByHash", `["0x` + strings.Repeat("0c", 32) + `"]`, ""},
		{"eth_getBlockTransactionCountByNumber", `["latest"]`, `"0x1"`},
		{"eth_getBlockTransactionCountByHash", `["0x` + strings.Repeat("0c", 32) + `"]`, `"0x1"`},
		{"eth_getTransactionByBlockNumberAndIndex", `["latest","0x0"]`, string(block.Transactions[0])},
		{"eth_getTransactionByBlockHashAndIndex", `["0x` + strings.Repeat("0c", 32) + `","0x0"]`, string(block.Transactions[0])},
		{"eth_gasPrice", `[]`, `"0xb2d05e000"`},
		{"eth_maxPriorityFeePerGas", `[]`, `"0x0"`},
		{"eth_feeHistory", `["0x2","latest"]`, `{"oldestBlock":"0xb","baseFeePerGas":["0x0","0x0","0x0"],"gasUsedRatio":[0.5,0.75]}`},
		{"eth_getBalance", `[` + address + `]`, `"0x0"`},
		{"eth_getTransactionCount", `[` + address + `]`, `"0x0"`},
		{"eth_getCode", `[` + address + `]`, `"0x"`},
		{"eth_getStorageAt", `[` + address + `,"0x0"]`, `"0x` + strings.Repeat("00", 32) + `"`},
		{"eth_call", `[{}]`, `"0x"`},
		{"eth_estimateGas", `[{}]`, `"0x0"`},
		{"net_version", `[]`, `"12494593"`},
		{"net_listening", `[]`, `false`},
		{"net_peerCount", `[]`, `"0x0"`},
		{"web3_clientVersion", `[]`, string(versionJSON)},
		{"web3_sha3", `["0x"]`, `"0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"`},
	} {
		t.Run(tt.method, func(t *testing.T) {
			response := ethServerRequest(t, handler, tt.method, tt.params)
			if response.Error != nil {
				require.NotEqual(t, -32601, response.Error.Code)
			}
			require.Nil(t, response.Error)
			if tt.want != "" {
				require.JSONEq(t, tt.want, string(response.Result))
			} else {
				var block struct {
					Number       string   `json:"number"`
					Transactions []string `json:"transactions"`
				}
				require.NoError(t, json.Unmarshal(response.Result, &block))
				require.Equal(t, "0xc", block.Number)
				require.Len(t, block.Transactions, 1)
			}
		})
	}
}

func TestEthServerOptionalParams(t *testing.T) {
	_, fixture := ethTestRPCHandler(t)
	handler, err := newEthRPCHandler(nil)
	require.NoError(t, err)
	for _, method := range []string{"eth_getBlockByNumber", "eth_getBlockByHash"} {
		blockID := "0xc"
		if method == "eth_getBlockByHash" {
			blockID = ethData(fixture.blocks[12].BlockID.Hash)
		}
		for _, suffix := range []string{"", ",null", ",false", ",true"} {
			t.Run(method+suffix, func(t *testing.T) {
				response := ethServerRequest(t, handler, method, `["`+blockID+`"`+suffix+`]`)
				require.Nil(t, response.Error)
				var block struct {
					Transactions []json.RawMessage `json:"transactions"`
				}
				require.NoError(t, json.Unmarshal(response.Result, &block))
				require.Len(t, block.Transactions, 1)
				wantHash := ethData(fixture.blocks[12].Block.Txs[0].Hash())
				if suffix == ",true" {
					var tx ethTransaction
					require.NoError(t, json.Unmarshal(block.Transactions[0], &tx))
					require.Equal(t, wantHash, tx.Hash)
					require.Equal(t, "0xbea701", tx.ChainID)
				} else {
					var hash string
					require.NoError(t, json.Unmarshal(block.Transactions[0], &hash))
					require.Equal(t, wantHash, hash)
				}
			})
		}
	}
	for _, params := range []string{`["0x2","latest"]`, `[2,"latest"]`, `["0x2","latest",null]`} {
		response := ethServerRequest(t, handler, "eth_feeHistory", params)
		require.Nil(t, response.Error)
		require.JSONEq(t, `{"oldestBlock":"0xb","baseFeePerGas":["0x0","0x0","0x0"],"gasUsedRatio":[0.5,0.75]}`, string(response.Result))
	}
	response := ethServerRequest(t, handler, "eth_feeHistory", `["0x2","latest",[]]`)
	require.Nil(t, response.Error)
	require.JSONEq(t, `{"oldestBlock":"0xb","baseFeePerGas":["0x0","0x0","0x0"],"gasUsedRatio":[0.5,0.75]}`, string(response.Result))
	response = ethServerRequest(t, handler, "eth_feeHistory", `["0x2","latest",[50]]`)
	require.Nil(t, response.Error)
	require.JSONEq(t, `{"oldestBlock":"0xb","baseFeePerGas":["0x0","0x0","0x0"],"gasUsedRatio":[0.5,0.75],"reward":[["0x0"],["0x0"]]}`, string(response.Result))
}

func TestEthServerInvalidParams(t *testing.T) {
	ethTestRPCHandler(t)
	handler, err := newEthRPCHandler(nil)
	require.NoError(t, err)
	for _, tt := range []struct {
		name, method, params, message string
	}{
		{"block tag format", "eth_getBlockByNumber", `["bad"]`, `hex string without 0x prefix`},
		{"block tag hex", "eth_getBlockByNumber", `["0xzz"]`, `invalid hex string`},
		{"block number object", "eth_getBlockByNumber", `[{"blockNumber":"0x0"}]`, `hex string without 0x prefix`},
		{"block tag object", "eth_getBlockByNumber", `[{"blockNumber":"earliest"}]`, `hex string without 0x prefix`},
		{"newest number object", "eth_feeHistory", `[2,{"blockNumber":"0x0"}]`, `hex string without 0x prefix`},
		{"newest tag object", "eth_feeHistory", `[2,{"blockNumber":"earliest"}]`, `hex string without 0x prefix`},
		{"block hash length", "eth_getBlockByHash", `["0x1234"]`, "want 64 for common.Hash"},
		{"block hash hex", "eth_getBlockByHash", `["0x` + strings.Repeat("zz", 32) + `"]`, "invalid hex string"},
		{"block count format", "eth_feeHistory", `["2.5","latest"]`, "invalid hex or decimal integer"},
		{"block count sign", "eth_feeHistory", `[-1,"latest"]`, "invalid hex or decimal integer"},
		{"newest block", "eth_feeHistory", `[2,"bad"]`, `hex string without 0x prefix`},
		{"percentiles type", "eth_feeHistory", `[2,"latest","bad"]`, "cannot unmarshal string"},
		{"percentiles order", "eth_feeHistory", `[2,"latest",[90,10]]`, "rewardPercentiles must be sorted numbers between 0 and 100"},
		{"percentiles range", "eth_feeHistory", `[2,"latest",[101]]`, "rewardPercentiles must be sorted numbers between 0 and 100"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			response := ethServerRequest(t, handler, tt.method, tt.params)
			require.NotNil(t, response.Error)
			require.Equal(t, -32602, response.Error.Code)
			require.Contains(t, response.Error.Message, tt.message)
			require.Empty(t, response.Result)
		})
	}
}

func TestEthServerUnknownMethod(t *testing.T) {
	ethTestRPCHandler(t)
	handler, err := newEthRPCHandler(nil)
	require.NoError(t, err)
	for _, method := range []string{
		"eth_unknown", "eth_protocolVersion", "eth_coinbase", "eth_sign", "eth_signTransaction", "eth_signTypedData",
		"eth_sendTransaction", "eth_getTransactionByHash", "eth_getTransactionReceipt", "eth_getLogs", "eth_newFilter",
		"eth_newBlockFilter", "eth_newPendingTransactionFilter", "eth_getFilterChanges", "eth_getFilterLogs", "eth_uninstallFilter",
		"eth_getUncleCountByBlockNumber", "eth_getUncleCountByBlockHash", "eth_getUncleByBlockNumberAndIndex", "eth_getUncleByBlockHashAndIndex",
	} {
		t.Run(method, func(t *testing.T) {
			response := ethServerRequest(t, handler, method, `[]`)
			require.NotNil(t, response.Error)
			require.Equal(t, -32601, response.Error.Code)
		})
	}
}

func TestEthServerBackendErrors(t *testing.T) {
	_, fixture := ethTestRPCHandler(t)
	fixture.queryError = fmt.Errorf("fixture query unavailable")
	handler, err := newEthRPCHandler(nil)
	require.NoError(t, err)
	for _, tt := range []struct {
		method, params string
	}{
		{"eth_chainId", `[]`},
		{"eth_gasPrice", `[]`},
		{"eth_getBlockByNumber", `["0xc"]`},
		{"eth_getBlockByHash", `["0x` + strings.Repeat("0c", 32) + `"]`},
		{"eth_feeHistory", `[2,"latest"]`},
		{"net_version", `[]`},
	} {
		t.Run(tt.method, func(t *testing.T) {
			response := ethServerRequest(t, handler, tt.method, tt.params)
			require.NotNil(t, response.Error)
			require.Equal(t, -32000, response.Error.Code)
			require.Contains(t, response.Error.Message, fixture.queryError.Error())
			require.Empty(t, response.Result)
		})
	}
}

func TestEthServerBatch(t *testing.T) {
	ethTestRPCHandler(t)
	handler, err := newEthRPCHandler(nil)
	require.NoError(t, err)
	body := ethServerPost(t, handler, `[
		{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]},
		{"jsonrpc":"2.0","id":"second","method":"eth_maxPriorityFeePerGas","params":[]}
	]`)
	var responses []ethServerTestResponse
	require.NoError(t, json.Unmarshal(body, &responses))
	require.Len(t, responses, 2)
	results := make(map[string]string)
	for _, response := range responses {
		require.Equal(t, "2.0", response.JSONRPC)
		require.Nil(t, response.Error)
		results[string(response.ID)] = string(response.Result)
	}
	require.Equal(t, map[string]string{"1": `"0xbea701"`, `"second"`: `"0x0"`}, results)
}

func TestEthServerBatchLimit(t *testing.T) {
	handler, err := newEthRPCHandler(nil)
	require.NoError(t, err)
	for _, count := range []int{100, 101} {
		requests := make([]string, count)
		for i := range requests {
			requests[i] = fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"eth_maxPriorityFeePerGas","params":[]}`, i+1)
		}
		body := ethServerPost(t, handler, "["+strings.Join(requests, ",")+"]")
		var responses []ethServerTestResponse
		require.NoError(t, json.Unmarshal(body, &responses))
		if count == 100 {
			require.Len(t, responses, count)
			for _, response := range responses {
				require.Nil(t, response.Error)
				require.JSONEq(t, `"0x0"`, string(response.Result))
			}
		} else {
			require.Len(t, responses, 1)
			require.NotNil(t, responses[0].Error)
			require.Equal(t, -32600, responses[0].Error.Code)
			require.Equal(t, "batch too large", responses[0].Error.Message)
		}
	}
}

func TestEthServerCORS(t *testing.T) {
	for _, tt := range []struct {
		name    string
		origins []string
		want    string
	}{
		{"disabled", nil, ""},
		{"wildcard", []string{"*"}, "*"},
		{"allowed", []string{"https://wallet.example"}, "https://wallet.example"},
		{"denied", []string{"https://other.example"}, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			handler, err := newEthRPCHandler(tt.origins)
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodOptions, "/", nil)
			req.Header.Set("Origin", "https://wallet.example")
			req.Header.Set("Access-Control-Request-Method", http.MethodPost)
			req.Header.Set("Access-Control-Request-Headers", "Content-Type")
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)
			require.Equal(t, tt.want, recorder.Header().Get("Access-Control-Allow-Origin"))
			if tt.want != "" {
				require.Contains(t, recorder.Header().Get("Access-Control-Allow-Methods"), http.MethodPost)
				require.Contains(t, recorder.Header().Get("Access-Control-Allow-Headers"), "Content-Type")
			}
		})
	}
}

func TestEthServerRootOnly(t *testing.T) {
	handler, err := newEthRPCHandler(nil)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/other", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"eth_maxPriorityFeePerGas","params":[]}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestStartEthServer(t *testing.T) {
	for _, laddr := range []string{"127.0.0.1:0", "tcp://127.0.0.1:0"} {
		t.Run(laddr, func(t *testing.T) {
			ethTestRPCHandler(t)
			server, listener, err := startEthServer(laddr, nil)
			require.NoError(t, err)
			t.Cleanup(func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				require.NoError(t, server.Shutdown(ctx))
			})
			require.Equal(t, 10*time.Second, server.ReadHeaderTimeout)
			require.Equal(t, 30*time.Second, server.ReadTimeout)
			require.Equal(t, 30*time.Second, server.WriteTimeout)
			require.Equal(t, 120*time.Second, server.IdleTimeout)
			client := &http.Client{Timeout: 5 * time.Second}
			t.Cleanup(client.CloseIdleConnections)
			response, err := client.Post("http://"+listener.Addr().String()+"/", "application/json", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}`))
			require.NoError(t, err)
			defer response.Body.Close()
			require.Equal(t, http.StatusOK, response.StatusCode)
			body, err := io.ReadAll(response.Body)
			require.NoError(t, err)
			require.JSONEq(t, `{"jsonrpc":"2.0","id":1,"result":"0xbea701"}`, string(body))

			otherServer, otherListener, err := startEthServer(listener.Addr().String(), nil)
			if otherServer != nil {
				t.Cleanup(func() { require.NoError(t, otherServer.Close()) })
			}
			require.Error(t, err)
			require.Nil(t, otherServer)
			require.Nil(t, otherListener)
		})
	}
}

func TestParseEthCORSOrigins(t *testing.T) {
	for _, tt := range []struct {
		input string
		want  []string
	}{
		{"", nil},
		{" , \t,\n ", nil},
		{" * ", []string{"*"}},
		{" https://wallet.example, ,\thttp://localhost:3000,\nhttps://other.example,", []string{"https://wallet.example", "http://localhost:3000", "https://other.example"}},
	} {
		require.Equal(t, tt.want, parseEthCORSOrigins(tt.input), tt.input)
	}
}

func TestEthRPCError(t *testing.T) {
	err := ethInvalidParams("invalid %s: %d", "value", 1)
	require.EqualError(t, err, "invalid value: 1")
	rpcErr, ok := err.(gethrpc.Error)
	require.True(t, ok)
	require.Equal(t, -32602, rpcErr.ErrorCode())
	dataErr, ok := err.(gethrpc.DataError)
	require.True(t, ok)
	require.Nil(t, dataErr.ErrorData())

	revert := &ethRPCError{code: 3, message: "execution reverted", data: "0xdeadbeef"}
	require.Equal(t, 3, revert.ErrorCode())
	require.Equal(t, "execution reverted", revert.Error())
	require.Equal(t, "0xdeadbeef", revert.ErrorData())
}
