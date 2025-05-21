package node

import (
	"github.com/beatoz/beatoz-go/libs/jsonx"
	rtypes "github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

func (ctrler *BeatozApp) Query(req abcitypes.RequestQuery) abcitypes.ResponseQuery {
	if req.Height == 0 {
		// last block height
		req.Height = ctrler.lastBlockCtx.Height()
	}

	response := abcitypes.ResponseQuery{
		Code:   abcitypes.CodeTypeOK,
		Key:    req.Data,
		Height: req.Height,
	}

	var xerr xerrors.XError

	switch req.Path {
	case "account":
		response.Value, xerr = ctrler.acctCtrler.Query(req)
		if xerr == nil {
			_acct := &struct {
				Address rtypes.Address `json:"address"`
				Name    string         `json:"name,omitempty"`
				Nonce   uint64         `json:"nonce,string"`
				Balance string         `json:"balance"`
				Code    bytes.HexBytes `json:"code,omitempty"`
				DocURL  string         `json:"docURL,omitempty"`
			}{}
			if err := jsonx.Unmarshal(response.Value, &_acct); err != nil {
				xerr = xerrors.ErrQuery.Wrap(err)
			}
		}

	case "stakes", "stakes/total_power", "stakes/voting_power", "delegatee":
		response.Value, xerr = ctrler.vpowCtrler.Query(
			req,
			func() interface{} {
				return ctrler.govCtrler.MaxValidatorCnt()
			},
			func() interface{} {
				return ctrler.govCtrler.MinValidatorPower()
			},
		)
	case "reward":
		response.Value, xerr = ctrler.supplyCtrler.Query(req)
	case "proposal", "gov_params":
		response.Value, xerr = ctrler.govCtrler.Query(req)
	case "vm_call", "vm_estimate_gas":
		response.Value, xerr = ctrler.vmCtrler.Query(req)
	default:
		response.Value, xerr = nil, xerrors.ErrInvalidQueryPath
	}

	if xerr != nil {
		ctrler.logger.Error("BeatozApp - Query returns error", "error", xerr, "request", req)
		response.Code = xerr.Code()
		response.Log = xerr.Error()
	}

	return response
}
