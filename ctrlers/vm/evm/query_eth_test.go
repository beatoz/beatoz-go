package evm

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"math/big"
	"testing"
	"time"

	cfg "github.com/beatoz/beatoz-go/cmd/config"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/libs/jsonx"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmlog "github.com/tendermint/tendermint/libs/log"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

func Test_QueryEth(t *testing.T) {
	config := cfg.DefaultConfig("1")
	config.App.QueryTimeout = 20 * time.Millisecond
	config.SetRoot(t.TempDir())

	acctHandler := &acctHandlerMock{}
	ctrler := NewEVMCtrler(config, acctHandler, tmlog.NewNopLogger())
	t.Cleanup(func() { require.NoError(t, ctrler.Close()) })

	bctx := ctrlertypes.NewBlockContext(
		abcitypes.RequestBeginBlock{
			Header: tmproto.Header{ChainID: config.ChainIdHex(), Height: 1, Time: time.Unix(1, 0)},
		},
		govMock, acctHandler, ctrler, nil, nil,
	)
	_, xerr := ctrler.BeginBlock(bctx)
	require.NoError(t, xerr)

	const blockGasLimit int64 = 3_000_000
	coinbase := common.HexToAddress("0x1122334455667788990011223344556677889900")
	from := types.RandAddress()
	echoAddr := types.RandAddress()
	revertAddr := types.RandAddress()
	loopAddr := types.RandAddress()
	valueAddr := types.RandAddress()
	contextAddr := types.RandAddress()
	fundedFrom := types.RandAddress()

	// Copy calldata to memory and return it.
	ctrler.stateDBWrapper.SetCode(echoAddr.Array20(), []byte{0x36, 0x60, 0x00, 0x60, 0x00, 0x37, 0x36, 0x60, 0x00, 0xf3})
	// REVERT with empty data.
	ctrler.stateDBWrapper.SetCode(revertAddr.Array20(), []byte{0x60, 0x00, 0x60, 0x00, 0xfd})
	// JUMPDEST; PUSH1 0; JUMP.
	ctrler.stateDBWrapper.SetCode(loopAddr.Array20(), []byte{0x5b, 0x60, 0x00, 0x56})
	// Return CALLVALUE as a 32-byte word.
	ctrler.stateDBWrapper.SetCode(valueAddr.Array20(), []byte{0x34, 0x60, 0x00, 0x52, 0x60, 0x20, 0x60, 0x00, 0xf3})
	// Return TIMESTAMP, COINBASE, GASLIMIT and NUMBER as four 32-byte words.
	ctrler.stateDBWrapper.SetCode(contextAddr.Array20(), []byte{
		0x42, 0x60, 0x00, 0x52, 0x41, 0x60, 0x20, 0x52,
		0x45, 0x60, 0x40, 0x52, 0x43, 0x60, 0x60, 0x52,
		0x60, 0x80, 0x60, 0x00, 0xf3,
	})
	// Initiate synchronizes the EVM sender from the simulated account ledger.
	require.NoError(t, acctHandler.SetBalance(fundedFrom, uint256.NewInt(5), true))
	ctrler.stateDBWrapper.SetBalance(fundedFrom.Array20(), uint256.NewInt(5))
	storageSlot, storageValue := common.HexToHash("0x1"), common.HexToHash("0xab")
	ctrler.stateDBWrapper.SetState(echoAddr.Array20(), storageSlot, storageValue)
	// Deploy the existing ERC20 fixture; transferring its full supply clears the sender's balance slot.
	constructor, err := abiERC20Contract.Pack("", "QueryToken", "QT")
	require.NoError(t, err)
	deployment := append(append([]byte{}, erc20BuildInfo.Bytecode...), constructor...)
	txctx := &ctrlertypes.TrxContext{
		BlockContext: bctx,
		Tx:           web3.NewTrxContract(from, types.ZeroAddress(), 0, 3_000_000, new(uint256.Int), new(uint256.Int), deployment),
		Exec:         true,
	}
	require.NoError(t, ctrler.ExecuteTrx(txctx))
	refundAddr := types.Address(txctx.RetData)
	fullBalance := toWei(1_000_000_000)
	refundInput, err := abiERC20Contract.Pack("transfer", fundedFrom.Array20(), fullBalance)
	require.NoError(t, err)
	_, height, xerr := ctrler.Commit()
	require.NoError(t, xerr)

	input := []byte{0x01, 0x02, 0x03, 0x04}
	// Dispatch compatibility cases through the same legacy/Ethereum split as the node.
	query := func(req abcitypes.RequestQuery) ([]byte, xerrors.XError) {
		if req.Path == "eth_call" || req.Path == "eth_estimateGas" {
			return ctrler.QueryEth(req)
		}
		return ctrler.Query(req)
	}

	t.Run("ethereum_zero_value_matches_legacy", func(t *testing.T) {
		for _, to := range []types.Address{echoAddr, revertAddr} {
			legacy, xerr := ctrler.Query(abcitypes.RequestQuery{
				Path: "vm_call", Data: queryTestData(from, to, input), Height: height,
			})
			require.NoError(t, xerr)
			value, xerr := ctrler.QueryEth(abcitypes.RequestQuery{
				Path: "eth_call", Data: ethQueryTestData(from, to, new(big.Int), input, uint64(blockGasLimit)), Height: height,
			})
			require.NoError(t, xerr)
			require.JSONEq(t, string(legacy), string(value))
		}
		value, xerr := ctrler.QueryEth(abcitypes.RequestQuery{
			Path: "eth_call", Data: ethQueryTestData(from, echoAddr, new(big.Int), input, uint64(blockGasLimit)),
		})
		require.NoError(t, xerr)
		var result ctrlertypes.VMCallResult
		require.NoError(t, jsonx.Unmarshal(value, &result))
		require.Equal(t, input, result.ReturnData)
	})

	t.Run("short_data", func(t *testing.T) {
		for _, path := range []string{"eth_call", "eth_estimateGas"} {
			for _, size := range []int{0, 19, 39, 40, 71, 72, 79, 80, 87, 88, 107, 108, 115} {
				value, xerr := ctrler.QueryEth(abcitypes.RequestQuery{Path: path, Data: make([]byte, size), Height: height})
				require.Nil(t, value)
				require.EqualError(t, xerr, fmt.Sprintf("invalid %s query data length: %d", path, size))
			}
		}
	})

	t.Run("call_gas_limit", func(t *testing.T) {
		for _, gasLimit := range []uint64{0, 100_000, math.MaxUint64, 21_064} {
			data := ethQueryTestData(from, echoAddr, new(big.Int), input, uint64(blockGasLimit))
			binary.BigEndian.PutUint64(data[72:80], gasLimit)
			value, xerr := ctrler.QueryEth(abcitypes.RequestQuery{Path: "eth_call", Data: data, Height: height})
			require.NoError(t, xerr)
			var result ctrlertypes.VMCallResult
			require.NoError(t, jsonx.Unmarshal(value, &result))
			if gasLimit == 21_064 {
				require.Equal(t, vm.ErrOutOfGas.Error(), result.Err)
				require.Empty(t, result.ReturnData)
			} else {
				require.Empty(t, result.Err)
				require.Equal(t, input, result.ReturnData)
			}
		}
	})

	t.Run("block_context", func(t *testing.T) {
		const blockTime = 1_700_000_123
		data := ethQueryTestData(from, contextAddr, new(big.Int), nil, uint64(blockGasLimit))
		binary.BigEndian.PutUint64(data[72:80], 100_000)
		binary.BigEndian.PutUint64(data[80:88], blockTime)
		copy(data[88:108], coinbase[:])
		value, xerr := ctrler.QueryEth(abcitypes.RequestQuery{Path: "eth_call", Data: data, Height: height})
		require.NoError(t, xerr)
		var result ctrlertypes.VMCallResult
		require.NoError(t, jsonx.Unmarshal(value, &result))
		require.Empty(t, result.Err)
		require.Len(t, result.ReturnData, 128)
		require.Equal(t, int64(blockTime), new(big.Int).SetBytes(result.ReturnData[:32]).Int64())
		require.Equal(t, common.BytesToHash(coinbase[:]).Bytes(), result.ReturnData[32:64])
		require.Equal(t, blockGasLimit, new(big.Int).SetBytes(result.ReturnData[64:96]).Int64())
		require.Equal(t, height, new(big.Int).SetBytes(result.ReturnData[96:]).Int64())
	})

	t.Run("call_gas_ceiling", func(t *testing.T) {
		const ceiling int64 = 30_000
		for _, gasLimit := range []uint64{0, 25_000, uint64(ceiling), uint64(ceiling) + 1, math.MaxUint64} {
			data := ethQueryTestData(from, loopAddr, new(big.Int), nil, uint64(ceiling))
			binary.BigEndian.PutUint64(data[72:80], gasLimit)
			value, xerr := ctrler.QueryEth(abcitypes.RequestQuery{Path: "eth_call", Data: data, Height: height})
			require.NoError(t, xerr)
			var result ctrlertypes.VMCallResult
			require.NoError(t, jsonx.Unmarshal(value, &result))
			require.Equal(t, vm.ErrOutOfGas.Error(), result.Err)
			want := ceiling
			if gasLimit > 0 && gasLimit < uint64(ceiling) {
				want = int64(gasLimit)
			}
			require.Equal(t, want, result.UsedGas)
			require.Empty(t, result.ReturnData)
		}
	})

	t.Run("estimate_gas_ceiling", func(t *testing.T) {
		for _, ceiling := range []int64{20_000, 30_000} {
			for _, gasLimit := range []uint64{0, 1, params.TxGas - 1, uint64(ceiling), uint64(ceiling) + 1, math.MaxUint64} {
				data := ethQueryTestData(from, loopAddr, new(big.Int), nil, uint64(ceiling))
				binary.BigEndian.PutUint64(data[72:80], gasLimit)
				value, xerr := ctrler.QueryEth(abcitypes.RequestQuery{Path: "eth_estimateGas", Data: data, Height: height})
				require.NoError(t, xerr)
				var result ctrlertypes.VMCallResult
				require.NoError(t, jsonx.Unmarshal(value, &result))
				require.Equal(t, fmt.Sprintf("gas required exceeds allowance (%d)", ceiling), result.Err)
				require.Zero(t, result.UsedGas)
			}
		}
	})

	t.Run("invalid_gas_ceiling", func(t *testing.T) {
		for _, path := range []string{"eth_call", "eth_estimateGas"} {
			for _, ceiling := range []uint64{0, math.MaxInt64 + 1, math.MaxUint64} {
				value, xerr := ctrler.QueryEth(abcitypes.RequestQuery{
					Path: path, Data: ethQueryTestData(from, echoAddr, new(big.Int), nil, ceiling), Height: height,
				})
				require.Nil(t, value)
				require.EqualError(t, xerr, fmt.Sprintf("invalid block gas limit: %d", ceiling))
			}
		}
	})

	t.Run("estimate_gas_limit", func(t *testing.T) {
		var estimate int64
		for _, gasLimit := range []uint64{0, 1, params.TxGas - 1, 100_000, math.MaxUint64, 21_064, 21_000} {
			data := ethQueryTestData(from, echoAddr, new(big.Int), input, uint64(blockGasLimit))
			binary.BigEndian.PutUint64(data[72:80], gasLimit)
			value, xerr := ctrler.QueryEth(abcitypes.RequestQuery{Path: "eth_estimateGas", Data: data, Height: height})
			require.NoError(t, xerr)
			var result ctrlertypes.VMCallResult
			require.NoError(t, jsonx.Unmarshal(value, &result))
			if gasLimit == 21_064 || gasLimit == 21_000 {
				require.Equal(t, fmt.Sprintf("gas required exceeds allowance (%d)", gasLimit), result.Err)
			} else {
				require.Empty(t, result.Err)
				if gasLimit == 0 {
					estimate = result.UsedGas
					require.Positive(t, estimate)
				} else {
					require.Equal(t, estimate, result.UsedGas)
				}
			}
		}
	})

	t.Run("estimate_non_gas_failure", func(t *testing.T) {
		var want string
		for _, gasLimit := range []uint64{0, 100_000} {
			estimate, result, xerr := ctrler.estimateGas(from, valueAddr, big.NewInt(1), nil, height, 1, gasLimit, blockGasLimit, coinbase)
			require.NoError(t, xerr)
			require.Zero(t, estimate)
			require.ErrorIs(t, result.Err, core.ErrInsufficientFunds)
			if gasLimit == 0 {
				want = result.Err.Error()
			} else {
				require.EqualError(t, result.Err, want)
			}
		}
	})

	t.Run("estimate_cap_below_optimistic_trial", func(t *testing.T) {
		baseline, xerr := ctrler.ethCallVM(context.Background(), from, echoAddr, new(big.Int), input, height, 1, uint64(blockGasLimit), blockGasLimit, coinbase)
		require.NoError(t, xerr)
		require.NoError(t, baseline.Err)
		gasLimit := baseline.UsedGas + baseline.RefundedGas + params.CallStipend
		require.Less(t, gasLimit, gasLimit*64/63)
		estimate, result, xerr := ctrler.estimateGas(from, echoAddr, new(big.Int), input, height, 1, gasLimit, blockGasLimit, coinbase)
		require.NoError(t, xerr)
		require.NoError(t, result.Err)
		require.Positive(t, estimate)
		require.LessOrEqual(t, estimate, gasLimit)
		result, xerr = ctrler.ethCallVM(context.Background(), from, echoAddr, new(big.Int), input, height, 1, estimate, blockGasLimit, coinbase)
		require.NoError(t, xerr)
		require.NoError(t, result.Err)
		require.Equal(t, input, result.ReturnData)
	})

	t.Run("estimate_refund_and_legacy_compatibility", func(t *testing.T) {
		legacy, xerr := ctrler.callVM(from, refundAddr, refundInput, height, 1)
		require.NoError(t, xerr)
		require.NoError(t, legacy.Err)
		require.Positive(t, legacy.RefundedGas)
		estimate, result, xerr := ctrler.estimateGas(from, refundAddr, new(big.Int), refundInput, height, 1, 0, blockGasLimit, coinbase)
		require.NoError(t, xerr)
		require.NoError(t, result.Err)
		require.GreaterOrEqual(t, estimate, legacy.UsedGas+legacy.RefundedGas)
		result, xerr = ctrler.ethCallVM(context.Background(), from, refundAddr, new(big.Int), refundInput, height, 1, estimate, blockGasLimit, coinbase)
		require.NoError(t, xerr)
		require.NoError(t, result.Err)
		result, xerr = ctrler.ethCallVM(context.Background(), from, refundAddr, new(big.Int), refundInput, height, 1, legacy.UsedGas, blockGasLimit, coinbase)
		require.NoError(t, xerr)
		require.ErrorIs(t, result.Err, vm.ErrOutOfGas)

		for _, path := range []string{"vm_estimate_gas", "eth_estimateGas"} {
			data, want := queryTestData(from, refundAddr, refundInput), legacy.UsedGas
			if path == "eth_estimateGas" {
				data, want = ethQueryTestData(from, refundAddr, new(big.Int), refundInput, uint64(blockGasLimit)), estimate
			}
			value, xerr := query(abcitypes.RequestQuery{Path: path, Data: data, Height: height})
			require.NoError(t, xerr)
			var result ctrlertypes.VMCallResult
			require.NoError(t, jsonx.Unmarshal(value, &result))
			require.Equal(t, int64(want), result.UsedGas)
			require.Empty(t, result.Err)
			require.Empty(t, result.ReturnData)
		}
		balanceInput, err := abiERC20Contract.Pack("balanceOf", from.Array20())
		require.NoError(t, err)
		balance, xerr := ctrler.callVM(from, refundAddr, balanceInput, height, 1)
		require.NoError(t, xerr)
		require.NoError(t, balance.Err)
		require.Equal(t, fullBalance, new(big.Int).SetBytes(balance.ReturnData))
	})

	t.Run("estimate_revert", func(t *testing.T) {
		estimate, result, xerr := ctrler.estimateGas(from, revertAddr, new(big.Int), nil, height, 1, 0, blockGasLimit, coinbase)
		require.NoError(t, xerr)
		require.Zero(t, estimate)
		require.ErrorIs(t, result.Err, vm.ErrExecutionReverted)
		for _, gasLimit := range []uint64{0, 100_000} {
			data := ethQueryTestData(from, revertAddr, new(big.Int), nil, uint64(blockGasLimit))
			binary.BigEndian.PutUint64(data[72:80], gasLimit)
			value, xerr := ctrler.QueryEth(abcitypes.RequestQuery{
				Path: "eth_estimateGas", Data: data, Height: height,
			})
			require.NoError(t, xerr)
			var decoded ctrlertypes.VMCallResult
			require.NoError(t, jsonx.Unmarshal(value, &decoded))
			require.Equal(t, vm.ErrExecutionReverted.Error(), decoded.Err)
		}
	})

	t.Run("estimate_shared_deadline", func(t *testing.T) {
		originalHandler, originalTimeout := ctrler.acctHandler, ctrler.queryTimeout
		t.Cleanup(func() {
			ctrler.acctHandler, ctrler.queryTimeout = originalHandler, originalTimeout
		})
		const timeout = 100 * time.Millisecond
		ctrler.queryTimeout = timeout
		calls := 0
		ctrler.acctHandler = &queryTestDelayedAccountHandler{
			IAccountHandler: originalHandler,
			beforeSimulation: func() {
				calls++
				// Each state load fits one per-call timeout, but two exceed the shared budget.
				time.Sleep(3 * timeout / 5)
			},
		}
		started := time.Now()
		estimate, result, xerr := ctrler.estimateGas(from, echoAddr, new(big.Int), input, height, 1, 0, blockGasLimit, coinbase)
		require.Error(t, xerr)
		require.ErrorIs(t, xerr.Cause(), context.DeadlineExceeded)
		require.Equal(t, xerrors.ErrCodeQuery, xerr.Code())
		require.Zero(t, estimate)
		require.Nil(t, result)
		require.LessOrEqual(t, calls, 2)
		// Allow scheduling overhead and the in-progress state load to finish.
		require.Less(t, time.Since(started), 4*timeout)
	})

	t.Run("estimate_deadline_rejects_initial_bound", func(t *testing.T) {
		baseline, xerr := ctrler.ethCallVM(context.Background(), from, echoAddr, new(big.Int), input, height, 1, uint64(blockGasLimit), blockGasLimit, coinbase)
		require.NoError(t, xerr)
		require.NoError(t, baseline.Err)
		// The optimistic trial is skipped because its 64/63 adjustment exceeds the ceiling.
		ceiling := int64(baseline.UsedGas + baseline.RefundedGas + params.CallStipend)

		originalHandler, originalTimeout := ctrler.acctHandler, ctrler.queryTimeout
		t.Cleanup(func() {
			ctrler.acctHandler, ctrler.queryTimeout = originalHandler, originalTimeout
		})
		ctrler.queryTimeout = 100 * time.Millisecond
		calls := 0
		ctrler.acctHandler = &queryTestDelayedAccountHandler{
			IAccountHandler: originalHandler,
			beforeSimulation: func() {
				calls++
				if calls == 2 {
					// The first bisection times out before any trial has lowered the ceiling.
					time.Sleep(ctrler.queryTimeout + 10*time.Millisecond)
				}
			},
		}
		estimate, result, xerr := ctrler.estimateGas(from, echoAddr, new(big.Int), input, height, 1, 0, ceiling, coinbase)
		require.Error(t, xerr)
		require.ErrorIs(t, xerr.Cause(), context.DeadlineExceeded)
		require.Equal(t, xerrors.ErrCodeQuery, xerr.Code())
		require.Equal(t, 2, calls)
		require.Zero(t, estimate)
		require.Nil(t, result)
	})

	t.Run("estimate_deadline_keeps_successful_bound", func(t *testing.T) {
		baseline, xerr := ctrler.ethCallVM(context.Background(), from, echoAddr, new(big.Int), input, height, 1, uint64(blockGasLimit), blockGasLimit, coinbase)
		require.NoError(t, xerr)
		require.NoError(t, baseline.Err)
		want := (baseline.UsedGas + baseline.RefundedGas + params.CallStipend) * 64 / 63

		originalHandler, originalTimeout := ctrler.acctHandler, ctrler.queryTimeout
		t.Cleanup(func() {
			ctrler.acctHandler, ctrler.queryTimeout = originalHandler, originalTimeout
		})
		ctrler.queryTimeout = 100 * time.Millisecond
		calls := 0
		ctrler.acctHandler = &queryTestDelayedAccountHandler{
			IAccountHandler: originalHandler,
			beforeSimulation: func() {
				calls++
				if calls == 3 {
					// The initial and optimistic trials succeed before the first bisection times out.
					time.Sleep(ctrler.queryTimeout + 10*time.Millisecond)
				}
			},
		}
		estimate, result, xerr := ctrler.estimateGas(from, echoAddr, new(big.Int), input, height, 1, 0, blockGasLimit, coinbase)
		require.NoError(t, xerr)
		require.NoError(t, result.Err)
		require.Equal(t, 3, calls)
		require.Equal(t, want, estimate)
		result, xerr = ctrler.ethCallVM(context.Background(), from, echoAddr, new(big.Int), input, height, 1, estimate, blockGasLimit, coinbase)
		require.NoError(t, xerr)
		require.NoError(t, result.Err)
		require.Equal(t, input, result.ReturnData)
	})

	t.Run("estimate_tiny_deadline", func(t *testing.T) {
		originalTimeout := ctrler.queryTimeout
		t.Cleanup(func() { ctrler.queryTimeout = originalTimeout })
		ctrler.queryTimeout = time.Nanosecond
		estimate, result, xerr := ctrler.estimateGas(from, echoAddr, new(big.Int), input, height, 1, 0, blockGasLimit, coinbase)
		require.Error(t, xerr)
		require.ErrorIs(t, xerr.Cause(), context.DeadlineExceeded)
		require.Zero(t, estimate)
		require.Nil(t, result)
	})

	t.Run("call_parent_canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		result, xerr := ctrler.ethCallVM(ctx, from, echoAddr, new(big.Int), input, height, 1, uint64(blockGasLimit), blockGasLimit, coinbase)
		require.Error(t, xerr)
		require.ErrorIs(t, xerr.Cause(), context.Canceled)
		require.Nil(t, result)
	})

	t.Run("transition_error_routing", func(t *testing.T) {
		for _, value := range []*big.Int{new(big.Int), big.NewInt(1)} {
			result, xerr := ctrler.ethCallVM(context.Background(), fundedFrom, echoAddr, value, nil, height, 1, 0, blockGasLimit, coinbase)
			require.NoError(t, xerr)
			require.ErrorIs(t, result.Err, core.ErrIntrinsicGas)
		}
		// Oversized creation code produces a transition error even with sufficient gas and zero value.
		input := make([]byte, params.MaxInitCodeSize+1)
		for _, path := range []string{"vm_call", "vm_estimate_gas", "eth_call", "eth_estimateGas"} {
			for _, amount := range []*big.Int{new(big.Int), big.NewInt(1)} {
				data := queryTestData(fundedFrom, types.ZeroAddress(), input)
				isEth := path == "eth_call" || path == "eth_estimateGas"
				if isEth {
					data = ethQueryTestData(fundedFrom, types.ZeroAddress(), amount, input, uint64(blockGasLimit))
				}
				value, xerr := query(abcitypes.RequestQuery{Path: path, Data: data, Height: height})
				if isEth {
					require.NoError(t, xerr)
					var result ctrlertypes.VMCallResult
					require.NoError(t, jsonx.Unmarshal(value, &result))
					require.Contains(t, result.Err, core.ErrMaxInitCodeSizeExceeded.Error())
				} else {
					require.Nil(t, value)
					require.ErrorContains(t, xerr, core.ErrMaxInitCodeSizeExceeded.Error())
				}
			}
		}
	})

	t.Run("ethereum_value", func(t *testing.T) {
		for _, path := range []string{"eth_call", "eth_estimateGas"} {
			value, xerr := ctrler.QueryEth(abcitypes.RequestQuery{
				Path: path, Data: ethQueryTestData(fundedFrom, valueAddr, big.NewInt(3), nil, uint64(blockGasLimit)), Height: height,
			})
			require.NoError(t, xerr)
			var result ctrlertypes.VMCallResult
			require.NoError(t, jsonx.Unmarshal(value, &result))
			require.Empty(t, result.Err)
			require.Greater(t, result.UsedGas, int64(0))
			if path == "eth_call" {
				require.Equal(t, big.NewInt(3).FillBytes(make([]byte, 32)), result.ReturnData)
			} else {
				require.Empty(t, result.ReturnData)
			}

			value, xerr = ctrler.QueryEth(abcitypes.RequestQuery{
				Path: path, Data: ethQueryTestData(from, valueAddr, big.NewInt(1), nil, uint64(blockGasLimit)), Height: height,
			})
			require.NoError(t, xerr)
			result = ctrlertypes.VMCallResult{}
			require.NoError(t, jsonx.Unmarshal(value, &result))
			// buyGas checks value along with the zero gas fee before CanTransfer.
			require.Equal(t, fmt.Sprintf("%s: address %v have 0 want 1", core.ErrInsufficientFunds, common.BytesToAddress(from).Hex()), result.Err)
			require.Empty(t, result.ReturnData)
		}
		state, xerr := ctrler.MemStateAt(height)
		require.NoError(t, xerr)
		require.Equal(t, uint256.NewInt(5), state.GetBalance(fundedFrom.Array20()))
		require.True(t, state.GetBalance(valueAddr.Array20()).IsZero())
	})

	t.Run("storage", func(t *testing.T) {
		for _, size := range []int{0, 20, 51, 53} {
			value, xerr := ctrler.QueryStorageAt(abcitypes.RequestQuery{Data: make([]byte, size), Height: height})
			require.Nil(t, value)
			require.EqualError(t, xerr, fmt.Sprintf("invalid eth_getStorageAt query data length: %d", size))
		}
		for _, tt := range []struct{ slot, want common.Hash }{
			{common.Hash{}, common.Hash{}}, {storageSlot, storageValue},
		} {
			data := append(append([]byte{}, echoAddr...), tt.slot.Bytes()...)
			value, xerr := ctrler.QueryStorageAt(abcitypes.RequestQuery{Data: data, Height: height})
			require.NoError(t, xerr)
			require.Len(t, value, 32)
			require.Equal(t, tt.want.Bytes(), value)
		}
	})

	for _, path := range []string{"vm_call", "vm_estimate_gas", "eth_call", "eth_estimateGas"} {
		t.Run("timeout/"+path, func(t *testing.T) {
			type queryResult struct {
				value []byte
				xerr  xerrors.XError
			}
			resultCh := make(chan queryResult, 1)
			go func() {
				data := queryTestData(from, loopAddr, nil)
				if path == "eth_call" || path == "eth_estimateGas" {
					data = ethQueryTestData(from, loopAddr, new(big.Int), nil, math.MaxInt64)
				}
				value, xerr := query(abcitypes.RequestQuery{Path: path, Data: data, Height: height})
				resultCh <- queryResult{value, xerr}
			}()

			select {
			case result := <-resultCh:
				require.Nil(t, result.value)
				require.Error(t, result.xerr)
				require.Equal(t, xerrors.ErrCodeQuery, result.xerr.Code())
				require.ErrorContains(t, result.xerr, context.DeadlineExceeded.Error())
			case <-time.After(2 * time.Second):
				t.Fatal("timeout query did not return")
			}
		})
	}
}

func Test_QueryEthHistoricalGasLimit(t *testing.T) {
	config := cfg.DefaultConfig("1")
	config.App.QueryTimeout = time.Second
	config.SetRoot(t.TempDir())
	acctHandler := &acctHandlerMock{}
	ctrler := NewEVMCtrler(config, acctHandler, tmlog.NewNopLogger())
	t.Cleanup(func() { require.NoError(t, ctrler.Close()) })
	from, contextAddr, loopAddr := types.RandAddress(), types.RandAddress(), types.RandAddress()
	limits := map[int64]int64{1: 30_000, 2: 40_000}
	for height := int64(1); height <= 2; height++ {
		bctx := ctrlertypes.NewBlockContext(abcitypes.RequestBeginBlock{
			Header: tmproto.Header{ChainID: config.ChainIdHex(), Height: height, Time: time.Unix(height, 0)},
		}, govMock, acctHandler, ctrler, nil, nil)
		_, xerr := ctrler.BeginBlock(bctx)
		require.NoError(t, xerr)
		// Return GASLIMIT, NUMBER and a storage value that changes each block.
		ctrler.stateDBWrapper.SetCode(contextAddr.Array20(), []byte{
			0x45, 0x60, 0x00, 0x52, 0x43, 0x60, 0x20, 0x52,
			0x60, 0x00, 0x54, 0x60, 0x40, 0x52, 0x60, 0x60, 0x60, 0x00, 0xf3,
		})
		ctrler.stateDBWrapper.SetState(contextAddr.Array20(), common.Hash{}, common.BigToHash(big.NewInt(height)))
		ctrler.stateDBWrapper.SetCode(loopAddr.Array20(), []byte{0x5b, 0x60, 0x00, 0x56})
		_, committed, xerr := ctrler.Commit()
		require.NoError(t, xerr)
		require.Equal(t, height, committed)
	}
	for _, requested := range []int64{1, 2, 0, -1} {
		height := requested
		if height <= 0 {
			height = 2
		}
		data := ethQueryTestData(from, contextAddr, new(big.Int), nil, uint64(limits[height]))
		value, xerr := ctrler.QueryEth(abcitypes.RequestQuery{Path: "eth_call", Data: data, Height: requested})
		require.NoError(t, xerr)
		var result ctrlertypes.VMCallResult
		require.NoError(t, jsonx.Unmarshal(value, &result))
		require.Empty(t, result.Err)
		require.Len(t, result.ReturnData, 96)
		require.Equal(t, limits[height], new(big.Int).SetBytes(result.ReturnData[:32]).Int64())
		require.Equal(t, height, new(big.Int).SetBytes(result.ReturnData[32:64]).Int64())
		require.Equal(t, height, new(big.Int).SetBytes(result.ReturnData[64:]).Int64())
		for _, path := range []string{"eth_call", "eth_estimateGas"} {
			for _, gas := range []uint64{0, 25_000, math.MaxUint64} {
				data := ethQueryTestData(from, loopAddr, new(big.Int), nil, uint64(limits[height]))
				binary.BigEndian.PutUint64(data[72:80], gas)
				value, xerr := ctrler.QueryEth(abcitypes.RequestQuery{Path: path, Data: data, Height: requested})
				require.NoError(t, xerr)
				var result ctrlertypes.VMCallResult
				require.NoError(t, jsonx.Unmarshal(value, &result))
				bound := limits[height]
				if gas == 25_000 {
					bound = int64(gas)
				}
				if path == "eth_call" {
					require.Equal(t, vm.ErrOutOfGas.Error(), result.Err)
					require.Equal(t, bound, result.UsedGas)
				} else {
					require.Equal(t, fmt.Sprintf("gas required exceeds allowance (%d)", bound), result.Err)
				}
			}
		}
	}
}

type queryTestDelayedAccountHandler struct {
	ctrlertypes.IAccountHandler
	beforeSimulation func()
}

func (handler *queryTestDelayedAccountHandler) SimuAcctCtrlerAt(height int64) (ctrlertypes.IAccountHandler, xerrors.XError) {
	handler.beforeSimulation()
	return handler.IAccountHandler.SimuAcctCtrlerAt(height)
}

func ethQueryTestData(from, to types.Address, value *big.Int, input []byte, blockGasLimit uint64) []byte {
	data := queryTestData(from, to, value.FillBytes(make([]byte, 32)))
	data = append(data, make([]byte, 44)...)
	binary.BigEndian.PutUint64(data[108:116], blockGasLimit)
	return append(data, input...)
}
