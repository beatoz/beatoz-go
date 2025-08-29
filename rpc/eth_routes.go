package rpc

import (
	tmrpccore "github.com/tendermint/tendermint/rpc/core"
	tmrpccore_server "github.com/tendermint/tendermint/rpc/jsonrpc/server"
)

func AddEthRoutes() {
	tmrpccore.Routes["eth_chainId"] = tmrpccore_server.NewRPCFunc(EthChainId, "")
	tmrpccore.Routes["eth_blockNumber"] = tmrpccore_server.NewRPCFunc(EthGetBlockNumber, "")
	tmrpccore.Routes["eth_getBlockByNumber"] = tmrpccore_server.NewRPCFunc(EthGetBlockByNumber, "blockNumber, txDetails")
}
