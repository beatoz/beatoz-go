package vpower

import (
	"errors"
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
	//frozenLedger v1.IStateLedger[*FrozenVPowerProto]
	vpowsLedger  v1.IStateLedger[*VPower]
	dgteesLedger v1.IStateLedger[*DelegateeV1]

	allDelegatees  []*DelegateeV1
	lastValidators []*DelegateeV1

	vpowLimiter *VPowerLimiter

	logger tmlog.Logger
	mtx    sync.RWMutex
}

func NewVPowerCtrler(config *cfg.Config, height int64, logger tmlog.Logger) (*VPowerCtrler, xerrors.XError) {
	lg := logger.With("module", "beatoz_VPowerCtrler")

	//frozenLedger, xerr := v1.NewStateLedger[*FrozenVPowerProto]("frozen", config.DBDir(), 2048, func() v1.ILedgerItem { return &FrozenVPowerProto{} }, lg)
	//if xerr != nil {
	//	return nil, xerr
	//}

	vpowsLedger, xerr := v1.NewStateLedger[*VPower]("vpows", config.DBDir(), 2048, func() v1.ILedgerItem { return &VPower{} }, lg)
	if xerr != nil {
		return nil, xerr
	}

	dgteesLedger, xerr := v1.NewStateLedger[*DelegateeV1]("dgtees", config.DBDir(), 21, func() v1.ILedgerItem { return &DelegateeV1{} }, lg)
	if xerr != nil {
		return nil, xerr
	}

	return &VPowerCtrler{
		//frozenLedger:   frozenLedger,
		vpowsLedger:  vpowsLedger,
		dgteesLedger: dgteesLedger,
		vpowLimiter:  nil, //NewVPowerLimiter(dgtees, govParams.MaxValidatorCnt(), govParams.MaxIndividualStakeRatio(), govParams.MaxUpdatableStakeRatio()),
		logger:       lg,
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
		//addr := crypto.PubKeyBytes2Addr(v.PubKey.GetSecp256K1())
		dgt := newDelegateeV1(v.PubKey.GetSecp256K1())
		vpow := newVPower(dgt.addr, dgt.PubKey)
		if xerr := ctrler.bondPowerChunk(dgt, vpow, v.Power, int64(1), bytes.ZeroBytes(32), true); xerr != nil {
			return xerr
		}
	}

	return nil
}

func (ctrler *VPowerCtrler) LoadLedger(height, ripeningBlocks int64, maxValCnt int) xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	return ctrler.loadLedger(height, ripeningBlocks, maxValCnt)
}

func (ctrler *VPowerCtrler) loadLedger(height, ripeningBlocks int64, maxValCnt int) xerrors.XError {
	dgtees, xerr := LoadAllDelegateeV1(ctrler.dgteesLedger)
	if xerr != nil {
		return xerr
	}

	ctrler.allDelegatees = dgtees
	ctrler.lastValidators = selectValidators(dgtees, maxValCnt)
	return nil
}

func (ctrler *VPowerCtrler) BeginBlock(bctx *ctrlertypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	//todo: all validator list 재구성
	//todo: slashing
	//todo: reward and signing check

	//ctrler.vpowLimiter.Reset(
	//	ctrler.allDelegatees,
	//	bctx.GovParams.MaxValidatorCnt(),
	//	bctx.GovParams.MaxIndividualStakeRatio(),
	//	bctx.GovParams.MaxUpdatableStakeRatio())
	return nil, nil
}

type bondingTrxOpt struct {
	dgtee   *DelegateeV1
	vpow    *VPower
	txPower int64
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

		// NOTE: Do not find from `allDelegatees`.
		// Only if there is no update on allDelegatees, it's possible to find from `allDelegatees`.
		dgtee, xerr := ctrler.dgteesLedger.Get(dgteeProtoKey(ctx.Tx.To), ctx.Exec)
		if xerr != nil && !errors.Is(xerr, xerrors.ErrNotFoundResult) {
			return xerr
		}
		if bytes.Equal(ctx.Tx.From, ctx.Tx.To) {
			// self bonding
			selfPower = txPower
			if dgtee != nil {
				selfPower += dgtee.SelfPower
				totalPower = dgtee.TotalPower
			}

			minPower, xerr := ctrlertypes.AmountToPower(ctx.GovParams.MinValidatorStake())
			if xerr != nil {
				return xerr
			}
			if selfPower < minPower {
				return xerrors.ErrInvalidTrx.Wrapf("too small stake to become validator: %v < %v(minimum)", ctx.Tx.Amount.Dec(), ctx.GovParams.MinValidatorStake())
			}
		} else {
			if dgtee == nil {
				return xerrors.ErrNotFoundDelegatee.Wrapf("address(%v)", ctx.Tx.To)
			}

			// RG-78: check minDelegatorStake
			minDelegatorPower, xerr := ctrlertypes.AmountToPower(ctx.GovParams.MinDelegatorStake())
			if xerr != nil {
				return xerr
			}
			if minDelegatorPower == 0 {
				return xerrors.ErrInvalidTrx.Wrapf("delegating is not allowed yet")
			}
			if minDelegatorPower > 0 && minDelegatorPower > txPower {
				return xerrors.ErrInvalidTrx.Wrapf("too small stake to become delegator: %v < %v", ctx.Tx.Amount.Dec(), ctx.GovParams.MinDelegatorStake())
			}

			// it's delegating. check minSelfStakeRatio
			selfatio := dgtee.SelfPower * int64(100) / (dgtee.TotalPower + txPower)
			if selfatio < ctx.GovParams.MinSelfStakeRatio() {
				return xerrors.From(fmt.Errorf("not enough self power of %v: self: %v, total: %v, new power: %v", dgtee.addr, dgtee.SelfPower, dgtee.TotalPower, txPower))
			}

			totalPower = dgtee.TotalPower
		}

		// check overflow
		if totalPower > math.MaxInt64-txPower {
			// Not reachable code.
			// The sender's balance is checked at `commonValidation()` at `trx_executor.go`
			// and `txPower` is converted from `ctx.Tx.Amount`.
			// Because of that, overflow can not be occurred.
			return xerrors.ErrOverFlow.Wrapf("validator(%v) power overflow occurs.\ntx:%v", ctx.Tx.To, ctx.Tx)
		}

		//
		// todo: Implement stake limiter
		//

		// set the result of ValidateTrx
		ctx.ValidateResult = &bondingTrxOpt{
			dgtee:   dgtee, // it is nil in self-bonding
			vpow:    nil,
			txPower: txPower,
		}

	case ctrlertypes.TRX_UNSTAKING:
		// todo: Resolve issue #34: check updatable stake ratio

		// NOTE: Do not find from `allDelegatees`.
		// `ctx.Tx.To` must already be a delegator (or validator), so it should be found in the `dgteesLedger`.
		dgtee, xerr := ctrler.dgteesLedger.Get(dgteeProtoKey(ctx.Tx.To), ctx.Exec)
		if xerr != nil {
			return xerrors.ErrNotFoundDelegatee.Wrap(xerr)
		}

		// find the voting power from a delegatee
		txhash := ctx.Tx.Payload.(*ctrlertypes.TrxPayloadUnstaking).TxHash
		if txhash == nil || len(txhash) != 32 {
			return xerrors.ErrInvalidTrxPayloadParams
		}

		// Since the bonding tx pointed to by `txhash` must have already been executed
		// and created a voting chunk as a result,
		// the voting power chunk with `txhash` must be found in `vpowsLedger`.
		vpow, xerr := ctrler.vpowsLedger.Get(vpowerProtoKey(ctx.Tx.From, ctx.Tx.To), ctx.Exec)
		if xerr != nil {
			return xerrors.ErrNotFoundStake.Wrap(xerr)
		}
		pc := vpow.findPowerChunk(txhash)
		if pc == nil {
			return xerrors.ErrNotFoundStake
		}

		// todo: implement checking updatable limitation.

		// set the result of ValidateTrx
		ctx.ValidateResult = &bondingTrxOpt{
			dgtee:   dgtee,
			vpow:    vpow,
			txPower: pc.Power,
		}
	case ctrlertypes.TRX_WITHDRAW:
		// todo: implement withdraw reward

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
	case ctrlertypes.TRX_UNSTAKING:
		return ctrler.exeUnbonding(ctx)
	//case ctrlertypes.TRX_WITHDRAW:
	//	return ctrler.exeWithdraw(ctx)
	default:
		return xerrors.ErrUnknownTrxType
	}
}

func (ctrler *VPowerCtrler) execBonding(ctx *ctrlertypes.TrxContext) xerrors.XError {
	// NOTE: DO NOT FIND a delegatee from `allDelegatees`.
	// If `allDelegatees` is updated, unexpected results may occur in CheckTx etc.
	dgtee := ctx.ValidateResult.(*bondingTrxOpt).dgtee
	if dgtee == nil && bytes.Compare(ctx.Tx.From, ctx.Tx.To) == 0 {
		// self bonding: add new delegatee
		dgtee = newDelegateeV1(ctx.SenderPubKey)
	}

	if dgtee == nil {
		// `newDelegateeV1` does not fail, so this code is not reachable.
		// there is no delegatee whose address is ctx.Tx.To
		return xerrors.ErrNotFoundDelegatee.Wrapf("address(%v)", ctx.Tx.To)
	}

	var vpow *VPower
	power := ctx.ValidateResult.(*bondingTrxOpt).txPower

	if dgtee.hasDelegator(ctx.Tx.From) {
		_vpow, xerr := ctrler.vpowsLedger.Get(vpowerProtoKey(ctx.Tx.From, dgtee.addr), ctx.Exec)
		if xerr != nil {
			return xerr
		}
		vpow = _vpow
	} else {
		vpow = newVPower(ctx.Tx.From, dgtee.PubKey)
	}
	if xerr := ctrler.bondPowerChunk(
		dgtee, vpow,
		power, ctx.Height, ctx.TxHash,
		ctx.Exec); xerr != nil {
		return xerr
	}

	// Update sender account balance
	if xerr := ctx.Sender.SubBalance(ctx.Tx.Amount); xerr != nil {
		return xerr
	}
	_ = ctx.AcctHandler.SetAccount(ctx.Sender, ctx.Exec)

	return nil
}

func (ctrler *VPowerCtrler) exeUnbonding(ctx *ctrlertypes.TrxContext) xerrors.XError {
	var freezingPowerChunks []*PowerChunk

	// find delegatee
	dgtee := ctx.ValidateResult.(*bondingTrxOpt).dgtee
	if dgtee == nil {
		panic("not reachable")
	}

	// remove the voting power chunk from a delegatee
	txhash := ctx.Tx.Payload.(*ctrlertypes.TrxPayloadUnstaking).TxHash
	if txhash == nil {
		panic("not reachable")
	}
	vpow := ctx.ValidateResult.(*bondingTrxOpt).vpow
	if vpow == nil {
		panic("not reachable")
	}

	//
	// Remove power
	//
	if pc, xerr := ctrler.unbondPowerChunk(dgtee, vpow, txhash, ctx.Exec); xerr != nil {
		return xerr
	} else {
		freezingPowerChunks = append(freezingPowerChunks, pc)
	}

	if dgtee.SelfPower == 0 {
		// todo: un-bonding all voting powers delegated to `dgteeProto`
		for _, _from := range dgtee.Delegators {
			_vpow, xerr := ctrler.vpowsLedger.Get(vpowerProtoKey(_from, dgtee.addr), ctx.Exec)
			if xerr != nil && !errors.Is(xerr, xerrors.ErrNotFoundResult) {
				return xerr
			}
			if _vpow != nil {
				freezingPowerChunks = append(freezingPowerChunks, _vpow.PowerChunks...)
				if xerr := ctrler.vpowsLedger.Del(_vpow.Key(), ctx.Exec); xerr != nil {
					return xerr
				}
			}
		}
		if xerr := ctrler.dgteesLedger.Del(dgtee.Key(), ctx.Exec); xerr != nil {
			return xerr
		}
	}

	//
	// todo: freeze the power chunk `pc` deleted from `vpowsLedger`
	//
	//refundHeight := ctx.Height + ctx.GovParams.LazyUnstakingBlocks()
	//frozen := &FrozenVPowerProto{
	//	newVPower(vpow.From, nil, pc.Power, refundHeight),
	//}
	//_ = ctrler.frozenLedger.Set(frozen, ctx.Exec) // add s0 to frozen ledger

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
