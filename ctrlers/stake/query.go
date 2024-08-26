package stake

import (
	"fmt"
	types2 "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/ledger"
	"github.com/beatoz/beatoz-go/libs"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmjson "github.com/tendermint/tendermint/libs/json"
	"sort"
)

func (ctrler *StakeCtrler) Query(req abcitypes.RequestQuery) ([]byte, xerrors.XError) {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	switch req.Path {
	case "reward":
		atledger, xerr := ctrler.rewardLedger.ImmutableLedgerAt(req.Height, 0)
		if xerr != nil {
			return nil, xerrors.ErrQuery.Wrap(xerr)
		}
		rwd, xerr := atledger.Read(ledger.ToLedgerKey(req.Data))
		if rwd == nil {
			return nil, xerrors.ErrQuery.Wrap(xerr)
		}
		bz, err := tmjson.Marshal(rwd)
		if err != nil {
			return nil, xerrors.ErrQuery.Wrap(err)
		}
		return bz, nil
	case "stakes":
		atledger, xerr := ctrler.delegateeLedger.ImmutableLedgerAt(req.Height, 0)
		if xerr != nil {
			return nil, xerrors.ErrQuery.Wrap(xerr)
		}

		var stakes []*Stake
		if err := atledger.IterateReadAllItems(func(d *Delegatee) xerrors.XError {
			for _, s0 := range d.Stakes {
				if bytes.Compare(s0.From, types.Address(req.Data)) == 0 {
					stakes = append(stakes, s0)
				}
			}
			return nil
		}); err != nil {
			return nil, xerrors.ErrQuery.Wrap(err)
		} else if bz, err := tmjson.Marshal(stakes); err != nil {
			return nil, xerrors.ErrQuery.Wrap(err)
		} else {
			return bz, nil
		}
	case "delegatee":
		atledger, xerr := ctrler.delegateeLedger.ImmutableLedgerAt(req.Height, 0)
		if xerr != nil {
			return nil, xerrors.ErrQuery.Wrap(xerr)
		}

		if delegatee, xerr := atledger.Read(ledger.ToLedgerKey(req.Data)); xerr != nil {
			if xerr == xerrors.ErrNotFoundResult {
				return nil, xerrors.ErrQuery.Wrap(xerrors.ErrNotFoundDelegatee)
			}
			return nil, xerrors.ErrQuery.Wrap(xerr)
		} else if v, err := tmjson.Marshal(delegatee); err != nil {
			return nil, xerrors.ErrQuery.Wrap(err)
		} else {
			return v, nil
		}
	case "stakes/total_power":
		atledger, xerr := ctrler.delegateeLedger.ImmutableLedgerAt(req.Height, 0)
		if xerr != nil {
			return nil, xerrors.ErrQuery.Wrap(xerr)
		}

		retPower := int64(0)
		xerr = atledger.IterateReadAllItems(func(d *Delegatee) xerrors.XError {
			retPower += d.TotalPower
			return nil
		})
		if xerr != nil {
			return nil, xerrors.ErrQuery.Wrap(xerr)
		}
		return []byte(fmt.Sprintf("%v", retPower)), nil

	case "stakes/voting_power":
		atledger, xerr := ctrler.delegateeLedger.ImmutableLedgerAt(req.Height, 0)
		if xerr != nil {
			return nil, xerrors.ErrQuery.Wrap(xerr)
		}

		var delegatees PowerOrderDelegatees
		xerr = atledger.IterateReadAllItems(func(d *Delegatee) xerrors.XError {
			minPower := types2.AmountToPower(ctrler.govParams.MinValidatorStake())
			if d.SelfPower >= minPower {
				delegatees = append(delegatees, d)
			}
			return nil
		})
		if xerr != nil {
			return nil, xerrors.ErrQuery.Wrap(xerr)
		}

		sort.Sort(delegatees)

		n := libs.MIN(len(delegatees), int(ctrler.govParams.MaxValidatorCnt()))
		validators := delegatees[:n]

		retPower := int64(0)
		for _, v := range validators {
			retPower += v.TotalPower
		}
		return []byte(fmt.Sprintf("%v", retPower)), nil
	default:
		return nil, xerrors.ErrQuery.Wrapf("unknown query path")
	}
}
