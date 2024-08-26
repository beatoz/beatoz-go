package gov

import (
	"github.com/beatoz/beatoz-go/ctrlers/gov/proposal"
	"github.com/beatoz/beatoz-go/ledger"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmjson "github.com/tendermint/tendermint/libs/json"
)

func (ctrler *GovCtrler) Query(req abcitypes.RequestQuery) ([]byte, xerrors.XError) {
	txhash := req.Data
	switch req.Path {
	case "proposal":
		atProposalLedger, xerr := ctrler.proposalLedger.ImmutableLedgerAt(req.Height, 0)
		if xerr != nil {
			return nil, xerrors.ErrQuery.Wrap(xerr)
		}

		atFrozenLedger, xerr := ctrler.frozenLedger.ImmutableLedgerAt(req.Height, 0)
		if xerr != nil {
			return nil, xerrors.ErrQuery.Wrap(xerr)
		}

		type _prop struct {
			Status   string                `json:"status"`
			Proposal *proposal.GovProposal `json:"proposal"`
		}

		if txhash == nil || len(txhash) == 0 {
			var readProposals []*_prop
			if xerr := atProposalLedger.IterateReadAllItems(func(prop *proposal.GovProposal) xerrors.XError {
				readProposals = append(readProposals, &_prop{
					Status:   "voting",
					Proposal: prop,
				})
				return nil
			}); xerr != nil {
				return nil, xerrors.ErrQuery.Wrap(xerr)
			}

			if xerr = atFrozenLedger.IterateReadAllItems(func(prop *proposal.GovProposal) xerrors.XError {
				readProposals = append(readProposals, &_prop{
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
			prop, xerr := atProposalLedger.Read(ledger.ToLedgerKey(txhash))
			readProposal := &_prop{}
			if xerr != nil {
				if xerr.Code() == xerrors.ErrCodeNotFoundResult {
					prop, xerr = atFrozenLedger.Read(ledger.ToLedgerKey(txhash))
					if xerr != nil {
						return nil, xerrors.ErrQuery.Wrap(xerr)
					}
					readProposal.Status = "frozen"
				} else {
					return nil, xerrors.ErrQuery.Wrap(xerr)
				}
			} else {
				readProposal.Status = "voting"
			}
			readProposal.Proposal = prop

			v, err := tmjson.Marshal(readProposal)
			if err != nil {
				return nil, xerrors.ErrQuery.Wrap(err)
			}

			return v, nil
		}
	case "gov_params":
		atledger, xerr := ctrler.paramsLedger.ImmutableLedgerAt(req.Height, 0)
		if xerr != nil {
			return nil, xerrors.ErrQuery.Wrap(xerr)
		}
		govParams, xerr := atledger.Read(ledger.ToLedgerKey(bytes.ZeroBytes(32)))
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
