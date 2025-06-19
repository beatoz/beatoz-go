package evm

import (
	"fmt"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/libs/jsonx"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"time"
)

func (ctrler *EVMCtrler) Query(req abcitypes.RequestQuery, opts ...ctrlertypes.Option) ([]byte, xerrors.XError) {
	from := req.Data[:types.AddrSize]
	to := req.Data[types.AddrSize : types.AddrSize*2]
	data := req.Data[types.AddrSize*2:]
	height := req.Height

	if height <= 0 {
		height = ctrler.lastBlockHeight
	}

	//_, err := tmrpccore.Block(nil, &height)
	//if err != nil {
	//	return nil, xerrors.From(err)
	//}

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

func (ctrler *EVMCtrler) QueryCode(addr types.Address, height int64) ([]byte, xerrors.XError) {
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

	vmmsg := evmMessage(sender, toAddr, 0, 3000000, uint256.NewInt(0), uint256.NewInt(0), data, true)
	blockContext := evmBlockContext(sender, height, blockTime, 3000000)

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
