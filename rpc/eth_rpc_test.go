package rpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/consensus"
	"github.com/tendermint/tendermint/crypto/secp256k1"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	tmstate "github.com/tendermint/tendermint/proto/tendermint/state"
	"github.com/tendermint/tendermint/proxy"
	tmrpccore "github.com/tendermint/tendermint/rpc/core"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	tmrpcserver "github.com/tendermint/tendermint/rpc/jsonrpc/server"
	tmrpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
	sm "github.com/tendermint/tendermint/state"
	tmtypes "github.com/tendermint/tendermint/types"
)

// Embedded interfaces panic on unexpected calls, keeping the fixture limited to RPC reads.
type ethRPCFixture struct {
	sm.BlockStore
	sm.Store
	proxy.AppConnQuery
	blocks       map[int64]*coretypes.ResultBlock
	queries      []abci.RequestQuery
	resultReads  []int64
	queryCode    uint32
	queryError   error
	resultsError error
}

func (f *ethRPCFixture) Base() int64   { return 10 }
func (f *ethRPCFixture) Height() int64 { return 12 }
func (f *ethRPCFixture) LoadBaseMeta() *tmtypes.BlockMeta {
	return f.LoadBlockMeta(f.Base())
}
func (f *ethRPCFixture) LoadBlockMeta(height int64) *tmtypes.BlockMeta {
	if block := f.blocks[height]; block != nil {
		return &tmtypes.BlockMeta{BlockID: block.BlockID, Header: block.Block.Header}
	}
	return nil
}
func (f *ethRPCFixture) LoadBlock(height int64) *tmtypes.Block {
	if block := f.blocks[height]; block != nil {
		return block.Block
	}
	return nil
}
func (f *ethRPCFixture) LoadBlockByHash(hash []byte) *tmtypes.Block {
	for _, block := range f.blocks {
		if bytes.Equal(hash, block.BlockID.Hash) {
			return block.Block
		}
	}
	return nil
}
func (f *ethRPCFixture) LoadValidators(int64) (*tmtypes.ValidatorSet, error) {
	return nil, fmt.Errorf("no validators in fixture")
}
func (f *ethRPCFixture) LoadABCIResponses(height int64) (*tmstate.ABCIResponses, error) {
	f.resultReads = append(f.resultReads, height)
	if f.resultsError != nil {
		return nil, f.resultsError
	}
	gas := map[int64]int64{10: 25, 11: 100, 12: 300}[height]
	return &tmstate.ABCIResponses{
		DeliverTxs: []*abci.ResponseDeliverTx{{GasUsed: gas}}, BeginBlock: &abci.ResponseBeginBlock{}, EndBlock: &abci.ResponseEndBlock{},
	}, nil
}
func (f *ethRPCFixture) QuerySync(req abci.RequestQuery) (*abci.ResponseQuery, error) {
	f.queries = append(f.queries, req)
	if f.queryError != nil {
		return nil, f.queryError
	}
	if req.Path == "chain_id" {
		return &abci.ResponseQuery{Value: []byte{0xbe, 0xa7, 0x01}}, nil
	}
	if req.Path != "gov_params" {
		return nil, fmt.Errorf("unexpected query %s", req.Path)
	}
	limit := map[int64]int{0: 400, 10: 100, 11: 200, 12: 400}[req.Height]
	return &abci.ResponseQuery{
		Code: f.queryCode, Log: "fixture query",
		Value: []byte(fmt.Sprintf(`{"blockGasLimit":"%d","gasPrice":"48000000000"}`, limit)),
	}, nil
}

type ethRPCTransport struct{}

func (ethRPCTransport) Listeners() []string    { return nil }
func (ethRPCTransport) IsListening() bool      { return false }
func (ethRPCTransport) NodeInfo() p2p.NodeInfo { return p2p.DefaultNodeInfo{} }

func ethTestRPCHandler(t *testing.T) (*http.ServeMux, *ethRPCFixture) {
	t.Helper()
	f := &ethRPCFixture{blocks: make(map[int64]*coretypes.ResultBlock)}
	for height := int64(10); height <= 12; height++ {
		block := ethTestBlock(height)
		block.BlockID.Hash = bytes.Repeat([]byte{byte(height)}, 32)
		block.Block.Txs = tmtypes.Txs{ethEncodeTrx(t, ethTestTrx())}
		f.blocks[height] = block
	}
	tmrpccore.SetEnvironment(&tmrpccore.Environment{
		BlockStore: f, StateStore: f, ProxyAppQuery: f, P2PTransport: ethRPCTransport{},
		ConsensusReactor: &consensus.Reactor{}, PubKey: secp256k1.PrivKey(bytes.Repeat([]byte{1}, 32)).PubKey(),
	})
	t.Cleanup(func() { tmrpccore.SetEnvironment(nil) })
	previous := make(map[string]*tmrpcserver.RPCFunc, len(tmrpccore.Routes))
	for name, route := range tmrpccore.Routes {
		previous[name] = route
	}
	t.Cleanup(func() { tmrpccore.Routes = previous })
	AddEthRoutes()
	mux := http.NewServeMux()
	tmrpcserver.RegisterRPCFuncs(mux, tmrpccore.Routes, log.NewNopLogger())
	return mux, f
}

func ethRPCRequest(t *testing.T, handler http.Handler, method, params string) map[string]json.RawMessage {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"`+method+`","params":`+params+`}`))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)
	var response map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func TestEthBlockRoutes(t *testing.T) {
	mux, f := ethTestRPCHandler(t)
	for _, tt := range []struct {
		tag    string
		height int64
	}{
		{"earliest", 10}, {"0xb", 11}, {"latest", 12}, {"pending", 12}, {"safe", 12}, {"finalized", 12},
	} {
		for _, details := range []bool{false, true} {
			f.queries = nil
			response := ethRPCRequest(t, mux, "eth_getBlockByNumber", fmt.Sprintf(`["%s",%t]`, tt.tag, details))
			require.Empty(t, response["error"])
			var block map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(response["result"], &block))
			require.JSONEq(t, `"`+ethQuantityUint64(uint64(tt.height))+`"`, string(block["number"]))
			require.NotContains(t, block, "baseFeePerGas")
			require.Equal(t, "gov_params", f.queries[0].Path)
			require.Equal(t, tt.height, f.queries[0].Height)
			require.False(t, f.queries[0].Prove)
			if details {
				require.Contains(t, string(block["transactions"]), `"chainId":"0xbea701"`)
			} else {
				require.NotContains(t, string(block["transactions"]), `"chainId"`)
			}
		}
	}
	hash := "0x" + strings.Repeat("0B", 32)
	byHash := ethRPCRequest(t, mux, "eth_getBlockByHash", `["`+hash+`",true]`)
	byNumber := ethRPCRequest(t, mux, "eth_getBlockByNumber", `{"blockNumber":"0xb","txDetails":true}`)
	require.Empty(t, byHash["error"])
	require.Empty(t, byNumber["error"])
	require.JSONEq(t, string(byNumber["result"]), string(byHash["result"]))
	for _, tag := range []string{"0x0", "0x1", "0x9", "0xd", "0x7fffffffffffffff"} {
		response := ethRPCRequest(t, mux, "eth_getBlockByNumber", `["`+tag+`",false]`)
		require.Empty(t, response["error"])
		require.Equal(t, "null", string(response["result"]))
	}
	response := ethRPCRequest(t, mux, "eth_getBlockByHash", `["0x`+strings.Repeat("ff", 32)+`",false]`)
	require.Empty(t, response["error"])
	require.Equal(t, "null", string(response["result"]))
	delete(f.blocks, 11)
	response = ethRPCRequest(t, mux, "eth_getBlockByNumber", `["0xb",true]`)
	require.Empty(t, response["error"])
	require.Equal(t, "null", string(response["result"]))
}

func TestEthGasRoutesAndParamBinding(t *testing.T) {
	mux, f := ethTestRPCHandler(t)
	response := ethRPCRequest(t, mux, "eth_gasPrice", `[]`)
	require.Empty(t, response["error"])
	require.Equal(t, `"0xb2d05e000"`, string(response["result"]))
	require.Equal(t, int64(0), f.queries[0].Height)
	response = ethRPCRequest(t, mux, "eth_maxPriorityFeePerGas", `[]`)
	require.Empty(t, response["error"])
	require.Equal(t, `"0x0"`, string(response["result"]))

	for _, params := range []string{
		`["0x4","latest",[0,50.5,100]]`, `[4,"latest",[0,50.5,100]]`,
		`{"blockCount":4,"newestBlock":"latest","rewardPercentiles":[0,50.5,100]}`,
	} {
		f.queries, f.resultReads = nil, nil
		response := ethRPCRequest(t, mux, "eth_feeHistory", params)
		require.Empty(t, response["error"], params)
		require.JSONEq(t, `{"oldestBlock":"0xa","baseFeePerGas":["0x0","0x0","0x0","0x0"],"gasUsedRatio":[0.25,0.5,0.75],"reward":[["0x0","0x0","0x0"],["0x0","0x0","0x0"],["0x0","0x0","0x0"]]}`, string(response["result"]))
		require.Equal(t, []int64{10, 11, 12}, f.resultReads)
		require.Len(t, f.queries, 3)
		for i, req := range f.queries {
			require.Equal(t, "gov_params", req.Path)
			require.Equal(t, int64(10+i), req.Height)
		}
	}
	for _, params := range []string{
		`[4,"latest",null]`, `{"blockCount":"0x4","newestBlock":"latest"}`, `{"blockCount":4,"newestBlock":"latest"}`,
	} {
		response := ethRPCRequest(t, mux, "eth_feeHistory", params)
		require.Empty(t, response["error"], params)
		require.JSONEq(t, `{"oldestBlock":"0xa","baseFeePerGas":["0x0","0x0","0x0","0x0"],"gasUsedRatio":[0.25,0.5,0.75]}`, string(response["result"]))
	}
	response = ethRPCRequest(t, mux, "eth_feeHistory", `[4,"latest",[]]`)
	require.Empty(t, response["error"])
	require.Contains(t, string(response["result"]), `"reward":[[],[],[]]`)
	for _, params := range []string{`["0x4","latest"]`, `[4,"latest"]`} {
		f.queries, f.resultReads = nil, nil
		response := ethRPCRequest(t, mux, "eth_feeHistory", params)
		require.Contains(t, string(response["error"]), `"code":-32602`)
		require.Contains(t, string(response["error"]), "expected 3 parameters")
		require.Empty(t, f.queries)
		require.Empty(t, f.resultReads)
	}
	for _, params := range []string{`[null,"latest",null]`, `[-1,"latest",null]`, `[4,"bad",null]`, `[4,"latest",[90,10]]`} {
		response := ethRPCRequest(t, mux, "eth_feeHistory", params)
		require.NotEmpty(t, response["error"])
	}

	// Demonstrate why a string argument cannot handle clients sending JSON numbers.
	stringMux := http.NewServeMux()
	tmrpcserver.RegisterRPCFuncs(stringMux, map[string]*tmrpcserver.RPCFunc{
		"count": tmrpcserver.NewRPCFunc(func(ctx *tmrpctypes.Context, count string) (string, error) { return count, nil }, "count"),
	}, log.NewNopLogger())
	response = ethRPCRequest(t, stringMux, "count", `[4]`)
	require.Contains(t, string(response["error"]), `"code":-32602`)
	response = ethRPCRequest(t, stringMux, "count", `["0x4"]`)
	require.Empty(t, response["error"])
	require.Equal(t, `"0x4"`, string(response["result"]))
}

func TestEthRPCBackendErrors(t *testing.T) {
	mux, f := ethTestRPCHandler(t)
	f.queryCode = 1
	for _, method := range []string{"eth_gasPrice", "eth_getBlockByNumber", "eth_feeHistory"} {
		params := map[string]string{"eth_gasPrice": `[]`, "eth_getBlockByNumber": `["latest",false]`, "eth_feeHistory": `[1,"latest",null]`}[method]
		response := ethRPCRequest(t, mux, method, params)
		require.Contains(t, string(response["error"]), "gov_params")
	}
	f.queryCode, f.queryError = 0, fmt.Errorf("query unavailable")
	response := ethRPCRequest(t, mux, "eth_gasPrice", `[]`)
	require.Contains(t, string(response["error"]), "query unavailable")
	f.queryError, f.resultsError = nil, fmt.Errorf("results unavailable")
	response = ethRPCRequest(t, mux, "eth_getBlockByNumber", `["latest",false]`)
	require.Contains(t, string(response["error"]), "results unavailable")
	require.NotContains(t, response, "result")
	response = ethRPCRequest(t, mux, "eth_feeHistory", `[1,"latest",null]`)
	require.Contains(t, string(response["error"]), "results unavailable")
}
