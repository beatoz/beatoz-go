package supply

import (
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

func (ctrler *SupplyCtrler) BeginBlock(bctx *ctrlertypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	if bctx.Height() > 0 && bctx.Height()%bctx.GovHandler.InflationCycleBlocks() == 0 {
		ctrler.requestMint(bctx)
	}
	return nil, nil
}

func (ctrler *SupplyCtrler) EndBlock(bctx *ctrlertypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	if bctx.Height() > 0 && bctx.Height()%bctx.GovHandler.InflationCycleBlocks() == 0 {
		if _, xerr := ctrler.waitMint(bctx); xerr != nil {
			ctrler.logger.Error("fail to requestMint", "error", xerr.Error())
			return nil, xerr
		}
	}

	return nil, nil
}

func (ctrler *SupplyCtrler) Commit() ([]byte, int64, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	h, v, xerr := ctrler.supplyState.Commit()
	if xerr != nil {
		return nil, 0, xerr
	}

	return h, v, nil
}
