package gov

import (
	"bytes"
	"errors"
	cfg "github.com/beatoz/beatoz-go/cmd/config"
	"github.com/beatoz/beatoz-go/ctrlers/gov/proposal"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/genesis"
	"github.com/beatoz/beatoz-go/ledger"
	"github.com/beatoz/beatoz-go/types"
	abytes "github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/crypto"
	"github.com/beatoz/beatoz-go/types/xerrors"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/json"
	"github.com/tendermint/tendermint/libs/log"
	"strconv"
	"strings"
	"sync"
)

type GovCtrler struct {
	ctrlertypes.GovParams
	newGovParams *ctrlertypes.GovParams

	paramsLedger   ledger.IFinalityLedger[*ctrlertypes.GovParams]
	proposalLedger ledger.IFinalityLedger[*proposal.GovProposal]
	frozenLedger   ledger.IFinalityLedger[*proposal.GovProposal]

	logger log.Logger
	mtx    sync.RWMutex
}

func NewGovCtrler(config *cfg.Config, logger log.Logger) (*GovCtrler, error) {
	newGovParamsProvider := func() *ctrlertypes.GovParams { return &ctrlertypes.GovParams{} }
	newProposalProvider := func() *proposal.GovProposal {
		return &proposal.GovProposal{}
	}

	paramsLedger, xerr := ledger.NewFinalityLedger[*ctrlertypes.GovParams]("gov_params", config.DBDir(), 1, newGovParamsProvider)
	if xerr != nil {
		return nil, xerr
	}

	params, xerr := paramsLedger.Get(ledger.ToLedgerKey(abytes.ZeroBytes(32)))
	// `params` could be nil
	if xerr != nil && xerr != xerrors.ErrNotFoundResult {
		return nil, xerr
	} else if params == nil {
		params = &ctrlertypes.GovParams{} // empty params
	}

	proposalLedger, xerr := ledger.NewFinalityLedger[*proposal.GovProposal]("proposal", config.DBDir(), 1, newProposalProvider)
	if xerr != nil {
		return nil, xerr
	}

	frozenLedger, xerr := ledger.NewFinalityLedger[*proposal.GovProposal]("frozen_proposal", config.DBDir(), 1, newProposalProvider)
	if xerr != nil {
		return nil, xerr
	}

	return &GovCtrler{
		GovParams:      *params,
		paramsLedger:   paramsLedger,
		proposalLedger: proposalLedger,
		frozenLedger:   frozenLedger,
		logger:         logger.With("module", "beatoz_GovCtrler"),
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
	_ = ctrler.paramsLedger.SetFinality(&ctrler.GovParams)
	return nil
}

func (ctrler *GovCtrler) BeginBlock(blockCtx *ctrlertypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	var evts []abcitypes.Event

	byzantines := blockCtx.BlockInfo().ByzantineValidators
	if byzantines != nil && len(byzantines) > 0 {
		ctrler.logger.Info("GovCtrler: Byzantine validators is found", "count", len(byzantines))
		for _, evi := range byzantines {
			if slashed, xerr := ctrler.doPunish(&evi); xerr != nil {
				ctrler.logger.Error("Error when punishing",
					"byzantine", types.Address(evi.Validator.Address),
					"evidenceType", abcitypes.EvidenceType_name[int32(evi.Type)])
			} else {
				evts = append(evts, abcitypes.Event{
					Type: "punishment.gov",
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

	return evts, nil
}

// DoPunish is called from BeatozApp::BeginBlock.
func (ctrler *GovCtrler) DoPunish(evi *abcitypes.Evidence) (int64, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	return ctrler.doPunish(evi)
}

func (ctrler *GovCtrler) doPunish(evi *abcitypes.Evidence) (int64, xerrors.XError) {
	targetAddr := types.Address(evi.Validator.Address)

	// punish in `proposalLedger`
	var targetPropsKeys []ledger.LedgerKey
	ctrler.proposalLedger.IterateReadAllFinalityItems(func(prop *proposal.GovProposal) xerrors.XError {
		for _, v := range prop.Voters {
			if bytes.Compare(v.Addr, targetAddr) == 0 {
				targetPropsKeys = append(targetPropsKeys, prop.Key())
				break
			}
		}
		return nil
	})

	slashedPower := int64(0)
	for _, k := range targetPropsKeys {
		prop, _ := ctrler.proposalLedger.GetFinality(k)
		slashed, _ := prop.DoPunish(targetAddr, ctrler.SlashRatio())
		slashedPower += slashed

		_ = ctrler.proposalLedger.SetFinality(prop)
	}

	return slashedPower, nil
}

func (ctrler *GovCtrler) ValidateTrx(ctx *ctrlertypes.TrxContext) xerrors.XError {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	getProposal := ctrler.proposalLedger.Get
	if ctx.Exec {
		getProposal = ctrler.proposalLedger.GetFinality
	}

	// validation by tx type
	switch ctx.Tx.GetType() {
	case ctrlertypes.TRX_PROPOSAL:
		if bytes.Compare(ctx.Tx.To, types.ZeroAddress()) != 0 {
			return xerrors.ErrInvalidTrx.Wrap(errors.New("wrong address: the 'to' field in TRX_PROPOSAL should be zero address"))
		}

		// check right
		if ctx.StakeHandler.IsValidator(ctx.Tx.From) == false {
			return xerrors.ErrNoRight
		}

		// check tx type
		txpayload, ok := ctx.Tx.Payload.(*ctrlertypes.TrxPayloadProposal)
		if !ok {
			return xerrors.ErrInvalidTrxPayloadType
		}

		// check already exist
		if prop, xerr := getProposal(ctx.TxHash.Array32()); xerr != nil && xerr != xerrors.ErrNotFoundResult {
			return xerr
		} else if prop != nil {
			return xerrors.ErrDuplicatedKey
		}

		// check start height
		if txpayload.StartVotingHeight <= ctx.Height {
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
		prop, xerr := getProposal(txpayload.TxHash.Array32())
		if xerr != nil {
			return xerr
		}
		if prop.IsVoter(ctx.Tx.From) == false {
			return xerrors.ErrNoRight
		}

		// check choice validation
		if txpayload.Choice < 0 || txpayload.Choice >= int32(len(prop.Options)) {
			return xerrors.ErrInvalidTrxPayloadParams
		}

		// check end height
		if ctx.Height > prop.EndVotingHeight ||
			ctx.Height < prop.StartVotingHeight {
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

	setProposal := ctrler.proposalLedger.Set
	if ctx.Exec {
		setProposal = ctrler.proposalLedger.SetFinality
	}

	txpayload, _ := ctx.Tx.Payload.(*ctrlertypes.TrxPayloadProposal)

	voters := make(map[string]*proposal.Voter)
	vals, totalVotingPower := ctx.StakeHandler.Validators()
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
	if xerr = setProposal(prop); xerr != nil {
		return xerr
	}

	return nil
}

func (ctrler *GovCtrler) execVoting(ctx *ctrlertypes.TrxContext) xerrors.XError {
	getProposal := ctrler.proposalLedger.Get
	setProposal := ctrler.proposalLedger.Set
	if ctx.Exec {
		getProposal = ctrler.proposalLedger.GetFinality
		setProposal = ctrler.proposalLedger.SetFinality
	}

	txpayload, _ := ctx.Tx.Payload.(*ctrlertypes.TrxPayloadVoting)
	prop, xerr := getProposal(ledger.ToLedgerKey(txpayload.TxHash))
	if xerr != nil {
		return xerr
	}
	if xerr = prop.DoVote(ctx.Tx.From, txpayload.Choice); xerr != nil {
		return xerr
	}
	if xerr = setProposal(prop); xerr != nil {
		return xerr
	}
	if prop.MajorOption != nil {
		ctrler.logger.Debug("Voting to proposal", "key", prop.TxHash, "voter", ctx.Tx.From, "choice", txpayload.Choice)
	}
	return nil
}

func (ctrler *GovCtrler) EndBlock(ctx *ctrlertypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	var evts []abcitypes.Event

	frozen, removed, xerr := ctrler.freezeProposals(ctx.Height())
	if xerr != nil {
		return nil, xerr
	}

	applied, xerr := ctrler.applyProposals(ctx.Height())
	if xerr != nil {
		return nil, xerr
	}

	for _, h := range frozen {
		evts = append(evts, abcitypes.Event{
			Type: "proposal",
			Attributes: []abcitypes.EventAttribute{
				{Key: []byte("frozen"), Value: []byte(h.String()), Index: true},
			},
		})
	}
	for _, h := range removed {
		evts = append(evts, abcitypes.Event{
			Type: "proposal",
			Attributes: []abcitypes.EventAttribute{
				{Key: []byte("removed"), Value: []byte(h.String()), Index: true},
			},
		})
	}
	for _, h := range applied {
		evts = append(evts, abcitypes.Event{
			Type: "proposal",
			Attributes: []abcitypes.EventAttribute{
				{Key: []byte("applied"), Value: []byte(h.String()), Index: true},
			},
		})
	}

	return evts, nil
}

// The following function is called by the Block Executor
//

func (ctrler *GovCtrler) freezeProposals(height int64) ([]abytes.HexBytes, []abytes.HexBytes, xerrors.XError) {
	var frozen []abytes.HexBytes
	var removed []abytes.HexBytes
	xerr := ctrler.proposalLedger.IterateReadAllItems(func(prop *proposal.GovProposal) xerrors.XError {
		if prop.EndVotingHeight < height {

			// freezing
			if _, xerr := ctrler.proposalLedger.DelFinality(prop.Key()); xerr != nil {
				return xerr
			}

			majorOpt := prop.UpdateMajorOption()
			if majorOpt != nil {
				// freeze the proposal
				if xerr := ctrler.frozenLedger.SetFinality(prop); xerr != nil {
					return xerr
				}
				frozen = append(frozen, prop.TxHash)
			} else {
				// do nothing. the proposal will be just removed.
				ctrler.logger.Debug("Freeze proposal", "warning", "not found major option")
				removed = append(removed, prop.TxHash)
			}
		}
		return nil
	})
	return frozen, removed, xerr
}

func (ctrler *GovCtrler) applyProposals(height int64) ([]abytes.HexBytes, xerrors.XError) {
	var applied []abytes.HexBytes
	xerr := ctrler.frozenLedger.IterateReadAllItems(func(prop *proposal.GovProposal) xerrors.XError {
		if prop.ApplyingHeight <= height {
			if _, xerr := ctrler.frozenLedger.DelFinality(prop.Key()); xerr != nil {
				return xerr
			}
			if prop.MajorOption != nil {
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
					if xerr := ctrler.paramsLedger.SetFinality(newGovParams); xerr != nil {
						ctrler.logger.Error("Apply proposal", "error", xerr, "newGovParams", newGovParams)
						return xerr
					}
					ctrler.newGovParams = newGovParams
				default:
					key := prop.Key()
					ctrler.logger.Debug("Apply proposal", "key(txHash)", abytes.HexBytes(key[:]), "type", prop.OptType)
				}

				applied = append(applied, prop.TxHash)
			} else {
				ctrler.logger.Error("Apply proposal", "error", "major option is nil")
			}
		}
		return nil
	})

	return applied, xerr
}

func (ctrler *GovCtrler) Commit() ([]byte, int64, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	h0, v0, xerr := ctrler.paramsLedger.Commit()
	if xerr != nil {
		return nil, -1, xerr
	}
	h1, v1, xerr := ctrler.proposalLedger.Commit()
	if xerr != nil {
		return nil, -1, xerr
	}
	h2, v2, xerr := ctrler.frozenLedger.Commit()
	if xerr != nil {
		return nil, -1, xerr
	}

	if v0 != v1 || v1 != v2 {
		return nil, -1, xerrors.ErrCommit.Wrapf("error: GovCtrler.Commit() has wrong version number - v0:%v, v1:%v, v2:%v", v0, v1, v2)
	}

	if ctrler.newGovParams != nil {
		ctrler.GovParams = *ctrler.newGovParams
		ctrler.newGovParams = nil
		ctrler.logger.Debug("New governance parameters is committed", "gov_params", ctrler.GovParams.String())
	}
	return crypto.DefaultHash(h0, h1, h2), v0, nil

}

func (ctrler *GovCtrler) Close() xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	if ctrler.paramsLedger != nil {
		if xerr := ctrler.paramsLedger.Close(); xerr != nil {
			ctrler.logger.Error("paramsLedger.Close()", "error", xerr.Error())
		}
		ctrler.paramsLedger = nil
	}
	if ctrler.proposalLedger != nil {
		if xerr := ctrler.proposalLedger.Close(); xerr != nil {
			ctrler.logger.Error("proposalLedger.Close()", "error", xerr.Error())
		}
		ctrler.proposalLedger = nil
	}
	return nil
}

func (ctrler *GovCtrler) GetGovParams() ctrlertypes.GovParams {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	return ctrler.GovParams
}

func (ctrler *GovCtrler) ReadAllProposals() ([]*proposal.GovProposal, xerrors.XError) {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	var proposals []*proposal.GovProposal

	if xerr := ctrler.proposalLedger.IterateReadAllItems(func(prop *proposal.GovProposal) xerrors.XError {
		proposals = append(proposals, prop)
		return nil
	}); xerr != nil {
		if xerr == xerrors.ErrNotFoundResult {
			return nil, xerrors.ErrNotFoundProposal
		}
		return nil, xerr
	}

	return proposals, nil
}

func (ctrler *GovCtrler) ReadProposal(txhash abytes.HexBytes) (*proposal.GovProposal, xerrors.XError) {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	if prop, xerr := ctrler.proposalLedger.Read(ledger.ToLedgerKey(txhash)); xerr != nil {
		if xerr == xerrors.ErrNotFoundResult {
			return nil, xerrors.ErrNotFoundProposal
		}
		return nil, xerr
	} else {
		return prop, nil
	}
}

var _ ctrlertypes.ILedgerHandler = (*GovCtrler)(nil)
var _ ctrlertypes.ITrxHandler = (*GovCtrler)(nil)
var _ ctrlertypes.IBlockHandler = (*GovCtrler)(nil)
var _ ctrlertypes.IGovHandler = (*GovCtrler)(nil)
