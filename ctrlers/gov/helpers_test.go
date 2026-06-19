package gov

import (
	"time"

	"github.com/beatoz/beatoz-go/ctrlers/mocks"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
)

type proposalSupplyHandlerStub struct {
	ctrlertypes.ISupplyHandler
	totalSupply *uint256.Int
}

func (stub *proposalSupplyHandlerStub) TotalSupply() *uint256.Int {
	return stub.totalSupply.Clone()
}

func makeTrxCtx(tx *ctrlertypes.Trx, height int64, exec bool) *ctrlertypes.TrxContext {

	supplyHandler := &proposalSupplyHandlerStub{
		totalSupply: new(uint256.Int),
	}
	txctx, xerr := mocks.MakeTrxCtxWithTrx(tx, config.ChainIdHex(), height, time.Now(), exec, govCtrler, acctMock, nil, supplyHandler, vpowMock)
	if xerr != nil {
		panic(xerr)
	}

	return txctx
}

func runCase(c *Case) xerrors.XError {
	return runTrx(c.txctx)
}

func runTrx(ctx *ctrlertypes.TrxContext) xerrors.XError {
	if xerr := govCtrler.ValidateTrx(ctx); xerr != nil {
		return xerr
	}
	if xerr := govCtrler.ExecuteTrx(ctx); xerr != nil {
		return xerr
	}
	return nil
}
