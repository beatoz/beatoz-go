package rpc

import (
	"encoding/json"
	"math/big"
	"runtime"

	"github.com/beatoz/beatoz-go/cmd/version"
	tmrpccore "github.com/tendermint/tendermint/rpc/core"
	tmrpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

func EthChainId(ctx *tmrpctypes.Context) (string, error) {
	if resp, err := tmrpccore.ABCIQuery(ctx, "chain_id", nil, 0, false); err != nil {
		return "", err
	} else {
		return ethQuantityBig(new(big.Int).SetBytes(resp.Response.Value))
	}
}

func EthGetBlockNumber(ctx *tmrpctypes.Context) (string, error) {
	if resp, err := tmrpccore.ABCIQuery(ctx, "block_height", nil, 0, false); err != nil {
		return "", err
	} else {
		return formatEthBlockNumber(resp.Response.Value)
	}
}

func NetVersion(ctx *tmrpctypes.Context) (string, error) {
	if resp, err := tmrpccore.ABCIQuery(ctx, "chain_id", nil, 0, false); err != nil {
		return "", err
	} else {
		return formatNetVersion(resp.Response.Value), nil
	}
}

func Web3ClientVersion(ctx *tmrpctypes.Context) (string, error) {
	return "beatoz/" + version.String() + "/" + runtime.GOOS + "-" + runtime.GOARCH + "/" + runtime.Version(), nil
}

func EthSyncing(ctx *tmrpctypes.Context) (json.RawMessage, error) {
	if resp, err := tmrpccore.Status(ctx); err != nil {
		return nil, err
	} else {
		return formatEthSyncing(resp.SyncInfo)
	}
}
