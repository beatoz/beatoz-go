package rpc

import (
	tmrpccore "github.com/tendermint/tendermint/rpc/core"
	tmrpccore_server "github.com/tendermint/tendermint/rpc/jsonrpc/server"
)

func AddEthRoutes() {
	tmrpccore.Routes["eth_chainId"] = tmrpccore_server.NewRPCFunc(EthChainId, "")
	tmrpccore.Routes["eth_blockNumber"] = tmrpccore_server.NewRPCFunc(EthBlockNumber, "")
	//tmrpccore.Routes["eth_getBlockByNumber"] = tmrpccore_server.NewRPCFunc(EthGetBlockByNumber, "blockNumber, txDetails")
	//tmrpccore.Routes["eth_getTransactionByHash"] = tmrpccore_server.NewRPCFunc(EthGetTxByHash, "txHash")
	//tmrpccore.Routes["eth_getTransactionReceipt"] = tmrpccore_server.NewRPCFunc(EthGetTransactionReceipt, "txHash")
	//tmrpccore.Routes["eth_getLogs"] = tmrpccore_server.NewRPCFunc(EthGetLogs, "fromBlock, toBlock, address, topics, blockHash")
	//tmrpccore.Routes["eth_getBlockByNumber"] = tmrpccore_server.NewRPCFunc(EthGetBlockByNumber, "blockNumber, txDetails")
	//tmrpccore.Routes["eth_getBlockByNumber"] = tmrpccore_server.NewRPCFunc(EthGetBlockByNumber, "blockNumber, txDetails")
	//tmrpccore.Routes["eth_getBlockByNumber"] = tmrpccore_server.NewRPCFunc(EthGetBlockByNumber, "blockNumber, txDetails")
	//tmrpccore.Routes["eth_getBlockByNumber"] = tmrpccore_server.NewRPCFunc(EthGetBlockByNumber, "blockNumber, txDetails")

}
