package supply

import (
	"github.com/beatoz/beatoz-go/ctrlers/types"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	btztypes "github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
)

// addReward distributes applies `mintedReward` to `supplyState`.
// this is called from waitMint that is called from EndBlock.
func (ctrler *SupplyCtrler) addReward(rewards []*mintedReward, height int64, poolAddr btztypes.Address) xerrors.XError {
	if poolAddr != nil && !bytes.Equal(poolAddr, btztypes.ZeroAddress()) {
		return ctrler.addRewqrdToPool()
	} else {
		return ctrler.addRewardToState(rewards, height)
	}
}

func (ctrler *SupplyCtrler) addRewardToState(rewards []*mintedReward, height int64) xerrors.XError {
	for _, nrwd := range rewards {
		item, xerr := ctrler.supplyState.Get(v1.LedgerKeyReward(nrwd.addr), true)
		if xerr != nil && !xerr.Contains(xerrors.ErrNotFoundResult) {
			return xerr
		}

		rwd, _ := item.(*Reward)
		if rwd == nil {
			rwd = NewReward(nrwd.addr)
		}
		_ = rwd.Issue(nrwd.amt, height)

		if xerr := ctrler.supplyState.Set(v1.LedgerKeyReward(nrwd.addr), rwd, true); xerr != nil {
			return xerr
		}
	}
	return nil
}

func (ctrler *SupplyCtrler) addRewqrdToPool() xerrors.XError {
	panic("not supported yet")
}

func (ctrler *SupplyCtrler) readReward(addr btztypes.Address) (*Reward, xerrors.XError) {
	item, xerr := ctrler.supplyState.Get(v1.LedgerKeyReward(addr), true)
	if xerr != nil {
		return nil, xerr
	}

	return item.(*Reward), nil
}

// withdrawReward calls `currReward.Withdraw` to refund `amt` from `currReward`.
// this is called from ExecuteTrx.
func (ctrler *SupplyCtrler) withdrawReward(currReward *Reward, amt *uint256.Int, height int64, acctHandler types.IAccountHandler, exec bool) xerrors.XError {
	_ = currReward.Withdraw(amt, height)
	if currReward.CumulatedAmount().IsZero() {
		if xerr := ctrler.supplyState.Del(v1.LedgerKeyReward(currReward.Address()), exec); xerr != nil {
			return xerr
		}
	} else {
		if xerr := ctrler.supplyState.Set(v1.LedgerKeyReward(currReward.Address()), currReward, exec); xerr != nil {
			return xerr
		}
	}

	if xerr := acctHandler.Reward(currReward.Address(), amt, exec); xerr != nil {
		return xerr
	}
	return nil
}
