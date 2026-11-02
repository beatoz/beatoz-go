package rpc

import (
	"encoding/json"
	"math/big"
	"regexp"
	"runtime"
	"strconv"

	"github.com/beatoz/beatoz-go/cmd/version"
	tmrpccore "github.com/tendermint/tendermint/rpc/core"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
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

var ethAddrReg = regexp.MustCompile(`^0x(?i)[a-f0-9]$`)

func EthGetBlockByNumber(ctx *tmrpctypes.Context, number string, txDetail bool) (*coretypes.ResultBlock, error) {
	ptrHeight := new(int64)
	*ptrHeight = int64(0) // latest block
	if ethAddrReg.MatchString(number) {
		h, err := strconv.ParseInt(number, 16, 64)
		if err != nil {
			return nil, err
		}
		*ptrHeight = h
	}
	if ptrHeight != nil && *ptrHeight == 0 {
		ptrHeight = nil
	}
	return tmrpccore.Block(ctx, ptrHeight)
}
