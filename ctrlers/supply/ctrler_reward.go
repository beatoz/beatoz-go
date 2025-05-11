package supply

import (
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	btztypes "github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
)

func (ctrler *SupplyCtrler) reward(rewards []*Reward, poolAddr btztypes.Address) xerrors.XError {
	if poolAddr != nil && !bytes.Equal(poolAddr, btztypes.ZeroAddress()) {
		return ctrler.rewqrdToPool()
	}

	for _, nrwd := range rewards {
		item, xerr := ctrler.supplyState.Get(v1.LedgerKeyReward(nrwd.addr), true)
		if xerr != nil && !xerr.Contains(xerrors.ErrNotFoundResult) {
			return xerr
		}

		rwd, _ := item.(*Reward)
		if rwd == nil {
			rwd = &Reward{
				addr: nrwd.addr,
				amt:  nrwd.amt,
			}
		} else {
			rwd.amt.Add(rwd.amt, nrwd.amt)
		}

		if xerr := ctrler.supplyState.Set(v1.LedgerKeyReward(nrwd.addr), rwd, true); xerr != nil {
			return xerr
		}
	}

	return nil
}

func (ctrler *SupplyCtrler) rewqrdToPool() xerrors.XError {
	panic("not supported yet")
}

func (ctrler *SupplyCtrler) readReward(addr btztypes.Address) (*Reward, xerrors.XError) {
	item, xerr := ctrler.supplyState.Get(v1.LedgerKeyReward(addr), true)
	if xerr != nil {
		return nil, xerr
	}

	return item.(*Reward), nil
}
