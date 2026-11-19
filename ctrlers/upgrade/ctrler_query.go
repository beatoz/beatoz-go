package upgrade

import (
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/ledger/common"
	"github.com/beatoz/beatoz-go/libs/jsonx"
	"github.com/beatoz/beatoz-go/types/xerrors"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

func (ctrler *UpgradeCtrler) Query(req abcitypes.RequestQuery, opts ...ctrlertypes.Option) ([]byte, xerrors.XError) {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	atledger, xerr := ctrler.upgradeState.ImitableLedgerAt(req.Height)
	if xerr != nil {
		return nil, xerrors.ErrQuery.Wrap(xerr)
	}

	switch req.Path {
	case "upgrade_reserved":
		item, xerr := atledger.Get(common.LedgerKeyUpgradePlan())
		if xerr != nil && !xerr.Contains(xerrors.ErrNotFoundResult) {
			return nil, xerrors.ErrQuery.Wrap(xerr)
		}
		plan, _ := item.(*UpgradePlan)
		bz, err := jsonx.Marshal(plan)
		if err != nil {
			return nil, xerrors.ErrQuery.Wrap(err)
		}
		return bz, nil
	case "upgrade_completed":
		name := string(req.Data)
		if name == "" {
			completed := []*UpgradeDone{}
			if xerr := atledger.Seek(common.KeyPrefixUpgradeDone, true, func(_ common.LedgerKey, item common.ILedgerItem) xerrors.XError {
				done, _ := item.(*UpgradeDone)
				completed = append(completed, done)
				return nil
			}); xerr != nil {
				return nil, xerrors.ErrQuery.Wrap(xerr)
			}
			bz, err := jsonx.Marshal(completed)
			if err != nil {
				return nil, xerrors.ErrQuery.Wrap(err)
			}
			return bz, nil
		}

		item, xerr := atledger.Get(common.LedgerKeyUpgradeDone(name))
		if xerr != nil {
			return nil, xerrors.ErrQuery.Wrap(xerr)
		}
		bz, err := jsonx.Marshal(item.(*UpgradeDone))
		if err != nil {
			return nil, xerrors.ErrQuery.Wrap(err)
		}
		return bz, nil
	default:
		return nil, xerrors.ErrQuery.Wrapf("unknown query path")
	}
}
