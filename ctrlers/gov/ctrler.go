package gov

import (
	"bytes"
	"errors"
	cfg "github.com/beatoz/beatoz-go/cmd/config"
	"github.com/beatoz/beatoz-go/ctrlers/gov/proposal"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/genesis"
	ledger "github.com/beatoz/beatoz-go/ledger"
	"github.com/beatoz/beatoz-go/ledger/common"
	"github.com/beatoz/beatoz-go/libs/jsonx"
	"github.com/beatoz/beatoz-go/types"
	abytes "github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	"github.com/tendermint/tendermint/libs/log"
	"sync"
)

type GovCtrler struct {
	ctrlertypes.GovParams
	newGovParams *ctrlertypes.GovParams

	govState *ledger.StateLedgerManager

	logger log.Logger
	mtx    sync.RWMutex
}

var defaultNewItemFor = func(key common.LedgerKey) common.ILedgerItem {
	if bytes.HasPrefix(key, common.KeyPrefixGovParams) {
		return &ctrlertypes.GovParams{}
	}
	if bytes.HasPrefix(key, common.KeyPrefixProposal) || bytes.HasPrefix(key, common.KeyPrefixFrozenProp) {
		return &proposal.GovProposal{}
	}
	panic("unknown key prefix")
	return nil
}

func NewGovCtrler(config *cfg.Config, logger log.Logger) (*GovCtrler, error) {
	lg := logger.With("module", "beatoz_GovCtrler")

	govState, xerr := ledger.NewStateLedgerManager("gov", config.DBDir(), 16, defaultNewItemFor, lg)
	if xerr != nil {
		return nil, xerr
	}

	params, xerr := govState.Get(common.LedgerKeyGovParams(), true)
	// `params` may be nil
	if xerr != nil && xerr != xerrors.ErrNotFoundResult {
		return nil, xerr
	} else if params == nil {
		params = &ctrlertypes.GovParams{} // empty params
	}

	return &GovCtrler{
		GovParams: *(params.(*ctrlertypes.GovParams)),
		govState:  govState,
		logger:    lg,
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
	_ = ctrler.govState.Set(common.LedgerKeyGovParams(), &ctrler.GovParams, true)
	return nil
}

func (ctrler *GovCtrler) ValidateTrx(ctx *ctrlertypes.TrxContext) xerrors.XError {
	var currentTotalSupply *uint256.Int
	if ctx.Tx.GetType() == ctrlertypes.TRX_PROPOSAL {
		txpayload, ok := ctx.Tx.Payload.(*ctrlertypes.TrxPayloadProposal)
		if ok && txpayload.OptType == proposal.PROPOSAL_GOVPARAMS {
			currentTotalSupply = ctx.SupplyHandler.TotalSupply()
		}
	}

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
		prop, xerr := ctrler.govState.Get(common.LedgerKeyProposal(ctx.TxHash), ctx.Exec)
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
			for _, option := range txpayload.Options {
				checkGovParams := &ctrlertypes.GovParams{}
				if err := jsonx.Unmarshal(option, checkGovParams); err != nil {
					return xerrors.ErrInvalidTrxPayloadParams.Wrap(err)
				}
				ctrlertypes.MergeGovParams(&ctrler.GovParams, checkGovParams)
				if xerr := checkGovParams.ValidateBasic(); xerr != nil {
					return xerrors.ErrInvalidTrxPayloadParams.Wrap(xerr)
				}
				if xerr := checkGovParams.ValidateCurrentSupply(currentTotalSupply); xerr != nil {
					return xerrors.ErrInvalidTrxPayloadParams.Wrap(xerr)
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
			return xerrors.ErrInvalidTrxPayloadParams.Wrapf(
				"wrong applyingHeight: must be set equal to or higher than minApplyingHeight. ApplyHeight:%v, minApplyingHeight:%v, endVotingHeight:%v, lazyApplyingBlocks:%v",
				txpayload.ApplyingHeight,
				minApplyingHeight,
				endVotingHeight,
				ctrler.LazyApplyingBlocks())
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
		item, xerr := ctrler.govState.Get(common.LedgerKeyProposal(txpayload.TxHash), ctx.Exec)
		if xerr != nil {
			return xerr
		}
		prop, _ := item.(*proposal.GovProposal)
		if prop.IsVoter(ctx.Tx.From) == false {
			return xerrors.ErrNoRight
		}

		// check choice validation
		if txpayload.Choice < 0 || txpayload.Choice >= int32(len(prop.Options())) {
			return xerrors.ErrInvalidTrxPayloadParams
		}

		// check end height
		if ctx.Height() > prop.Header().EndVotingHeight ||
			ctx.Height() < prop.Header().StartVotingHeight {
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

	vals, totalVotingPower := ctx.VPowerHandler.Validators()
	voters := make([]*proposal.VoterProto, len(vals))
	for i, v := range vals {
		voters[i] = &proposal.VoterProto{
			Address: v.Address,
			Power:   v.Power,
			Choice:  proposal.NOT_CHOICE, // -1
		}
	}

	prop := proposal.NewGovProposal(txpayload.OptType, ctx.TxHash,
		txpayload.StartVotingHeight, txpayload.VotingPeriodBlocks,
		totalVotingPower, txpayload.ApplyingHeight)

	for _, v := range voters {
		prop.AddVoter(v.Address, v.Power)
	}
	for _, opt := range txpayload.Options {
		prop.AddOption(opt)
	}
	if xerr := ctrler.govState.Set(common.LedgerKeyProposal(prop.Header().TxHash), prop, ctx.Exec); xerr != nil {
		return xerr
	}

	return nil
}

func (ctrler *GovCtrler) execVoting(ctx *ctrlertypes.TrxContext) xerrors.XError {
	txpayload, _ := ctx.Tx.Payload.(*ctrlertypes.TrxPayloadVoting)
	item, xerr := ctrler.govState.Get(common.LedgerKeyProposal(txpayload.TxHash), ctx.Exec)
	if xerr != nil {
		return xerr
	}
	prop, _ := item.(*proposal.GovProposal)
	if xerr = prop.DoVote(ctx.Tx.From, txpayload.Choice); xerr != nil {
		return xerr
	}
	if xerr = ctrler.govState.Set(common.LedgerKeyProposal(prop.Header().TxHash), prop, ctx.Exec); xerr != nil {
		return xerr
	}
	if prop.MajorOption() != nil {
		ctrler.logger.Debug("Voting to proposal", "key", prop.Header().TxHash, "voter", ctx.Tx.From, "choice", txpayload.Choice)
	}
	return nil
}

// freezeProposals is called from EndBlock
func (ctrler *GovCtrler) freezeProposals(height int64) ([]common.LedgerKey, []common.LedgerKey, xerrors.XError) {
	var frozenProps []common.LedgerKey
	var removedProps []common.LedgerKey
	var newFrozens []*proposal.GovProposal

	defer func() {
		for _, _prop := range newFrozens {
			// set new frozen proposal with common.LedgerKeyFrozenProp
			_ = ctrler.govState.Set(common.LedgerKeyFrozenProp(_prop.Header().TxHash), _prop, true)
		}
		for _, k := range frozenProps {
			// remove frozen proposal
			_ = ctrler.govState.Del(k, true)
		}
		for _, k := range removedProps {
			// remove proposal
			_ = ctrler.govState.Del(k, true)
		}
	}()

	xerr := ctrler.govState.Seek(common.KeyPrefixProposal, true, func(key common.LedgerKey, item common.ILedgerItem) xerrors.XError {
		prop, _ := item.(*proposal.GovProposal)
		if prop.Header().EndVotingHeight < height {

			// DO NOT REMOVE `prop` from `proposalState`

			majorOpt := prop.UpdateMajorOption()
			if majorOpt != nil {
				// freeze the proposal
				newFrozens = append(newFrozens, prop)
				frozenProps = append(frozenProps, key)
			} else {
				// do nothing. the proposal will be just removedProps.
				ctrler.logger.Debug("Freeze proposal", "warning", "not found major option")
				removedProps = append(removedProps, key)
			}
		}
		return nil
	}, true)
	return frozenProps, removedProps, xerr
}

// applyProposals is called from EndBlock
func (ctrler *GovCtrler) applyProposals(height int64) ([]common.LedgerKey, []common.LedgerKey, xerrors.XError) {
	var applied []common.LedgerKey
	var rejected []common.LedgerKey

	defer func() {
		if ctrler.newGovParams != nil {
			_ = ctrler.govState.Set(common.LedgerKeyGovParams(), ctrler.newGovParams, true)
		}

		for _, k := range applied {
			// remove
			_ = ctrler.govState.Del(k, true)
		}
		for _, k := range rejected {
			// remove
			_ = ctrler.govState.Del(k, true)
		}
	}()

	xerr := ctrler.govState.Seek(common.KeyPrefixFrozenProp, true, func(key common.LedgerKey, item common.ILedgerItem) xerrors.XError {
		prop, _ := item.(*proposal.GovProposal)
		if prop.Header().ApplyHeight <= height {

			// DO NOT REMOVE `prop` from `frozenState` in Seek.

			if prop.MajorOption() == nil {
				// not reachable.
				ctrler.logger.Error("Apply proposal", "error", "major option is nil")
				rejected = append(rejected, key)
				return nil
			}

			switch prop.Header().PropType {
			case proposal.PROPOSAL_GOVPARAMS:
				newGovParams := &ctrlertypes.GovParams{}

				strOpt := string(prop.MajorOption().Option)
				if err := jsonx.Unmarshal([]byte(strOpt), newGovParams); err != nil {
					ctrler.logger.Error("Apply proposal", "error", err, "option", string(prop.MajorOption().Option))
					rejected = append(rejected, key)
					return nil
				}

				ctrlertypes.MergeGovParams(&ctrler.GovParams, newGovParams)
				if xerr := newGovParams.ValidateBasic(); xerr != nil {
					ctrler.logger.Error("Apply proposal", "error", xerr, "option", string(prop.MajorOption().Option))
					rejected = append(rejected, key)
					return nil
				}
				ctrler.newGovParams = newGovParams
			default:
				ctrler.logger.Debug("Apply proposal", "key(txHash)", prop.Header().TxHash, "type", prop.Header().PropType)
			}

			applied = append(applied, key) // this key will be removed from frozenState

		}
		return nil
	}, true)

	return applied, rejected, xerr
}

func (ctrler *GovCtrler) CreateCache(exec bool) xerrors.XError {
	return ctrler.govState.CreateCache(exec)
}

func (ctrler *GovCtrler) WriteCache(exec bool) xerrors.XError {
	return ctrler.govState.WriteCache(exec)
}

func (ctrler *GovCtrler) ClearCache(exec bool) xerrors.XError {
	return ctrler.govState.ClearCache(exec)
}

func (ctrler *GovCtrler) Close() xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	if ctrler.govState != nil {
		if xerr := ctrler.govState.Close(); xerr != nil {
			ctrler.logger.Error("govState.Close()", "error", xerr.Error())
		}
		ctrler.govState = nil
	}
	return nil
}

func (ctrler *GovCtrler) ReadAllProposals(exec bool) ([]*proposal.GovProposal, xerrors.XError) {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	var proposals []*proposal.GovProposal

	if xerr := ctrler.govState.Seek(common.KeyPrefixProposal, true, func(key common.LedgerKey, item common.ILedgerItem) xerrors.XError {
		prop, _ := item.(*proposal.GovProposal)
		proposals = append(proposals, prop)
		return nil
	}, exec); xerr != nil {
		if xerr.Contains(xerrors.ErrNotFoundResult) {
			return nil, xerrors.ErrNotFoundProposal
		}
		return nil, xerr
	}

	return proposals, nil
}

func (ctrler *GovCtrler) ReadProposal(txhash abytes.HexBytes, exec bool) (*proposal.GovProposal, xerrors.XError) {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	item, xerr := ctrler.govState.Get(common.LedgerKeyProposal(txhash), exec)
	if xerr != nil {
		if xerr.Contains(xerrors.ErrNotFoundResult) {
			return nil, xerrors.ErrNotFoundProposal
		}
		return nil, xerr
	}
	prop, _ := item.(*proposal.GovProposal)
	return prop, nil
}

func (ctrler *GovCtrler) UpgradeLedgerVersion(target common.LedgerVersion) xerrors.XError {
	return ctrler.govState.UpgradeLedgerVersion(target)
}

var _ ctrlertypes.ILedgerHandler = (*GovCtrler)(nil)
var _ ctrlertypes.ITrxHandler = (*GovCtrler)(nil)
var _ ctrlertypes.IBlockHandler = (*GovCtrler)(nil)
var _ ctrlertypes.IGovParams = (*GovCtrler)(nil)
var _ ctrlertypes.ILedgerVersionHandler = (*GovCtrler)(nil)
