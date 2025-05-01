package vpower

import (
	bytes2 "bytes"
	"errors"
	"fmt"
	cfg "github.com/beatoz/beatoz-go/cmd/config"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmlog "github.com/tendermint/tendermint/libs/log"
	"math"
	"sync"
)

type VPowerCtrler struct {
	powersState v1.IStateLedger

	allDelegatees  []*Delegatee
	lastValidators []*Delegatee

	vpowLimiter *VPowerLimiter

	logger tmlog.Logger
	mtx    sync.RWMutex
}

func defaultNewItem(key v1.LedgerKey) v1.ILedgerItem {
	if bytes2.HasPrefix(key, v1.KeyPrefixVPower) {
		return &VPower{}
	} else if bytes2.HasPrefix(key, v1.KeyPrefixDelegatee) {
		return &Delegatee{}
	} else if bytes2.HasPrefix(key, v1.KeyPrefixFrozenVPower) {
		return &FrozenVPower{}
	}
	panic(fmt.Errorf("invalid key prefix:0x%x", key[0]))
}

func NewVPowerCtrler(config *cfg.Config, maxValCnt int, logger tmlog.Logger) (*VPowerCtrler, xerrors.XError) {
	lg := logger.With("module", "beatoz_VPowerCtrler")

	powersState, xerr := v1.NewStateLedger("vpows", config.DBDir(), 21*2048, defaultNewItem, lg)
	if xerr != nil {
		return nil, xerr
	}

	ret := &VPowerCtrler{
		powersState: powersState,
		vpowLimiter: nil, //NewVPowerLimiter(dgtees, govParams.MaxValidatorCnt(), govParams.MaxIndividualStakeRatio(), govParams.MaxUpdatableStakeRatio()),
		logger:      lg,
	}
	if xerr := ret.LoadDelegatees(maxValCnt); xerr != nil {
		return nil, xerr
	}
	return ret, nil
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

	var dgtees []*Delegatee
	var lastVals []*Delegatee
	for _, v := range initValidators {
		dgt := newDelegatee(v.PubKey.GetSecp256K1())
		vpow := newVPower(dgt.addr, dgt.PubKey)
		if xerr := ctrler.bondPowerChunk(dgt, vpow, v.Power, int64(1), bytes.ZeroBytes(32), true); xerr != nil {
			return xerr
		}
		dgtees = append(dgtees, dgt)
	}

	if len(dgtees) > 0 {
		// In `InitLedger`, all delegatees become the initial validator set.
		lastVals = selectValidators(dgtees, len(dgtees))
	}
	ctrler.allDelegatees = dgtees
	ctrler.lastValidators = lastVals

	return nil
}

func (ctrler *VPowerCtrler) LoadDelegatees(maxValCnt int) xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	dgtees, xerr := ctrler.loadDelegatees(true)
	if xerr != nil {
		return xerr
	}

	var lastVals []*Delegatee
	if dgtees != nil {
		lastVals = selectValidators(dgtees, maxValCnt)
	}

	ctrler.allDelegatees = dgtees
	ctrler.lastValidators = lastVals
	return nil
}

func (ctrler *VPowerCtrler) BeginBlock(bctx *ctrlertypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	if bctx.Height()%bctx.GovParams.InflationCycleBlocks() == 0 {
		// calculate reward...
	}

	if bctx.Height()%bctx.GovParams.LazyUnstakingBlocks() == 0 {
		//todo: signing check and reward
	}

	//todo: slashing

	//ctrler.vpowLimiter.Reset(
	//	ctrler.allDelegatees,
	//	bctx.GovParams.MaxValidatorCnt(),
	//	bctx.GovParams.MaxIndividualStakeRatio(),
	//	bctx.GovParams.MaxUpdatableStakeRatio())
	return nil, nil
}

type bondingTrxOpt struct {
	dgtee   *Delegatee
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
		dgtee, xerr := ctrler.readDelegatee(ctx.Tx.To, ctx.Exec)
		if xerr != nil && !errors.Is(xerr, xerrors.ErrNotFoundResult) {
			return xerr
		}

		if bytes.Equal(ctx.Tx.From, ctx.Tx.To) {
			// self bonding
			selfPower = txPower
			if dgtee != nil {
				selfPower += dgtee.SelfPower
				totalPower = dgtee.SumPower
			}

			if selfPower < ctx.GovParams.MinValidatorPower() {
				return xerrors.ErrInvalidTrx.Wrapf("too small power to become validator: %v < %v(minimum)", txPower, ctx.GovParams.MinValidatorPower())
			}
		} else {
			if dgtee == nil {
				return xerrors.ErrNotFoundDelegatee.Wrapf("address(%v)", ctx.Tx.To)
			}

			// RG-78: check minDelegatorPower
			minDelegatorPower := ctx.GovParams.MinDelegatorPower()
			if minDelegatorPower > txPower {
				return xerrors.ErrInvalidTrx.Wrapf("too small stake to become delegator: %v < %v", txPower, minDelegatorPower)
			}

			// it's delegating. check minSelfStakeRatio
			selfatio := dgtee.SelfPower * int64(100) / (dgtee.SumPower + txPower)
			if selfatio < ctx.GovParams.MinSelfStakeRatio() {
				return xerrors.From(fmt.Errorf("not enough self power of %v: self: %v, total: %v, new power: %v", dgtee.addr, dgtee.SelfPower, dgtee.SumPower, txPower))
			}

			totalPower = dgtee.SumPower
		}

		// check overflow
		if totalPower > math.MaxInt64-txPower {
			// Not reachable code.
			// The sender's balance is checked at `commonValidation()` at `trx_executor.go`
			// and `txPower` is converted from `ctx.Tx.Amount`.
			// Because of that, overflow can not be occurred.
			return xerrors.ErrOverFlow.Wrapf("validator(%v) power overflow occurs.\ntx:%v", ctx.Tx.To, ctx.Tx)
		}

		{
			//
			// todo: Implement stake limiter
			//
		}

		// set the result of ValidateTrx
		ctx.ValidateResult = &bondingTrxOpt{
			dgtee:   dgtee, // it is nil in self-bonding
			vpow:    nil,
			txPower: txPower,
		}

	case ctrlertypes.TRX_UNSTAKING:

		// NOTE: Do not find from `allDelegatees`.
		// `ctx.Tx.To` must already be a delegator (or validator), so it should be found in the `dgteesLedger`.
		dgtee, xerr := ctrler.readDelegatee(ctx.Tx.To, ctx.Exec)
		if xerr != nil {
			return xerrors.ErrNotFoundDelegatee.Wrap(xerr)
		}

		// find the voting power from a delegatee
		txhash := ctx.Tx.Payload.(*ctrlertypes.TrxPayloadUnstaking).TxHash
		if txhash == nil || len(txhash) != 32 {
			return xerrors.ErrInvalidTrxPayloadParams
		}

		// Since the bonding tx identified by `txhash` must have already been executed
		// and created a power chunk as a result,
		// the voting power chunk with `txhash` must be found.
		vpow, xerr := ctrler.readVPower(ctx.Tx.From, ctx.Tx.To, ctx.Exec)
		if xerr != nil {
			return xerrors.ErrNotFoundStake.Wrap(xerr)
		}

		pc := vpow.findPowerChunk(txhash)
		if pc == nil {
			//fmt.Printf("-------------------------------f:%x, t:%x, h:%x\n", vpow.From, vpow.to, txhash)
			//for i, _pc := range vpow.PowerChunks {
			//	fmt.Printf("-------------------------------[%d] %x\n", i, _pc.TxHash)
			//}
			return xerrors.ErrNotFoundStake
		}

		{
			//
			// todo: implement checking updatable limitation.
			// todo: Resolve issue #34: check updatable stake ratio
			//
		}

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
		dgtee = newDelegatee(ctx.SenderPubKey)
	}

	if dgtee == nil {
		// `newDelegatee` does not fail, so this code is not reachable.
		// there is no delegatee whose address is ctx.Tx.To
		return xerrors.ErrNotFoundDelegatee.Wrapf("address(%v)", ctx.Tx.To)
	}

	var vpow *VPower
	power := ctx.ValidateResult.(*bondingTrxOpt).txPower

	if dgtee.hasDelegator(ctx.Tx.From) {
		_vpow, xerr := ctrler.readVPower(ctx.Tx.From, dgtee.addr, ctx.Exec)
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

	// found delegatee
	dgtee := ctx.ValidateResult.(*bondingTrxOpt).dgtee
	if dgtee == nil {
		panic("not reachable")
	}

	// the power chunk pointed by txhash will be frozen (removed from `dgtee`)
	txhash := ctx.Tx.Payload.(*ctrlertypes.TrxPayloadUnstaking).TxHash
	if txhash == nil {
		panic("not reachable")
	}
	vpow := ctx.ValidateResult.(*bondingTrxOpt).vpow
	if vpow == nil {
		panic("not reachable")
	}

	refundHeight := ctx.Height + ctx.GovParams.LazyUnstakingBlocks()

	//
	// Remove power
	//

	if pc, xerr := ctrler.unbondPowerChunk(dgtee, vpow, txhash, ctx.Exec); xerr != nil {
		return xerr
	} else if xerr = ctrler.freezePowerChunk(vpow.From, pc, refundHeight, ctx.Exec); xerr != nil {
		return xerr
	}

	if dgtee.SelfPower == 0 {
		// un-bonding all vpowers delegated to `dgtee`
		for _, _from := range dgtee.Delegators {
			_vpow, xerr := ctrler.readVPower(_from, dgtee.addr, ctx.Exec)
			if xerr != nil && !errors.Is(xerr, xerrors.ErrNotFoundResult) {
				return xerr
			}

			if _vpow != nil {
				if xerr := ctrler.delVPower(_vpow.From, _vpow.to, ctx.Exec); xerr != nil {
					return xerr
				}

				for _, _pc := range _vpow.PowerChunks {
					// freeze all power chunks that the `_vpow` has
					if xerr = ctrler.freezePowerChunk(_vpow.From, _pc, refundHeight, ctx.Exec); xerr != nil {
						return xerr
					}
				}
			}
		}
		if xerr := ctrler.delDelegatee(dgtee.addr, ctx.Exec); xerr != nil {
			return xerr
		}
	}

	return nil
}

func (ctrler *VPowerCtrler) EndBlock(bctx *ctrlertypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	if xerr := ctrler.unfreezePowerChunk(bctx); xerr != nil {
		return nil, xerr
	}

	//
	// read all delegatee list again.
	// it will be used to calculate new validator
	//
	// NOTE:
	// `loadDelegatees()` returns all delegatees, which are updated by the bonding txs in this block(`bctx.Height()`).
	// (At `EndBlock()`, the transactions in the current block have been executed to ledger, but not committed yet.)
	// So, if the bonding tx(including TrxPayloadStaking/TrxPayloadUnStaking) is executed and the stake is saved at block height `N`,
	//     the updated validators is notified to the consensus engine via `EndBlock()` of block height `N`,
	//	   the consensus engine applies these accounts to the ValidatorSet at block height `(N)+2`.
	//	   (Refer to the comments in `updateState(...)` at github.com/tendermint/tendermint@v0.34.20/state/execution.go)
	// So, the accounts can start signing from block height `N+2`
	// and the Beatoz can check the signatures through `lastVotes` from the block height `N+3`.
	dgtees, xerr := ctrler.loadDelegatees(true)
	if xerr != nil {
		return nil, xerr
	}
	ctrler.allDelegatees = dgtees

	newValUps := ctrler.updateValidators(int(bctx.GovParams.MaxValidatorCnt()))
	bctx.SetValUpdates(newValUps)

	return nil, nil
}

func (ctrler *VPowerCtrler) Commit() ([]byte, int64, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	h0, v0, xerr := ctrler.powersState.Commit()
	if xerr != nil {
		return nil, 0, xerr
	}

	return h0, v0, nil
}

func (ctrler *VPowerCtrler) Close() xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	if ctrler.powersState != nil {
		if xerr := ctrler.powersState.Close(); xerr != nil {
			ctrler.logger.Error("powersState.Close()", "error", xerr.Error())
		}
		ctrler.powersState = nil
	}
	return nil
}

func (ctrler *VPowerCtrler) Validators() ([]*abcitypes.Validator, int64) {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	totalPower := int64(0)
	var ret []*abcitypes.Validator
	for _, v := range ctrler.lastValidators {
		totalPower += v.SumPower
		ret = append(ret, &abcitypes.Validator{
			Address: v.addr,
			Power:   v.SumPower,
		})
	}

	return ret, totalPower
}

func (ctrler *VPowerCtrler) IsValidator(addr types.Address) bool {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	for _, v := range ctrler.lastValidators {
		if bytes.Equal(v.addr, addr) {
			return true
		}
	}
	return false
}

func (ctrler *VPowerCtrler) SumPowerOf(addr types.Address) int64 {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	for _, dgtee := range ctrler.allDelegatees {
		if bytes.Equal(dgtee.addr, addr) {
			return dgtee.SumPower
		}
	}
	return 0
}

// DEPRECATED
func (ctrler *VPowerCtrler) TotalPowerOf(addr types.Address) int64 {
	return ctrler.SumPowerOf(addr)
}

func (ctrler *VPowerCtrler) SelfPowerOf(addr types.Address) int64 {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	for _, dgtee := range ctrler.allDelegatees {
		if bytes.Equal(dgtee.addr, addr) {
			return dgtee.SelfPower
		}
	}
	return 0
}

func (ctrler *VPowerCtrler) PowerOf(addr types.Address) int64 {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	delegatePower := int64(0)
	_ = ctrler.seekVPowersOf(addr, func(key v1.LedgerKey, item v1.ILedgerItem) xerrors.XError {
		vpow, _ := item.(*VPower)
		delegatePower += vpow.SumPower
		return nil
	}, true)

	return delegatePower
}

// DEPRECATED
func (ctrler *VPowerCtrler) DelegatedPowerOf(addr types.Address) int64 {
	return ctrler.PowerOf(addr)
}

func (ctrler *VPowerCtrler) Query(query abcitypes.RequestQuery) ([]byte, xerrors.XError) {
	//TODO implement me
	panic("implement me")
}

var _ ctrlertypes.ILedgerHandler = (*VPowerCtrler)(nil)
var _ ctrlertypes.ITrxHandler = (*VPowerCtrler)(nil)
var _ ctrlertypes.IBlockHandler = (*VPowerCtrler)(nil)
var _ ctrlertypes.IStakeHandler = (*VPowerCtrler)(nil)
