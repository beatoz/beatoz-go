package rpc

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/stretchr/testify/require"
	tmtypes "github.com/tendermint/tendermint/types"
)

func TestEthBlockTransactionCounts(t *testing.T) {
	handler, fixture := ethTestRPCHandler(t)
	fixture.blocks[10].Block.Txs = nil
	fixture.blocks[11].Block.Txs = tmtypes.Txs{ethEncodeTrx(t, ethTestTrx()), {0xff}}
	fixture.blocks[12].Block.Txs = tmtypes.Txs{ethEncodeTrx(t, ethTestTrx()), {0xff}, {0xfe}}
	for _, tt := range []struct {
		tag, want string
	}{
		{"earliest", `"0x0"`}, {"0xa", `"0x0"`}, {"0xb", `"0x2"`},
		{"latest", `"0x3"`}, {"pending", `"0x3"`}, {"safe", `"0x3"`}, {"finalized", `"0x3"`},
		{"0x0", "null"}, {"0X0", "null"}, {"0x9", "null"}, {"0xd", "null"}, {"0x7fffffffffffffff", "null"},
	} {
		t.Run(tt.tag, func(t *testing.T) {
			response := ethServerRequest(t, handler, "eth_getBlockTransactionCountByNumber", fmt.Sprintf(`[%q]`, tt.tag))
			require.Nil(t, response.Error)
			require.Equal(t, tt.want, string(response.Result))
		})
	}
	for height := int64(10); height <= 12; height++ {
		block := fixture.blocks[height]
		response := ethServerRequest(t, handler, "eth_getBlockTransactionCountByHash", fmt.Sprintf(`[%q]`, ethData(block.BlockID.Hash)))
		require.Nil(t, response.Error)
		want := fmt.Sprintf(`%q`, ethQuantityUint64(uint64(len(block.Block.Txs))))
		require.Equal(t, want, string(response.Result))
	}
	response := ethServerRequest(t, handler, "eth_getBlockTransactionCountByHash", `["0x`+strings.Repeat("ff", 32)+`"]`)
	require.Nil(t, response.Error)
	require.Equal(t, "null", string(response.Result))
	delete(fixture.blocks, 11)
	response = ethServerRequest(t, handler, "eth_getBlockTransactionCountByNumber", `["0xb"]`)
	require.Nil(t, response.Error)
	require.Equal(t, "null", string(response.Result))
	require.Empty(t, fixture.resultReads)
	for _, query := range fixture.queries {
		require.Equal(t, "block_height", query.Path)
	}
}

func TestEthTransactionByBlockIndexMatchesFullBlock(t *testing.T) {
	handler, fixture := ethTestRPCHandler(t)
	transfer, contract := ethTestTrx(), ethTestTrx()
	contract.Type = ctrlertypes.TRX_CONTRACT
	contract.To = nil
	contract.Payload = &ctrlertypes.TrxPayloadContract{Data: []byte{0x60, 0x00}}
	fixture.blocks[12].Block.Txs = tmtypes.Txs{
		ethEncodeTrx(t, transfer), ethEncodeTrx(t, contract), {0xff}, ethEncodeTrx(t, transfer),
	}
	hash := ethData(fixture.blocks[12].BlockID.Hash)
	for _, tt := range []struct {
		blockMethod, txMethod, blockID string
	}{
		{"eth_getBlockByNumber", "eth_getTransactionByBlockNumberAndIndex", "latest"},
		{"eth_getBlockByHash", "eth_getTransactionByBlockHashAndIndex", hash},
	} {
		t.Run(tt.txMethod, func(t *testing.T) {
			response := ethServerRequest(t, handler, tt.blockMethod, fmt.Sprintf(`[%q,true]`, tt.blockID))
			require.Nil(t, response.Error)
			var block struct {
				Timestamp    string            `json:"timestamp"`
				Transactions []json.RawMessage `json:"transactions"`
			}
			require.NoError(t, json.Unmarshal(response.Result, &block))
			require.Len(t, block.Transactions, 4)
			for index, want := range block.Transactions {
				response := ethServerRequest(t, handler, tt.txMethod, fmt.Sprintf(`[%q,%q]`, tt.blockID, ethQuantityUint64(uint64(index))))
				require.Nil(t, response.Error)
				require.Equal(t, string(want), string(response.Result))
				var tx ethTransaction
				require.NoError(t, json.Unmarshal(response.Result, &tx))
				require.Equal(t, block.Timestamp, tx.BlockTimestamp)
				require.Equal(t, hash, tx.BlockHash)
				require.Equal(t, ethQuantityUint64(uint64(index)), tx.TransactionIndex)
				if index == 1 {
					require.Nil(t, tx.To)
					require.Equal(t, "0x6000", tx.Input)
				}
				if index == 2 {
					require.Equal(t, "0xff", tx.Input)
				}
			}
			for _, index := range []string{"0x4", "0x5", "0xffffffff"} {
				response := ethServerRequest(t, handler, tt.txMethod, fmt.Sprintf(`[%q,%q]`, tt.blockID, index))
				require.Nil(t, response.Error)
				require.Equal(t, "null", string(response.Result))
			}
		})
	}
	for _, tag := range []string{"earliest", "0xb", "latest", "pending", "safe", "finalized"} {
		response := ethServerRequest(t, handler, "eth_getBlockByNumber", fmt.Sprintf(`[%q,true]`, tag))
		require.Nil(t, response.Error)
		var block struct {
			Transactions []json.RawMessage `json:"transactions"`
		}
		require.NoError(t, json.Unmarshal(response.Result, &block))
		require.NotEmpty(t, block.Transactions)
		response = ethServerRequest(t, handler, "eth_getTransactionByBlockNumberAndIndex", fmt.Sprintf(`[%q,"0x0"]`, tag))
		require.Nil(t, response.Error)
		require.Equal(t, string(block.Transactions[0]), string(response.Result))
	}
}

func TestEthTransactionByBlockIndexMissing(t *testing.T) {
	handler, fixture := ethTestRPCHandler(t)
	fixture.blocks[10].Block.Txs = nil
	missingHash := ethData(fixture.blocks[11].BlockID.Hash)
	delete(fixture.blocks, 11)
	for _, tag := range []string{"earliest", "0xa", "0x0", "0x9", "0xb", "0xd"} {
		response := ethServerRequest(t, handler, "eth_getTransactionByBlockNumberAndIndex", fmt.Sprintf(`[%q,"0x0"]`, tag))
		require.Nil(t, response.Error)
		require.Equal(t, "null", string(response.Result))
	}
	for _, hash := range []string{ethData(fixture.blocks[10].BlockID.Hash), missingHash, "0x" + strings.Repeat("ff", 32)} {
		response := ethServerRequest(t, handler, "eth_getTransactionByBlockHashAndIndex", fmt.Sprintf(`[%q,"0x0"]`, hash))
		require.Nil(t, response.Error)
		require.Equal(t, "null", string(response.Result))
	}
	require.Empty(t, fixture.resultReads)
	for _, query := range fixture.queries {
		require.Equal(t, "block_height", query.Path)
	}
}

func TestEthBlockReadsUseAppHeight(t *testing.T) {
	handler, fixture := ethTestRPCHandler(t)
	fixture.appHeight = 11
	fixture.blocks[12].Block.Txs = append(fixture.blocks[12].Block.Txs, tmtypes.Tx{0xff})
	for _, tag := range []string{"latest", "pending", "safe", "finalized"} {
		response := ethServerRequest(t, handler, "eth_getBlockTransactionCountByNumber", fmt.Sprintf(`[%q]`, tag))
		require.Nil(t, response.Error)
		require.Equal(t, `"0x1"`, string(response.Result))
		response = ethServerRequest(t, handler, "eth_getTransactionByBlockNumberAndIndex", fmt.Sprintf(`[%q,"0x0"]`, tag))
		require.Nil(t, response.Error)
		var tx ethTransaction
		require.NoError(t, json.Unmarshal(response.Result, &tx))
		require.Equal(t, "0xb", tx.BlockNumber)
	}
	for _, tt := range []struct{ method, params string }{
		{"eth_getBlockTransactionCountByNumber", `["0xc"]`},
		{"eth_getBlockTransactionCountByHash", fmt.Sprintf(`[%q]`, ethData(fixture.blocks[12].BlockID.Hash))},
		{"eth_getTransactionByBlockNumberAndIndex", `["0xc","0x0"]`},
		{"eth_getTransactionByBlockHashAndIndex", fmt.Sprintf(`[%q,"0x0"]`, ethData(fixture.blocks[12].BlockID.Hash))},
	} {
		response := ethServerRequest(t, handler, tt.method, tt.params)
		require.Nil(t, response.Error)
		require.Equal(t, "null", string(response.Result))
	}
}

func TestEthBlockReadsInvalidParams(t *testing.T) {
	handler, fixture := ethTestRPCHandler(t)
	for _, tt := range []struct{ method, blockID string }{
		{"eth_getTransactionByBlockNumberAndIndex", "latest"},
		{"eth_getTransactionByBlockHashAndIndex", ethData(fixture.blocks[12].BlockID.Hash)},
	} {
		for _, index := range []string{`0`, `1`, `"1"`, `"0x"`, `"0x00"`, `"0xgg"`, `"-0x1"`, `"0x10000000000000000"`, `null`, `{}`, `true`} {
			t.Run(tt.method+"/"+index, func(t *testing.T) {
				response := ethServerRequest(t, handler, tt.method, fmt.Sprintf(`[%q,%s]`, tt.blockID, index))
				require.NotNil(t, response.Error)
				require.Equal(t, -32602, response.Error.Code)
				require.Empty(t, response.Result)
			})
		}
		response := ethServerRequest(t, handler, tt.method, fmt.Sprintf(`[%q]`, tt.blockID))
		require.NotNil(t, response.Error)
		require.Equal(t, -32602, response.Error.Code)
	}
	for _, method := range []string{
		"eth_getBlockTransactionCountByNumber", "eth_getBlockTransactionCountByHash",
		"eth_getTransactionByBlockNumberAndIndex", "eth_getTransactionByBlockHashAndIndex",
	} {
		for _, blockID := range []string{`"bad"`, `"0xzz"`, `null`, `{}`} {
			params := `[` + blockID + `]`
			if strings.Contains(method, "AndIndex") {
				params = `[` + blockID + `,"0x0"]`
			}
			response := ethServerRequest(t, handler, method, params)
			require.NotNil(t, response.Error)
			require.Equal(t, -32602, response.Error.Code)
		}
		response := ethServerRequest(t, handler, method, `[]`)
		require.NotNil(t, response.Error)
		require.Equal(t, -32602, response.Error.Code)
	}
	require.Empty(t, fixture.queries)
}

func TestEthBlockReadsBackendErrors(t *testing.T) {
	handler, fixture := ethTestRPCHandler(t)
	fixture.queryError = fmt.Errorf("height unavailable")
	for _, tt := range []struct{ method, params string }{
		{"eth_getBlockTransactionCountByNumber", `["latest"]`},
		{"eth_getBlockTransactionCountByHash", fmt.Sprintf(`[%q]`, ethData(fixture.blocks[12].BlockID.Hash))},
		{"eth_getTransactionByBlockNumberAndIndex", `["latest","0x0"]`},
		{"eth_getTransactionByBlockHashAndIndex", fmt.Sprintf(`[%q,"0x0"]`, ethData(fixture.blocks[12].BlockID.Hash))},
	} {
		response := ethServerRequest(t, handler, tt.method, tt.params)
		require.NotNil(t, response.Error)
		require.Equal(t, -32000, response.Error.Code)
		require.Contains(t, response.Error.Message, "height unavailable")
		require.Empty(t, response.Result)
	}
}
