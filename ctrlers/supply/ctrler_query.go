package supply

import (
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types/xerrors"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmjson "github.com/tendermint/tendermint/libs/json"
)

func (ctrler *SupplyCtrler) Query(qry abcitypes.RequestQuery) ([]byte, xerrors.XError) {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	switch qry.Path {
	case "reward":
		atledger, xerr := ctrler.supplyState.ImitableLedgerAt(qry.Height)
		if xerr != nil {
			return nil, xerrors.ErrQuery.Wrap(xerr)
		}
		item, xerr := atledger.Get(v1.LedgerKeyReward(qry.Data))
		if xerr != nil {
			return nil, xerrors.ErrQuery.Wrap(xerr)
		}

		rwd, _ := item.(*Reward)
		bz, err := tmjson.Marshal(rwd)
		if err != nil {
			return nil, xerrors.ErrQuery.Wrap(err)
		}
		return bz, nil
	default:
		return nil, xerrors.ErrQuery.Wrapf("unknown query path")
	}
}
