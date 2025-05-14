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

	//
	// Punish ByzantineValidators
	byzantines := bctx.BlockInfo().ByzantineValidators
	if len(byzantines) > 0 {
		ctrler.logger.Info("Byzantine validators is found", "count", len(byzantines))
		for _, evi := range byzantines {
			if slashed, xerr := ctrler.doPunish(
				&evi, bctx.GovHandler.SlashRate()); xerr != nil {
				ctrler.logger.Error("Error when punishing",
					"byzantine", types.Address(evi.Validator.Address),
					"evidenceType", abcitypes.EvidenceType_name[int32(evi.Type)])
			} else if xerr = bctx.SupplyHandler.Burn(bctx, ctrlertypes.PowerToAmount(slashed)); xerr != nil {
				return nil, xerr
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
	// Punish missing blocks
	if len(bctx.BlockInfo().LastCommitInfo.Votes) <= 0 {
		return evts, nil
	}
	for _, vote := range bctx.BlockInfo().LastCommitInfo.Votes {
		if !vote.SignedLastBlock {
			missedCnt, xerr := ctrler.addMissedBlockCount(vote.Validator.Address, true)
			if xerr != nil {
				return nil, xerr
			}

			// `missedCnt` is reset every GovParams.InflationCycleBlocks
			allowedDownCnt := bctx.GovHandler.InflationCycleBlocks() - bctx.GovHandler.MinSignedBlocks()
			if int64(missedCnt) >= allowedDownCnt {
				// un-bonding all voting power of validators

				ctrler.logger.Info("Validator stop",
					"address", types.Address(vote.Validator.Address),
					"power", vote.Validator.Power,
					"missed_blocks", missedCnt)

				refundHeight := bctx.Height() + bctx.GovHandler.LazyUnstakingBlocks()

				dgtee, xerr := ctrler.readDelegatee(vote.Validator.Address, true)
				if xerr != nil {
					return nil, xerr
				}
				// un-bonding all vpowers delegated to `dgtee`
				for _, _from := range dgtee.Delegators {
					_vpow, xerr := ctrler.readVPower(_from, dgtee.addr, true)
					if xerr != nil {
						return nil, xerr
					}

					if xerr := ctrler.freezePowerChunkList(_vpow.from, _vpow.PowerChunks, refundHeight, true); xerr != nil {
						return nil, xerr
					}
					if xerr := ctrler.removeVPower(_vpow.from, _vpow.to, true); xerr != nil {
						return nil, xerr
					}
				}
				if xerr := ctrler.removeDelegatee(dgtee.addr, true); xerr != nil {
					return nil, xerr
				}
			}
		}
	}
	if bctx.Height() > 0 && bctx.Height()%bctx.GovHandler.InflationCycleBlocks() == 0 {
		_ = ctrler.resetAllMissedBlockCount(true)
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
