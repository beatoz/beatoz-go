package rpc

import (
	"encoding/json"

	gethrpc "github.com/ethereum/go-ethereum/rpc"
	tmrpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

type ethAPI struct{}
type netAPI struct{}
type web3API struct{}

func ethTMContext() *tmrpctypes.Context {
	return &tmrpctypes.Context{}
}

func (*ethAPI) ChainId() (string, error) {
	return EthChainId(ethTMContext())
}

func (*ethAPI) BlockNumber() (string, error) {
	return EthGetBlockNumber(ethTMContext())
}

func (*ethAPI) Syncing() (json.RawMessage, error) {
	return EthSyncing(ethTMContext())
}

func (*ethAPI) GetBlockByNumber(blockNumber string, txDetails *bool) (json.RawMessage, error) {
	return EthGetBlockByNumber(ethTMContext(), blockNumber, txDetails != nil && *txDetails)
}

func (*ethAPI) GetBlockByHash(blockHash string, txDetails *bool) (json.RawMessage, error) {
	return EthGetBlockByHash(ethTMContext(), blockHash, txDetails != nil && *txDetails)
}

func (*ethAPI) GasPrice() (string, error) {
	return EthGasPrice(ethTMContext())
}

func (*ethAPI) MaxPriorityFeePerGas() (string, error) {
	return EthMaxPriorityFeePerGas(ethTMContext())
}

func (*ethAPI) FeeHistory(blockCount json.RawMessage, newestBlock string, rewardPercentiles *json.RawMessage) (*EthFeeHistoryResult, error) {
	var percentiles json.RawMessage
	if rewardPercentiles != nil {
		percentiles = *rewardPercentiles
	}
	return EthFeeHistory(ethTMContext(), blockCount, newestBlock, percentiles)
}

func (*netAPI) Version() (string, error) {
	return NetVersion(ethTMContext())
}

func (*web3API) ClientVersion() (string, error) {
	return Web3ClientVersion(ethTMContext())
}

func newEthRPCServer() (*gethrpc.Server, error) {
	server := gethrpc.NewServer()
	for _, service := range []struct {
		name string
		api  interface{}
	}{
		{"eth", &ethAPI{}},
		{"net", &netAPI{}},
		{"web3", &web3API{}},
	} {
		if err := server.RegisterName(service.name, service.api); err != nil {
			server.Stop()
			return nil, err
		}
	}
	return server, nil
}
