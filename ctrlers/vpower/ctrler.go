package vpower

import (
	"fmt"
	cfg "github.com/beatoz/beatoz-go/cmd/config"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/crypto"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmlog "github.com/tendermint/tendermint/libs/log"
	"math"
	"sync"
)

type VPowerCtrler struct {
	vpowsLedger    v1.IStateLedger[*VPowerProto]
	dgteesLedger   v1.IStateLedger[*DelegateeProto]
	allDelegatees  DelegateeArray
	lastValidators DelegateeArray
	vpowLimiter    *VPowerLimiter

	logger tmlog.Logger
	mtx    sync.RWMutex
}

func NewVPowerCtrler(config *cfg.Config, height int64, govParams ctrlertypes.IGovParams, logger tmlog.Logger) (*VPowerCtrler, xerrors.XError) {
	lg := logger.With("module", "beatoz_VPowerCtrler")

	vpowsLedger, xerr := v1.NewStateLedger[*VPowerProto]("vpows", config.DBDir(), 2048, func() *VPowerProto { return &VPowerProto{} }, lg)
	if xerr != nil {
		return nil, xerr
	}

	dgteesLedger, xerr := v1.NewStateLedger[*DelegateeProto]("dgtees", config.DBDir(), 21, func() *DelegateeProto { return &DelegateeProto{} }, lg)
	if xerr != nil {
		return nil, xerr
	}

	dgtees, xerr := LoadAllDelegatees(dgteesLedger, vpowsLedger, height, govParams.RipeningBlocks())
	if xerr != nil {
		return nil, xerr
	}

	lastValidators := selectValidators(dgtees, int(govParams.MaxValidatorCnt()))

	return &VPowerCtrler{
		vpowsLedger:    vpowsLedger,
		dgteesLedger:   dgteesLedger,
		allDelegatees:  dgtees,
		lastValidators: lastValidators,
		vpowLimiter:    nil, //NewVPowerLimiter(dgtees, govParams.MaxValidatorCnt(), govParams.MaxIndividualStakeRatio(), govParams.MaxUpdatableStakeRatio()),
		logger:         lg,
	}, nil
}

// InitLedger creates the voting power of the genesis validators.
func (ctrler *VPowerCtrler) InitLedger(req interface{}) xerrors.XError {
	// init validators
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	initValidators, ok := req.([]abcitypes.ValidatorUpdate)
	if !ok {
		return xerrors.ErrInitChain.Wrapf("wrong parameter: StakeCtrler::InitLedger() requires []*InitStake")
	}

	for _, v := range initValidators {
		dgtee := NewDelegatee(v.PubKey.GetSecp256K1())
		vpow := dgtee.AddPower(dgtee.addr, v.Power, int64(1))

		if xerr := ctrler.vpowsLedger.Set(vpow, true); xerr != nil {
			return xerr
		}
		if xerr := ctrler.dgteesLedger.Set(dgtee.GetDelegateeProto(), true); xerr != nil {
			return xerr
		}

		ctrler.allDelegatees = append(ctrler.allDelegatees, dgtee)
	}

	return nil
}

func (ctrler *VPowerCtrler) BeginBlock(bctx *ctrlertypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	//todo: validator list 재구성
	//todo: slashing
	//todo: reward and signing check

	//ctrler.vpowLimiter.Reset(
	//	ctrler.allDelegatees,
	//	bctx.GovParams.MaxValidatorCnt(),
	//	bctx.GovParams.MaxIndividualStakeRatio(),
	//	bctx.GovParams.MaxUpdatableStakeRatio())
	return nil, nil
}

func (ctrler *VPowerCtrler) ValidateTrx(ctx *ctrlertypes.TrxContext) xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	switch ctx.Tx.GetType() {
	case ctrlertypes.TRX_STAKING:
		q, r := new(uint256.Int).DivMod(ctx.Tx.Amount, ctrlertypes.AmountPerPower(), new(uint256.Int))
		// `ctx.Tx.Amount` MUST be greater than or equal to `AmountPerPower()`
		//    ==> q.Sign() > 0
		if q.Sign() <= 0 {
			return xerrors.ErrInvalidTrx.Wrapf("wrong amount: it should be greater than %v", ctrlertypes.AmountPerPower())
		}
		// `ctx.Tx.Amount` MUST be multiple to `AmountPerPower()`
		//    ==> r.Sign() == 0
		if r.Sign() != 0 {
			return xerrors.ErrInvalidTrx.Wrapf("wrong amount: it should be multiple of %v", ctrlertypes.AmountPerPower())
		}

		txPower := int64(q.Uint64())
		if txPower <= 0 {
			return xerrors.ErrOverFlow.Wrapf("voting power is converted as negative(%v) from amount(%v)", txPower, ctx.Tx.Amount)
		}

		totalPower, selfPower := int64(0), int64(0)

		// todo: Do not find from `allDelegatees`
		val := findByAddr(ctx.Tx.To, ctrler.allDelegatees)
		if bytes.Equal(ctx.Tx.From, ctx.Tx.To) {
			// self bonding
			selfPower = txPower
			if val != nil {
				selfPower += val.SelfPower()
				totalPower = val.TotalPower()
			}

			minPower, xerr := ctrlertypes.AmountToPower(ctx.GovParams.MinValidatorStake())
			if xerr != nil {
				return xerr
			}
			if selfPower < minPower {
				return xerrors.ErrInvalidTrx.Wrapf("too small stake to become validator: a minimum is %v", ctx.GovParams.MinValidatorStake())
			}
		} else if val == nil {
			return xerrors.ErrNotFoundDelegatee.Wrapf("address(%v)", ctx.Tx.To)
		} else {
			// RG-78: check minDelegatorStake
			minDelegatorPower, xerr := ctrlertypes.AmountToPower(ctx.GovParams.MinDelegatorStake())
			if xerr != nil {
				return xerr
			}
			if minDelegatorPower == 0 {
				return xerrors.ErrInvalidTrx.Wrapf("delegating is not allowed yet")
			}
			if minDelegatorPower > 0 && minDelegatorPower > txPower {
				return xerrors.ErrInvalidTrx.Wrapf("too small stake to become delegator: a minimum is %v", ctx.GovParams.MinDelegatorStake())
			}

			// it's delegating. check minSelfStakeRatio
			selfRatio := val.ExpectedSelfStakeRatio(txPower)
			if selfRatio < ctx.GovParams.MinSelfStakeRatio() {
				return xerrors.From(fmt.Errorf("not enough self power of %v: self: %v, total: %v, added: %v", val.addr, val.SelfPower(), val.TotalPower(), txPower))
			}

			totalPower = val.TotalPower()
		}

		// check overflow
		if totalPower > math.MaxInt64-txPower {
			// Not reachable code.
			// The sender's balance is checked at `commonValidation()` at `trx_executor.go`
			// and `txPower` is converted from `ctx.Tx.Amount`.
			// Because of that, overflow can not be occurred.
			return xerrors.ErrOverFlow.Wrapf("validator(%v) power overflow occurs.\ntx:%v", val, ctx.Tx)
		}

		//
		// begin: issue #34: check updatable stake ratio
		if len(ctrler.lastValidators) >= 3 {
			_delg := val
			if _delg == nil {
				_delg = &Delegatee{
					addr:       ctx.Tx.To,
					totalPower: 0,
				}
			}
			if xerr := ctrler.vpowLimiter.CheckLimit(_delg, txPower); xerr != nil {
				return xerrors.ErrUpdatableStakeRatio.Wrap(xerr)
			}
		}
		// end: issue #34: check updatable stake ratio
		//
	case ctrlertypes.TRX_UNSTAKING:
		//
		// begin: issue #34: check updatable stake ratio
		// find delegatee
		// todo: Do not find from `allDelegatees`
		delegatee := findByAddr(ctx.Tx.To, ctrler.allDelegatees)
		if delegatee == nil {
			return xerrors.ErrNotFoundDelegatee.Wrapf("validator(%v)", ctx.Tx.To)
		}

		// find the stake from a delegatee
		txhash := ctx.Tx.Payload.(*ctrlertypes.TrxPayloadUnstaking).TxHash
		if txhash == nil || len(txhash) != 32 {
			return xerrors.ErrInvalidTrxPayloadParams
		}

		vpow, _ := delegatee.DelPowerWithTxHash(ctx.Tx.From, txhash)
		if vpow == nil {
			return xerrors.ErrNotFoundStake
		}

		if ctx.Tx.From.Compare(vpow.From) != 0 {
			return xerrors.ErrNotFoundStake.Wrapf("you are not the stake owner")
		}

		// todo: implement checking updatable limitation.
		//if len(ctrler.lastValidators) >= 3 {
		//	if xerr := ctrler.stakeLimiter.CheckLimit(delegatee, -1*s0.Power); xerr != nil {
		//		return xerrors.ErrUpdatableStakeRatio.Wrap(xerr)
		//	}
		//}
		// end: issue #34: check updatable stake ratio
		//
	case ctrlertypes.TRX_WITHDRAW:
		// todo: implement withdraw reward
		//if ctx.Tx.Amount.Sign() != 0 {
		//	return xerrors.ErrInvalidTrx.Wrapf("amount must be 0")
		//}
		//txpayload, ok := ctx.Tx.Payload.(*ctrlertypes.TrxPayloadWithdraw)
		//if !ok {
		//	return xerrors.ErrInvalidTrxPayloadType
		//}

		//rwd, xerr := ctrler.rewardLedger.Get(ctx.Tx.From, ctx.Exec)
		//if xerr != nil {
		//	return xerr
		//}
		//
		//if txpayload.ReqAmt.Cmp(rwd.cumulated) > 0 {
		//	return xerrors.ErrInvalidTrx.Wrapf("insufficient reward")
		//}
	default:
		return xerrors.ErrUnknownTrxType
	}

	return nil
}

func (ctrler *VPowerCtrler) ExecuteTrx(ctx *ctrlertypes.TrxContext) xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	switch ctx.Tx.GetType() {
	case ctrlertypes.TRX_STAKING:
		return ctrler.execBonding(ctx)
	//case ctrlertypes.TRX_UNSTAKING:
	//	return ctrler.exeUnstaking(ctx)
	//case ctrlertypes.TRX_WITHDRAW:
	//	return ctrler.exeWithdraw(ctx)
	default:
		return xerrors.ErrUnknownTrxType
	}
}

func (ctrler *VPowerCtrler) execBonding(ctx *ctrlertypes.TrxContext) xerrors.XError {
	//delegatee, xerr := findByAddr(ctrler.allDelegatees, ctx.Tx.To)
	//if xerr != nil && xerr != xerrors.ErrNotFoundResult {
	//	return xerr
	//}
	//
	//if delegatee == nil && bytes.Compare(ctx.Tx.From, ctx.Tx.To) == 0 {
	//	// add new delegatee
	//	delegatee = NewDelegatee(ctx.Tx.From, ctx.SenderPubKey)
	//}
	//
	//if delegatee == nil {
	//	// there is no delegatee whose address is ctx.Tx.To
	//	return xerrors.ErrNotFoundDelegatee.Wrapf("address(%v)", ctx.Tx.To)
	//}
	//
	//// Update sender account balance
	//if xerr := ctx.Sender.SubBalance(ctx.Tx.Amount); xerr != nil {
	//	return xerr
	//}
	//_ = ctx.AcctHandler.SetAccount(ctx.Sender, ctx.Exec)
	//
	//// create stake and delegate it to `delegatee`
	//// the reward for this stake will be started at ctx.Height + 1. (issue #29)
	//power, xerr := ctrlertypes.AmountToPower(ctx.Tx.Amount)
	//if xerr != nil {
	//	return xerr
	//}
	//s0 := NewStakeWithPower(ctx.Tx.From, ctx.Tx.To, power, ctx.Height+1, ctx.TxHash)
	//
	//if xerr := delegatee.AddStake(s0); xerr != nil {
	//	return xerr
	//}
	//if xerr := ctrler.delegateeLedger.Set(delegatee, ctx.Exec); xerr != nil {
	//	return xerr
	//}

	return nil
}

func (ctrler *VPowerCtrler) exeUnbonding(ctx *ctrlertypes.TrxContext) xerrors.XError {
	//// find delegatee
	//delegatee, xerr := ctrler.delegateeLedger.Get(ctx.Tx.To, ctx.Exec)
	//if xerr != nil {
	//	return xerr
	//}
	//
	//// delete the stake from a delegatee
	//txhash := ctx.Tx.Payload.(*ctrlertypes.TrxPayloadUnstaking).TxHash
	//if txhash == nil || len(txhash) != 32 {
	//	return xerrors.ErrInvalidTrxPayloadParams
	//}
	//
	//_, s0 := delegatee.FindStake(txhash)
	//if s0 == nil {
	//	return xerrors.ErrNotFoundStake
	//}
	//
	//// issue #43
	//// check that tx's sender is stake's owner
	//if ctx.Tx.From.Compare(s0.From) != 0 {
	//	return xerrors.ErrNotFoundStake.Wrapf("you are not the stake owner")
	//}
	//
	//_ = delegatee.DelStake(txhash)
	//
	//s0.RefundHeight = ctx.Height + ctx.GovParams.LazyUnstakingBlocks()
	//_ = ctrler.frozenLedger.Set(s0, ctx.Exec) // add s0 to frozen ledger
	//
	//if delegatee.SelfPower == 0 {
	//	stakes := delegatee.DelAllStakes()
	//	for _, _s0 := range stakes {
	//		_s0.RefundHeight = ctx.Height + ctx.GovParams.LazyUnstakingBlocks()
	//		_ = ctrler.frozenLedger.Set(_s0, ctx.Exec) // add s0 to frozen ledger
	//	}
	//}
	//
	//if delegatee.TotalPower == 0 {
	//	// this changed delegate will be committed at Commit()
	//	if xerr := ctrler.delegateeLedger.Del(delegatee.Key(), ctx.Exec); xerr != nil {
	//		return xerr
	//	}
	//
	//} else {
	//	// this changed delegate will be committed at Commit()
	//	if xerr := ctrler.delegateeLedger.Set(delegatee, ctx.Exec); xerr != nil {
	//		return xerr
	//	}
	//}

	return nil
}

func (ctrler *VPowerCtrler) EndBlock(bctx *ctrlertypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	return nil, nil
}

func (ctrler *VPowerCtrler) Commit() ([]byte, int64, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	h0, v0, xerr := ctrler.vpowsLedger.Commit()
	if xerr != nil {
		return nil, 0, xerr
	}

	h1, v1, xerr := ctrler.dgteesLedger.Commit()
	if xerr != nil {
		return nil, 0, xerr
	}

	if v0 != v1 {
		return nil, -1, xerrors.ErrCommit.Wrapf("error: VPowerCtrler.Commit() has wrong version - ver0:%v, ver1:%v", v0, v1)
	}
	return crypto.DefaultHash(h0, h1), v0, nil
}

func (ctrler *VPowerCtrler) Close() xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	if ctrler.vpowsLedger != nil {
		if xerr := ctrler.vpowsLedger.Close(); xerr != nil {
			ctrler.logger.Error("vpowsLedger.Close()", "error", xerr.Error())
		}
		ctrler.vpowsLedger = nil
	}
	if ctrler.dgteesLedger != nil {
		if xerr := ctrler.dgteesLedger.Close(); xerr != nil {
			ctrler.logger.Error("dgteesLedger.Close()", "error", xerr.Error())
		}
		ctrler.dgteesLedger = nil
	}
	return nil
}

func (ctrler *VPowerCtrler) Bond(pow, height int64, from types.Address, pubTo bytes.HexBytes, txhash bytes.HexBytes, exec bool) xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	valAddr := crypto.PubKeyBytes2Addr(pubTo)
	val := findByAddr(valAddr, ctrler.allDelegatees)
	if val == nil {
		if bytes.Equal(from, valAddr) {
			// self bonding
			val = NewDelegatee(pubTo)
			ctrler.allDelegatees = append(ctrler.allDelegatees, val)
		} else {
			return xerrors.ErrNotFoundDelegatee
		}
	}

	vpow := val.AddPowerWithTxHash(from, pow, height, txhash)
	if xerr := ctrler.vpowsLedger.Set(vpow, exec); xerr != nil {
		return xerr
	}

	return nil
}

func (ctrler *VPowerCtrler) Unbond(pow int64, from, to types.Address, exec bool) xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	//val := findByAddr(to, ctrler.allDelegatees)
	//if val == nil {
	//	return xerrors.ErrNotFoundDelegatee
	//}
	//
	//snap := ctrler.vpowLedger.Snapshot(exec)
	//
	//removed, updated := val.DelPower(from, pow)
	//
	//if removed != nil && updated != nil {
	//	if xerr := ctrler.vpowLedger.Set(updated, exec); xerr != nil {
	//		_ = ctrler.vpowLedger.RevertToSnapshot(snap, exec)
	//		return xerr
	//	}
	//}
	return nil
}

func (ctrler *VPowerCtrler) Validators() ([]*abcitypes.Validator, int64) {
	//TODO implement me
	panic("implement me")
}

func (ctrler *VPowerCtrler) IsValidator(addr types.Address) bool {
	//TODO implement me
	panic("implement me")
}

func (ctrler *VPowerCtrler) TotalPowerOf(addr types.Address) int64 {
	//TODO implement me
	panic("implement me")
}

func (ctrler *VPowerCtrler) SelfPowerOf(addr types.Address) int64 {
	//TODO implement me
	panic("implement me")
}

func (ctrler *VPowerCtrler) DelegatedPowerOf(addr types.Address) int64 {
	//TODO implement me
	panic("implement me")
}

func (ctrler *VPowerCtrler) Query(query abcitypes.RequestQuery) ([]byte, xerrors.XError) {
	//TODO implement me
	panic("implement me")
}

var _ ctrlertypes.ILedgerHandler = (*VPowerCtrler)(nil)
var _ ctrlertypes.ITrxHandler = (*VPowerCtrler)(nil)
var _ ctrlertypes.IBlockHandler = (*VPowerCtrler)(nil)
var _ ctrlertypes.IStakeHandler = (*VPowerCtrler)(nil)
