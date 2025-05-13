package supply

import (
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmjson "github.com/tendermint/tendermint/libs/json"
)

func (ctrler *SupplyCtrler) Query(qry abcitypes.RequestQuery) ([]byte, xerrors.XError) {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	switch qry.Path {
	case "reward":
		return ctrler.queryReward(qry.Height, types.Address(qry.Data))
	default:
		return nil, xerrors.ErrQuery.Wrapf("unknown query path")
	}
}

func (ctrler *SupplyCtrler) queryReward(height int64, address types.Address) ([]byte, xerrors.XError) {
	atledger, xerr := ctrler.supplyState.ImitableLedgerAt(height)
	if xerr != nil {
		return nil, xerrors.ErrQuery.Wrap(xerr)
	}
	item, xerr := atledger.Get(v1.LedgerKeyReward(address))
	if xerr != nil && !xerr.Contains(xerrors.ErrNotFoundResult) {
		return nil, xerrors.ErrQuery.Wrap(xerr)
	}

	rwd, _ := item.(*Reward)
	if rwd == nil {
		rwd = NewReward(address) // all fields are 0
	}
	bz, err := tmjson.Marshal(rwd)
	if err != nil {
		return nil, xerrors.ErrQuery.Wrap(err)
	}
	return bz, nil
}
