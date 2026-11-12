package evm

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/big"

	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/libs/jsonx"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

func (ctrler *EVMCtrler) QueryEth(req abcitypes.RequestQuery) ([]byte, xerrors.XError) {
	gasOffset := types.AddrSize*2 + common.HashLength
	timeOffset := gasOffset + 8
	coinbaseOffset := timeOffset + 8
	blockGasOffset := coinbaseOffset + types.AddrSize
	dataOffset := blockGasOffset + 8
	if len(req.Data) < dataOffset {
		return nil, xerrors.From(fmt.Errorf("invalid %s query data length: %d", req.Path, len(req.Data)))
	}
	blockGasLimit := binary.BigEndian.Uint64(req.Data[blockGasOffset:dataOffset])
	if blockGasLimit == 0 || blockGasLimit > math.MaxInt64 {
		return nil, xerrors.From(fmt.Errorf("invalid block gas limit: %d", blockGasLimit))
	}
	value := new(big.Int).SetBytes(req.Data[types.AddrSize*2 : gasOffset])
	gasLimit := binary.BigEndian.Uint64(req.Data[gasOffset:timeOffset])
	blockTime := int64(binary.BigEndian.Uint64(req.Data[timeOffset:coinbaseOffset]))
	coinbase := common.BytesToAddress(req.Data[coinbaseOffset:blockGasOffset])
	from := req.Data[:types.AddrSize]
	to := req.Data[types.AddrSize : types.AddrSize*2]
	data := req.Data[dataOffset:]
	height := req.Height

	if height <= 0 {
		height = ctrler.lastBlockHeight
	}

	var (
		gasUsed uint64
		execRet *core.ExecutionResult
		xerr    xerrors.XError
	)
	if req.Path == "eth_estimateGas" {
		gasUsed, execRet, xerr = ctrler.estimateGas(from, to, value, data, height, blockTime, gasLimit, int64(blockGasLimit), coinbase)
	} else {
		if gasLimit == 0 || gasLimit > blockGasLimit {
			gasLimit = blockGasLimit
		}
		execRet, xerr = ctrler.ethCallVM(context.Background(), from, to, value, data, height, blockTime, gasLimit, int64(blockGasLimit), coinbase)
		if xerr == nil {
			gasUsed = execRet.UsedGas
		}
	}
	if xerr != nil {
		return nil, xerr
	}

	vmCallRet := &ctrlertypes.VMCallResult{
		UsedGas: int64(gasUsed),
	}
	if execRet.Err != nil {
		vmCallRet.Err = execRet.Err.Error()
		vmCallRet.ReturnData = execRet.Revert()
	} else if req.Path == "eth_call" {
		vmCallRet.ReturnData = execRet.Return()
	}

	retbz, err := jsonx.Marshal(vmCallRet)
	if err != nil {
		return nil, xerrors.From(err)
	}

	return retbz, nil
}

func (ctrler *EVMCtrler) QueryStorageAt(req abcitypes.RequestQuery) ([]byte, xerrors.XError) {
	if len(req.Data) != types.AddrSize+common.HashLength {
		return nil, xerrors.From(fmt.Errorf("invalid eth_getStorageAt query data length: %d", len(req.Data)))
	}

	addr := types.Address(req.Data[:types.AddrSize])
	slot := common.BytesToHash(req.Data[types.AddrSize:])
	value, xerr := ctrler.GetStorageAt(addr, slot, req.Height)
	if xerr != nil {
		return nil, xerr
	}
	return value.Bytes(), nil
}

func (ctrler *EVMCtrler) GetStorageAt(addr types.Address, slot common.Hash, height int64) (common.Hash, xerrors.XError) {
	state, xerr := ctrler.MemStateAt(height)
	if xerr != nil {
		return common.Hash{}, xerr
	}

	return state.GetState(addr.Array20(), slot), nil
}

const (
	estimateGasErrorRatio    = 0.015
	estimateGasMaxExecutions = 32
)

func (ctrler *EVMCtrler) estimateGas(from, to types.Address, value *big.Int, data []byte, height, blockTime int64, gasLimit uint64, blockGasLimit int64, coinbase common.Address) (uint64, *core.ExecutionResult, xerrors.XError) {
	// All trials share one query deadline because ABCI queries also block consensus.
	ctx, cancel := context.WithTimeout(context.Background(), ctrler.queryTimeout)
	defer cancel()

	hi := uint64(blockGasLimit)
	if gasLimit >= params.TxGas && gasLimit < hi {
		hi = gasLimit
	}
	initialHi := hi
	result, xerr := ctrler.ethCallVM(ctx, from, to, value, data, height, blockTime, hi, blockGasLimit, coinbase)
	if xerr != nil || result.Err != nil {
		if xerr == nil && (errors.Is(result.Err, vm.ErrOutOfGas) || errors.Is(result.Err, core.ErrIntrinsicGas)) {
			result.Err = fmt.Errorf("gas required exceeds allowance (%d)", hi)
		}
		return 0, result, xerr
	}
	executions := 1
	peakGas := result.UsedGas + result.RefundedGas
	lo := peakGas - 1

	// Include refunds and the 63/64 forwarding rule, as in geth's gas estimator.
	// Divide before multiplying to avoid overflow for very large block gas limits.
	optimisticGas := peakGas + params.CallStipend
	optimisticGas = optimisticGas/63*64 + optimisticGas%63*64/63
	if optimisticGas < hi {
		trial, xerr := ctrler.ethCallVM(ctx, from, to, value, data, height, blockTime, optimisticGas, blockGasLimit, coinbase)
		executions++
		if xerr != nil {
			return 0, nil, xerr
		}
		if trial.Err != nil {
			lo = optimisticGas
		} else {
			hi = optimisticGas
		}
	}
	for lo+1 < hi && float64(hi-lo)/float64(hi) > estimateGasErrorRatio && executions < estimateGasMaxExecutions {
		mid := (hi + lo) / 2
		if mid > lo*2 {
			mid = lo * 2
		}
		trial, xerr := ctrler.ethCallVM(ctx, from, to, value, data, height, blockTime, mid, blockGasLimit, coinbase)
		executions++
		if xerr != nil {
			if hi < initialHi && xerr.Cause() == context.DeadlineExceeded {
				// A successful trial has already narrowed the sufficient gas limit.
				return hi, result, nil
			}
			return 0, nil, xerr
		}
		if trial.Err != nil {
			lo = mid
		} else {
			hi = mid
		}
	}
	return hi, result, nil
}

// VM setup is intentionally duplicated so Ethereum and legacy queries can evolve independently.
func (ctrler *EVMCtrler) ethCallVM(parent context.Context, from, to types.Address, value *big.Int, data []byte, height, blockTime int64, gasLimit uint64, blockGasLimit int64, coinbase common.Address) (*core.ExecutionResult, xerrors.XError) {
	ctx, cancel := context.WithTimeout(parent, ctrler.queryTimeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, xerrors.ErrQuery.Wrap(err)
	}

	// Get the stateDB at block<height> and the `stateDBWrapper` that has account ledger(acctCtrler)
	state, xerr := ctrler.MemStateAt(height)
	if xerr != nil {
		return nil, xerr
	}

	state.Initiate(nil, 0, from, to, 0, false)
	defer func() { state = nil }()

	var sender common.Address
	var toAddr *common.Address
	copy(sender[:], from)
	if to != nil &&
		!types.IsZeroAddress(to) {
		toAddr = new(common.Address)
		copy(toAddr[:], to)
	}

	vmmsg := &core.Message{
		From:              sender,
		To:                toAddr,
		Value:             value,
		GasLimit:          gasLimit,
		GasPrice:          new(big.Int),
		GasFeeCap:         new(big.Int),
		GasTipCap:         new(big.Int),
		Data:              data,
		AccessList:        nil,
		SkipAccountChecks: true,
	}

	blockContext := evmBlockContext(coinbase, blockGasLimit, height, blockTime, ctrler.blockHashProvider(height))

	txContext := core.NewEVMTxContext(vmmsg)
	vmevm := vm.NewEVM(blockContext, txContext, state, ctrler.ethChainConfig, vm.Config{NoBaseFee: true})

	gp := new(core.GasPool).AddGas(blockContext.GasLimit)
	if err := ctx.Err(); err != nil {
		return nil, xerrors.ErrQuery.Wrap(err)
	}

	clearTimeoutCallback := context.AfterFunc(ctx, vmevm.Cancel)
	result, err := NewVMStateTransition(vmevm, vmmsg, gp).TransitionDb()
	timeoutCallbackCleared := clearTimeoutCallback()
	if !timeoutCallbackCleared || ctx.Err() != nil {
		return nil, xerrors.ErrQuery.Wrap(ctx.Err())
	}
	if err != nil {
		return &core.ExecutionResult{Err: err}, nil
	}
	return result, nil
}
