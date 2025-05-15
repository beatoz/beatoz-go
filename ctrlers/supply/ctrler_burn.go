package supply

import (
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
)

func (ctrler *SupplyCtrler) Burn(bctx *ctrlertypes.BlockContext, amt *uint256.Int) xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	return ctrler.burn(bctx.Height(), amt)
}

func (ctrler *SupplyCtrler) burn(height int64, amt *uint256.Int) xerrors.XError {
	ctrler.lastTotalSupply.AdjustSub(height, amt)
	return nil
}
