package rpc

import (
	"encoding/json"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
	tmrpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

type ethAPI struct{}
type netAPI struct{}
type web3API struct{}

const (
	ethBatchItemLimit         = 100
	ethBatchResponseSizeLimit = 10 * 1024 * 1024
)

func ethTMContext() *tmrpctypes.Context {
	return &tmrpctypes.Context{}
}

func (*ethAPI) ChainId() (string, error) {
	return ethChainID(ethTMContext())
}

func (*ethAPI) BlockNumber() (string, error) {
	return ethBlockNumber(ethTMContext())
}

func (*ethAPI) Syncing() (json.RawMessage, error) {
	return ethSyncing(ethTMContext())
}

func (*ethAPI) Accounts() []common.Address {
	return []common.Address{}
}

func (*ethAPI) GetBalance(address common.Address, blockTag *ethBlockNumberOrHashParam) (string, error) {
	return ethGetBalance(ethTMContext(), address, blockTag)
}

func (*ethAPI) GetTransactionCount(address common.Address, blockTag *ethBlockNumberOrHashParam) (string, error) {
	return ethGetTransactionCount(ethTMContext(), address, blockTag)
}

func (*ethAPI) GetCode(address common.Address, blockTag *ethBlockNumberOrHashParam) (string, error) {
	return ethGetCode(ethTMContext(), address, blockTag)
}

func (*ethAPI) GetStorageAt(address common.Address, slot string, blockTag *ethBlockNumberOrHashParam) (string, error) {
	return ethGetStorageAt(ethTMContext(), address, slot, blockTag)
}

func (*ethAPI) Call(args ethCallArgs, blockTag *ethBlockNumberOrHashParam) (string, error) {
	return ethCall(ethTMContext(), args, blockTag)
}

func (*ethAPI) EstimateGas(args ethCallArgs, blockTag *ethBlockNumberOrHashParam) (string, error) {
	return ethEstimateGas(ethTMContext(), args, blockTag)
}

func (*ethAPI) GetBlockByNumber(blockNumber ethBlockNumberParam, txDetails *bool) (json.RawMessage, error) {
	return ethGetBlockByNumber(ethTMContext(), blockNumber, txDetails != nil && *txDetails)
}

func (*ethAPI) GetBlockByHash(blockHash common.Hash, txDetails *bool) (json.RawMessage, error) {
	return ethGetBlockByHash(ethTMContext(), blockHash, txDetails != nil && *txDetails)
}

func (*ethAPI) GetBlockTransactionCountByNumber(blockNumber ethBlockNumberParam) (*string, error) {
	block, err := ethBlockByNumber(ethTMContext(), blockNumber)
	if err != nil {
		return nil, err
	}
	return ethBlockTransactionCount(block), nil
}

func (*ethAPI) GetBlockTransactionCountByHash(blockHash common.Hash) (*string, error) {
	block, err := ethBlockByHash(ethTMContext(), blockHash)
	if err != nil {
		return nil, err
	}
	return ethBlockTransactionCount(block), nil
}

func (*ethAPI) GetTransactionByBlockNumberAndIndex(blockNumber ethBlockNumberParam, index hexutil.Uint) (*ethTransaction, error) {
	ctx := ethTMContext()
	block, err := ethBlockByNumber(ctx, blockNumber)
	if err != nil {
		return nil, err
	}
	return ethBlockTransactionByIndex(ctx, block, uint64(index))
}

func (*ethAPI) GetTransactionByBlockHashAndIndex(blockHash common.Hash, index hexutil.Uint) (*ethTransaction, error) {
	ctx := ethTMContext()
	block, err := ethBlockByHash(ctx, blockHash)
	if err != nil {
		return nil, err
	}
	return ethBlockTransactionByIndex(ctx, block, uint64(index))
}

func (*ethAPI) GasPrice() (string, error) {
	return ethGasPrice(ethTMContext())
}

func (*ethAPI) MaxPriorityFeePerGas() (string, error) {
	return ethMaxPriorityFeePerGas(ethTMContext())
}

func (*ethAPI) FeeHistory(blockCount math.HexOrDecimal64, newestBlock ethBlockNumberParam, rewardPercentiles *[]float64) (*ethFeeHistoryResult, error) {
	var percentiles []float64
	if rewardPercentiles != nil {
		percentiles = *rewardPercentiles
	}
	return ethFeeHistory(ethTMContext(), blockCount, newestBlock, percentiles)
}

func (*netAPI) Version() (string, error) {
	return ethNetVersion(ethTMContext())
}

func (*netAPI) Listening() (bool, error) {
	return ethNetListening(ethTMContext())
}

func (*netAPI) PeerCount() (string, error) {
	return ethNetPeerCount(ethTMContext())
}

func (*web3API) ClientVersion() (string, error) {
	return ethClientVersion(ethTMContext())
}

func (*web3API) Sha3(input hexutil.Bytes) common.Hash {
	return crypto.Keccak256Hash(input)
}

func newEthRPCServer() (*gethrpc.Server, error) {
	server := gethrpc.NewServer()
	// Bound batch work and response memory; a standalone geth server has no limits by default.
	server.SetBatchLimits(ethBatchItemLimit, ethBatchResponseSizeLimit)
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
