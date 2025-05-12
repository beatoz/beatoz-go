package gov

import (
	"github.com/beatoz/beatoz-go/ctrlers/mocks"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"time"
)

func makeTrxCtx(tx *ctrlertypes.Trx, height int64, exec bool) *ctrlertypes.TrxContext {

	txctx, xerr := mocks.MakeTrxCtxWithTrx(tx, "", height, time.Now(), exec, govCtrler, acctMock, nil, nil, stakeHelper)
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
