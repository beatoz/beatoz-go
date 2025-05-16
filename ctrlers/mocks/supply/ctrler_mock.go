package supply

import (
	"github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

type SupplyHandlerMock struct{}

func NewSupplyHandlerMock() *SupplyHandlerMock {
	return &SupplyHandlerMock{}
}

func (mock *SupplyHandlerMock) ValidateTrx(ctx *types.TrxContext) xerrors.XError {
	//TODO implement me
	panic("implement me")
}

func (mock *SupplyHandlerMock) ExecuteTrx(ctx *types.TrxContext) xerrors.XError {
	//TODO implement me
	panic("implement me")
}

func (mock *SupplyHandlerMock) BeginBlock(bctx *types.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	//TODO implement me
	panic("implement me")
}

func (mock *SupplyHandlerMock) EndBlock(bctx *types.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	//TODO implement me
	panic("implement me")
}

func (mock *SupplyHandlerMock) Commit() ([]byte, int64, xerrors.XError) {
	//TODO implement me
	panic("implement me")
}

func (mock *SupplyHandlerMock) RequestMint(bctx *types.BlockContext) {
	//TODO implement me
	panic("implement me")
}

func (mock *SupplyHandlerMock) Burn(bctx *types.BlockContext, amt *uint256.Int) xerrors.XError {
	return nil
}

var _ types.ISupplyHandler = (*SupplyHandlerMock)(nil)
