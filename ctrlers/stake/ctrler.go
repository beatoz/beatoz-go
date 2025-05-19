package stake

import (
	"fmt"
	cfg "github.com/beatoz/beatoz-go/cmd/config"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/libs"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/crypto"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmlog "github.com/tendermint/tendermint/libs/log"
	"math"
	"sort"
	"strconv"
	"sync"
)

// DEPRECATED
type InitStake struct {
	PubKeys bytes.HexBytes
	Stakes  []*Stake
}

// DEPRECATED
type StakeCtrler struct {
	rwdHashDB *ctrlertypes.MetaDB

	allDelegatees     DelegateeArray
	lastValidators    DelegateeArray
	delegateeLedger   v1.IStateLedger
	frozenLedger      v1.IStateLedger
	rewardLedger      v1.IStateLedger
	rwdLedgUpInterval int64
	lastRwdHash       []byte
	stakeLimiter      *StakeLimiter
	govParams         ctrlertypes.IGovParams

	logger tmlog.Logger
	mtx    sync.RWMutex
}

func NewStakeCtrler(config *cfg.Config, govHandler ctrlertypes.IGovParams, logger tmlog.Logger) (*StakeCtrler, xerrors.XError) {
	rwdHashDB, err := ctrlertypes.OpenMetaDB("beatoz_app_rwd_hash", config.DBDir())
	if err != nil {
		panic(err)
	}

	newDelegateeProvider := func(key v1.LedgerKey) v1.ILedgerItem { return &Delegatee{} }
	newStakeProvider := func(key v1.LedgerKey) v1.ILedgerItem { return &Stake{} }
	newRewardProvider := func(key v1.LedgerKey) v1.ILedgerItem { return &Reward{} }

	lg := logger.With("module", "beatoz_StakeCtrler")

	// for all delegatees
	delegateeLedger, xerr := v1.NewStateLedger("delegatees", config.DBDir(), 2048, newDelegateeProvider, lg)
	if xerr != nil {
		return nil, xerr
	}

	frozenLedger, xerr := v1.NewStateLedger("frozen", config.DBDir(), 2048, newStakeProvider, lg)
	if xerr != nil {
		return nil, xerr
	}

	rewardLedger, xerr := v1.NewStateLedger("rewards", config.DBDir(), 2048, newRewardProvider, lg)
	if xerr != nil {
		return nil, xerr
	}

	ret := &StakeCtrler{
		rwdHashDB:         rwdHashDB,
		delegateeLedger:   delegateeLedger,
		frozenLedger:      frozenLedger,
		rewardLedger:      rewardLedger,
		rwdLedgUpInterval: int64(10),
		lastRwdHash:       rwdHashDB.LastRewardHash(),
		stakeLimiter:      NewStakeLimiter(nil, govHandler.MaxValidatorCnt(), govHandler.MaxIndividualStakeRate(), govHandler.MaxUpdatableStakeRate()),
		govParams:         govHandler,
		logger:            lg,
	}
	ret.logger.Debug("get last reward hash", "lastRwdHash", bytes.HexBytes(ret.lastRwdHash))

	// set `lastValidators` of StakeCtrler
	_ = ret.UpdateValidators(int(govHandler.MaxValidatorCnt()))

	return ret, nil
}

func (ctrler *StakeCtrler) InitLedger(req interface{}) xerrors.XError {
	// init validators
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	initStakes, ok := req.([]*InitStake)
	if !ok {
		return xerrors.ErrInitChain.Wrapf("wrong parameter: StakeCtrler::InitLedger() requires []*InitStake")
	}

	for _, initS0 := range initStakes {
		for _, s0 := range initS0.Stakes {
			d := NewDelegatee(s0.To, initS0.PubKeys)
			if xerr := d.AddStake(s0); xerr != nil {
				return xerr
			}
			if xerr := ctrler.delegateeLedger.Set(d.Key(), d, true); xerr != nil {
				return xerr
			}
		}
	}

	return nil
}

// BeginBlock are called in BeatozApp::BeginBlock
func (ctrler *StakeCtrler) BeginBlock(blockCtx *ctrlertypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	//
	// Begin of code from EndBlock
	//
	ctrler.allDelegatees = nil
	// NOTE:
	// Iterate() returns delegatees, which are committed at previous block(which height is `blockCtx.Height() - 1`).
	// (At `BeginBlock()`, the transactions in the current block is not applied to ledger yet.)
	// So, if the staking tx(including TrxPayloadStaking) is executed and the stake is saved(committed) at block height `N`,
	//     the updated validators is notified to the consensus engine via EndBlock() at block height `N+1`,
	//	   the consensus engine applies these accounts to the `ValidatorSet` at block height `(N+1)+2`.
	//	   (Refer to the comments in updateState(...) at github.com/tendermint/tendermint@v0.34.20/state/execution.go)
	// So, the accounts can start signing from block height `N+3`
	// and the Beatoz can check the signatures through `lastVotes` in block height `N+4`.
	if xerr := ctrler.delegateeLedger.Iterate(func(key v1.LedgerKey, item v1.ILedgerItem) xerrors.XError {
		// issue #59
		// Only `Delegatee` who has deposited more than `MinValidatorPower` can become validator.
		d, _ := item.(*Delegatee)
		if d.SelfPower >= ctrler.govParams.MinValidatorPower() {
			ctrler.allDelegatees = append(ctrler.allDelegatees, d)
		}
		return nil
	}, true); xerr != nil {
		return nil, xerr
	}

	sort.Sort(PowerOrderDelegatees(ctrler.allDelegatees)) // sort by power

	ctrler.stakeLimiter.Reset(PowerOrderDelegatees(ctrler.allDelegatees),
		ctrler.govParams.MaxValidatorCnt(), ctrler.govParams.MaxIndividualStakeRate(), ctrler.govParams.MaxUpdatableStakeRate())

	//
	// End of code from EndBlock
	//

	var evts []abcitypes.Event

	// Slashing
	byzantines := blockCtx.BlockInfo().ByzantineValidators
	if byzantines != nil && len(byzantines) > 0 {
		ctrler.logger.Info("StakeCtrler: Byzantine validators is found", "count", len(byzantines))
		for _, evi := range byzantines {
			if slashed, xerr := ctrler.doPunish(
				&evi, blockCtx.GovHandler.SlashRate()); xerr != nil {
				ctrler.logger.Error("Error when punishing",
					"byzantine", types.Address(evi.Validator.Address),
					"evidenceType", abcitypes.EvidenceType_name[int32(evi.Type)])
			} else {
				evts = append(evts, abcitypes.Event{
					Type: "punishment.stake",
					Attributes: []abcitypes.EventAttribute{
						{Key: []byte("byzantine"), Value: []byte(types.Address(evi.Validator.Address).String()), Index: true},
						{Key: []byte("type"), Value: []byte(abcitypes.EvidenceType_name[int32(evi.Type)]), Index: false},
						{Key: []byte("height"), Value: []byte(strconv.FormatInt(evi.Height, 10)), Index: false},
						{Key: []byte("slashed"), Value: []byte(strconv.FormatInt(slashed, 10)), Index: false},
					},
				})
			}
		}
	}

	//
	// Reward and Check MinSignedBlocks
	//
	if len(blockCtx.BlockInfo().LastCommitInfo.Votes) <= 0 {
		return nil, nil
	}

	// issue #70
	// The validators power of `lastVotes` is based on `height` - 4
	//   N       : commit stakes of a validator.
	//   N+1     : `updateValidators` is called at EndBlock and the updated validators are reported to consensus engine.
	//   (N+1)+2 : the updated validators are applied (start signing)
	//   (N+1)+3 : the updated validators are included into `lastVotes`.
	//           : At this point, the validators have their power committed at block N(`curr_height` - 4).
	issuedReward := uint256.NewInt(0)
	heightOfPower := blockCtx.Height() - 4
	if heightOfPower <= 0 {
		heightOfPower = 1
	}

	// ImitableLedgerAt is used to get the delegator's stakes at the block[height-4] and to give rewards based on it.
	// Solution: When a stake is deposited, the rewards start after 4 blocks from the block containing TrxPayloadStaking, (check by using Stake.StartHeight)
	// and when stake is un-staking(executing TrxPayloadUnstaking), immediately stop to reward. (don't reward for stakes existed 4 blocks ago and un-staked at now.)
	immuDelegateeLedger, xerr := ctrler.delegateeLedger.ImitableLedgerAt(heightOfPower)
	if xerr != nil {
		return nil, xerr
	}

	for _, vote := range blockCtx.BlockInfo().LastCommitInfo.Votes {
		if vote.SignedLastBlock {
			// Reward
			item, xerr := immuDelegateeLedger.Get(vote.Validator.Address)
			if xerr != nil || item == nil {
				ctrler.logger.Error("Reward - Not found validator",
					"error", xerr,
					"address", types.Address(vote.Validator.Address),
					"power", vote.Validator.Power,
					"target height", heightOfPower, "current height", blockCtx.Height())
				continue
			}
			delegatee, _ := item.(*Delegatee)
			if delegatee.TotalPower != vote.Validator.Power {
				ctrler.logger.Error("Wrong power", "delegatee", delegatee.Addr, "power of ledger", delegatee.TotalPower, "power of VoteInfo", vote.Validator.Power)
				continue
			}

			issued, _ := ctrler.doRewardTo(delegatee, blockCtx.Height())
			_ = issuedReward.Add(issuedReward, issued)
		} else {
			// check MinSignedBlocks
			signedHeight := blockCtx.Height() - 1
			item, xerr := ctrler.delegateeLedger.Get(vote.Validator.Address, true)
			if xerr != nil {
				// it's possible that a `delegatee` is not found.
				// `vote.Validator.Address` has existed since block[height - 4],
				// and the validator may be removed from `delegateeLedger` while the last 4 blocks are being processed.
				ctrler.logger.Error("MinSignedBlocks - Not found validator",
					"error", xerr,
					"address", types.Address(vote.Validator.Address),
					"power", vote.Validator.Power,
					"target height", heightOfPower, "current height", blockCtx.Height())
				continue
			}

			delegatee, _ := item.(*Delegatee)
			_ = delegatee.ProcessNotSignedBlock(signedHeight)
			_ = ctrler.delegateeLedger.Set(delegatee.Key(), delegatee, true)

			s := signedHeight - ctrler.govParams.SignedBlocksWindow()
			if s < 0 {
				s = 0
			}
			notSigned := delegatee.GetNotSignedBlockCount(s, signedHeight)

			if ctrler.govParams.SignedBlocksWindow()-int64(notSigned) < ctrler.govParams.MinSignedBlocks() {
				// Stop validator: force un-staking all

				ctrler.logger.Info("Validator stop",
					"address", types.Address(vote.Validator.Address),
					"power", vote.Validator.Power,
					"from", s, "to", signedHeight,
					"signed_blocks_window", ctrler.govParams.SignedBlocksWindow(),
					"signed_blocks", ctrler.govParams.SignedBlocksWindow()-int64(notSigned),
					"missed_blocks", notSigned)

				stakes := delegatee.DelAllStakes()
				for _, _s0 := range stakes {
					_s0.RefundHeight = blockCtx.Height() + ctrler.govParams.LazyUnstakingBlocks()
					_ = ctrler.frozenLedger.Set(_s0.Key(), _s0, true) // add s0 to frozen ledger
				}
				_ = ctrler.delegateeLedger.Del(delegatee.Key(), true)
			}
		}
	}

	evts = append(evts, abcitypes.Event{
		Type: "reward",
		Attributes: []abcitypes.EventAttribute{
			{Key: []byte("issued"), Value: []byte(issuedReward.Dec()), Index: false},
		},
	})

	return evts, nil
}

// DoPunish is only used to test
func (ctrler *StakeCtrler) DoPunish(evi *abcitypes.Evidence, slashRatio int32) (int64, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	return ctrler.doPunish(evi, slashRatio)
}

// doPunish is executed at BeginBlock
func (ctrler *StakeCtrler) doPunish(evi *abcitypes.Evidence, slashRatio int32) (int64, xerrors.XError) {
	item, xerr := ctrler.delegateeLedger.Get(evi.Validator.Address, true)
	if xerr != nil {
		return 0, xerr
	}

	// Punish the delegators as well as validator. issue #51
	delegatee, _ := item.(*Delegatee)
	slashed := delegatee.DoSlash(slashRatio)
	_ = ctrler.delegateeLedger.Set(delegatee.Key(), delegatee, true)

	return slashed, nil
}

// DoReward is only used to test
func (ctrler *StakeCtrler) DoReward(height int64, votes []abcitypes.VoteInfo) (*uint256.Int, xerrors.XError) {
	if len(votes) <= 0 {
		return nil, nil
	}

	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	issuedReward := uint256.NewInt(0)

	heightForReward := height - 4
	if heightForReward <= 0 {
		heightForReward = 1
	}
	immuDelegateeLedger, xerr := ctrler.delegateeLedger.ImitableLedgerAt(heightForReward)
	if xerr != nil {
		return nil, xerr
	}

	for _, vote := range votes {
		if vote.SignedLastBlock {
			item, xerr := immuDelegateeLedger.Get(vote.Validator.Address)
			if xerr != nil || item == nil {
				ctrler.logger.Error("Reward - Not found validator",
					"error", xerr,
					"address", types.Address(vote.Validator.Address),
					"power", vote.Validator.Power,
					"target height", heightForReward, "current height", height)
				continue
			}
			delegatee, _ := item.(*Delegatee)
			if delegatee.TotalPower != vote.Validator.Power {
				panic(fmt.Errorf("delegatee(%v)'s power(%v) is not same as the power(%v) of VoteInfo at block[%v]",
					delegatee.Addr, delegatee.TotalPower, vote.Validator.Power, heightForReward))
			}

			issued, _ := ctrler.doRewardTo(delegatee, height)
			_ = issuedReward.Add(issuedReward, issued)
		} else {
			ctrler.logger.Debug("Validator didn't sign the last block", "address", types.Address(vote.Validator.Address), "power", vote.Validator.Power)
		}
	}

	return issuedReward, nil
}

// doRewardTo executes to issue reward to `delegatee` per `stake`.
// It is executed at BeginBlock
func (ctrler *StakeCtrler) doRewardTo(delegatee *Delegatee, height int64) (*uint256.Int, xerrors.XError) {

	issuedReward := uint256.NewInt(0)

	for _, s0 := range delegatee.Stakes {
		item, xerr := ctrler.rewardLedger.Get(s0.From, true)
		if xerr == xerrors.ErrNotFoundResult {
			item = NewReward(s0.From)
		} else if xerr != nil {
			ctrler.logger.Error("fail to find reward object of", s0.From)
			continue
		}

		power := uint256.NewInt(uint64(s0.Power))
		rwd := new(uint256.Int).Mul(power, ctrler.govParams.RewardPerPower())
		rwdObj, _ := item.(*Reward)
		_ = rwdObj.Issue(rwd, height)

		if xerr := ctrler.rewardLedger.Set(rwdObj.Key(), rwdObj, true); xerr != nil {
			ctrler.logger.Error("fail to reward to", s0.From, "err:", xerr)
			continue
		}

		_ = issuedReward.Add(issuedReward, rwd)
	}

	return issuedReward, nil
}

func (ctrler *StakeCtrler) ValidateTrx(ctx *ctrlertypes.TrxContext) xerrors.XError {
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
		totalPower := int64(0)

		item, xerr := ctrler.delegateeLedger.Get(ctx.Tx.To, ctx.Exec)
		if xerr != nil && xerr != xerrors.ErrNotFoundResult {
			return xerr
		}

		delegatee, _ := item.(*Delegatee) // item may be nil
		if bytes.Compare(ctx.Tx.From, ctx.Tx.To) == 0 {
			// self staking

			// issue #59
			// check MinValidatorPower

			selfPower := txPower
			if delegatee != nil {
				selfPower += delegatee.GetSelfPower()
				totalPower = delegatee.GetTotalPower()
			}

			if selfPower < ctrler.govParams.MinValidatorPower() {
				return xerrors.ErrInvalidTrx.Wrapf("too small stake to become validator: a minimum is %v", ctrler.govParams.MinValidatorPower())
			}
		} else {
			// delegating

			if delegatee == nil {
				return xerrors.ErrNotFoundDelegatee.Wrapf("address(%v)", ctx.Tx.To)
			}

			// RG-78: check minDelegatorStake
			minDelegatorPower := ctx.GovHandler.MinDelegatorPower()
			if minDelegatorPower > txPower {
				return xerrors.ErrInvalidTrx.Wrapf("too small stake to become delegator: a minimum is %v", minDelegatorPower)
			}

			// it's delegating. check minSelfStakeRatio
			selfRatio := delegatee.SelfStakeRatio(txPower)
			if selfRatio < int64(ctx.GovHandler.MinSelfStakeRate()) {
				return xerrors.From(fmt.Errorf("not enough self power - validator: %v, self power: %v, total power: %v", delegatee.Addr, delegatee.GetSelfPower(), delegatee.GetTotalPower()))
			}

			totalPower = delegatee.GetTotalPower()
		}

		// check overflow
		if totalPower > math.MaxInt64-txPower {
			// Not reachable code.
			// The sender's balance is checked at `commonValidation()` at `trx_executor.go`
			// and `txPower` is converted from `ctx.Tx.Amount`.
			// Because of that, overflow can not be occurred.
			return xerrors.ErrOverFlow.Wrapf("delegatee power overflow occurs.\ndelegatee: %v\ntx:%v", delegatee, ctx.Tx)
		}

		//
		// begin: issue #34: check updatable stake ratio
		if len(ctrler.lastValidators) >= 3 {
			_delg := delegatee
			if _delg == nil {
				_delg = &Delegatee{
					Addr:       ctx.Tx.To,
					TotalPower: 0,
				}
			}
			if xerr := ctrler.stakeLimiter.CheckLimit(_delg, txPower); xerr != nil {
				return xerrors.ErrUpdatableStakeRatio.Wrap(xerr)
			}
		}
		// end: issue #34: check updatable stake ratio
		//

	case ctrlertypes.TRX_UNSTAKING:
		//
		// begin: issue #34: check updatable stake ratio
		// find delegatee
		item, xerr := ctrler.delegateeLedger.Get(ctx.Tx.To, ctx.Exec)
		if xerr != nil {
			return xerr
		}

		// find the stake from a delegatee
		txhash := ctx.Tx.Payload.(*ctrlertypes.TrxPayloadUnstaking).TxHash
		if txhash == nil || len(txhash) != 32 {
			return xerrors.ErrInvalidTrxPayloadParams
		}

		delegatee, _ := item.(*Delegatee)
		_, s0 := delegatee.FindStake(txhash)
		if s0 == nil {
			return xerrors.ErrNotFoundStake
		}

		if ctx.Tx.From.Compare(s0.From) != 0 {
			return xerrors.ErrNotFoundStake.Wrapf("you are not the stake owner")
		}

		if len(ctrler.lastValidators) >= 3 {
			if xerr := ctrler.stakeLimiter.CheckLimit(delegatee, -1*s0.Power); xerr != nil {
				return xerrors.ErrUpdatableStakeRatio.Wrap(xerr)
			}
		}
		// end: issue #34: check updatable stake ratio
		//
	case ctrlertypes.TRX_WITHDRAW:
		if ctx.Tx.Amount.Sign() != 0 {
			return xerrors.ErrInvalidTrx.Wrapf("amount must be 0")
		}
		txpayload, ok := ctx.Tx.Payload.(*ctrlertypes.TrxPayloadWithdraw)
		if !ok {
			return xerrors.ErrInvalidTrxPayloadType
		}

		item, xerr := ctrler.rewardLedger.Get(ctx.Tx.From, ctx.Exec)
		if xerr != nil {
			return xerr
		}

		rwd, _ := item.(*Reward)
		if txpayload.ReqAmt.Cmp(rwd.cumulated) > 0 {
			return xerrors.ErrInvalidTrx.Wrapf("insufficient reward")
		}
	default:
		return xerrors.ErrUnknownTrxType
	}

	return nil
}

func (ctrler *StakeCtrler) ExecuteTrx(ctx *ctrlertypes.TrxContext) xerrors.XError {
	// executing staking and un-staking txs

	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	switch ctx.Tx.GetType() {
	case ctrlertypes.TRX_STAKING:
		return ctrler.exeStaking(ctx)
	case ctrlertypes.TRX_UNSTAKING:
		return ctrler.exeUnstaking(ctx)
	case ctrlertypes.TRX_WITHDRAW:
		return ctrler.exeWithdraw(ctx)
	default:
		return xerrors.ErrUnknownTrxType
	}
}

func (ctrler *StakeCtrler) exeStaking(ctx *ctrlertypes.TrxContext) xerrors.XError {
	item, xerr := ctrler.delegateeLedger.Get(ctx.Tx.To, ctx.Exec)
	if xerr != nil && xerr != xerrors.ErrNotFoundResult {
		return xerr
	}
	if item == nil && bytes.Compare(ctx.Tx.From, ctx.Tx.To) == 0 {
		// add new delegatee
		item = NewDelegatee(ctx.Tx.From, ctx.SenderPubKey)
	}
	if item == nil {
		// there is no delegatee whose address is ctx.Tx.To
		return xerrors.ErrNotFoundDelegatee.Wrapf("address(%v)", ctx.Tx.To)
	}

	// Update sender account balance
	if xerr := ctx.Sender.SubBalance(ctx.Tx.Amount); xerr != nil {
		return xerr
	}
	_ = ctx.AcctHandler.SetAccount(ctx.Sender, ctx.Exec)

	// create stake and delegate it to `delegatee`
	// the reward for this stake will be started at ctx.Height + 1. (issue #29)
	power, xerr := ctrlertypes.AmountToPower(ctx.Tx.Amount)
	if xerr != nil {
		return xerr
	}
	s0 := NewStakeWithPower(ctx.Tx.From, ctx.Tx.To, power, ctx.Height()+1, ctx.TxHash)

	delegatee, _ := item.(*Delegatee)
	if xerr := delegatee.AddStake(s0); xerr != nil {
		return xerr
	}
	if xerr := ctrler.delegateeLedger.Set(delegatee.Key(), delegatee, ctx.Exec); xerr != nil {
		return xerr
	}

	return nil
}

func (ctrler *StakeCtrler) exeUnstaking(ctx *ctrlertypes.TrxContext) xerrors.XError {
	// find delegatee
	item, xerr := ctrler.delegateeLedger.Get(ctx.Tx.To, ctx.Exec)
	if xerr != nil {
		return xerr
	}

	// delete the stake from a delegatee
	txhash := ctx.Tx.Payload.(*ctrlertypes.TrxPayloadUnstaking).TxHash
	if txhash == nil || len(txhash) != 32 {
		return xerrors.ErrInvalidTrxPayloadParams
	}

	delegatee, _ := item.(*Delegatee)
	_, s0 := delegatee.FindStake(txhash)
	if s0 == nil {
		return xerrors.ErrNotFoundStake
	}

	// issue #43
	// check that tx's sender is stake's owner
	if ctx.Tx.From.Compare(s0.From) != 0 {
		return xerrors.ErrNotFoundStake.Wrapf("you are not the stake owner")
	}

	_ = delegatee.DelStake(txhash)

	s0.RefundHeight = ctx.Height() + ctx.GovHandler.LazyUnstakingBlocks()
	_ = ctrler.frozenLedger.Set(s0.Key(), s0, ctx.Exec) // add s0 to frozen ledger

	if delegatee.SelfPower == 0 {
		stakes := delegatee.DelAllStakes()
		for _, _s0 := range stakes {
			_s0.RefundHeight = ctx.Height() + ctx.GovHandler.LazyUnstakingBlocks()
			_ = ctrler.frozenLedger.Set(_s0.Key(), _s0, ctx.Exec) // add s0 to frozen ledger
		}
	}

	if delegatee.TotalPower == 0 {
		// this changed delegate will be committed at Commit()
		if xerr := ctrler.delegateeLedger.Del(delegatee.Key(), ctx.Exec); xerr != nil {
			return xerr
		}

	} else {
		// this changed delegate will be committed at Commit()
		if xerr := ctrler.delegateeLedger.Set(delegatee.Key(), delegatee, ctx.Exec); xerr != nil {
			return xerr
		}
	}

	return nil
}

func (ctrler *StakeCtrler) exeWithdraw(ctx *ctrlertypes.TrxContext) xerrors.XError {
	txpayload, ok := ctx.Tx.Payload.(*ctrlertypes.TrxPayloadWithdraw)
	if !ok {
		return xerrors.ErrInvalidTrxPayloadType
	}

	//getReward := ctrler.rewardLedger.Get
	//setReward := ctrler.rewardLedger.Set
	//cancelSetReward := ctrler.rewardLedger.CancelSet
	//if ctx.Exec {
	//	getReward = ctrler.rewardLedger.GetFinality
	//	setReward = ctrler.rewardLedger.SetFinality
	//	cancelSetReward = ctrler.rewardLedger.CancelSetFinality
	//}

	snap := ctrler.rewardLedger.Snapshot(ctx.Exec)

	item, xerr := ctrler.rewardLedger.Get(ctx.Tx.From, ctx.Exec)
	if xerr != nil {
		return xerr
	}

	rwd, _ := item.(*Reward)
	xerr = rwd.Withdraw(txpayload.ReqAmt, ctx.Height())
	if xerr != nil {
		return xerr
	}

	xerr = ctrler.rewardLedger.Set(rwd.Key(), rwd, ctx.Exec)
	if xerr != nil {
		return xerr
	}

	xerr = ctx.AcctHandler.Reward(ctx.Sender.Address, txpayload.ReqAmt, ctx.Exec)
	if xerr != nil {
		if xerr = ctrler.rewardLedger.RevertToSnapshot(snap, ctx.Exec); xerr != nil {
			ctrler.logger.Error("rewardLedger is failed to be reverted to", "rev", snap, "err", xerr)
		}
		return xerr
	}
	return nil
}

func (ctrler *StakeCtrler) EndBlock(ctx *ctrlertypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	if xerr := ctrler.unfreezingStakes(ctx.Height(), ctx.AcctHandler); xerr != nil {
		return nil, xerr
	}

	ctx.SetValUpdates(ctrler.updateValidators(int(ctx.GovHandler.MaxValidatorCnt())))

	return nil, nil
}

func (ctrler *StakeCtrler) unfreezingStakes(height int64, acctHandler ctrlertypes.IAccountHandler) xerrors.XError {
	var removed []bytes.HexBytes
	defer func() {
		for _, k := range removed {
			//_ = ctrler.frozenLedger.Del(s0.TxHash, true)
			_ = ctrler.frozenLedger.Del(k, true)
		}
	}()
	return ctrler.frozenLedger.Iterate(func(key v1.LedgerKey, item v1.ILedgerItem) xerrors.XError {
		s0, _ := item.(*Stake)
		if s0.RefundHeight <= height {
			// un-freezing s0
			refundAmt := ctrlertypes.PowerToAmount(s0.Power)

			xerr := acctHandler.Reward(s0.From, refundAmt, true)
			if xerr != nil {
				return xerr
			}

			removed = append(removed, s0.TxHash)
		}
		return nil
	}, true)
}

func (ctrler *StakeCtrler) UpdateValidators(maxVals int) []abcitypes.ValidatorUpdate {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	return ctrler.updateValidators(maxVals)
}

// UpdateValidators is called after executing staking/unstaking txs and before committing the result of the executing.
// `ctrler.allDelegatees` has delegatees committed at previous block.
// It means that UpdateValidators consider the stakes updated at the previous block, not the current block.
func (ctrler *StakeCtrler) updateValidators(maxVals int) []abcitypes.ValidatorUpdate {

	newValidators := selectValidators(PowerOrderDelegatees(ctrler.allDelegatees), maxVals)

	sort.Sort(AddressOrderDelegatees(ctrler.lastValidators))
	sort.Sort(AddressOrderDelegatees(newValidators))
	upVals := validatorUpdates(ctrler.lastValidators, newValidators)

	// update lastValidators
	sort.Sort(PowerOrderDelegatees(newValidators))
	ctrler.lastValidators = newValidators

	return upVals
}

func validatorUpdates(existing, newers DelegateeArray) []abcitypes.ValidatorUpdate {
	valUpdates := make(abcitypes.ValidatorUpdates, 0, len(existing)+len(newers))

	i, j := 0, 0
	for i < len(existing) && j < len(newers) {
		ret := bytes.Compare(existing[i].Addr, newers[j].Addr)
		if ret < 0 {
			// this `existing` validator will be removed because it is not included in `newers`
			valUpdates = append(valUpdates, abcitypes.UpdateValidator(existing[i].PubKey, 0, "secp256k1"))
			i++
		} else if ret == 0 {
			if existing[i].TotalPower != newers[j].TotalPower {
				// if power is changed, add newer who has updated power
				valUpdates = append(valUpdates, abcitypes.UpdateValidator(newers[j].PubKey, int64(newers[j].TotalPower), "secp256k1"))
			} else {
				// if the power is not changed, exclude the validator in updated validators
			}
			i++
			j++
		} else { // ret > 0
			valUpdates = append(valUpdates, abcitypes.UpdateValidator(newers[j].PubKey, int64(newers[j].TotalPower), "secp256k1"))
			j++
		}
	}

	for ; i < len(existing); i++ {
		// removed
		valUpdates = append(valUpdates, abcitypes.UpdateValidator(existing[i].PubKey, 0, "secp256k1"))
	}
	for ; j < len(newers); j++ {
		// added newer
		valUpdates = append(valUpdates, abcitypes.UpdateValidator(newers[j].PubKey, int64(newers[j].TotalPower), "secp256k1"))
	}

	return valUpdates
}

func selectValidators(delegatees PowerOrderDelegatees, maxVals int) DelegateeArray {
	return DelegateeArray(delegatees[:libs.MinInt(len(delegatees), maxVals)])
}

func (ctrler *StakeCtrler) Commit() ([]byte, int64, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	h0, ver0, xerr := ctrler.delegateeLedger.Commit()
	if xerr != nil {
		return nil, -1, xerr
	}
	ctrler.logger.Debug("delegateeLedger commit", "height", ver0, "hash", bytes.HexBytes(h0))

	h1, ver1, xerr := ctrler.frozenLedger.Commit()
	if xerr != nil {
		return nil, -1, xerr
	}
	ctrler.logger.Debug("fronzenLedger commit", "height", ver1, "hash", bytes.HexBytes(h1))

	h2, ver2, xerr := ctrler.rewardLedger.Commit()
	if xerr != nil {
		return nil, -1, xerr
	}
	ctrler.logger.Debug("rewardLedger commit", "height", ver2, "hash", bytes.HexBytes(h2))

	if ver0 != ver1 || ver1 != ver2 {
		return nil, -1, xerrors.ErrCommit.Wrapf("error: StakeCtrler.Commit() has wrong version number - ver0:%v, ver1:%v, ver2:%v", ver0, ver1, ver2)
	}

	if ver0%ctrler.rwdLedgUpInterval == 0 {
		_ = ctrler.rwdHashDB.PutLastRewardHash(h2)
		ctrler.lastRwdHash = h2
	}
	ctrler.logger.Debug("use rewardLedger's hash", "height", ver2, "hash", bytes.HexBytes(ctrler.lastRwdHash))

	return crypto.DefaultHash(h0, h1, ctrler.lastRwdHash), ver0, nil
}

func (ctrler *StakeCtrler) Close() xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	if ctrler.delegateeLedger != nil {
		if xerr := ctrler.delegateeLedger.Close(); xerr != nil {
			ctrler.logger.Error("delegateeLedger.Close()", "error", xerr.Error())
		}
		ctrler.delegateeLedger = nil
	}
	if ctrler.frozenLedger != nil {
		if xerr := ctrler.frozenLedger.Close(); xerr != nil {
			ctrler.logger.Error("frozenLedger.Close()", "error", xerr.Error())
		}
		ctrler.frozenLedger = nil
	}
	if ctrler.rewardLedger != nil {
		if xerr := ctrler.rewardLedger.Close(); xerr != nil {
			ctrler.logger.Error("rewardLedger.Close()", "error", xerr.Error())
		}
		ctrler.rewardLedger = nil
	}
	return nil
}

// IStakeHandler's methods
func (ctrler *StakeCtrler) Validators() ([]*abcitypes.Validator, int64) {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	totalPower := int64(0)
	var ret []*abcitypes.Validator
	for _, v := range ctrler.lastValidators {
		totalPower += v.TotalPower
		ret = append(ret, &abcitypes.Validator{
			Address: v.Addr,
			Power:   int64(v.TotalPower),
		})
	}

	return ret, totalPower
}

func (ctrler *StakeCtrler) IsValidator(addr types.Address) bool {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	for _, v := range ctrler.lastValidators {
		if bytes.Compare(v.Addr, addr) == 0 {
			return true
		}
	}
	return false
}

// Delegatee is used only to test
func (ctrler *StakeCtrler) Delegatee(addr types.Address) *Delegatee {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	if item, xerr := ctrler.delegateeLedger.Get(addr, true); xerr != nil {
		return nil
	} else {
		delegatee, _ := item.(*Delegatee)
		return delegatee
	}
}

// TotalPowerOf is used only to test
func (ctrler *StakeCtrler) TotalPowerOf(addr types.Address) int64 {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	if item, xerr := ctrler.delegateeLedger.Get(addr, true); xerr != nil {
		return 0
	} else if item == nil {
		return 0
	} else {
		delegatee, _ := item.(*Delegatee)
		return delegatee.TotalPower
	}
}

// SelfPowerOf is used only to test
func (ctrler *StakeCtrler) SelfPowerOf(addr types.Address) int64 {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	if item, xerr := ctrler.delegateeLedger.Get(addr, true); xerr != nil {
		return 0
	} else if item == nil {
		return 0
	} else {
		delegatee, _ := item.(*Delegatee)
		return delegatee.SelfPower
	}
}

// DelegatedPowerOf is used only to test
func (ctrler *StakeCtrler) DelegatedPowerOf(addr types.Address) int64 {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	if item, xerr := ctrler.delegateeLedger.Get(addr, true); xerr != nil {
		return 0
	} else if item == nil {
		return 0
	} else {
		delegatee, _ := item.(*Delegatee)
		return delegatee.TotalPower - delegatee.SelfPower
	}
}

// ReadTotalAmount is used only to test
func (ctrler *StakeCtrler) ReadTotalAmount() *uint256.Int {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	ret := uint256.NewInt(0)
	_ = ctrler.delegateeLedger.Iterate(func(key v1.LedgerKey, item v1.ILedgerItem) xerrors.XError {
		delegatee, _ := item.(*Delegatee)
		amt := ctrlertypes.PowerToAmount(delegatee.TotalPower)
		_ = ret.Add(ret, amt)
		return nil
	}, true)
	return ret
}

// ReadTotalPower is used only to test
func (ctrler *StakeCtrler) ReadTotalPower() int64 {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	ret := int64(0)
	_ = ctrler.delegateeLedger.Iterate(func(key v1.LedgerKey, item v1.ILedgerItem) xerrors.XError {
		delegatee, _ := item.(*Delegatee)
		ret += delegatee.GetTotalPower()
		return nil
	}, true)
	return ret
}

// ReadFrozenStakes is used only to test
func (ctrler *StakeCtrler) ReadFrozenStakes() []*Stake {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	var ret []*Stake
	_ = ctrler.frozenLedger.Iterate(func(key v1.LedgerKey, item v1.ILedgerItem) xerrors.XError {
		s0, _ := item.(*Stake)
		ret = append(ret, s0)
		return nil
	}, true)
	return ret
}

func (ctrler *StakeCtrler) rewardOf(addr types.Address) *Reward {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	item, xerr := ctrler.rewardLedger.Get(addr, true)
	if xerr != nil {
		return nil
	}

	rwd, _ := item.(*Reward)
	return rwd
}

func (ctrler *StakeCtrler) readRewardOf(addr types.Address) *Reward {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	item, xerr := ctrler.rewardLedger.Get(addr, true)
	if xerr != nil {
		return nil
	}

	rwd, _ := item.(*Reward)
	return rwd
}

func (s *StakeCtrler) ComputeWeight(height, inflationCycle, ripeningBlocks int64, tau int32, totalSupply *uint256.Int) (*ctrlertypes.Weight, xerrors.XError) {
	return nil, nil
}

var _ ctrlertypes.ILedgerHandler = (*StakeCtrler)(nil)
var _ ctrlertypes.ITrxHandler = (*StakeCtrler)(nil)
var _ ctrlertypes.IBlockHandler = (*StakeCtrler)(nil)
var _ ctrlertypes.IStakeHandler = (*StakeCtrler)(nil)
