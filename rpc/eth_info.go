package rpc

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"runtime"

	"github.com/beatoz/beatoz-go/cmd/version"
	tmrpccore "github.com/tendermint/tendermint/rpc/core"
	tmrpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

func ethChainID(ctx *tmrpctypes.Context) (string, error) {
	value, err := ethABCIQuery(ctx, "chain_id", nil, 0)
	if err != nil {
		return "", err
	}
	if len(value) == 0 {
		return "", fmt.Errorf("chain_id returned empty value")
	}
	return ethQuantityBig(new(big.Int).SetBytes(value))
}

func ethBlockNumber(ctx *tmrpctypes.Context) (string, error) {
	value, err := ethABCIQuery(ctx, "block_height", nil, 0)
	if err != nil {
		return "", err
	}
	return formatEthBlockNumber(value)
}

// The block store can be one height ahead while the app is still applying it.
func ethLatestHeight(ctx *tmrpctypes.Context) (int64, error) {
	value, err := ethABCIQuery(ctx, "block_height", nil, 0)
	if err != nil {
		return 0, err
	}
	height, err := decodeEthBlockHeight(value)
	if err != nil {
		return 0, err
	}
	if height > math.MaxInt64 {
		return 0, fmt.Errorf("block_height overflows int64: %d", height)
	}
	return int64(height), nil
}

func ethNetVersion(ctx *tmrpctypes.Context) (string, error) {
	value, err := ethABCIQuery(ctx, "chain_id", nil, 0)
	if err != nil {
		return "", err
	}
	if len(value) == 0 {
		return "", fmt.Errorf("chain_id returned empty value")
	}
	return formatNetVersion(value), nil
}

func ethNetListening(ctx *tmrpctypes.Context) (bool, error) {
	info, err := tmrpccore.NetInfo(ctx)
	if err != nil {
		return false, err
	}
	return info.Listening, nil
}

func ethNetPeerCount(ctx *tmrpctypes.Context) (string, error) {
	info, err := tmrpccore.NetInfo(ctx)
	if err != nil {
		return "", err
	}
	return ethQuantityUint64(uint64(info.NPeers)), nil
}

func ethClientVersion(ctx *tmrpctypes.Context) (string, error) {
	return "beatoz/" + version.String() + "/" + runtime.GOOS + "-" + runtime.GOARCH + "/" + runtime.Version(), nil
}

func ethSyncing(ctx *tmrpctypes.Context) (json.RawMessage, error) {
	if resp, err := tmrpccore.Status(ctx); err != nil {
		return nil, err
	} else {
		return formatEthSyncing(resp.SyncInfo)
	}
}
