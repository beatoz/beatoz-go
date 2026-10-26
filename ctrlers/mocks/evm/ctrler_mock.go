package evm

import (
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	ethcore "github.com/ethereum/go-ethereum/core"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

// EVMHandlerMock provides basic account, payload and intrinsic gas validation.
// It does not execute EVM code or implement all EVMCtrler validation rules.
type EVMHandlerMock struct{}

func NewEVMHandlerMock() *EVMHandlerMock {
	return &EVMHandlerMock{}
}

func (mock *EVMHandlerMock) ValidateTrx(ctx *ctrlertypes.TrxContext) xerrors.XError {
	receiver := ctx.Receiver()
	if !types.IsZeroAddress(receiver.Address) && receiver.Code == nil {
		return xerrors.ErrInvalidAccountType
	}

	var inputData []byte
	if payload, ok := ctx.Tx.Payload.(*ctrlertypes.TrxPayloadContract); ok {
		inputData = payload.Data
	}
	if types.IsZeroAddress(receiver.Address) && len(inputData) == 0 {
		return xerrors.ErrInvalidTrxPayloadParams
	}

	gas, err := ethcore.IntrinsicGas(inputData, nil, types.IsZeroAddress(ctx.Tx.To), true, true, true)
	if err != nil {
		return xerrors.From(err)
	}
	if ctx.Tx.Gas < 0 || uint64(ctx.Tx.Gas) < gas {
		return xerrors.ErrInvalidGas.Wrapf("invalid gas: expected(%v+) got(%v)", gas, ctx.Tx.Gas)
	}
	if !ctx.Exec {
		ctx.GasUsed = int64(gas)
	}
	return nil
}

func (mock *EVMHandlerMock) ExecuteTrx(*ctrlertypes.TrxContext) xerrors.XError {
	//TODO implement me
	panic("implement me")
}

func (mock *EVMHandlerMock) BeginBlock(*ctrlertypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	//TODO implement me
	panic("implement me")
}

func (mock *EVMHandlerMock) EndBlock(*ctrlertypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	//TODO implement me
	panic("implement me")
}

func (mock *EVMHandlerMock) Commit() ([]byte, int64, xerrors.XError) {
	//TODO implement me
	panic("implement me")
}

var _ ctrlertypes.IEVMHandler = (*EVMHandlerMock)(nil)
