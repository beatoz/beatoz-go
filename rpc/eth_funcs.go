package rpc

import (
	"encoding/binary"
	tmrpccore "github.com/tendermint/tendermint/rpc/core"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	tmrpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
	"regexp"
	"strconv"
)

func EthChainId(ctx *tmrpctypes.Context) (string, error) {
	//if resp, err := tmrpccore.ABCIQuery(ctx, "chain_id", nil, 0, false); err != nil {
	//	return "", err
	//} else {
	//	return "0x" + hex.EncodeToString(resp.Response.Value), nil
	//}

	// todo: Implement chain id as number type
	// This chain ID encodes a modulo-based mapping onto 'beatoz'.
	return "0xbed83", nil
}

func EthGetBlockNumber(ctx *tmrpctypes.Context) (string, error) {
	if resp, err := tmrpccore.ABCIQuery(ctx, "block_height", nil, 0, false); err != nil {
		return "", err
	} else {
		return strconv.FormatInt(int64(binary.BigEndian.Uint64(resp.Response.Value)), 10), nil
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
