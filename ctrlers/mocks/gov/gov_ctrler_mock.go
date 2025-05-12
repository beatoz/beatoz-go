package gov

import (
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

type GovHandlerMock struct {
	ctrlertypes.GovParams
}

func NewGovHandlerMock(params *ctrlertypes.GovParams) *GovHandlerMock {
	return &GovHandlerMock{
		GovParams: *params,
	}
}

func (mock *GovHandlerMock) BeginBlock(context *ctrlertypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	//TODO implement me
	panic("implement me")
}

func (mock *GovHandlerMock) EndBlock(context *ctrlertypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	//TODO implement me
	panic("implement me")
}

func (mock *GovHandlerMock) ValidateTrx(context *ctrlertypes.TrxContext) xerrors.XError {
	//TODO implement me
	panic("implement me")
}

func (mock *GovHandlerMock) ExecuteTrx(context *ctrlertypes.TrxContext) xerrors.XError {
	//TODO implement me
	panic("implement me")
}

var _ ctrlertypes.IGovHandler = (*GovHandlerMock)(nil)
