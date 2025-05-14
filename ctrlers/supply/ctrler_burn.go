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
	// todo: Consider the case of burning multiply in one block
	burn := NewSupply(height, nil, amt)
	ctrler.burnedSupply = append(ctrler.burnedSupply, burn)

	//adjusted := new(uint256.Int).Sub(ctrler.lastTotalSupply, amt)
	//burn := &Supply{
	//	SupplyProto: SupplyProto{
	//		Height:  height,
	//		XSupply: adjusted.Bytes(),
	//		XChange: amt.Bytes(),
	//	},
	//}
	//if xerr := ctrler.supplyState.Set(v1.LedgerKeyAdjustedSupply(), burn, true); xerr != nil {
	//	ctrler.logger.Error("fail to set adjusted supply", "error", xerr.Error())
	//	return xerr
	//}
	//
	//ctrler.lastTotalSupply = adjusted
	//ctrler.lastAdjustedSupply = adjusted
	//ctrler.lastAdjustedHeight = height

	return nil
}
