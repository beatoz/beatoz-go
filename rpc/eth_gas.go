package rpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	gethmath "github.com/ethereum/go-ethereum/common/math"
	tmrpccore "github.com/tendermint/tendermint/rpc/core"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	tmrpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

const ethMaxFeeHistoryBlocks = 1024

type ethGasParams struct {
	blockGasLimit uint64
	gasPrice      *big.Int
}

func ethGovParams(ctx *tmrpctypes.Context, height int64) (*ethGasParams, error) {
	resp, err := tmrpccore.ABCIQuery(ctx, "gov_params", nil, height, false)
	if err != nil {
		return nil, err
	}
	if resp.Response.Code != 0 {
		return nil, fmt.Errorf("gov_params at height %d: %s (code %d)", height, resp.Response.Log, resp.Response.Code)
	}
	return parseEthGasParams(resp.Response.Value)
}

func parseEthGasParams(value []byte) (*ethGasParams, error) {
	var params struct {
		BlockGasLimit string `json:"blockGasLimit"`
		GasPrice      string `json:"gasPrice"`
	}
	if err := json.Unmarshal(value, &params); err != nil {
		return nil, fmt.Errorf("invalid gov_params: %w", err)
	}
	for _, number := range []string{params.BlockGasLimit, params.GasPrice} {
		if number == "" || strings.IndexFunc(number, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
			return nil, fmt.Errorf("gov_params gas values must be non-negative decimal strings")
		}
	}
	limit, err := strconv.ParseUint(params.BlockGasLimit, 10, 64)
	if err != nil || limit == 0 {
		return nil, fmt.Errorf("invalid gov_params blockGasLimit %q", params.BlockGasLimit)
	}
	price, ok := new(big.Int).SetString(params.GasPrice, 10)
	if !ok {
		return nil, fmt.Errorf("invalid gov_params gasPrice %q", params.GasPrice)
	}
	return &ethGasParams{blockGasLimit: limit, gasPrice: price}, nil
}

func ethGasPrice(ctx *tmrpctypes.Context) (string, error) {
	params, err := ethGovParams(ctx, 0)
	if err != nil {
		return "", err
	}
	return ethQuantityBig(params.gasPrice)
}

func ethMaxPriorityFeePerGas(ctx *tmrpctypes.Context) (string, error) {
	return "0x0", nil
}

func ethFeeHistory(ctx *tmrpctypes.Context, blockCount gethmath.HexOrDecimal64, newestBlock ethBlockNumberParam, percentiles []float64) (*ethFeeHistoryResult, error) {
	if err := validateEthRewardPercentiles(percentiles); err != nil {
		return nil, err
	}
	if newestBlock.BlockNumber == 0 && !newestBlock.earliest {
		return nil, errors.New("header not found")
	}
	status, err := tmrpccore.Status(ctx)
	if err != nil {
		return nil, err
	}
	info := status.SyncInfo
	info.LatestBlockHeight, err = ethLatestHeight(ctx)
	if err != nil {
		return nil, err
	}
	if newestBlock.Int64() > info.LatestBlockHeight {
		return nil, fmt.Errorf("request beyond head block: requested %d, head %d", newestBlock.Int64(), info.LatestBlockHeight)
	}
	if newestBlock.BlockNumber > 0 && newestBlock.Int64() < info.EarliestBlockHeight {
		return nil, errors.New("header not found")
	}
	oldest, count := ethFeeHistoryRange(uint64(blockCount), newestBlock, info)
	blocks := make([]ethFeeHistoryBlock, 0, count)
	for i := uint64(0); i < count; i++ {
		h := oldest + int64(i)
		results, err := tmrpccore.BlockResults(ctx, &h)
		if err != nil {
			return nil, err
		}
		gasUsed, err := ethBlockGasUsed(results)
		if err != nil {
			return nil, err
		}
		params, err := ethGovParams(ctx, h)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, ethFeeHistoryBlock{gasUsed: gasUsed, gasLimit: params.blockGasLimit})
	}
	return formatEthFeeHistory(oldest, blocks, percentiles)
}

func validateEthRewardPercentiles(percentiles []float64) error {
	for i, p := range percentiles {
		if math.IsNaN(p) || math.IsInf(p, 0) || p < 0 || p > 100 || (i > 0 && p < percentiles[i-1]) {
			return ethInvalidParams("rewardPercentiles must be sorted numbers between 0 and 100")
		}
	}
	return nil
}

// ethFeeHistoryRange requires an end block validated against the stored range and app height.
func ethFeeHistoryRange(count uint64, blockNumber ethBlockNumberParam, info coretypes.SyncInfo) (int64, uint64) {
	if count > ethMaxFeeHistoryBlocks {
		count = ethMaxFeeHistoryBlocks
	}
	if count == 0 || info.EarliestBlockHeight <= 0 || info.LatestBlockHeight < info.EarliestBlockHeight {
		return 0, 0
	}
	newest := info.LatestBlockHeight
	if blockNumber.earliest {
		newest = info.EarliestBlockHeight
	} else if blockNumber.BlockNumber >= 0 {
		newest = blockNumber.Int64()
	}
	available := uint64(newest-info.EarliestBlockHeight) + 1
	if count > available {
		count = available
	}
	return newest - int64(count) + 1, count
}

type ethFeeHistoryBlock struct {
	gasUsed  uint64
	gasLimit uint64
}

func formatEthFeeHistory(oldest int64, blocks []ethFeeHistoryBlock, percentiles []float64) (*ethFeeHistoryResult, error) {
	oldestBlock, err := ethQuantityInt64(oldest)
	if err != nil {
		return nil, err
	}
	result := &ethFeeHistoryResult{
		OldestBlock: oldestBlock, BaseFeePerGas: make([]string, len(blocks)+1), GasUsedRatio: make([]float64, len(blocks)),
	}
	for i := range result.BaseFeePerGas {
		result.BaseFeePerGas[i] = "0x0"
	}
	if len(percentiles) > 0 {
		result.Reward = make([][]string, len(blocks))
	}
	for i, block := range blocks {
		if block.gasLimit == 0 {
			return nil, fmt.Errorf("zero gas limit at height %d", oldest+int64(i))
		}
		result.GasUsedRatio[i] = float64(block.gasUsed) / float64(block.gasLimit)
		if result.Reward != nil {
			row := make([]string, len(percentiles))
			for j := range row {
				row[j] = "0x0"
			}
			result.Reward[i] = row
		}
	}
	return result, nil
}
