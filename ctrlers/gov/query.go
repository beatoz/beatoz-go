package gov

import (
	"github.com/beatoz/beatoz-go/ctrlers/gov/proposal"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/libs/jsonx"
	"github.com/beatoz/beatoz-go/types/xerrors"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

func (ctrler *GovCtrler) Query(req abcitypes.RequestQuery, opts ...ctrlertypes.Option) ([]byte, xerrors.XError) {

	atledger, xerr := ctrler.govState.ImitableLedgerAt(req.Height)
	if xerr != nil {
		return nil, xerrors.ErrQuery.Wrap(xerr)
	}

	switch req.Path {
	case "proposal":
		type _response struct {
			Status   string                `json:"status"`
			Proposal *proposal.GovProposal `json:"proposal"`
		}

		txhash := req.Data
		if len(txhash) == 0 {
			var readProposals []*_response
			if xerr := atledger.Seek(v1.KeyPrefixProposal, true, func(key v1.LedgerKey, item v1.ILedgerItem) xerrors.XError {
				prop, _ := item.(*proposal.GovProposal)
				readProposals = append(readProposals, &_response{
					Status:   "voting",
					Proposal: prop,
				})
				return nil
			}); xerr != nil {
				return nil, xerrors.ErrQuery.Wrap(xerr)
			}

			if xerr = atledger.Seek(v1.KeyPrefixFrozenProp, true, func(key v1.LedgerKey, item v1.ILedgerItem) xerrors.XError {
				prop, _ := item.(*proposal.GovProposal)
				readProposals = append(readProposals, &_response{
					Status:   "frozen",
					Proposal: prop,
				})
				return nil
			}); xerr != nil {
				return nil, xerrors.ErrQuery.Wrap(xerr)
			}

			v, err := jsonx.Marshal(readProposals)
			if err != nil {
				return nil, xerrors.ErrQuery.Wrap(err)
			}
			return v, nil
		} else {
			item, xerr := atledger.Get(v1.LedgerKeyProposal(txhash))
			resp := &_response{Status: "voting"}
			if xerr != nil {
				if xerr.Code() == xerrors.ErrCodeNotFoundResult {
					item, xerr = atledger.Get(v1.LedgerKeyFrozenProp(txhash))
					if xerr != nil {
						return nil, xerrors.ErrQuery.Wrap(xerr)
					}
					resp.Status = "frozen"
				} else {
					return nil, xerrors.ErrQuery.Wrap(xerr)
				}
			}
			prop, _ := item.(*proposal.GovProposal)
			resp.Proposal = prop

			v, err := jsonx.Marshal(resp)
			if err != nil {
				return nil, xerrors.ErrQuery.Wrap(err)
			}

			return v, nil
		}
	case "gov_params":
		govParams, xerr := atledger.Get(v1.LedgerKeyGovParams())
		if xerr != nil {
			return nil, xerrors.ErrQuery.Wrap(xerr)
		}
		bz, err := jsonx.Marshal(govParams)
		if err != nil {
			return nil, xerrors.ErrQuery.Wrap(err)
		}
		return bz, nil
	}

	return nil, nil
}
