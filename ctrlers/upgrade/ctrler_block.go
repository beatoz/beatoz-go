package upgrade

import (
	"fmt"

	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

func (ctrler *UpgradeCtrler) BeginBlock(bctx *ctrlertypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	if ctrler.upgradeState.Version() == 0 {
		height := bctx.Height()
		if height > 1 && types.IsBTIP64(bctx.ChainID(), height-1) {
			return nil, xerrors.NewOrdinary(fmt.Sprintf("upgrade ledger is empty at non-activation height: %d", height))
		}
		if xerr := ctrler.upgradeState.SetInitialVersion(height); xerr != nil {
			return nil, xerr
		}
	}
	return nil, nil
}

func (ctrler *UpgradeCtrler) EndBlock(_ *ctrlertypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	return nil, nil
}

func (ctrler *UpgradeCtrler) Commit() ([]byte, int64, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	h, v, xerr := ctrler.upgradeState.Commit()
	if xerr != nil {
		return nil, 0, xerr
	}

	return h, v, nil
}
