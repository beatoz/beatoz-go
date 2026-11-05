package rpc

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	abci "github.com/tendermint/tendermint/abci/types"
	tmjson "github.com/tendermint/tendermint/libs/json"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	tmtypes "github.com/tendermint/tendermint/types"
)

func ethTestBlock(height int64) *coretypes.ResultBlock {
	return &coretypes.ResultBlock{
		BlockID: tmtypes.BlockID{Hash: bytes.Repeat([]byte{0xaa}, 32)},
		Block: &tmtypes.Block{Header: tmtypes.Header{
			Height: height, Time: time.Unix(16, 123).UTC(),
			AppHash: []byte{0x12, 0x34}, ProposerAddress: bytes.Repeat([]byte{0xab}, 20),
		}},
	}
}

func ethTestTrx() *ctrlertypes.Trx {
	sig := make([]byte, 65)
	sig[31], sig[63], sig[64] = 1, 2, 27
	return &ctrlertypes.Trx{
		Version: 1, Time: 16, Nonce: 15, From: bytes.Repeat([]byte{0xab}, 20), To: bytes.Repeat([]byte{0xcd}, 20),
		Amount: uint256.NewInt(256), Gas: 5000, GasPrice: uint256.NewInt(48000000000), Type: ctrlertypes.TRX_TRANSFER, Sig: sig,
	}
}

func ethEncodeTrx(t *testing.T, tx *ctrlertypes.Trx) []byte {
	t.Helper()
	bz, err := tx.Encode()
	require.NoError(t, err)
	return bz
}

func TestFormatEthBlockEmpty(t *testing.T) {
	block := ethTestBlock(1)
	pb, err := block.Block.ToProto()
	require.NoError(t, err)
	bz, err := pb.Marshal()
	require.NoError(t, err)
	zeroHash := "0x" + strings.Repeat("00", 32)
	for _, details := range []bool{false, true} {
		result, err := formatEthBlock(block, &coretypes.ResultBlockResults{Height: 1}, 120000000, "0xbea701", details)
		require.NoError(t, err)
		encoded, err := json.Marshal(result)
		require.NoError(t, err)
		want := map[string]interface{}{
			"number": "0x1", "hash": "0x" + strings.Repeat("aa", 32), "parentHash": zeroHash,
			"nonce": "0x0000000000000000", "sha3Uncles": "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
			"logsBloom": "0x" + strings.Repeat("00", 256), "transactionsRoot": zeroHash,
			"stateRoot": "0x" + strings.Repeat("00", 30) + "1234", "receiptsRoot": zeroHash,
			"miner": "0x" + strings.Repeat("ab", 20), "difficulty": "0x0", "totalDifficulty": "0x0",
			"extraData": "0x", "size": ethQuantityUint64(uint64(len(bz))), "gasLimit": "0x7270e00",
			"gasUsed": "0x0", "timestamp": "0x10", "transactions": []string{}, "uncles": []string{}, "mixHash": zeroHash,
		}
		wantJSON, err := json.Marshal(want)
		require.NoError(t, err)
		require.JSONEq(t, string(wantJSON), string(encoded))
		require.NotContains(t, string(encoded), "baseFeePerGas")
		// The actual route returns RawMessage, including its interface-valued transactions.
		raw := json.RawMessage(encoded)
		tmEncoded, err := tmjson.Marshal(&raw)
		require.NoError(t, err)
		require.JSONEq(t, string(wantJSON), string(tmEncoded))
	}
}

func TestFormatEthBlockTransactions(t *testing.T) {
	transfer, contract := ethTestTrx(), ethTestTrx()
	contract.Nonce = 16
	contract.Type = ctrlertypes.TRX_CONTRACT
	contract.Payload = &ctrlertypes.TrxPayloadContract{Data: []byte{0x60, 0x00, 0xab, 0xff}}
	block := ethTestBlock(42)
	block.Block.Txs = tmtypes.Txs{ethEncodeTrx(t, transfer), ethEncodeTrx(t, contract)}
	block.Block.LastBlockID.Hash = bytes.Repeat([]byte{0xbb}, 32)
	block.Block.DataHash = bytes.Repeat([]byte{0xcc}, 32)
	block.Block.LastResultsHash = bytes.Repeat([]byte{0xdd}, 32)
	block.Block.AppHash = append([]byte{0xff, 0xee}, bytes.Repeat([]byte{0x11}, 32)...)
	results := &coretypes.ResultBlockResults{Height: 42, TxsResults: []*abci.ResponseDeliverTx{{GasUsed: 1000}, {GasUsed: 2000}}}
	hashes := make([]string, 0, 2)
	for _, bz := range block.Block.Txs {
		hash := sha256.Sum256(bz)
		hashes = append(hashes, "0x"+hex.EncodeToString(hash[:]))
	}
	result, err := formatEthBlock(block, results, 120000000, "0xbea701", false)
	require.NoError(t, err)
	require.Equal(t, hashes, result.Transactions)
	require.Equal(t, "0xbb8", result.GasUsed)
	require.Equal(t, "0x"+strings.Repeat("bb", 32), result.ParentHash)
	require.Equal(t, "0x"+strings.Repeat("cc", 32), result.TransactionsRoot)
	require.Equal(t, "0x"+strings.Repeat("dd", 32), result.ReceiptsRoot)
	require.Equal(t, "0x"+strings.Repeat("11", 32), result.StateRoot)

	result, err = formatEthBlock(block, results, 120000000, "0xbea701", true)
	require.NoError(t, err)
	txs := result.Transactions.([]*EthTransactionResult)
	require.Len(t, txs, 2)
	to := "0x" + strings.Repeat("cd", 20)
	require.Equal(t, &EthTransactionResult{
		Hash: hashes[0], Nonce: "0xf", BlockHash: "0x" + strings.Repeat("aa", 32), BlockNumber: "0x2a", TransactionIndex: "0x0",
		From: "0x" + strings.Repeat("ab", 20), To: &to, Value: "0x100", Gas: "0x1388", GasPrice: "0xb2d05e000",
		Input: "0x", Type: "0x0", ChainID: "0xbea701", V: "0x1b", R: "0x1", S: "0x2",
	}, txs[0])
	require.Equal(t, hashes[1], txs[1].Hash)
	require.Equal(t, "0x1", txs[1].TransactionIndex)
	require.Equal(t, "0x10", txs[1].Nonce)
	require.Equal(t, "0x6000abff", txs[1].Input)
	require.Equal(t, &to, txs[1].To)
	encoded, err := json.Marshal(result)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"transactions":[{"hash":`)
	require.NotContains(t, string(encoded), "baseFeePerGas")
}

func TestFormatEthTransactionCreationAndSignature(t *testing.T) {
	for _, to := range [][]byte{nil, make([]byte, 20)} {
		for _, v := range []byte{0, 1, 27, 28} {
			tx := ethTestTrx()
			tx.To, tx.Type, tx.Payload, tx.Sig[64] = to, ctrlertypes.TRX_CONTRACT, &ctrlertypes.TrxPayloadContract{}, v
			result, err := formatEthTransaction(ethEncodeTrx(t, tx), nil, 1, 0, "0x1")
			require.NoError(t, err)
			require.Nil(t, result.To)
			require.Equal(t, ethQuantityUint64(uint64(v)), result.V)
			require.Equal(t, "0x", result.Input)
			encoded, err := json.Marshal(result)
			require.NoError(t, err)
			require.Contains(t, string(encoded), `"to":null`)
		}
	}
	// A transfer to the zero address is still a transfer.
	tx := ethTestTrx()
	tx.To = make([]byte, 20)
	tx.Amount = new(uint256.Int).SetAllOne()
	result, err := formatEthTransaction(ethEncodeTrx(t, tx), nil, 1, 0, "0x1")
	require.NoError(t, err)
	require.Equal(t, "0x"+strings.Repeat("00", 20), *result.To)
	require.Equal(t, "0x"+strings.Repeat("ff", 32), result.Value)
}

func TestEthConversionErrors(t *testing.T) {
	block := ethTestBlock(1)
	block.Block.Txs = tmtypes.Txs{{0xff}}
	results := &coretypes.ResultBlockResults{Height: 1, TxsResults: []*abci.ResponseDeliverTx{{GasUsed: 1}}}
	_, err := formatEthBlock(block, results, 100, "0x1", true)
	require.ErrorContains(t, err, "transaction 0: decode")
	for _, change := range []func(*ctrlertypes.Trx){
		func(tx *ctrlertypes.Trx) { tx.Sig = nil },
		func(tx *ctrlertypes.Trx) { tx.From = nil },
		func(tx *ctrlertypes.Trx) { tx.Nonce = -1 },
		func(tx *ctrlertypes.Trx) { tx.Gas = -1 },
	} {
		tx := ethTestTrx()
		change(tx)
		_, err := formatEthTransaction(ethEncodeTrx(t, tx), nil, 1, 0, "0x1")
		require.Error(t, err)
	}
	for _, results := range []*coretypes.ResultBlockResults{
		nil, {TxsResults: []*abci.ResponseDeliverTx{nil}}, {TxsResults: []*abci.ResponseDeliverTx{{GasUsed: -1}}},
		{TxsResults: []*abci.ResponseDeliverTx{{GasUsed: math.MaxInt64}, {GasUsed: math.MaxInt64}, {GasUsed: 2}}},
	} {
		_, err := ethBlockGasUsed(results)
		require.Error(t, err)
	}
	_, err = formatEthBlock(block, &coretypes.ResultBlockResults{Height: 1}, 100, "0x1", false)
	require.ErrorContains(t, err, "do not match")
}

func TestParseEthBlockHash(t *testing.T) {
	for _, hash := range []string{"0x" + strings.Repeat("ab", 32), "0x" + strings.Repeat("AB", 32)} {
		got, err := parseEthBlockHash(hash)
		require.NoError(t, err)
		require.Equal(t, bytes.Repeat([]byte{0xab}, 32), got)
	}
	for _, hash := range []string{"", "0x", "0xab", strings.Repeat("ab", 32), "0X" + strings.Repeat("ab", 32),
		"0x" + strings.Repeat("ab", 31), "0x" + strings.Repeat("ab", 33), "0x" + strings.Repeat("zz", 32)} {
		_, err := parseEthBlockHash(hash)
		require.Error(t, err)
		_, err = EthGetBlockByHash(nil, hash, false)
		require.Error(t, err)
	}
	for _, tag := range []string{"bad", "0x01", "0x-1", "0x8000000000000000"} {
		_, err := EthGetBlockByNumber(nil, tag, false)
		require.Error(t, err)
	}
}

func TestEthNullEncoding(t *testing.T) {
	for _, block := range []*coretypes.ResultBlock{nil, {}} {
		result, err := ethBlockResponse(nil, block, true)
		require.NoError(t, err)
		encoded, err := tmjson.Marshal(&result)
		require.NoError(t, err)
		require.Equal(t, "null", string(encoded))
	}
}
