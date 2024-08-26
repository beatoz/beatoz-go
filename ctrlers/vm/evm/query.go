package evm

import (
	"fmt"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmjson "github.com/tendermint/tendermint/libs/json"
	tmrpccore "github.com/tendermint/tendermint/rpc/core"
)

func (ctrler *EVMCtrler) Query(req abcitypes.RequestQuery) ([]byte, xerrors.XError) {
	from := req.Data[:types.AddrSize]
	to := req.Data[types.AddrSize : types.AddrSize*2]
	data := req.Data[types.AddrSize*2:]
	height := req.Height

	if height <= 0 {
		height = ctrler.lastBlockHeight
	}

	block, err := tmrpccore.Block(nil, &height)
	if err != nil {
		return nil, xerrors.From(err)
	}
	btm := block.Block.Time.Unix()

	execRet, xerr := ctrler.callVM(from, to, data, height, btm)
	if xerr != nil {
		return nil, xerr
	}

	returnData := &ctrlertypes.VMCallResult{
		UsedGas:    execRet.UsedGas,
		ReturnData: execRet.ReturnData,
	}
	if execRet.Err != nil {
		returnData.Err = execRet.Err.Error()
	} else {
		returnData.Err = ""
	}

	retbz, err := tmjson.Marshal(returnData)
	if err != nil {
		return nil, xerrors.From(err)
	}

	return retbz, nil
}

func (ctrler *EVMCtrler) QueryCode(addr types.Address, height int64) ([]byte, xerrors.XError) {
	state, xerr := ctrler.ImmutableStateAt(height)
	if xerr != nil {
		return nil, xerr
	}

	return state.GetCode(addr.Array20()), nil
}

func (ctrler *EVMCtrler) callVM(from, to types.Address, data []byte, height, blockTime int64) (*core.ExecutionResult, xerrors.XError) {

	// block<height> 시점의 stateDB 와 account ledger(acctCtrler) 를 갖는 `stateDBWrapper` 획득
	state, xerr := ctrler.ImmutableStateAt(height)
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

	vmmsg := evmMessage(sender, toAddr, 0, gasLimit, uint256.NewInt(0), uint256.NewInt(0), data, true)
	blockContext := evmBlockContext(sender, height, blockTime)

	txContext := core.NewEVMTxContext(vmmsg)
	vmevm := vm.NewEVM(blockContext, txContext, state, ctrler.ethChainConfig, vm.Config{NoBaseFee: true})

	gp := new(core.GasPool).AddGas(gasLimit)
	result, err := core.ApplyMessage(vmevm, vmmsg, gp)
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
