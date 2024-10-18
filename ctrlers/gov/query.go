package gov

import (
	"github.com/beatoz/beatoz-go/ctrlers/gov/proposal"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmjson "github.com/tendermint/tendermint/libs/json"
)

func (ctrler *GovCtrler) Query(req abcitypes.RequestQuery) ([]byte, xerrors.XError) {
	txhash := req.Data
	switch req.Path {
	case "proposal":
		atProposalLedger, xerr := ctrler.proposalState.ImitableLedgerAt(req.Height)
		if xerr != nil {
			return nil, xerrors.ErrQuery.Wrap(xerr)
		}

		atFrozenLedger, xerr := ctrler.frozenState.ImitableLedgerAt(req.Height)
		if xerr != nil {
			return nil, xerrors.ErrQuery.Wrap(xerr)
		}

		type _response struct {
			Status   string                `json:"status"`
			Proposal *proposal.GovProposal `json:"proposal"`
		}

		if txhash == nil || len(txhash) == 0 {
			var readProposals []*_response
			if xerr := atProposalLedger.Iterate(func(item v1.ILedgerItem) xerrors.XError {
				prop := item.(*proposal.GovProposal)
				readProposals = append(readProposals, &_response{
					Status:   "voting",
					Proposal: prop,
				})
				return nil
			}); xerr != nil {
				return nil, xerrors.ErrQuery.Wrap(xerr)
			}

			if xerr = atFrozenLedger.Iterate(func(item v1.ILedgerItem) xerrors.XError {
				prop := item.(*proposal.GovProposal)
				readProposals = append(readProposals, &_response{
					Status:   "frozen",
					Proposal: prop,
				})
				return nil
			}); xerr != nil {
				return nil, xerrors.ErrQuery.Wrap(xerr)
			}

			v, err := tmjson.Marshal(readProposals)
			if err != nil {
				return nil, xerrors.ErrQuery.Wrap(err)
			}
			return v, nil
		} else {
			item, xerr := atProposalLedger.Get(txhash)
			prop := item.(*proposal.GovProposal)
			resp := &_response{Status: "voting"}
			if xerr != nil {
				if xerr.Code() == xerrors.ErrCodeNotFoundResult {
					item, xerr = atFrozenLedger.Get(txhash)
					prop = item.(*proposal.GovProposal)
					if xerr != nil {
						return nil, xerrors.ErrQuery.Wrap(xerr)
					}
					resp.Status = "frozen"
				} else {
					return nil, xerrors.ErrQuery.Wrap(xerr)
				}
			}
			resp.Proposal = prop

			v, err := tmjson.Marshal(resp)
			if err != nil {
				return nil, xerrors.ErrQuery.Wrap(err)
			}

			return v, nil
		}
	case "gov_params":
		atledger, xerr := ctrler.paramsState.ImitableLedgerAt(req.Height)
		if xerr != nil {
			return nil, xerrors.ErrQuery.Wrap(xerr)
		}
		govParams, xerr := atledger.Get(bytes.ZeroBytes(32))
		if xerr != nil {
			return nil, xerrors.ErrQuery.Wrap(xerr)
		}
		bz, err := tmjson.Marshal(govParams)
		if err != nil {
			return nil, xerrors.ErrQuery.Wrap(err)
		}
		return bz, nil
	}

	return nil, nil
}
