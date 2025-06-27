package rpc

import (
	"encoding/binary"
	"encoding/hex"
	tmrpccore "github.com/tendermint/tendermint/rpc/core"
	tmrpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
	"strconv"
)

func EthChainId(ctx *tmrpctypes.Context) (string, error) {
	if resp, err := tmrpccore.ABCIQuery(ctx, "chain_id", nil, 0, false); err != nil {
		return "", err
	} else {
		return "0x" + hex.EncodeToString(resp.Response.Value), nil
	}
}

func EthBlockNumber(ctx *tmrpctypes.Context) (string, error) {
	if resp, err := tmrpccore.ABCIQuery(ctx, "block_height", nil, 0, false); err != nil {
		return "", err
	} else {
		return strconv.FormatInt(int64(binary.BigEndian.Uint64(resp.Response.Value)), 10), nil
	}
}
