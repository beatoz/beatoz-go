package vpower

import (
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"strconv"
)

func (ctrler *VPowerCtrler) BeginBlock(bctx *ctrlertypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	var evts []abcitypes.Event

	// set not sign count
	votes := bctx.BlockInfo().LastCommitInfo.Votes
	for _, vote := range votes {
		if !vote.SignedLastBlock {
			if c, xerr := ctrler.addMissedBlockCount(vote.Validator.Address, true); xerr != nil {
				return nil, xerr
			} else if int64(c) > bctx.GovHandler.SignedBlocksWindow()-bctx.GovHandler.MinSignedBlocks() {
				// todo: slashing....
			}

		}
	}
	if xerr := ctrler.resetAllMissedBlockCount(true); xerr != nil {
		return nil, xerr
	}

	// Punish ByzantineValidators
	byzantines := bctx.BlockInfo().ByzantineValidators
	if byzantines != nil && len(byzantines) > 0 {
		ctrler.logger.Info("Byzantine validators is found", "count", len(byzantines))
		for _, evi := range byzantines {
			if slashed, xerr := ctrler.doPunish(
				&evi, bctx.GovHandler.SlashRate()); xerr != nil {
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

	// todo: reset limiter
	//ctrler.vpowLimiter.Reset(
	//	ctrler.allDelegatees,
	//	bctx.GovHandler.MaxValidatorCnt(),
	//	bctx.GovHandler.MaxIndividualStakeRate(),
	//	bctx.GovHandler.MaxUpdatableStakeRate())

	return evts, nil
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
	// So, if the bonding tx(including TrxPayloadStaking/TrxPayloadUnStaking) is executed at block height `N`,
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

	newValUps, newValidators := ctrler.updateValidators(
		ctrler.allDelegatees,
		ctrler.lastValidators,
		int(bctx.GovHandler.MaxValidatorCnt()),
	)
	bctx.SetValUpdates(newValUps)
	ctrler.lastValidators = newValidators

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
