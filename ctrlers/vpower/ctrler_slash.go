package vpower

import (
	"github.com/beatoz/beatoz-go/types/xerrors"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

// DoPunish is only used to test
func (ctrler *VPowerCtrler) DoPunish(evi *abcitypes.Evidence, slashRatio int64) (int64, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	return ctrler.doPunish(evi, slashRatio)
}

// doPunish is executed at BeginBlock
func (ctrler *VPowerCtrler) doPunish(evi *abcitypes.Evidence, slashRatio int64) (int64, xerrors.XError) {
	//delegatee, xerr := ctrler.vpowLedger.Get(evi.Validator.Address, true)
	//if xerr != nil {
	//	return 0, xerr
	//}

	// Punish the delegators as well as validator. issue #51
	//slashed := delegatee.DoSlash(slashRatio)
	//_ = ctrler.delegateeLedger.Set(delegatee, true)

	//return slashed, nil
	return 0, nil
}
