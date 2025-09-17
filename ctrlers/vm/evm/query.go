package evm

import (
	"fmt"
	"math"
	"math/big"
	"time"

	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/libs/jsonx"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	ethcoretypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

func (ctrler *EVMCtrler) Query(req abcitypes.RequestQuery, opts ...ctrlertypes.Option) ([]byte, xerrors.XError) {
	from := req.Data[:types.AddrSize]
	to := req.Data[types.AddrSize : types.AddrSize*2]
	data := req.Data[types.AddrSize*2:]
	height := req.Height

	if height <= 0 {
		height = ctrler.lastBlockHeight
	}

	execRet, xerr := ctrler.callVM(from, to, data, height, time.Now().Unix())
	if xerr != nil {
		return nil, xerr
	}

	retData := execRet.ReturnData
	if req.Path == "vm_estimate_gas" {
		retData = nil
	}
	vmCallRet := &ctrlertypes.VMCallResult{
		UsedGas:    int64(execRet.UsedGas),
		ReturnData: retData,
	}
	if execRet.Err != nil {
		vmCallRet.Err = execRet.Err.Error()
	} else {
		vmCallRet.Err = ""
	}

	retbz, err := jsonx.Marshal(vmCallRet)
	if err != nil {
		return nil, xerrors.From(err)
	}

	return retbz, nil
}

func (ctrler *EVMCtrler) GetCode(addr types.Address, height int64) ([]byte, xerrors.XError) {
	state, xerr := ctrler.MemStateAt(height)
	if xerr != nil {
		return nil, xerr
	}

	return state.GetCode(addr.Array20()), nil
}
func (ctrler *EVMCtrler) callVM(from, to types.Address, data []byte, height, blockTime int64) (*core.ExecutionResult, xerrors.XError) {

	// Get the stateDB at block<height> and the `stateDBWrapper` that has account ledger(acctCtrler)
	state, xerr := ctrler.MemStateAt(height)
	if xerr != nil {
		return nil, xerr
	}

	state.Prepare(nil, 0, from, to, 0, false)
	defer func() { state = nil }()

	var sender common.Address
	var toAddr *common.Address
	copy(sender[:], from)
	if to != nil &&
		!types.IsZeroAddress(to) {
		toAddr = new(common.Address)
		copy(toAddr[:], to)
	}

	vmmsg := callMsg{
		ethereum.CallMsg{
			From:     sender,
			To:       toAddr,
			Data:     data,
			Gas:      50000000,
			GasPrice: new(big.Int), GasFeeCap: new(big.Int), GasTipCap: new(big.Int),
			Value: new(big.Int),
		},
	}

	blockContext := evmBlockContext(sender, height, blockTime, math.MaxInt64)

	txContext := core.NewEVMTxContext(vmmsg)
	vmevm := vm.NewEVM(blockContext, txContext, state, ctrler.ethChainConfig, vm.Config{NoBaseFee: true})

	gp := new(core.GasPool).AddGas(blockContext.GasLimit)
	result, err := NewVMStateTransition(vmevm, vmmsg, gp).TransitionDb()
	if err != nil {
		return nil, xerrors.From(err)
	}

	// If the timer caused an abort, return an appropriate error message
	if vmevm.Cancelled() {
		return nil, xerrors.From(fmt.Errorf("execution aborted (timeout ???)"))
	}
	if err != nil {
		return nil, xerrors.From(fmt.Errorf("err: %w (supplied gas %d)", err, vmmsg.Gas()))
	}

	return result, nil
}

type callMsg struct {
	ethereum.CallMsg
}

func (m callMsg) From() common.Address                { return m.CallMsg.From }
func (m callMsg) Nonce() uint64                       { return 0 }
func (m callMsg) IsFake() bool                        { return true }
func (m callMsg) To() *common.Address                 { return m.CallMsg.To }
func (m callMsg) GasPrice() *big.Int                  { return m.CallMsg.GasPrice }
func (m callMsg) GasFeeCap() *big.Int                 { return m.CallMsg.GasFeeCap }
func (m callMsg) GasTipCap() *big.Int                 { return m.CallMsg.GasTipCap }
func (m callMsg) Gas() uint64                         { return m.CallMsg.Gas }
func (m callMsg) Value() *big.Int                     { return m.CallMsg.Value }
func (m callMsg) Data() []byte                        { return m.CallMsg.Data }
func (m callMsg) AccessList() ethcoretypes.AccessList { return m.CallMsg.AccessList }
