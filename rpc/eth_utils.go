package rpc

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"

	coretypes "github.com/tendermint/tendermint/rpc/core/types"
)

func ethQuantityUint64(n uint64) string {
	return "0x" + strconv.FormatUint(n, 16)
}

func ethQuantityInt64(n int64) (string, error) {
	if n < 0 {
		return "", fmt.Errorf("negative Ethereum quantity: %d", n)
	}
	return ethQuantityUint64(uint64(n)), nil
}

func ethQuantityBig(n *big.Int) (string, error) {
	if n == nil || n.Sign() < 0 {
		return "", fmt.Errorf("Ethereum quantity must be a non-negative integer")
	}
	return "0x" + n.Text(16), nil
}

func formatEthBlockNumber(value []byte) (string, error) {
	height, err := decodeEthBlockHeight(value)
	if err != nil {
		return "", err
	}
	return ethQuantityUint64(height), nil
}

func decodeEthBlockHeight(value []byte) (uint64, error) {
	if len(value) != 8 {
		return 0, fmt.Errorf("invalid block_height length: got %d, want 8", len(value))
	}
	return binary.BigEndian.Uint64(value), nil
}

func formatNetVersion(chainID []byte) string {
	return new(big.Int).SetBytes(chainID).Text(10)
}

func formatEthSyncing(info coretypes.SyncInfo) (json.RawMessage, error) {
	if !info.CatchingUp {
		return json.RawMessage("false"), nil
	}
	startingBlock, err := ethQuantityInt64(info.EarliestBlockHeight)
	if err != nil {
		return nil, err
	}
	currentBlock, err := ethQuantityInt64(info.LatestBlockHeight)
	if err != nil {
		return nil, err
	}
	return json.Marshal(&struct {
		StartingBlock string `json:"startingBlock"`
		CurrentBlock  string `json:"currentBlock"`
		HighestBlock  string `json:"highestBlock"`
	}{
		StartingBlock: startingBlock,
		CurrentBlock:  currentBlock,
		HighestBlock:  currentBlock,
	})
}
