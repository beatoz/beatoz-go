package rpc

import (
	abytes "github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	tmrpccore "github.com/tendermint/tendermint/rpc/core"
	tmrpccoretypes "github.com/tendermint/tendermint/rpc/core/types"
	tmrpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

func QueryAccount(ctx *tmrpctypes.Context, addr abytes.HexBytes, heightPtr *int64) (*QueryResult, error) {
	height := parseHeight(heightPtr)
	path := parsePath(ctx)
	if resp, err := tmrpccore.ABCIQuery(ctx, path, tmbytes.HexBytes(addr), height, false); err != nil {
		return nil, err
	} else {
		return &QueryResult{resp.Response}, nil
	}
}

func QueryDelegatee(ctx *tmrpctypes.Context, addr abytes.HexBytes, heightPtr *int64) (*QueryResult, error) {
	height := parseHeight(heightPtr)
	path := parsePath(ctx)
	if resp, err := tmrpccore.ABCIQuery(ctx, path, tmbytes.HexBytes(addr), height, false); err != nil {
		return nil, err
	} else {
		return &QueryResult{resp.Response}, nil
	}
}

func QueryStakes(ctx *tmrpctypes.Context, addr abytes.HexBytes, heightPtr *int64) (*QueryResult, error) {
	height := parseHeight(heightPtr)
	path := parsePath(ctx)
	if resp, err := tmrpccore.ABCIQuery(ctx, path, tmbytes.HexBytes(addr), height, false); err != nil {
		return nil, err
	} else {
		return &QueryResult{resp.Response}, nil
	}
}

func QueryStakes1(ctx *tmrpctypes.Context, heightPtr *int64) (*QueryResult, error) {
	height := parseHeight(heightPtr)
	path := parsePath(ctx)
	if resp, err := tmrpccore.ABCIQuery(ctx, path, nil, height, false); err != nil {
		return nil, err
	} else {
		return &QueryResult{resp.Response}, nil
	}
}

func QueryStakes2(ctx *tmrpctypes.Context, heightPtr *int64) (*QueryResult, error) {
	height := parseHeight(heightPtr)
	path := parsePath(ctx)
	if resp, err := tmrpccore.ABCIQuery(ctx, path, nil, height, false); err != nil {
		return nil, err
	} else {
		return &QueryResult{resp.Response}, nil
	}
}

func QueryReward(ctx *tmrpctypes.Context, addr abytes.HexBytes, heightPtr *int64) (*QueryResult, error) {
	height := parseHeight(heightPtr)
	path := parsePath(ctx)
	if resp, err := tmrpccore.ABCIQuery(ctx, path, tmbytes.HexBytes(addr), height, false); err != nil {
		return nil, err
	} else {
		return &QueryResult{resp.Response}, nil
	}
}

func QueryProposal(ctx *tmrpctypes.Context, txhash abytes.HexBytes, heightPtr *int64) (*QueryResult, error) {
	height := parseHeight(heightPtr)
	path := parsePath(ctx)
	if resp, err := tmrpccore.ABCIQuery(ctx, path, tmbytes.HexBytes(txhash), height, false); err != nil {
		return nil, err
	} else {
		return &QueryResult{resp.Response}, nil
	}
}

func QueryGovParams(ctx *tmrpctypes.Context, heightPtr *int64) (*QueryResult, error) {
	height := parseHeight(heightPtr)
	path := parsePath(ctx)
	if resp, err := tmrpccore.ABCIQuery(ctx, path, nil, height, false); err != nil {
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

	height := parseHeight(heightPtr)
	path := parsePath(ctx)
	if resp, err := tmrpccore.ABCIQuery(ctx, path, params, height, false); err != nil {
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

	height := parseHeight(heightPtr)
	path := parsePath(ctx)
	if resp, err := tmrpccore.ABCIQuery(ctx, path, params, height, false); err != nil {
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
	return tmrpccore.Subscribe(ctx, hexToUpper(query))
}

func Unsubscribe(ctx *tmrpctypes.Context, query string) (*tmrpccoretypes.ResultUnsubscribe, error) {
	return tmrpccore.Unsubscribe(ctx, hexToUpper(query))
}

func TxSearch(
	ctx *tmrpctypes.Context,
	query string,
	prove bool,
	pagePtr, perPagePtr *int,
	orderBy string,
) (*tmrpccoretypes.ResultTxSearch, error) {
	return tmrpccore.TxSearch(ctx, hexToUpper(query), prove, pagePtr, perPagePtr, orderBy)
}

func Validators(ctx *tmrpctypes.Context, heightPtr *int64, pagePtr, perPagePtr *int) (*tmrpccoretypes.ResultValidators, error) {
	if heightPtr != nil && *heightPtr == 0 {
		heightPtr = nil
	}
	return tmrpccore.Validators(ctx, heightPtr, pagePtr, perPagePtr)
}
