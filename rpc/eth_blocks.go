package rpc

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"strings"

	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	tmrpccore "github.com/tendermint/tendermint/rpc/core"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	tmrpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
	tmtypes "github.com/tendermint/tendermint/types"
)

const ethEmptyUncleHash = "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347"

func EthGetBlockByNumber(ctx *tmrpctypes.Context, blockNumber string, txDetails bool) (json.RawMessage, error) {
	height, earliest, err := parseEthBlockTag(blockNumber)
	if err != nil {
		return nil, err
	}
	status, err := tmrpccore.Status(ctx)
	if err != nil {
		return nil, err
	}
	h := status.SyncInfo.LatestBlockHeight
	if earliest {
		h = status.SyncInfo.EarliestBlockHeight
	} else if height != nil {
		h = *height
	}
	if h <= 0 || h < status.SyncInfo.EarliestBlockHeight || h > status.SyncInfo.LatestBlockHeight {
		return json.RawMessage("null"), nil
	}
	block, err := tmrpccore.Block(ctx, &h)
	if err != nil {
		// Pruning can advance the base between Status and Block.
		if current, statusErr := tmrpccore.Status(ctx); statusErr == nil && h < current.SyncInfo.EarliestBlockHeight {
			return json.RawMessage("null"), nil
		}
		return nil, err
	}
	return ethBlockResponse(ctx, block, txDetails)
}

func EthGetBlockByHash(ctx *tmrpctypes.Context, blockHash string, txDetails bool) (json.RawMessage, error) {
	hash, err := parseEthBlockHash(blockHash)
	if err != nil {
		return nil, err
	}
	block, err := tmrpccore.BlockByHash(ctx, hash)
	if err != nil {
		return nil, err
	}
	return ethBlockResponse(ctx, block, txDetails)
}

func ethBlockResponse(ctx *tmrpctypes.Context, block *coretypes.ResultBlock, txDetails bool) (json.RawMessage, error) {
	if block == nil || block.Block == nil {
		// A RawMessage keeps null intact through Tendermint's interface encoder.
		return json.RawMessage("null"), nil
	}
	height := block.Block.Height
	results, err := tmrpccore.BlockResults(ctx, &height)
	if err != nil {
		return nil, err
	}
	params, err := ethGovParams(ctx, height)
	if err != nil {
		return nil, err
	}
	var chainID string
	if txDetails && len(block.Block.Txs) > 0 {
		chainID, err = EthChainId(ctx)
		if err != nil {
			return nil, err
		}
	}
	result, err := formatEthBlock(block, results, params.blockGasLimit, chainID, txDetails)
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

func parseEthBlockHash(hash string) ([]byte, error) {
	if !strings.HasPrefix(hash, "0x") || len(hash) != 66 {
		return nil, ethInvalidParams("invalid Ethereum block hash: expected 0x-prefixed 32-byte hex")
	}
	bz, err := hex.DecodeString(hash[2:])
	if err != nil {
		return nil, ethInvalidParams("invalid Ethereum block hash: %v", err)
	}
	return bz, nil
}

func ethData(bz []byte) string {
	return "0x" + hex.EncodeToString(bz)
}

func ethFixedData(bz []byte, size int) string {
	data := make([]byte, size)
	if len(bz) > size {
		bz = bz[len(bz)-size:]
	}
	copy(data[size-len(bz):], bz)
	return ethData(data)
}

func ethBlockGasUsed(results *coretypes.ResultBlockResults) (uint64, error) {
	if results == nil {
		return 0, fmt.Errorf("missing block results")
	}
	var total uint64
	for i, result := range results.TxsResults {
		if result == nil || result.GasUsed < 0 {
			return 0, fmt.Errorf("invalid gas used for transaction %d", i)
		}
		gas := uint64(result.GasUsed)
		if gas > math.MaxUint64-total {
			return 0, fmt.Errorf("block gas used overflows uint64")
		}
		total += gas
	}
	return total, nil
}

func formatEthBlock(block *coretypes.ResultBlock, results *coretypes.ResultBlockResults, gasLimit uint64, chainID string, txDetails bool) (*EthBlockResult, error) {
	if block == nil || block.Block == nil {
		return nil, fmt.Errorf("missing block")
	}
	b := block.Block
	number, err := ethQuantityInt64(b.Height)
	if err != nil {
		return nil, err
	}
	timestamp, err := ethQuantityInt64(b.Time.Unix())
	if err != nil {
		return nil, err
	}
	gasUsed, err := ethBlockGasUsed(results)
	if err != nil {
		return nil, err
	}
	if results.Height != b.Height || len(results.TxsResults) != len(b.Txs) {
		return nil, fmt.Errorf("block results do not match block %d", b.Height)
	}
	pb, err := b.ToProto()
	if err != nil {
		return nil, err
	}
	result := &EthBlockResult{
		Number: number, Hash: ethFixedData(block.BlockID.Hash, 32),
		ParentHash: ethFixedData(b.LastBlockID.Hash, 32),
		Nonce:      "0x0000000000000000", Sha3Uncles: ethEmptyUncleHash,
		LogsBloom: ethFixedData(nil, 256), TransactionsRoot: ethFixedData(b.DataHash, 32),
		// AppHash is left-padded with zeros, or truncated to its rightmost 32 bytes.
		StateRoot: ethFixedData(b.AppHash, 32),
		// LastResultsHash commits to the previous height's ABCI results, not an Ethereum receipts trie.
		// An absent hash is represented by 32 zero bytes.
		ReceiptsRoot: ethFixedData(b.LastResultsHash, 32),
		Miner:        ethFixedData(b.ProposerAddress, 20), Difficulty: "0x0", TotalDifficulty: "0x0", ExtraData: "0x",
		// Size is the Tendermint protobuf block size, not Ethereum RLP size.
		Size:     ethQuantityUint64(uint64(pb.Size())),
		GasLimit: ethQuantityUint64(gasLimit), GasUsed: ethQuantityUint64(gasUsed), Timestamp: timestamp,
		Uncles: []string{}, MixHash: ethFixedData(nil, 32),
	}
	if txDetails {
		txs := make([]*EthTransactionResult, 0, len(b.Txs))
		for i, bz := range b.Txs {
			tx, err := formatEthTransaction(bz, block.BlockID.Hash, b.Height, uint64(i), chainID)
			if err != nil {
				return nil, fmt.Errorf("transaction %d: %w", i, err)
			}
			txs = append(txs, tx)
		}
		result.Transactions = txs
	} else {
		hashes := make([]string, 0, len(b.Txs))
		for _, bz := range b.Txs {
			hashes = append(hashes, ethData(bz.Hash()))
		}
		result.Transactions = hashes
	}
	// No baseFeePerGas: BEATOZ requires the exact governance gas price for legacy transactions.
	return result, nil
}

// formatEthTransaction converts an included BEATOZ transaction; chainID is an Ethereum quantity.
func formatEthTransaction(bz []byte, blockHash []byte, height int64, index uint64, chainID string) (*EthTransactionResult, error) {
	tx := &ctrlertypes.Trx{}
	if err := tx.Decode(bz); err != nil {
		return nil, fmt.Errorf("decode BEATOZ transaction: %w", err)
	}
	if len(tx.From) != types.AddrSize || (len(tx.To) != types.AddrSize && !(tx.Type == ctrlertypes.TRX_CONTRACT && len(tx.To) == 0)) {
		return nil, fmt.Errorf("invalid transaction address length")
	}
	if len(tx.Sig) != 65 {
		return nil, fmt.Errorf("invalid transaction signature length: got %d, want 65", len(tx.Sig))
	}
	nonce, err := ethQuantityInt64(tx.Nonce)
	if err != nil {
		return nil, err
	}
	gas, err := ethQuantityInt64(tx.Gas)
	if err != nil {
		return nil, err
	}
	blockNumber, err := ethQuantityInt64(height)
	if err != nil {
		return nil, err
	}
	value, err := ethQuantityBig(tx.Amount.ToBig())
	if err != nil {
		return nil, err
	}
	gasPrice, err := ethQuantityBig(tx.GasPrice.ToBig())
	if err != nil {
		return nil, err
	}
	to := ethData(tx.To)
	result := &EthTransactionResult{
		Hash: ethData(tmtypes.Tx(bz).Hash()), Nonce: nonce,
		BlockHash: ethFixedData(blockHash, 32), BlockNumber: blockNumber, TransactionIndex: ethQuantityUint64(index),
		From: ethData(tx.From), To: &to, Value: value, Gas: gas, GasPrice: gasPrice,
		Input: "0x", Type: "0x0", ChainID: chainID,
	}
	if tx.Type == ctrlertypes.TRX_CONTRACT {
		payload, ok := tx.Payload.(*ctrlertypes.TrxPayloadContract)
		if !ok || payload == nil {
			return nil, fmt.Errorf("missing contract payload")
		}
		result.Input = ethData(payload.Data)
		// The BEATOZ VM treats both an absent and an all-zero recipient as contract creation.
		if len(tx.To) == 0 || types.IsZeroAddress(tx.To) {
			result.To = nil
		}
	}
	// Sig is r[0:32] || s[32:64] || v[64]. Preserve BEATOZ's v (0/1 or 27/28),
	// without inventing EIP-155 protection for its non-Ethereum signing preimage.
	result.R, _ = ethQuantityBig(new(big.Int).SetBytes(tx.Sig[:32]))
	result.S, _ = ethQuantityBig(new(big.Int).SetBytes(tx.Sig[32:64]))
	result.V = ethQuantityUint64(uint64(tx.Sig[64]))
	return result, nil
}
