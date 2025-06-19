package mocks

import (
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"time"
)

func MakeTrxCtxWithTrx(
	tx *ctrlertypes.Trx,
	chainId string, height int64, btm time.Time, exec bool,
	g ctrlertypes.IGovHandler,
	a ctrlertypes.IAccountHandler,
	e ctrlertypes.IEVMHandler,
	s ctrlertypes.ISupplyHandler,
	v ctrlertypes.IVPowerHandler,
) (*ctrlertypes.TrxContext, xerrors.XError) {
	bctx := ctrlertypes.TempBlockContext(chainId, height, btm, g, a, e, s, v)
	return MakeTrxCtxWithTrxBctx(tx, bctx, exec)
}

func MakeTrxCtxWithTrxBctx(
	tx *ctrlertypes.Trx,
	bctx *ctrlertypes.BlockContext,
	exec bool,
) (*ctrlertypes.TrxContext, xerrors.XError) {
	txbz, xerr := tx.Encode()
	if xerr != nil {
		return nil, xerr
	}
	return ctrlertypes.NewTrxContext(txbz, bctx, exec)
}
func MakeTrxCtxWithBz(
	txbz []byte,
	chainId string, height int64, btm time.Time, exec bool,
	g ctrlertypes.IGovHandler,
	a ctrlertypes.IAccountHandler,
	e ctrlertypes.IEVMHandler,
	s ctrlertypes.ISupplyHandler,
	v ctrlertypes.IVPowerHandler,
) (*ctrlertypes.TrxContext, xerrors.XError) {
	bctx := ctrlertypes.TempBlockContext(chainId, height, btm, g, a, e, s, v)
	return MakeTrxCtxWithBzBctx(txbz, bctx, exec)
}

func MakeTrxCtxWithBzBctx(
	txbz []byte,
	bctx *ctrlertypes.BlockContext,
	exec bool,
) (*ctrlertypes.TrxContext, xerrors.XError) {
	return ctrlertypes.NewTrxContext(txbz, bctx, exec)
}
