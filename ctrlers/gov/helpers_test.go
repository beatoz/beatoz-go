package gov

import (
	"time"

	"github.com/beatoz/beatoz-go/ctrlers/mocks"
	supplymock "github.com/beatoz/beatoz-go/ctrlers/mocks/supply"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
)

func makeTrxCtx(tx *ctrlertypes.Trx, height int64, exec bool) *ctrlertypes.TrxContext {
	supplyHandler := supplymock.NewSupplyHandlerMock()
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
