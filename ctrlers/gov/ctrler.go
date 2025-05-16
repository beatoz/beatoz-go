package gov

import (
	"bytes"
	"errors"
	cfg "github.com/beatoz/beatoz-go/cmd/config"
	"github.com/beatoz/beatoz-go/ctrlers/gov/proposal"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/genesis"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types"
	abytes "github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/tendermint/tendermint/libs/json"
	"github.com/tendermint/tendermint/libs/log"
	"strings"
	"sync"
)

type GovCtrler struct {
	ctrlertypes.GovParams
	newGovParams *ctrlertypes.GovParams

	paramsState   v1.IStateLedger
	proposalState v1.IStateLedger
	frozenState   v1.IStateLedger

	logger log.Logger
	mtx    sync.RWMutex
}

var defaultNewItemFor = func(key v1.LedgerKey) v1.ILedgerItem {
	if bytes.HasPrefix(key, v1.KeyPrefixGovParams) {
		return &ctrlertypes.GovParams{}
	}
	if bytes.HasPrefix(key, v1.KeyPrefixProposal) || bytes.HasPrefix(key, v1.KeyPrefixFrozenProp) {
		return &proposal.GovProposal{}
	}
	panic("unknown key prefix")
	return nil
}

func NewGovCtrler(config *cfg.Config, logger log.Logger) (*GovCtrler, error) {
	lg := logger.With("module", "beatoz_GovCtrler")

	paramsState, xerr := v1.NewStateLedger("gov_params", config.DBDir(), 2048, defaultNewItemFor, lg)
	if xerr != nil {
		return nil, xerr
	}

	params, xerr := paramsState.Get(v1.LedgerKeyGovParams(), true)
	// `params` may be nil
	if xerr != nil && xerr != xerrors.ErrNotFoundResult {
		return nil, xerr
	} else if params == nil {
		params = &ctrlertypes.GovParams{} // empty params
	}

	proposalState, xerr := v1.NewStateLedger("proposal", config.DBDir(), 2048, defaultNewItemFor, lg)
	if xerr != nil {
		return nil, xerr
	}

	frozenState, xerr := v1.NewStateLedger("frozen_proposal", config.DBDir(), 1, defaultNewItemFor, lg)
	if xerr != nil {
		return nil, xerr
	}

	return &GovCtrler{
		GovParams:     *(params.(*ctrlertypes.GovParams)),
		paramsState:   paramsState,
		proposalState: proposalState,
		frozenState:   frozenState,
		logger:        lg,
	}, nil
}

func (ctrler *GovCtrler) InitLedger(req interface{}) xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	genAppState, ok := req.(*genesis.GenesisAppState)
	if !ok {
		return xerrors.ErrInitChain.Wrapf("wrong parameter: GovCtrler::InitLedger requires *genesis.GenesisAppState")
	}
	ctrler.GovParams = *genAppState.GovParams
	_ = ctrler.paramsState.Set(v1.LedgerKeyGovParams(), &ctrler.GovParams, true)
	return nil
}

func (ctrler *GovCtrler) ValidateTrx(ctx *ctrlertypes.TrxContext) xerrors.XError {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	// validation by tx type
	switch ctx.Tx.GetType() {
	case ctrlertypes.TRX_PROPOSAL:
		if bytes.Compare(ctx.Tx.To, types.ZeroAddress()) != 0 {
			return xerrors.ErrInvalidTrx.Wrap(errors.New("wrong address: the 'to' field in TRX_PROPOSAL should be zero address"))
		}

		// check right
		if ctx.VPowerHandler.IsValidator(ctx.Tx.From) == false {
			return xerrors.ErrNoRight
		}

		// check tx type
		txpayload, ok := ctx.Tx.Payload.(*ctrlertypes.TrxPayloadProposal)
		if !ok {
			return xerrors.ErrInvalidTrxPayloadType
		}

		// check already exist
		prop, xerr := ctrler.proposalState.Get(v1.LedgerKeyProposal(ctx.TxHash), ctx.Exec)
		if xerr != nil && xerr != xerrors.ErrNotFoundResult {
			return xerr
		} else if prop != nil {
			return xerrors.ErrDuplicatedKey
		}

		// check start height
		if txpayload.StartVotingHeight <= ctx.Height() {
			return xerrors.ErrInvalidTrxPayloadParams
		}
		// check voting period
		if txpayload.VotingPeriodBlocks > ctrler.MaxVotingPeriodBlocks() ||
			txpayload.VotingPeriodBlocks < ctrler.MinVotingPeriodBlocks() {
			return xerrors.ErrInvalidTrxPayloadParams
		}
		// check governance proposal consistency
		if txpayload.OptType == proposal.PROPOSAL_GOVPARAMS {
			//check options
			checkGovParams := &ctrlertypes.GovParams{}
			for _, option := range txpayload.Options {
				if err := json.Unmarshal(option, checkGovParams); err != nil {
					return xerrors.ErrInvalidTrxPayloadParams.Wrap(err)
				}
			}
		}
		endVotingHeight := txpayload.StartVotingHeight + txpayload.VotingPeriodBlocks
		minApplyingHeight := endVotingHeight + ctrler.LazyApplyingBlocks()
		// check overflow: issue #51
		if txpayload.StartVotingHeight > endVotingHeight {
			return xerrors.ErrInvalidTrxPayloadParams.Wrapf("overflow occurs: startHeight:%v, endVotingHeight:%v",
				txpayload.StartVotingHeight, endVotingHeight)
		}
		// check applying blocks
		if txpayload.ApplyingHeight < minApplyingHeight || endVotingHeight > txpayload.ApplyingHeight {
			return xerrors.ErrInvalidTrxPayloadParams.Wrapf("wrong applyingHeight: must be set equal to or higher than minApplyingHeight. ApplyingHeight:%v, minApplyingHeight:%v, endVotingHeight:%v, lazyApplyingBlocks:%v", txpayload.ApplyingHeight, minApplyingHeight, endVotingHeight, ctrler.LazyApplyingBlocks())
		}

		// check options
		if len(txpayload.Options) == 0 || txpayload.Options == nil {
			return xerrors.ErrInvalidTrxPayloadParams.Wrapf("wrong options: must have at least one value")
		}
	case ctrlertypes.TRX_VOTING:
		if bytes.Compare(ctx.Tx.To, types.ZeroAddress()) != 0 {
			return xerrors.ErrInvalidTrxPayloadParams.Wrap(errors.New("wrong address: the 'to' field in TRX_VOTING should be zero address"))
		}
		// check tx type
		txpayload, ok := ctx.Tx.Payload.(*ctrlertypes.TrxPayloadVoting)
		if !ok {
			return xerrors.ErrInvalidTrxPayloadType
		}

		// check already exist
		item, xerr := ctrler.proposalState.Get(v1.LedgerKeyProposal(txpayload.TxHash), ctx.Exec)
		if xerr != nil {
			return xerr
		}
		prop, _ := item.(*proposal.GovProposal)
		if prop.IsVoter(ctx.Tx.From) == false {
			return xerrors.ErrNoRight
		}

		// check choice validation
		if txpayload.Choice < 0 || txpayload.Choice >= int32(len(prop.Options)) {
			return xerrors.ErrInvalidTrxPayloadParams
		}

		// check end height
		if ctx.Height() > prop.EndVotingHeight ||
			ctx.Height() < prop.StartVotingHeight {
			return xerrors.ErrNotVotingPeriod
		}
	default:
		return xerrors.ErrUnknownTrxType
	}

	return nil
}

func (ctrler *GovCtrler) ExecuteTrx(ctx *ctrlertypes.TrxContext) xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	switch ctx.Tx.GetType() {
	case ctrlertypes.TRX_PROPOSAL:
		return ctrler.execProposing(ctx)
	case ctrlertypes.TRX_VOTING:
		return ctrler.execVoting(ctx)
	default:
		return xerrors.ErrUnknownTrxType
	}
}

func (ctrler *GovCtrler) execProposing(ctx *ctrlertypes.TrxContext) xerrors.XError {
	txpayload, _ := ctx.Tx.Payload.(*ctrlertypes.TrxPayloadProposal)

	voters := make(map[string]*proposal.Voter)
	vals, totalVotingPower := ctx.VPowerHandler.Validators()
	for _, v := range vals {
		voters[types.Address(v.Address).String()] = &proposal.Voter{
			Addr:   v.Address,
			Power:  v.Power,
			Choice: proposal.NOT_CHOICE, // -1
		}
	}

	prop, xerr := proposal.NewGovProposal(ctx.TxHash, txpayload.OptType,
		txpayload.StartVotingHeight, txpayload.VotingPeriodBlocks,
		totalVotingPower, txpayload.ApplyingHeight, voters, txpayload.Options...)
	if xerr != nil {
		return xerr
	}
	if xerr = ctrler.proposalState.Set(v1.LedgerKeyProposal(prop.TxHash), prop, ctx.Exec); xerr != nil {
		return xerr
	}

	return nil
}

func (ctrler *GovCtrler) execVoting(ctx *ctrlertypes.TrxContext) xerrors.XError {
	txpayload, _ := ctx.Tx.Payload.(*ctrlertypes.TrxPayloadVoting)
	item, xerr := ctrler.proposalState.Get(v1.LedgerKeyProposal(txpayload.TxHash), ctx.Exec)
	if xerr != nil {
		return xerr
	}
	prop, _ := item.(*proposal.GovProposal)
	if xerr = prop.DoVote(ctx.Tx.From, txpayload.Choice); xerr != nil {
		return xerr
	}
	if xerr = ctrler.proposalState.Set(v1.LedgerKeyProposal(prop.TxHash), prop, ctx.Exec); xerr != nil {
		return xerr
	}
	if prop.MajorOption != nil {
		ctrler.logger.Debug("Voting to proposal", "key", prop.TxHash, "voter", ctx.Tx.From, "choice", txpayload.Choice)
	}
	return nil
}

// freezeProposals is called from EndBlock
func (ctrler *GovCtrler) freezeProposals(height int64) ([]v1.LedgerKey, []v1.LedgerKey, xerrors.XError) {
	var frozen []v1.LedgerKey
	var removed []v1.LedgerKey

	defer func() {
		for _, k := range frozen {
			// freezing
			_ = ctrler.proposalState.Del(k, true)
		}
		for _, k := range removed {
			// remove
			_ = ctrler.proposalState.Del(k, true)
		}
	}()

	xerr := ctrler.proposalState.Seek(v1.KeyPrefixProposal, true, func(key v1.LedgerKey, item v1.ILedgerItem) xerrors.XError {
		prop, _ := item.(*proposal.GovProposal)
		if prop.EndVotingHeight < height {

			// DO NOT REMOVE `prop` from `proposalState`

			majorOpt := prop.UpdateMajorOption()
			if majorOpt != nil {
				// freeze the proposal
				if xerr := ctrler.frozenState.Set(v1.LedgerKeyFrozenProp(prop.TxHash), prop, true); xerr != nil {
					return xerr
				}
				frozen = append(frozen, key)
			} else {
				// do nothing. the proposal will be just removed.
				ctrler.logger.Debug("Freeze proposal", "warning", "not found major option")
				removed = append(removed, key)
			}
		}
		return nil
	}, true)
	return frozen, removed, xerr
}

// applyProposals is called from EndBlock
func (ctrler *GovCtrler) applyProposals(height int64) ([]v1.LedgerKey, xerrors.XError) {
	var applied []v1.LedgerKey

	defer func() {
		for _, k := range applied {
			// remove
			_ = ctrler.frozenState.Del(k, true)
		}
	}()

	xerr := ctrler.frozenState.Seek(v1.KeyPrefixFrozenProp, true, func(key v1.LedgerKey, item v1.ILedgerItem) xerrors.XError {
		prop, _ := item.(*proposal.GovProposal)
		if prop.ApplyingHeight <= height {

			// DO NOT REMOVE `prop` from `frozenState` at here.

			if prop.MajorOption == nil {
				// not reachable.
				ctrler.logger.Error("Apply proposal", "error", "major option is nil")
			}

			switch prop.OptType {
			case proposal.PROPOSAL_GOVPARAMS:
				newGovParams := &ctrlertypes.GovParams{}

				//
				// hotfix
				strOpt := string(prop.MajorOption.Option())
				if strings.HasSuffix(strOpt, `""}`) {
					strOpt = strings.ReplaceAll(strOpt, `""}`, `"}`)
				}
				//
				//

				if err := json.Unmarshal([]byte(strOpt), newGovParams); err != nil {
					ctrler.logger.Error("Apply proposal", "error", err, "option", string(prop.MajorOption.Option()))
					return xerrors.From(err)
				}
				ctrlertypes.MergeGovParams(&ctrler.GovParams, newGovParams)
				if xerr := ctrler.paramsState.Set(v1.LedgerKeyGovParams(), newGovParams, true); xerr != nil {
					ctrler.logger.Error("Apply proposal", "error", xerr, "newGovParams", newGovParams)
					return xerr
				}
				ctrler.newGovParams = newGovParams
			default:
				ctrler.logger.Debug("Apply proposal", "key(txHash)", prop.TxHash, "type", prop.OptType)
			}

			applied = append(applied, key) // this key will be removed from frozenState

		}
		return nil
	}, true)

	return applied, xerr
}

func (ctrler *GovCtrler) Close() xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	if ctrler.paramsState != nil {
		if xerr := ctrler.paramsState.Close(); xerr != nil {
			ctrler.logger.Error("paramsLedger.Close()", "error", xerr.Error())
		}
		ctrler.paramsState = nil
	}
	if ctrler.proposalState != nil {
		if xerr := ctrler.proposalState.Close(); xerr != nil {
			ctrler.logger.Error("proposalLedger.Close()", "error", xerr.Error())
		}
		ctrler.proposalState = nil
	}

	if ctrler.frozenState != nil {
		if xerr := ctrler.frozenState.Close(); xerr != nil {
			ctrler.logger.Error("frozenState.Close()", "error", xerr.Error())
		}
		ctrler.frozenState = nil
	}
	return nil
}

func (ctrler *GovCtrler) GetGovParams() ctrlertypes.GovParams {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	return ctrler.GovParams
}

func (ctrler *GovCtrler) ReadAllProposals(exec bool) ([]*proposal.GovProposal, xerrors.XError) {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	var proposals []*proposal.GovProposal

	if xerr := ctrler.proposalState.Seek(v1.KeyPrefixProposal, true, func(key v1.LedgerKey, item v1.ILedgerItem) xerrors.XError {
		prop, _ := item.(*proposal.GovProposal)
		proposals = append(proposals, prop)
		return nil
	}, exec); xerr != nil {
		if xerr == xerrors.ErrNotFoundResult {
			return nil, xerrors.ErrNotFoundProposal
		}
		return nil, xerr
	}

	return proposals, nil
}

func (ctrler *GovCtrler) ReadProposal(txhash abytes.HexBytes, exec bool) (*proposal.GovProposal, xerrors.XError) {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	item, xerr := ctrler.proposalState.Get(v1.LedgerKeyProposal(txhash), exec)
	if xerr != nil {
		if errors.Is(xerr, xerrors.ErrNotFoundResult) {
			return nil, xerrors.ErrNotFoundProposal
		}
		return nil, xerr
	}
	prop, _ := item.(*proposal.GovProposal)
	return prop, nil
}

var _ ctrlertypes.ILedgerHandler = (*GovCtrler)(nil)
var _ ctrlertypes.ITrxHandler = (*GovCtrler)(nil)
var _ ctrlertypes.IBlockHandler = (*GovCtrler)(nil)
var _ ctrlertypes.IGovParams = (*GovCtrler)(nil)
