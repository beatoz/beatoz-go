package rpc

import (
	abytes "github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	tmrpccore "github.com/tendermint/tendermint/rpc/core"
	tmrpccoretypes "github.com/tendermint/tendermint/rpc/core/types"
	tmrpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
	"regexp"
	"strings"
)

var hexReg = regexp.MustCompile(`(?i)[a-f0-9]{40,}`)

func adjustHeight(ctx *tmrpctypes.Context, heightPtr *int64) int64 {
	if heightPtr == nil {
		return 0
	}
	return *heightPtr
}

func QueryAccount(ctx *tmrpctypes.Context, addr abytes.HexBytes, heightPtr *int64) (*QueryResult, error) {
	height := adjustHeight(ctx, heightPtr)
	if resp, err := tmrpccore.ABCIQuery(ctx, "account", tmbytes.HexBytes(addr), height, false); err != nil {
		return nil, err
	} else {
		return &QueryResult{resp.Response}, nil
	}
}

func QueryDelegatee(ctx *tmrpctypes.Context, addr abytes.HexBytes, heightPtr *int64) (*QueryResult, error) {
	height := adjustHeight(ctx, heightPtr)
	if resp, err := tmrpccore.ABCIQuery(ctx, "delegatee", tmbytes.HexBytes(addr), height, false); err != nil {
		return nil, err
	} else {
		return &QueryResult{resp.Response}, nil
	}
}

func QueryStakes(ctx *tmrpctypes.Context, addr abytes.HexBytes, heightPtr *int64) (*QueryResult, error) {
	height := adjustHeight(ctx, heightPtr)
	if resp, err := tmrpccore.ABCIQuery(ctx, "stakes", tmbytes.HexBytes(addr), height, false); err != nil {
		return nil, err
	} else {
		return &QueryResult{resp.Response}, nil
	}
}

func QueryStakes1(ctx *tmrpctypes.Context, heightPtr *int64) (*QueryResult, error) {
	height := adjustHeight(ctx, heightPtr)
	if resp, err := tmrpccore.ABCIQuery(ctx, "stakes/total_power", nil, height, false); err != nil {
		return nil, err
	} else {
		return &QueryResult{resp.Response}, nil
	}
}

func QueryStakes2(ctx *tmrpctypes.Context, heightPtr *int64) (*QueryResult, error) {
	height := adjustHeight(ctx, heightPtr)
	if resp, err := tmrpccore.ABCIQuery(ctx, "stakes/voting_power", nil, height, false); err != nil {
		return nil, err
	} else {
		return &QueryResult{resp.Response}, nil
	}
}

func QueryReward(ctx *tmrpctypes.Context, addr abytes.HexBytes, heightPtr *int64) (*QueryResult, error) {
	height := adjustHeight(ctx, heightPtr)
	if resp, err := tmrpccore.ABCIQuery(ctx, "reward", tmbytes.HexBytes(addr), height, false); err != nil {
		return nil, err
	} else {
		return &QueryResult{resp.Response}, nil
	}
}

func QueryProposal(ctx *tmrpctypes.Context, txhash abytes.HexBytes, heightPtr *int64) (*QueryResult, error) {
	height := adjustHeight(ctx, heightPtr)
	if resp, err := tmrpccore.ABCIQuery(ctx, "proposal", tmbytes.HexBytes(txhash), height, false); err != nil {
		return nil, err
	} else {
		return &QueryResult{resp.Response}, nil
	}
}

func QueryGovParams(ctx *tmrpctypes.Context, heightPtr *int64) (*QueryResult, error) {
	height := adjustHeight(ctx, heightPtr)
	if resp, err := tmrpccore.ABCIQuery(ctx, "gov_params", nil, height, false); err != nil {
		return nil, err
	} else {
		return &QueryResult{resp.Response}, nil
	}
}

func QueryVM(
	ctx *tmrpctypes.Context,
	addr abytes.HexBytes,
	to abytes.HexBytes,
	heightPtr *int64,
	data []byte,
) (*QueryResult, error) {
	params := make([]byte, len(addr)+len(to)+len(data))
	copy(params, addr)
	copy(params[len(addr):], to)
	copy(params[len(addr)+len(to):], data)

	height := adjustHeight(ctx, heightPtr)
	if resp, err := tmrpccore.ABCIQuery(ctx, "vm_call", params, height, false); err != nil {
		return nil, err
	} else {
		return &QueryResult{resp.Response}, nil
	}
}

func QueryEstimateGas(
	ctx *tmrpctypes.Context,
	addr abytes.HexBytes,
	to abytes.HexBytes,
	heightPtr *int64,
	data []byte,
) (*QueryResult, error) {
	params := make([]byte, len(addr)+len(to)+len(data))
	copy(params, addr)
	copy(params[len(addr):], to)
	copy(params[len(addr)+len(to):], data)

	height := adjustHeight(ctx, heightPtr)
	if resp, err := tmrpccore.ABCIQuery(ctx, "vm_estimate_gas", params, height, false); err != nil {
		return nil, err
	} else {
		return &QueryResult{resp.Response}, nil
	}
}

func Subscribe(ctx *tmrpctypes.Context, query string) (*tmrpccoretypes.ResultSubscribe, error) {
	// return error when the event subscription request is received over http session.
	// related to: #103
	if ctx.WSConn == nil || ctx.JSONReq == nil {
		return nil, xerrors.NewOrdinary("error connection type: no websocket connection")
	}
	// make hex string like address or hash be uppercase
	//  address's size is 20bytes(40characters)
	//  hash's size is 32bytes(64characters)
	return tmrpccore.Subscribe(ctx, hexReg.ReplaceAllStringFunc(query, strings.ToUpper))
}

func Unsubscribe(ctx *tmrpctypes.Context, query string) (*tmrpccoretypes.ResultUnsubscribe, error) {
	return tmrpccore.Unsubscribe(ctx, hexReg.ReplaceAllStringFunc(query, strings.ToUpper))
}

func TxSearch(
	ctx *tmrpctypes.Context,
	query string,
	prove bool,
	pagePtr, perPagePtr *int,
	orderBy string,
) (*tmrpccoretypes.ResultTxSearch, error) {
	return tmrpccore.TxSearch(ctx, hexReg.ReplaceAllStringFunc(query, strings.ToUpper), prove, pagePtr, perPagePtr, orderBy)
}

func Validators(ctx *tmrpctypes.Context, heightPtr *int64, pagePtr, perPagePtr *int) (*tmrpccoretypes.ResultValidators, error) {
	if *heightPtr == 0 {
		heightPtr = nil
	}
	return tmrpccore.Validators(ctx, heightPtr, pagePtr, perPagePtr)
}
