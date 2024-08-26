package stake

import (
	"fmt"
	cfg "github.com/beatoz/beatoz-go/cmd/config"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/ledger/v0"
	"github.com/beatoz/beatoz-go/libs"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/crypto"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmlog "github.com/tendermint/tendermint/libs/log"
	"sort"
	"strconv"
	"sync"
)

type InitStake struct {
	PubKeys bytes.HexBytes
	Stakes  []*Stake
}

type StakeCtrler struct {
	rwdHashDB *ctrlertypes.MetaDB

	allDelegatees     DelegateeArray
	lastValidators    DelegateeArray
	delegateeLedger   v0.IFinalityLedger[*Delegatee]
	frozenLedger      v0.IFinalityLedger[*Stake]
	rewardLedger      v0.IFinalityLedger[*Reward]
	rwdLedgUpInterval int64
	lastRwdHash       []byte
	stakeLimiter      *StakeLimiter
	govParams         ctrlertypes.IGovHandler

	logger tmlog.Logger
	mtx    sync.RWMutex
}

func NewStakeCtrler(config *cfg.Config, govHandler ctrlertypes.IGovHandler, logger tmlog.Logger) (*StakeCtrler, xerrors.XError) {
	rwdHashDB, err := ctrlertypes.OpenMetaDB("beatoz_app_rwd_hash", config.DBDir())
	if err != nil {
		panic(err)
	}

	newDelegateeProvider := func() *Delegatee { return &Delegatee{} }
	newStakeProvider := func() *Stake { return &Stake{} }
	newRewardProvider := func() *Reward { return &Reward{} }

	// for all delegatees
	delegateeLedger, xerr := v0.NewFinalityLedger[*Delegatee]("delegatees", config.DBDir(), 128, newDelegateeProvider)
	if xerr != nil {
		return nil, xerr
	}

	frozenLedger, xerr := v0.NewFinalityLedger[*Stake]("frozen", config.DBDir(), 128, newStakeProvider)
	if xerr != nil {
		return nil, xerr
	}

	rewardLedger, xerr := v0.NewFinalityLedger[*Reward]("rewards", config.DBDir(), 2048, newRewardProvider)
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
		stakeLimiter:      NewStakeLimiter(nil, govHandler.MaxValidatorCnt(), govHandler.MaxIndividualStakeRatio(), govHandler.MaxUpdatableStakeRatio()),
		govParams:         govHandler,
		logger:            logger.With("module", "beatoz_StakeCtrler"),
	}

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
			if xerr := ctrler.delegateeLedger.SetFinality(d); xerr != nil {
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
	// IterateReadAllFinalityItems() returns delegatees, which are committed at previous block.
	// So, if staking tx is executed at block N,
	//     stake is saved(committed) at block N,
	//     it(updated validators) is notified to consensus engine at block N+1,
	//	   consensus add this account to validator set at block (N+1)+2.
	//	   (Refer to the comments in updateState(...) at github.com/tendermint/tendermint@v0.34.20/state/execution.go)
	// So, the account can sign a block from block N+3 in consensus engine
	if xerr := ctrler.delegateeLedger.IterateReadAllFinalityItems(func(d *Delegatee) xerrors.XError {
		// issue #59
		// Only delegatee who have deposited more than `MinValidatorStake` can become validator.
		minPower := ctrlertypes.AmountToPower(ctrler.govParams.MinValidatorStake())
		if d.SelfPower >= minPower {
			ctrler.allDelegatees = append(ctrler.allDelegatees, d)
		}
		return nil
	}); xerr != nil {
		//
	}

	sort.Sort(PowerOrderDelegatees(ctrler.allDelegatees)) // sort by power

	ctrler.stakeLimiter.Reset(PowerOrderDelegatees(ctrler.allDelegatees),
		ctrler.govParams.MaxValidatorCnt(), ctrler.govParams.MaxIndividualStakeRatio(), ctrler.govParams.MaxUpdatableStakeRatio())

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
				&evi, blockCtx.GovHandler.SlashRatio()); xerr != nil {
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
	//           : At this point, the validators have their power committed at block N (= `height` - 4).
	issuedReward := uint256.NewInt(0)
	heightOfPower := blockCtx.Height() - 4
	if heightOfPower < 0 {
		heightOfPower = 1
	}

	// todo: remove ImmutableLedgerAt. How? => Rewarding is started after 4 blocks, but is immediately stopped in un-staking
	// ImmutableLedgerAt is used to get the delegator's stakes at the block[height-4] and to give rewards based on it.
	// Solution: When a stake is deposited, the rewards start after 4 blocks from the block containing TrxPayloadStaking, (check by using Stake.StartHeight)
	// and when staking is un-staking(executing TrxPayloadUnstaking), immediately stop to reward. (don't reward for stakes existed 4 blocks ago and un-staked at now.)
	immuDelegateeLedger, xerr := ctrler.delegateeLedger.ImmutableLedgerAt(heightOfPower, 128)
	if xerr != nil {
		return nil, xerr
	}

	for _, vote := range blockCtx.BlockInfo().LastCommitInfo.Votes {
		if vote.SignedLastBlock {
			// Reward
			delegatee, xerr := immuDelegateeLedger.Get(v0.ToLedgerKey(vote.Validator.Address))
			if xerr != nil || delegatee == nil {
				ctrler.logger.Error("Reward - Not found validator", "error", xerr, "address", types.Address(vote.Validator.Address), "power", vote.Validator.Power)
				continue
			}

			if delegatee.TotalPower != vote.Validator.Power {
				//panic(fmt.Errorf("delegatee(%v)'s power(%v) is not same as the power(%v) of VoteInfo",
				//	delegatee.Addr, delegatee.TotalPower, vote.Validator.Power))
				ctrler.logger.Error("Wrong power", "delegatee", delegatee.Addr, "power of ledger", delegatee.TotalPower, "power of VoteInfo", vote.Validator.Power)
				continue
			}

			issued, _ := ctrler.doRewardTo(delegatee, blockCtx.Height())
			_ = issuedReward.Add(issuedReward, issued)
		} else {
			// check MinSignedBlocks
			signedHeight := blockCtx.Height() - 1
			delegatee, xerr := ctrler.delegateeLedger.GetFinality(v0.ToLedgerKey(vote.Validator.Address))
			if xerr != nil {
				// it's possible that a `delegatee` is not found.
				// `vote.Validator.Address` has existed since block[height - 4],
				// and the validator may be removed from `delegateeLedger` while the last 4 blocks are being processed.
				ctrler.logger.Error("MinSignedBlocks - Not found validator", "error", xerr, "address", types.Address(vote.Validator.Address), "power", vote.Validator.Power)
				continue
			}

			_ = delegatee.ProcessNotSignedBlock(signedHeight)
			_ = ctrler.delegateeLedger.SetFinality(delegatee)

			s := signedHeight - ctrler.govParams.SignedBlocksWindow()
			if s < 0 {
				s = 0
			}
			notSigned := delegatee.GetNotSignedBlockCount(s, signedHeight)

			if ctrler.govParams.SignedBlocksWindow()-int64(notSigned) < ctrler.govParams.MinSignedBlocks() {
				// Stop validator: do un-staking all

				ctrler.logger.Info("Validator stop",
					"address", types.Address(vote.Validator.Address),
					"power", vote.Validator.Power,
					"from", s, "to", signedHeight,
					"signed_blocks_window", ctrler.govParams.SignedBlocksWindow(),
					"signed_blocks", ctrler.govParams.SignedBlocksWindow()-int64(notSigned),
					"missed_blocks", notSigned)

				stakes := delegatee.DelAllStakes()
				for _, _s0 := range stakes {
					_s0.RefundHeight = blockCtx.Height() + ctrler.govParams.LazyRewardBlocks()
					_ = ctrler.frozenLedger.SetFinality(_s0) // add s0 to frozen ledger
				}

				_, _ = ctrler.delegateeLedger.DelFinality(delegatee.Key())
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

func (ctrler *StakeCtrler) DoPunish(evi *abcitypes.Evidence, slashRatio int64) (int64, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	return ctrler.doPunish(evi, slashRatio)
}

func (ctrler *StakeCtrler) doPunish(evi *abcitypes.Evidence, slashRatio int64) (int64, xerrors.XError) {
	delegatee, xerr := ctrler.delegateeLedger.GetFinality(v0.ToLedgerKey(evi.Validator.Address))
	if xerr != nil {
		return 0, xerr
	}

	// Punish the delegators as well as validator. issue #51
	slashed := delegatee.DoSlash(slashRatio)
	_ = ctrler.delegateeLedger.SetFinality(delegatee)

	return slashed, nil
}

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
	immuDelegateeLedger, xerr := ctrler.delegateeLedger.ImmutableLedgerAt(heightForReward, 128)
	if xerr != nil {
		return nil, xerr
	}

	for _, vote := range votes {
		if vote.SignedLastBlock {
			delegatee, xerr := immuDelegateeLedger.Get(v0.ToLedgerKey(vote.Validator.Address))
			if xerr != nil || delegatee == nil {
				ctrler.logger.Error("Reward - Not found validator", "error", xerr, "address", types.Address(vote.Validator.Address), "power", vote.Validator.Power)
				continue
			}

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

func (ctrler *StakeCtrler) doRewardTo(delegatee *Delegatee, height int64) (*uint256.Int, xerrors.XError) {

	issuedReward := uint256.NewInt(0)

	for _, s0 := range delegatee.Stakes {
		rwdObj, xerr := ctrler.rewardLedger.GetFinality(v0.ToLedgerKey(s0.From))
		if xerr == xerrors.ErrNotFoundResult {
			rwdObj = NewReward(s0.From)
		} else if xerr != nil {
			ctrler.logger.Error("fail to find reward object of", s0.From)
			continue
		}

		power := uint256.NewInt(uint64(s0.Power))
		rwd := new(uint256.Int).Mul(power, ctrler.govParams.RewardPerPower())
		_ = rwdObj.Issue(rwd, height)

		if xerr := ctrler.rewardLedger.SetFinality(rwdObj); xerr != nil {
			ctrler.logger.Error("fail to reward to", s0.From, "err:", xerr)
			continue
		}

		_ = issuedReward.Add(issuedReward, rwd)
	}

	return issuedReward, nil
}

func (ctrler *StakeCtrler) ValidateTrx(ctx *ctrlertypes.TrxContext) xerrors.XError {
	getDelegatee := ctrler.delegateeLedger.Get
	if ctx.Exec {
		getDelegatee = ctrler.delegateeLedger.GetFinality
	}

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

		txPower := ctrlertypes.AmountToPower(ctx.Tx.Amount)
		totalPower := int64(0)

		delegatee, xerr := getDelegatee(v0.ToLedgerKey(ctx.Tx.To))
		if xerr != nil && xerr != xerrors.ErrNotFoundResult {
			return xerr
		}

		if bytes.Compare(ctx.Tx.From, ctx.Tx.To) == 0 {
			// self staking

			// isseu #59
			// check MinValidatorStake

			selfPower := txPower
			if delegatee != nil {
				selfPower += delegatee.GetSelfPower()
				totalPower = delegatee.GetTotalPower()
			}

			minPower := ctrlertypes.AmountToPower(ctrler.govParams.MinValidatorStake())
			if selfPower < minPower {
				return xerrors.ErrInvalidTrx.Wrapf("too small stake to become validator: a minimum is %v", ctrler.govParams.MinValidatorStake())
			}
		} else {
			// delegating

			if delegatee == nil {
				return xerrors.ErrNotFoundDelegatee.Wrapf("address(%v)", ctx.Tx.To)
			}

			// RG-78: check minDelegatorStake
			minDelegatorPower := ctrlertypes.AmountToPower(ctx.GovHandler.MinDelegatorStake())
			if minDelegatorPower > 0 && minDelegatorPower > txPower {
				return xerrors.ErrInvalidTrx.Wrapf("too small stake to become delegator: a minimum is %v", ctrler.govParams.MinDelegatorStake())
			}

			// it's delegating. check minSelfStakeRatio
			selfRatio := delegatee.SelfStakeRatio(txPower)
			if selfRatio < ctx.GovHandler.MinSelfStakeRatio() {
				return xerrors.From(fmt.Errorf("not enough self power - validator: %v, self power: %v, total power: %v", delegatee.Addr, delegatee.GetSelfPower(), delegatee.GetTotalPower()))
			}

			totalPower = delegatee.GetTotalPower()
		}

		// check overflow
		if (totalPower + txPower) <= 0 {
			panic(fmt.Errorf("delegatee power overflow occurs.\ndelegatee: %v\ntx:%v", delegatee, ctx.Tx))
		}

		//
		// begin: issue #34: check updatable stake ratio
		_delg := delegatee
		if _delg == nil {
			_delg = &Delegatee{
				Addr:       ctx.Tx.To,
				TotalPower: 0,
			}
		}
		if len(ctrler.lastValidators) >= 3 {
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
		delegatee, xerr := getDelegatee(v0.ToLedgerKey(ctx.Tx.To))
		if xerr != nil {
			return xerr
		}

		// find the stake from a delegatee
		txhash := ctx.Tx.Payload.(*ctrlertypes.TrxPayloadUnstaking).TxHash
		if txhash == nil || len(txhash) != 32 {
			return xerrors.ErrInvalidTrxPayloadParams
		}

		_, s0 := delegatee.FindStake(txhash)
		if s0 == nil {
			return xerrors.ErrNotFoundStake
		}

		if ctx.Tx.From.Compare(s0.From) != 0 {
			return xerrors.ErrNotFoundStake.Wrapf("you not stake owner")
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

		getReward := ctrler.rewardLedger.Get
		if ctx.Exec {
			getReward = ctrler.rewardLedger.GetFinality
		}
		rwd, xerr := getReward(v0.ToLedgerKey(ctx.Tx.From))
		if xerr != nil {
			return xerr
		}

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
	getDelegatee := ctrler.delegateeLedger.Get
	setUpdateDelegatee := ctrler.delegateeLedger.Set
	if ctx.Exec {
		getDelegatee = ctrler.delegateeLedger.GetFinality
		setUpdateDelegatee = ctrler.delegateeLedger.SetFinality
	}

	delegatee, xerr := getDelegatee(v0.ToLedgerKey(ctx.Tx.To))
	if xerr != nil && xerr != xerrors.ErrNotFoundResult {
		return xerr
	}

	if delegatee == nil && bytes.Compare(ctx.Tx.From, ctx.Tx.To) == 0 {
		// add new delegatee
		delegatee = NewDelegatee(ctx.Tx.From, ctx.SenderPubKey)
	}

	if delegatee == nil {
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
	power := ctrlertypes.AmountToPower(ctx.Tx.Amount)
	s0 := NewStakeWithPower(ctx.Tx.From, ctx.Tx.To, power, ctx.Height+1, ctx.TxHash)

	if xerr := delegatee.AddStake(s0); xerr != nil {
		return xerr
	}
	if xerr := setUpdateDelegatee(delegatee); xerr != nil {
		return xerr
	}

	return nil
}

func (ctrler *StakeCtrler) exeUnstaking(ctx *ctrlertypes.TrxContext) xerrors.XError {
	getDelegatee := ctrler.delegateeLedger.Get
	setUpdateDelegatee := ctrler.delegateeLedger.Set
	delDelegatee := ctrler.delegateeLedger.Del
	setUpdateFrozen := ctrler.frozenLedger.Set
	if ctx.Exec {
		getDelegatee = ctrler.delegateeLedger.GetFinality
		setUpdateDelegatee = ctrler.delegateeLedger.SetFinality
		delDelegatee = ctrler.delegateeLedger.DelFinality
		setUpdateFrozen = ctrler.frozenLedger.SetFinality
	}

	// find delegatee
	delegatee, xerr := getDelegatee(v0.ToLedgerKey(ctx.Tx.To))
	if xerr != nil {
		return xerr
	}

	// delete the stake from a delegatee
	txhash := ctx.Tx.Payload.(*ctrlertypes.TrxPayloadUnstaking).TxHash
	if txhash == nil || len(txhash) != 32 {
		return xerrors.ErrInvalidTrxPayloadParams
	}

	_, s0 := delegatee.FindStake(txhash)
	if s0 == nil {
		return xerrors.ErrNotFoundStake
	}

	// issue #43
	// check that tx's sender is stake's owner
	if ctx.Tx.From.Compare(s0.From) != 0 {
		return xerrors.ErrNotFoundStake.Wrapf("you not stake owner")
	}

	_ = delegatee.DelStake(txhash)

	s0.RefundHeight = ctx.Height + ctx.GovHandler.LazyRewardBlocks()
	_ = setUpdateFrozen(s0) // add s0 to frozen ledger

	if delegatee.SelfPower == 0 {
		stakes := delegatee.DelAllStakes()
		for _, _s0 := range stakes {
			_s0.RefundHeight = ctx.Height + ctx.GovHandler.LazyRewardBlocks()
			_ = setUpdateFrozen(_s0) // add s0 to frozen ledger
		}
	}

	if delegatee.TotalPower == 0 {
		// this changed delegate will be committed at Commit()
		if _, xerr := delDelegatee(delegatee.Key()); xerr != nil {
			return xerr
		}

	} else {
		// this changed delegate will be committed at Commit()
		if xerr := setUpdateDelegatee(delegatee); xerr != nil {
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

	getReward := ctrler.rewardLedger.Get
	setReward := ctrler.rewardLedger.Set
	cancelSetReward := ctrler.rewardLedger.CancelSet
	if ctx.Exec {
		getReward = ctrler.rewardLedger.GetFinality
		setReward = ctrler.rewardLedger.SetFinality
		cancelSetReward = ctrler.rewardLedger.CancelSetFinality
	}
	rwd, xerr := getReward(v0.ToLedgerKey(ctx.Tx.From))
	if xerr != nil {
		return xerr
	}

	xerr = rwd.Withdraw(txpayload.ReqAmt, ctx.Height)
	if xerr != nil {
		return xerr
	}

	xerr = setReward(rwd)
	if xerr != nil {
		return xerr
	}

	xerr = ctx.AcctHandler.Reward(ctx.Sender.Address, txpayload.ReqAmt, ctx.Exec)
	if xerr != nil {
		cancelSetReward(rwd.Key())
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
	return ctrler.frozenLedger.IterateReadAllFinalityItems(func(s0 *Stake) xerrors.XError {
		if s0.RefundHeight <= height {
			// un-freezing s0
			// return s0. not only s0.ReceivedReward but also s0.Amount

			refundAmt := ctrlertypes.PowerToAmount(s0.Power)
			xerr := acctHandler.Reward(s0.From, refundAmt, true)
			if xerr != nil {
				return xerr
			}

			_, _ = ctrler.frozenLedger.DelFinality(v0.ToLedgerKey(s0.TxHash))
		}
		return nil
	})
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
	//
	// the item of delegatees has TotaPower greater than power for govParams.MinValidatorStake()
	//

	//var validators DelegateeArray
	//for i, val := range delegatees {
	//	if val.TotalPower <= 0 {
	//		break
	//	} else if i >= maxVals {
	//		break
	//	}
	//	validators = append(validators, val)
	//}
	//
	//return validators
	return DelegateeArray(delegatees[:libs.MIN(len(delegatees), maxVals)])
}

func (ctrler *StakeCtrler) Commit() ([]byte, int64, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	h0, v0, xerr := ctrler.delegateeLedger.Commit()
	if xerr != nil {
		return nil, -1, xerr
	}
	h1, v1, xerr := ctrler.frozenLedger.Commit()
	if xerr != nil {
		return nil, -1, xerr
	}
	h2, v2, xerr := ctrler.rewardLedger.Commit()
	if xerr != nil {
		return nil, -1, xerr
	}
	if v0 != v1 || v1 != v2 {
		return nil, -1, xerrors.ErrCommit.Wrapf("error: StakeCtrler.Commit() has wrong version number - v0:%v, v1:%v, v2:%v", v0, v1, v2)
	}

	if v0%ctrler.rwdLedgUpInterval == 0 {
		_ = ctrler.rwdHashDB.PutLastRewardHash(h2)
		ctrler.lastRwdHash = h2
	}

	return crypto.DefaultHash(h0, h1, ctrler.lastRwdHash), v0, nil
}

func (ctrler *StakeCtrler) Close() xerrors.XError {
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

func (ctrler *StakeCtrler) Delegatee(addr types.Address) *Delegatee {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	if delegatee, xerr := ctrler.delegateeLedger.GetFinality(v0.ToLedgerKey(addr)); xerr != nil {
		return nil
	} else {
		return delegatee
	}
}

func (ctrler *StakeCtrler) TotalPowerOf(addr types.Address) int64 {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	if delegatee, xerr := ctrler.delegateeLedger.GetFinality(v0.ToLedgerKey(addr)); xerr != nil {
		return 0
	} else if delegatee == nil {
		return 0
	} else {
		return delegatee.TotalPower
	}
}

func (ctrler *StakeCtrler) SelfPowerOf(addr types.Address) int64 {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	if delegatee, xerr := ctrler.delegateeLedger.GetFinality(v0.ToLedgerKey(addr)); xerr != nil {
		return 0
	} else if delegatee == nil {
		return 0
	} else {
		return delegatee.SelfPower
	}
}

func (ctrler *StakeCtrler) DelegatedPowerOf(addr types.Address) int64 {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	if delegatee, xerr := ctrler.delegateeLedger.GetFinality(v0.ToLedgerKey(addr)); xerr != nil {
		return 0
	} else if delegatee == nil {
		return 0
	} else {
		return delegatee.TotalPower - delegatee.SelfPower
	}
}

func (ctrler *StakeCtrler) ReadTotalAmount() *uint256.Int {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	ret := uint256.NewInt(0)
	_ = ctrler.delegateeLedger.IterateReadAllFinalityItems(func(delegatee *Delegatee) xerrors.XError {
		amt := ctrlertypes.PowerToAmount(delegatee.TotalPower)
		_ = ret.Add(ret, amt)
		return nil
	})
	return ret
}

func (ctrler *StakeCtrler) ReadTotalPower() int64 {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	ret := int64(0)
	_ = ctrler.delegateeLedger.IterateReadAllFinalityItems(func(delegatee *Delegatee) xerrors.XError {
		ret += delegatee.GetTotalPower()
		return nil
	})
	return ret
}

func (ctrler *StakeCtrler) ReadFrozenStakes() []*Stake {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	var ret []*Stake
	_ = ctrler.frozenLedger.IterateReadAllFinalityItems(func(s0 *Stake) xerrors.XError {
		ret = append(ret, s0)
		return nil
	})
	return ret
}

func (ctrler *StakeCtrler) RewardOf(addr types.Address) *Reward {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	rwd, xerr := ctrler.rewardLedger.GetFinality(v0.ToLedgerKey(addr))
	if xerr != nil {
		return nil
	}
	return rwd
}

func (ctrler *StakeCtrler) ReadRewardOf(addr types.Address) *Reward {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	rwd, xerr := ctrler.rewardLedger.Read(v0.ToLedgerKey(addr))
	if xerr != nil {
		return nil
	}

	return rwd
}

var _ ctrlertypes.ILedgerHandler = (*StakeCtrler)(nil)
var _ ctrlertypes.ITrxHandler = (*StakeCtrler)(nil)
var _ ctrlertypes.IBlockHandler = (*StakeCtrler)(nil)
var _ ctrlertypes.IStakeHandler = (*StakeCtrler)(nil)
