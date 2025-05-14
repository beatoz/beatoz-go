package account

import (
	btztypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

func (ctrler *AcctCtrler) BeginBlock(*btztypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	// do nothing
	return nil, nil
}

func (ctrler *AcctCtrler) EndBlock(*btztypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	// do nothing
	return nil, nil
}

func (ctrler *AcctCtrler) Commit() ([]byte, int64, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	h, v, xerr := ctrler.acctState.Commit()
	return h, v, xerr
}
