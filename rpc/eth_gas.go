package rpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

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

func EthGasPrice(ctx *tmrpctypes.Context) (string, error) {
	params, err := ethGovParams(ctx, 0)
	if err != nil {
		return "", err
	}
	return ethQuantityBig(params.gasPrice)
}

func EthMaxPriorityFeePerGas(ctx *tmrpctypes.Context) (string, error) {
	return "0x0", nil
}

// RawMessage accepts hex strings and JSON numbers, and preserves an explicit []
// for rewardPercentiles (tmjson decodes an empty float slice as nil). This fork
// requires all three positional parameters (use null or [] for no rewards), but
// allows rewardPercentiles to be omitted from named parameters.
func EthFeeHistory(ctx *tmrpctypes.Context, blockCount json.RawMessage, newestBlock string, rewardPercentiles json.RawMessage) (*EthFeeHistoryResult, error) {
	count, err := parseEthBlockCount(blockCount)
	if err != nil {
		return nil, err
	}
	height, earliest, err := parseEthBlockTag(newestBlock)
	if err != nil {
		return nil, err
	}
	percentiles, err := parseEthRewardPercentiles(rewardPercentiles)
	if err != nil {
		return nil, err
	}
	status, err := tmrpccore.Status(ctx)
	if err != nil {
		return nil, err
	}
	oldest, count := ethFeeHistoryRange(count, height, earliest, status.SyncInfo)
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

func parseEthBlockCount(raw json.RawMessage) (uint64, error) {
	raw = bytes.TrimSpace(raw)
	var number string
	base := 10
	if len(raw) > 0 && raw[0] == '"' {
		if err := json.Unmarshal(raw, &number); err != nil {
			return 0, ethInvalidParams("invalid blockCount: %v", err)
		}
		if !strings.HasPrefix(number, "0x") || len(number) < 3 || (len(number) > 3 && number[2] == '0') {
			return 0, ethInvalidParams("blockCount must be a hex quantity or JSON integer")
		}
		number, base = number[2:], 16
	} else {
		if !json.Valid(raw) {
			return 0, ethInvalidParams("invalid blockCount JSON")
		}
		number = string(raw)
	}
	if number == "" {
		return 0, ethInvalidParams("missing blockCount")
	}
	for _, r := range number {
		if (r >= '0' && r <= '9') || (base == 16 && ((r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F'))) {
			continue
		}
		return 0, ethInvalidParams("blockCount must be a non-negative integer")
	}
	n, ok := new(big.Int).SetString(number, base)
	if !ok {
		return 0, ethInvalidParams("invalid blockCount")
	}
	if n.Cmp(big.NewInt(ethMaxFeeHistoryBlocks)) > 0 {
		return ethMaxFeeHistoryBlocks, nil
	}
	return n.Uint64(), nil
}

func parseEthRewardPercentiles(raw json.RawMessage) ([]float64, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var values []*float64
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, ethInvalidParams("invalid rewardPercentiles: %v", err)
	}
	if values == nil {
		return nil, nil
	}
	percentiles := make([]float64, len(values))
	for i, value := range values {
		if value == nil {
			return nil, ethInvalidParams("rewardPercentiles cannot contain null")
		}
		percentiles[i] = *value
	}
	return percentiles, validateEthRewardPercentiles(percentiles)
}

func validateEthRewardPercentiles(percentiles []float64) error {
	for i, p := range percentiles {
		if math.IsNaN(p) || math.IsInf(p, 0) || p < 0 || p > 100 || (i > 0 && p < percentiles[i-1]) {
			return ethInvalidParams("rewardPercentiles must be sorted numbers between 0 and 100")
		}
	}
	return nil
}

func ethFeeHistoryRange(count uint64, height *int64, earliest bool, info coretypes.SyncInfo) (int64, uint64) {
	if count > ethMaxFeeHistoryBlocks {
		count = ethMaxFeeHistoryBlocks
	}
	if count == 0 || info.EarliestBlockHeight <= 0 || info.LatestBlockHeight < info.EarliestBlockHeight {
		return 0, 0
	}
	newest := info.LatestBlockHeight
	if earliest {
		newest = info.EarliestBlockHeight
	} else if height != nil && *height < newest {
		newest = *height
	}
	// Future heights start at latest; an entirely pruned range is empty.
	if newest < info.EarliestBlockHeight {
		return 0, 0
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

func formatEthFeeHistory(oldest int64, blocks []ethFeeHistoryBlock, percentiles []float64) (*EthFeeHistoryResult, error) {
	oldestBlock, err := ethQuantityInt64(oldest)
	if err != nil {
		return nil, err
	}
	if err := validateEthRewardPercentiles(percentiles); err != nil {
		return nil, err
	}
	result := &EthFeeHistoryResult{
		OldestBlock: oldestBlock, BaseFeePerGas: make([]string, len(blocks)+1), GasUsedRatio: make([]float64, len(blocks)),
	}
	for i := range result.BaseFeePerGas {
		result.BaseFeePerGas[i] = "0x0"
	}
	if percentiles != nil {
		reward := make([][]string, len(blocks))
		result.Reward = &reward
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
			(*result.Reward)[i] = row
		}
	}
	return result, nil
}
