package rpc

import (
	tmrpccore "github.com/tendermint/tendermint/rpc/core"
	tmrpccore_server "github.com/tendermint/tendermint/rpc/jsonrpc/server"
)

func AddEthRoutes() {
	tmrpccore.Routes["eth_chainId"] = tmrpccore_server.NewRPCFunc(EthChainId, "")
	tmrpccore.Routes["eth_blockNumber"] = tmrpccore_server.NewRPCFunc(EthGetBlockNumber, "")
	tmrpccore.Routes["net_version"] = tmrpccore_server.NewRPCFunc(NetVersion, "")
	tmrpccore.Routes["web3_clientVersion"] = tmrpccore_server.NewRPCFunc(Web3ClientVersion, "")
	tmrpccore.Routes["eth_syncing"] = tmrpccore_server.NewRPCFunc(EthSyncing, "")
	tmrpccore.Routes["eth_getBlockByNumber"] = tmrpccore_server.NewRPCFunc(EthGetBlockByNumber, "blockNumber, txDetails")
}
