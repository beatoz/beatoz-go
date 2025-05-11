package supply

import (
	"github.com/beatoz/beatoz-go/ctrlers/types"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	btztypes "github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	"github.com/shopspring/decimal"
)

func (ctrler *SupplyCtrler) addReward(rewards []*Reward, poolAddr btztypes.Address) xerrors.XError {
	if poolAddr != nil && !bytes.Equal(poolAddr, btztypes.ZeroAddress()) {
		return ctrler.addRewqrdToPool()
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

func (ctrler *SupplyCtrler) withdrawReward(currReward *Reward, amt *uint256.Int, acctHandler types.IAccountHandler, exec bool) (retXerr xerrors.XError) {
	snap := ctrler.supplyState.Snapshot(exec)
	defer func() {
		if retXerr != nil {
			_ = ctrler.supplyState.RevertToSnapshot(snap, exec)
		}
	}()

	_ = currReward.amt.Sub(currReward.amt, amt)
	if xerr := ctrler.supplyState.Set(v1.LedgerKeyReward(currReward.addr), currReward, exec); xerr != nil {
		return xerr
	}

	if xerr := acctHandler.Reward(currReward.addr, amt, exec); xerr != nil {
		return xerr
	}
	return nil
}

func calculateRewards(weight *types.Weight, mintedAlls, mintedVals decimal.Decimal) []*Reward {
	wa := weight.SumWeight().Truncate(6)
	waVals := weight.ValWeight().Truncate(6)

	beneficiaries := weight.Beneficiaries()
	rewards := make([]*Reward, len(beneficiaries))
	for i, benef := range weight.Beneficiaries() {
		wi := benef.Weight().Truncate(6)

		// for all delegators
		rwd := mintedAlls.Mul(wi).Div(wa)
		if benef.IsValidator() {
			// for only validators
			rwd = rwd.Add(mintedVals.Mul(wi).Div(waVals))
		}
		// give `rwd` to `benef.Address()``
		rewards[i] = &Reward{
			addr: benef.Address(),
			amt:  uint256.MustFromBig(rwd.BigInt()),
		}
	}
	return rewards
}
