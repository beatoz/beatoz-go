package rpc

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	gethrpc "github.com/ethereum/go-ethereum/rpc"
	tmrpccore "github.com/tendermint/tendermint/rpc/core"
	tmrpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

// go-ethereum decodes both the earliest tag and numeric zero to zero.
// This chain has no block 0, so the tag must be remembered.
type ethBlockNumberParam struct {
	gethrpc.BlockNumber
	earliest bool
}

func (number *ethBlockNumberParam) UnmarshalJSON(data []byte) error {
	var decoded gethrpc.BlockNumber
	if err := decoded.UnmarshalJSON(data); err != nil {
		return err
	}
	*number = ethBlockNumberParam{
		BlockNumber: decoded,
		earliest:    strings.Trim(strings.TrimSpace(string(data)), `"`) == "earliest",
	}
	return nil
}

type ethBlockNumberOrHashParam struct {
	gethrpc.BlockNumberOrHash
	earliest bool
}

func (block *ethBlockNumberOrHashParam) UnmarshalJSON(data []byte) error {
	var decoded gethrpc.BlockNumberOrHash
	if err := decoded.UnmarshalJSON(data); err != nil {
		return err
	}
	earliest := false
	if number, ok := decoded.Number(); ok && number == 0 {
		var tag string
		if json.Unmarshal(data, &tag) != nil {
			var object struct {
				BlockNumber string `json:"blockNumber"`
			}
			// The number is already validated; only its original spelling is needed.
			_ = json.Unmarshal(data, &object)
			tag = object.BlockNumber
		}
		earliest = tag == "earliest"
	}
	*block = ethBlockNumberOrHashParam{BlockNumberOrHash: decoded, earliest: earliest}
	return nil
}

// Storage keys require a hex prefix and allow an odd number of hex digits.
func parseEthStorageSlot(s string) ([]byte, error) {
	if !strings.HasPrefix(s, "0x") && !strings.HasPrefix(s, "0X") {
		return nil, ethInvalidParams("storage slot must be a 0x-prefixed hex string")
	}
	s = s[2:]
	if len(s)%2 != 0 {
		s = "0" + s
	}
	data, err := hex.DecodeString(s)
	if err != nil {
		return nil, ethInvalidParams("hex string invalid")
	}
	if len(data) > 32 {
		return nil, ethInvalidParams("hex string too long, want at most 32 bytes")
	}
	slot := make([]byte, 32)
	copy(slot[len(slot)-len(data):], data)
	return slot, nil
}

func ethQueryHeight(ctx *tmrpctypes.Context, tag *ethBlockNumberOrHashParam) (int64, error) {
	if tag == nil {
		return 0, nil
	}
	var height int64
	if number, ok := tag.Number(); ok {
		switch number {
		case gethrpc.LatestBlockNumber, gethrpc.PendingBlockNumber, gethrpc.SafeBlockNumber, gethrpc.FinalizedBlockNumber:
			return 0, nil
		case gethrpc.EarliestBlockNumber:
			if !tag.earliest {
				return 0, errors.New("header not found")
			}
			status, err := tmrpccore.Status(ctx)
			if err != nil {
				return 0, err
			}
			height = status.SyncInfo.EarliestBlockHeight
		default:
			height = int64(number)
		}
	} else if hash, ok := tag.Hash(); ok {
		block, err := tmrpccore.BlockByHash(ctx, hash[:])
		if err != nil {
			return 0, err
		}
		if block == nil || block.Block == nil {
			return 0, errors.New("header for hash not found")
		}
		// Every stored block is canonical on this chain.
		height = block.Block.Height
	} else {
		return 0, ethInvalidParams("invalid arguments; neither block nor hash specified")
	}
	if height <= 0 {
		return 0, errors.New("header not found")
	}
	latest, err := ethLatestHeight(ctx)
	if err != nil {
		return 0, err
	}
	if height > latest {
		return 0, errors.New("header not found")
	}
	return height, nil
}

func ethABCIQuery(ctx *tmrpctypes.Context, path string, data []byte, height int64) ([]byte, error) {
	resp, err := tmrpccore.ABCIQuery(ctx, path, data, height, false)
	if err != nil {
		return nil, err
	}
	if resp.Response.Code != 0 {
		if resp.Response.Log != "" {
			return nil, errors.New(resp.Response.Log)
		}
		return nil, fmt.Errorf("%s failed with code %d", path, resp.Response.Code)
	}
	return resp.Response.Value, nil
}
