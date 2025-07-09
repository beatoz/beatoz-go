package supply

import (
	"fmt"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/libs/jsonx"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

func (ctrler *SupplyCtrler) Query(req abcitypes.RequestQuery, opts ...ctrlertypes.Option) ([]byte, xerrors.XError) {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	switch req.Path {
	case "reward":
		return ctrler.queryReward(req.Height, types.Address(req.Data))
	case "total_supply":
		return ctrler.queryTotalSupply(req.Height)
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
	bz, err := jsonx.Marshal(rwd)
	if err != nil {
		return nil, xerrors.ErrQuery.Wrap(err)
	}
	return bz, nil
}

func (ctrler *SupplyCtrler) queryTotalSupply(height int64) ([]byte, xerrors.XError) {
	atledger, xerr := ctrler.supplyState.ImitableLedgerAt(height)
	if xerr != nil {
		return nil, xerrors.ErrQuery.Wrap(xerr)
	}

	// get supply info from ledger
	item, xerr := atledger.Get(v1.LedgerKeyTotalSupply())
	if xerr != nil {
		return nil, xerrors.ErrQuery.Wrap(xerr)
	}
	supply, _ := item.(*Supply)
	if supply == nil {
		return nil, xerrors.ErrQuery.Wrap(xerrors.ErrNotFoundResult)
	}

	return []byte(fmt.Sprintf("\"%v\"", supply.totalSupply.Dec())), nil
}
