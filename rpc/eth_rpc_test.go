package rpc

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/libs/jsonx"
	"github.com/stretchr/testify/require"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/consensus"
	"github.com/tendermint/tendermint/crypto/secp256k1"
	tmlog "github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	tmstate "github.com/tendermint/tendermint/proto/tendermint/state"
	"github.com/tendermint/tendermint/proxy"
	tmrpccore "github.com/tendermint/tendermint/rpc/core"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	sm "github.com/tendermint/tendermint/state"
	tmtypes "github.com/tendermint/tendermint/types"
)

// Embedded interfaces panic on unexpected calls, keeping the fixture limited to RPC reads.
type ethRPCFixture struct {
	sm.BlockStore
	sm.Store
	proxy.AppConnQuery
	env          *tmrpccore.Environment
	blocks       map[int64]*coretypes.ResultBlock
	baseHeight   int64
	blockHeight  int64
	appHeight    int64
	chainIDValue []byte
	heightValue  []byte
	heightCode   uint32
	queries      []abci.RequestQuery
	blockReads   []int64
	resultReads  []int64
	queryCode    uint32
	codePath     string
	queryError   error
	resultsError error
	queryLog     string
	accounts     map[string][]byte
	storageValue []byte
	callResult   ctrlertypes.VMCallResult
	callValue    []byte
}

func (f *ethRPCFixture) Base() int64   { return f.baseHeight }
func (f *ethRPCFixture) Height() int64 { return f.blockHeight }
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
	f.blockReads = append(f.blockReads, height)
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
	if height > f.appHeight {
		return nil, fmt.Errorf("block %d not yet applied", height)
	}
	gas := map[int64]int64{10: 25, 11: 100, 12: 300}[height]
	var results []*abci.ResponseDeliverTx
	if block := f.blocks[height]; block != nil {
		for range block.Block.Txs {
			results = append(results, &abci.ResponseDeliverTx{GasUsed: gas})
		}
	}
	return &tmstate.ABCIResponses{
		DeliverTxs: results, BeginBlock: &abci.ResponseBeginBlock{}, EndBlock: &abci.ResponseEndBlock{},
	}, nil
}
func (f *ethRPCFixture) QuerySync(req abci.RequestQuery) (*abci.ResponseQuery, error) {
	f.queries = append(f.queries, req)
	if f.queryError != nil {
		return nil, f.queryError
	}
	if req.Path == "block_height" {
		value := f.heightValue
		if value == nil {
			value = make([]byte, 8)
			binary.BigEndian.PutUint64(value, uint64(f.appHeight))
		}
		return &abci.ResponseQuery{Code: f.heightCode, Value: value}, nil
	}
	code := f.queryCode
	if f.codePath != "" && f.codePath != req.Path {
		code = 0
	}
	if req.Path == "chain_id" {
		return &abci.ResponseQuery{Code: code, Log: f.queryLog, Value: f.chainIDValue}, nil
	}
	var value []byte
	switch req.Path {
	case "account":
		value = f.accounts[string(req.Data)]
		if value == nil {
			value = []byte(`{"nonce":"0","balance":"0"}`)
		}
	case "eth_getStorageAt":
		value = f.storageValue
	case "eth_call", "eth_estimateGas":
		value = f.callValue
		if value == nil {
			var err error
			value, err = jsonx.Marshal(f.callResult)
			if err != nil {
				return nil, err
			}
		}
	}
	if req.Path == "account" || req.Path == "eth_getStorageAt" || req.Path == "eth_call" || req.Path == "eth_estimateGas" {
		return &abci.ResponseQuery{Code: code, Log: f.queryLog, Value: value}, nil
	}
	if req.Path != "gov_params" {
		return nil, fmt.Errorf("unexpected query %s", req.Path)
	}
	if req.Height > f.appHeight {
		return nil, fmt.Errorf("gov_params at height %d not yet committed", req.Height)
	}
	limit := map[int64]int{0: 400, 10: 100, 11: 200, 12: 400}[req.Height]
	return &abci.ResponseQuery{
		Code: code, Log: "fixture query",
		Value: []byte(fmt.Sprintf(`{"blockGasLimit":"%d","gasPrice":"48000000000"}`, limit)),
	}, nil
}

type ethRPCTransport struct{ listening bool }

func (ethRPCTransport) Listeners() []string    { return nil }
func (t ethRPCTransport) IsListening() bool    { return t.listening }
func (ethRPCTransport) NodeInfo() p2p.NodeInfo { return p2p.DefaultNodeInfo{} }

type ethRPCPeers struct {
	*p2p.Switch
	p2p.IPeerSet
	list []p2p.Peer
}

func (p *ethRPCPeers) Peers() p2p.IPeerSet { return p }
func (p *ethRPCPeers) List() []p2p.Peer    { return p.list }

func ethTestRPCHandler(t *testing.T) (http.Handler, *ethRPCFixture) {
	t.Helper()
	f := &ethRPCFixture{
		blocks:       make(map[int64]*coretypes.ResultBlock),
		accounts:     make(map[string][]byte),
		baseHeight:   10,
		blockHeight:  12,
		appHeight:    12,
		chainIDValue: []byte{0xbe, 0xa7, 0x01},
	}
	for height := int64(10); height <= 12; height++ {
		block := ethTestBlock(height)
		block.BlockID.Hash = bytes.Repeat([]byte{byte(height)}, 32)
		block.Block.Txs = tmtypes.Txs{ethEncodeTrx(t, ethTestTrx())}
		f.blocks[height] = block
	}
	f.env = &tmrpccore.Environment{
		Logger:     tmlog.NewNopLogger(),
		BlockStore: f, StateStore: f, ProxyAppQuery: f, P2PTransport: ethRPCTransport{},
		P2PPeers:         &ethRPCPeers{},
		ConsensusReactor: &consensus.Reactor{}, PubKey: secp256k1.PrivKey(bytes.Repeat([]byte{1}, 32)).PubKey(),
	}
	tmrpccore.SetEnvironment(f.env)
	t.Cleanup(func() { tmrpccore.SetEnvironment(nil) })
	handler, err := newEthRPCHandler(nil)
	require.NoError(t, err)
	return handler, f
}

func TestEthChainInfoQueries(t *testing.T) {
	for _, method := range []struct{ name, want string }{
		{"eth_chainId", `"0xbea701"`},
		{"net_version", `"12494593"`},
	} {
		for _, tt := range []struct {
			name  string
			code  uint32
			log   string
			err   error
			empty bool
			want  string
		}{
			{name: "success"},
			{name: "code with log", code: 7, log: "chain query rejected", want: "chain query rejected"},
			{name: "code without log", code: 7, want: "chain_id failed with code 7"},
			{name: "backend error", err: fmt.Errorf("chain query unavailable"), want: "chain query unavailable"},
			{name: "empty value", empty: true, want: "chain_id returned empty value"},
		} {
			t.Run(method.name+"/"+tt.name, func(t *testing.T) {
				handler, fixture := ethTestRPCHandler(t)
				fixture.queryCode, fixture.codePath, fixture.queryLog = tt.code, "chain_id", tt.log
				fixture.queryError = tt.err
				if tt.empty {
					fixture.chainIDValue = nil
				}
				response := ethServerRequest(t, handler, method.name, `[]`)
				if tt.want == "" {
					require.Nil(t, response.Error)
					require.Equal(t, method.want, string(response.Result))
				} else {
					require.NotNil(t, response.Error)
					require.Equal(t, -32000, response.Error.Code)
					require.Equal(t, tt.want, response.Error.Message)
					require.Empty(t, response.Result)
				}
				require.Equal(t, []abci.RequestQuery{{Path: "chain_id"}}, fixture.queries)
			})
		}
	}
}

func TestEthBlockChainIDErrors(t *testing.T) {
	for _, empty := range []bool{false, true} {
		t.Run(fmt.Sprintf("empty=%t", empty), func(t *testing.T) {
			handler, fixture := ethTestRPCHandler(t)
			fixture.queryCode, fixture.codePath, fixture.queryLog = 7, "chain_id", "chain query rejected"
			want := fixture.queryLog
			if empty {
				fixture.queryCode, fixture.chainIDValue = 0, nil
				want = "chain_id returned empty value"
			}
			hash := ethData(fixture.blocks[12].BlockID.Hash)
			for _, request := range []struct{ method, params string }{
				{"eth_getBlockByNumber", `["latest",true]`},
				{"eth_getBlockByHash", fmt.Sprintf(`[%q,true]`, hash)},
				{"eth_getTransactionByBlockNumberAndIndex", `["latest","0x0"]`},
				{"eth_getTransactionByBlockHashAndIndex", fmt.Sprintf(`[%q,"0x0"]`, hash)},
			} {
				t.Run(request.method, func(t *testing.T) {
					response := ethServerRequest(t, handler, request.method, request.params)
					require.NotNil(t, response.Error)
					require.Equal(t, -32000, response.Error.Code)
					require.Equal(t, want, response.Error.Message)
					require.Empty(t, response.Result)
				})
			}
		})
	}
}

func TestEthBlockRoutes(t *testing.T) {
	handler, f := ethTestRPCHandler(t)
	for _, tt := range []struct {
		tag    string
		height int64
	}{
		{"earliest", 10}, {"0xb", 11}, {"latest", 12}, {"pending", 12}, {"safe", 12}, {"finalized", 12},
	} {
		for _, details := range []bool{false, true} {
			f.queries = nil
			response := ethServerRequest(t, handler, "eth_getBlockByNumber", fmt.Sprintf(`["%s",%t]`, tt.tag, details))
			require.Empty(t, response.Error)
			var block map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(response.Result, &block))
			require.JSONEq(t, `"`+ethQuantityUint64(uint64(tt.height))+`"`, string(block["number"]))
			require.NotContains(t, block, "baseFeePerGas")
			require.Equal(t, "block_height", f.queries[0].Path)
			require.Equal(t, "gov_params", f.queries[1].Path)
			require.Equal(t, tt.height, f.queries[1].Height)
			require.False(t, f.queries[1].Prove)
			if details {
				var transactions []map[string]json.RawMessage
				require.NoError(t, json.Unmarshal(block["transactions"], &transactions))
				for _, tx := range transactions {
					require.Equal(t, block["timestamp"], tx["blockTimestamp"])
				}
				require.Contains(t, string(block["transactions"]), `"chainId":"0xbea701"`)
			} else {
				require.NotContains(t, string(block["transactions"]), `"chainId"`)
			}
		}
	}
	hash := "0x" + strings.Repeat("0B", 32)
	byHash := ethServerRequest(t, handler, "eth_getBlockByHash", `["`+hash+`",true]`)
	byNumber := ethServerRequest(t, handler, "eth_getBlockByNumber", `["0xb",true]`)
	require.Empty(t, byHash.Error)
	require.Empty(t, byNumber.Error)
	require.JSONEq(t, string(byNumber.Result), string(byHash.Result))
	for _, tag := range []string{"0x0", "0X0", "0x1", "0x9", "0xd", "0x7fffffffffffffff"} {
		response := ethServerRequest(t, handler, "eth_getBlockByNumber", `["`+tag+`",false]`)
		require.Empty(t, response.Error)
		require.Equal(t, "null", string(response.Result))
	}
	response := ethServerRequest(t, handler, "eth_getBlockByHash", `["0x`+strings.Repeat("ff", 32)+`",false]`)
	require.Empty(t, response.Error)
	require.Equal(t, "null", string(response.Result))
	delete(f.blocks, 11)
	response = ethServerRequest(t, handler, "eth_getBlockByNumber", `["0xb",true]`)
	require.Empty(t, response.Error)
	require.Equal(t, "null", string(response.Result))
}

func TestEthGasRoutesAndParams(t *testing.T) {
	handler, f := ethTestRPCHandler(t)
	response := ethServerRequest(t, handler, "eth_gasPrice", `[]`)
	require.Empty(t, response.Error)
	require.Equal(t, `"0xb2d05e000"`, string(response.Result))
	require.Equal(t, int64(0), f.queries[0].Height)
	response = ethServerRequest(t, handler, "eth_maxPriorityFeePerGas", `[]`)
	require.Empty(t, response.Error)
	require.Equal(t, `"0x0"`, string(response.Result))

	for _, params := range []string{
		`["0x4","latest",[0,50.5,100]]`, `[4,"latest",[0,50.5,100]]`,
	} {
		f.queries, f.resultReads = nil, nil
		response := ethServerRequest(t, handler, "eth_feeHistory", params)
		require.Empty(t, response.Error, params)
		require.JSONEq(t, `{"oldestBlock":"0xa","baseFeePerGas":["0x0","0x0","0x0","0x0"],"gasUsedRatio":[0.25,0.5,0.75],"reward":[["0x0","0x0","0x0"],["0x0","0x0","0x0"],["0x0","0x0","0x0"]]}`, string(response.Result))
		require.Equal(t, []int64{10, 11, 12}, f.resultReads)
		require.Len(t, f.queries, 4)
		require.Equal(t, "block_height", f.queries[0].Path)
		for i, req := range f.queries[1:] {
			require.Equal(t, "gov_params", req.Path)
			require.Equal(t, int64(10+i), req.Height)
		}
	}
	for _, params := range []string{
		`[4,"latest",null]`, `["0x4","latest"]`, `[4,"latest"]`, `[4,"latest",[]]`,
	} {
		response := ethServerRequest(t, handler, "eth_feeHistory", params)
		require.Empty(t, response.Error, params)
		require.JSONEq(t, `{"oldestBlock":"0xa","baseFeePerGas":["0x0","0x0","0x0","0x0"],"gasUsedRatio":[0.25,0.5,0.75]}`, string(response.Result))
	}
	response = ethServerRequest(t, handler, "eth_feeHistory", `[4,"latest",[25,75]]`)
	require.Empty(t, response.Error)
	require.Contains(t, string(response.Result), `"reward":[["0x0","0x0"],["0x0","0x0"],["0x0","0x0"]]`)
	for _, params := range []string{`[null,"latest",null]`, `[-1,"latest",null]`, `[4,"bad",null]`, `[4,"latest",[90,10]]`} {
		response := ethServerRequest(t, handler, "eth_feeHistory", params)
		require.NotNil(t, response.Error)
		require.Equal(t, -32602, response.Error.Code)
	}
}

func TestEthRPCBackendErrors(t *testing.T) {
	handler, f := ethTestRPCHandler(t)
	f.queryCode = 1
	for _, method := range []string{"eth_gasPrice", "eth_getBlockByNumber", "eth_feeHistory"} {
		params := map[string]string{"eth_gasPrice": `[]`, "eth_getBlockByNumber": `["latest",false]`, "eth_feeHistory": `[1,"latest",null]`}[method]
		response := ethServerRequest(t, handler, method, params)
		require.NotNil(t, response.Error)
		require.Equal(t, -32000, response.Error.Code)
		require.Contains(t, response.Error.Message, "gov_params")
		require.Empty(t, response.Result)
	}
	f.queryCode, f.queryError = 0, fmt.Errorf("query unavailable")
	response := ethServerRequest(t, handler, "eth_gasPrice", `[]`)
	require.NotNil(t, response.Error)
	require.Equal(t, -32000, response.Error.Code)
	require.Contains(t, response.Error.Message, "query unavailable")
	require.Empty(t, response.Result)
	f.queryError, f.resultsError = nil, fmt.Errorf("results unavailable")
	response = ethServerRequest(t, handler, "eth_getBlockByNumber", `["latest",false]`)
	require.NotNil(t, response.Error)
	require.Equal(t, -32000, response.Error.Code)
	require.Contains(t, response.Error.Message, "results unavailable")
	require.Empty(t, response.Result)
	response = ethServerRequest(t, handler, "eth_feeHistory", `[1,"latest",null]`)
	require.NotNil(t, response.Error)
	require.Equal(t, -32000, response.Error.Code)
	require.Contains(t, response.Error.Message, "results unavailable")
	require.Empty(t, response.Result)
}

func TestEthLatestUsesAppHeight(t *testing.T) {
	_, f := ethTestRPCHandler(t)
	f.appHeight = 11 // SaveBlock(12) has completed, but ApplyBlock(12) has not.
	handler, err := newEthRPCHandler(nil)
	require.NoError(t, err)
	response := ethServerRequest(t, handler, "eth_blockNumber", `[]`)
	require.Nil(t, response.Error)
	require.JSONEq(t, `"0xb"`, string(response.Result))

	for _, tag := range []string{"latest", "pending", "safe", "finalized", "earliest", "0xb"} {
		for _, details := range []bool{false, true} {
			f.resultReads = nil
			response := ethServerRequest(t, handler, "eth_getBlockByNumber", fmt.Sprintf(`[%q,%t]`, tag, details))
			require.Nil(t, response.Error)
			var block ethBlock
			require.NoError(t, json.Unmarshal(response.Result, &block))
			want := int64(11)
			if tag == "earliest" {
				want = 10
			}
			require.Equal(t, ethQuantityUint64(uint64(want)), block.Number)
			require.Equal(t, []int64{want}, f.resultReads)
		}
	}
	for _, request := range []struct{ method, params string }{
		{"eth_getBlockByNumber", `["0xc",true]`},
		{"eth_getBlockByNumber", `["0x0",true]`},
		{"eth_getBlockByHash", `["` + ethData(f.blocks[12].BlockID.Hash) + `",true]`},
	} {
		f.queries, f.resultReads = nil, nil
		response := ethServerRequest(t, handler, request.method, request.params)
		require.Nil(t, response.Error)
		require.Equal(t, "null", string(response.Result))
		require.Empty(t, f.resultReads)
		for _, query := range f.queries {
			require.Equal(t, "block_height", query.Path)
		}
	}
	for _, newest := range []string{"latest", "pending", "safe", "finalized", "0xb"} {
		f.queries, f.resultReads = nil, nil
		response := ethServerRequest(t, handler, "eth_feeHistory", fmt.Sprintf(`[2,%q]`, newest))
		require.Nil(t, response.Error)
		require.JSONEq(t, `{"oldestBlock":"0xa","baseFeePerGas":["0x0","0x0","0x0"],"gasUsedRatio":[0.25,0.5]}`, string(response.Result))
		require.Equal(t, []int64{10, 11}, f.resultReads)
		require.Len(t, f.queries, 3)
		require.Equal(t, "block_height", f.queries[0].Path)
		for i, query := range f.queries[1:] {
			require.Equal(t, "gov_params", query.Path)
			require.Equal(t, int64(10+i), query.Height)
		}
	}
	for _, newest := range []int64{12, 13} {
		f.queries, f.resultReads = nil, nil
		response := ethServerRequest(t, handler, "eth_feeHistory", fmt.Sprintf(`[2,%q]`, ethQuantityUint64(uint64(newest))))
		require.NotNil(t, response.Error)
		require.Equal(t, -32000, response.Error.Code)
		require.Equal(t, fmt.Sprintf("request beyond head block: requested %d, head 11", newest), response.Error.Message)
		require.Empty(t, response.Result)
		require.Empty(t, f.resultReads)
		require.Len(t, f.queries, 1)
		require.Equal(t, "block_height", f.queries[0].Path)
	}
	address := "0x" + strings.Repeat("ab", 20)
	for _, tag := range []string{"0x0", "0xc"} {
		f.queries = nil
		response := ethServerRequest(t, handler, "eth_getBalance", fmt.Sprintf(`[%q,%q]`, address, tag))
		require.NotNil(t, response.Error)
		require.Equal(t, -32000, response.Error.Code)
		require.Equal(t, "header not found", response.Error.Message)
		for _, query := range f.queries {
			require.Equal(t, "block_height", query.Path)
		}
	}
	response = ethServerRequest(t, handler, "eth_getBalance", fmt.Sprintf(`[%q,"0xb"]`, address))
	require.Nil(t, response.Error)
	require.Equal(t, int64(11), f.queries[len(f.queries)-1].Height)
}

func TestEthLatestHeightErrors(t *testing.T) {
	_, f := ethTestRPCHandler(t)
	for _, value := range [][]byte{{}, {0x01}, make([]byte, 9), bytes.Repeat([]byte{0xff}, 8)} {
		f.heightValue = value
		_, err := ethLatestHeight(ethTMContext())
		require.Error(t, err)
	}
	f.heightValue = nil
	f.heightCode = 7
	_, err := ethLatestHeight(ethTMContext())
	require.EqualError(t, err, "block_height failed with code 7")
	f.heightCode = 0
	f.queryError = fmt.Errorf("height unavailable")
	_, err = ethLatestHeight(ethTMContext())
	require.EqualError(t, err, "height unavailable")
}
