package rpc

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"

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

// Latest tags return a nil height; earliest must be resolved against the block store.
func parseEthBlockTag(tag string) (height *int64, earliest bool, err error) {
	switch tag {
	case "latest", "pending", "safe", "finalized":
		return nil, false, nil
	case "earliest":
		return nil, true, nil
	}
	if !strings.HasPrefix(tag, "0x") || len(tag) < 3 || (len(tag) > 3 && tag[2] == '0') {
		return nil, false, ethInvalidParams("invalid Ethereum block tag %q: expected a tag or hex quantity", tag)
	}
	n, err := strconv.ParseUint(tag[2:], 16, 63)
	if err != nil {
		return nil, false, ethInvalidParams("invalid Ethereum block tag %q: %v", tag, err)
	}
	h := int64(n)
	return &h, false, nil
}

func formatEthBlockNumber(value []byte) (string, error) {
	if len(value) != 8 {
		return "", fmt.Errorf("invalid block_height length: got %d, want 8", len(value))
	}
	return ethQuantityUint64(binary.BigEndian.Uint64(value)), nil
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
