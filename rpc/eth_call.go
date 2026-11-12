package rpc

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/libs/jsonx"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/vm"
	tmrpccore "github.com/tendermint/tendermint/rpc/core"
	tmrpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

type ethCallArgs struct {
	From                 *common.Address `json:"from"`
	To                   *common.Address `json:"to"`
	Gas                  *hexutil.Uint64 `json:"gas"`
	GasPrice             *hexutil.Big    `json:"gasPrice"`
	MaxFeePerGas         *hexutil.Big    `json:"maxFeePerGas"`
	MaxPriorityFeePerGas *hexutil.Big    `json:"maxPriorityFeePerGas"`
	Value                *hexutil.Big    `json:"value"`
	Data                 *hexutil.Bytes  `json:"data"`
	Input                *hexutil.Bytes  `json:"input"`
}

func (args ethCallArgs) queryData() ([]byte, error) {
	data := make([]byte, 116)
	for i, address := range []*common.Address{args.From, args.To} {
		if address != nil {
			copy(data[i*20:(i+1)*20], address[:])
		}
	}
	if args.Value != nil {
		args.Value.ToInt().FillBytes(data[40:72])
	}
	if args.Gas != nil {
		binary.BigEndian.PutUint64(data[72:80], uint64(*args.Gas))
	}
	// Fee fields are decoded for compatibility; BEATOZ queries execute at zero gas price.
	if args.Data != nil && args.Input != nil && !bytes.Equal(*args.Data, *args.Input) {
		return nil, ethInvalidParams("both \"data\" and \"input\" are set and not equal")
	}
	input := args.Input
	if input == nil {
		input = args.Data
	}
	if input != nil {
		data = append(data, *input...)
	}
	return data, nil
}

func ethQueryCall(ctx *tmrpctypes.Context, path string, args ethCallArgs, blockTag *ethBlockNumberOrHashParam) (*ctrlertypes.VMCallResult, error) {
	data, err := args.queryData()
	if err != nil {
		return nil, err
	}
	height, err := ethQueryHeight(ctx, blockTag)
	if err != nil {
		return nil, err
	}
	if height == 0 {
		height, err = ethLatestHeight(ctx)
		if err != nil {
			return nil, err
		}
	}
	if height <= 0 {
		return nil, errors.New("header not found")
	}
	info, err := tmrpccore.BlockchainInfo(ctx, height, height)
	if err != nil || info == nil || len(info.BlockMetas) != 1 || info.BlockMetas[0] == nil || info.BlockMetas[0].Header.Height != height {
		return nil, errors.New("header not found")
	}
	header := info.BlockMetas[0].Header
	params, err := ethGovParams(ctx, height)
	if err != nil {
		return nil, err
	}
	binary.BigEndian.PutUint64(data[80:88], uint64(header.Time.Unix()))
	coinbase := common.BytesToAddress(header.ProposerAddress)
	copy(data[88:108], coinbase[:])
	binary.BigEndian.PutUint64(data[108:116], params.blockGasLimit)
	value, err := ethABCIQuery(ctx, path, data, height)
	if err != nil {
		return nil, err
	}
	var result ctrlertypes.VMCallResult
	if err := jsonx.Unmarshal(value, &result); err != nil {
		return nil, err
	}
	if result.Err == vm.ErrExecutionReverted.Error() {
		message := result.Err
		if reason, err := abi.UnpackRevert(result.ReturnData); err == nil {
			message += ": " + reason
		}
		return nil, &ethRPCError{code: 3, message: message, data: ethData(result.ReturnData)}
	}
	if result.Err != "" {
		return nil, errors.New(result.Err)
	}
	return &result, nil
}

func ethCall(ctx *tmrpctypes.Context, args ethCallArgs, blockTag *ethBlockNumberOrHashParam) (string, error) {
	result, err := ethQueryCall(ctx, "eth_call", args, blockTag)
	if err != nil {
		return "", err
	}
	return ethData(result.ReturnData), nil
}

// ethEstimateGas uses the eth_estimateGas ABCI query's geth-style estimate: peak gas
// (used + refunded) with the 63/64 rule, then binary search within 32 total executions
// and one shared query timeout. Only legacy vm_estimate_gas returns single-run usage.
func ethEstimateGas(ctx *tmrpctypes.Context, args ethCallArgs, blockTag *ethBlockNumberOrHashParam) (string, error) {
	result, err := ethQueryCall(ctx, "eth_estimateGas", args, blockTag)
	if err != nil {
		return "", err
	}
	if result.UsedGas < 0 {
		return "", fmt.Errorf("invalid negative gas used: %d", result.UsedGas)
	}
	return ethQuantityUint64(uint64(result.UsedGas)), nil
}
