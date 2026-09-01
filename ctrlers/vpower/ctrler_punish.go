package vpower

import (
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
)

// doSlash is executed at BeginBlock
func (ctrler *VPowerCtrler) doSlash(targetAddr types.Address, slashRate int32, refundHeight int64) (int64, xerrors.XError) {
	dgtee, xerr := ctrler.readDelegatee(targetAddr, true)
	if xerr != nil {
		return 0, xerr
	}

	// Slash only the validator's self-delegated power.
	selfPowerBefore := dgtee.SelfPower
	if xerr := ctrler.forceUnbondDelegatee(dgtee, refundHeight, true, slashRate); xerr != nil {
		return 0, xerr
	}
	if xerr := ctrler.setTombstone(dgtee.addr, true); xerr != nil {
		return 0, xerr
	}
	slashedPower := selfPowerBefore - dgtee.SelfPower
	return slashedPower, nil
}
