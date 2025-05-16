package gov

import (
	"encoding/hex"
	"github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/ledger/v1"
	types3 "github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/crypto"
	"github.com/beatoz/beatoz-go/types/xerrors"
	types2 "github.com/tendermint/tendermint/abci/types"
	"strconv"
)

func (ctrler *GovCtrler) BeginBlock(blockCtx *types.BlockContext) ([]types2.Event, xerrors.XError) {
	var evts []types2.Event

	byzantines := blockCtx.BlockInfo().ByzantineValidators
	if byzantines != nil && len(byzantines) > 0 {
		ctrler.logger.Info("GovCtrler: Byzantine validators is found", "count", len(byzantines))
		for _, evi := range byzantines {
			if slashed, xerr := ctrler.doPunish(&evi); xerr != nil {
				ctrler.logger.Error("Error when punishing",
					"byzantine", types3.Address(evi.Validator.Address),
					"evidenceType", types2.EvidenceType_name[int32(evi.Type)])
			} else {
				evts = append(evts, types2.Event{
					Type: "punishment.gov",
					Attributes: []types2.EventAttribute{
						{Key: []byte("byzantine"), Value: []byte(types3.Address(evi.Validator.Address).String()), Index: true},
						{Key: []byte("type"), Value: []byte(types2.EvidenceType_name[int32(evi.Type)]), Index: false},
						{Key: []byte("height"), Value: []byte(strconv.FormatInt(evi.Height, 10)), Index: false},
						{Key: []byte("slashed"), Value: []byte(strconv.FormatInt(slashed, 10)), Index: false},
					},
				})
			}
		}
	}

	return evts, nil
}

func (ctrler *GovCtrler) EndBlock(ctx *types.BlockContext) ([]types2.Event, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	var evts []types2.Event

	frozen, removed, xerr := ctrler.freezeProposals(ctx.Height())
	if xerr != nil {
		return nil, xerr
	}

	applied, xerr := ctrler.applyProposals(ctx.Height())
	if xerr != nil {
		return nil, xerr
	}

	for _, k := range frozen {
		evts = append(evts, types2.Event{
			Type: "proposal",
			Attributes: []types2.EventAttribute{
				{Key: []byte("frozen"), Value: []byte(hex.EncodeToString(v1.UnwrapKeyPrefix(k))), Index: true},
			},
		})
	}
	for _, k := range removed {
		evts = append(evts, types2.Event{
			Type: "proposal",
			Attributes: []types2.EventAttribute{
				{Key: []byte("removed"), Value: []byte(hex.EncodeToString(v1.UnwrapKeyPrefix(k))), Index: true},
			},
		})
	}
	for _, k := range applied {
		evts = append(evts, types2.Event{
			Type: "proposal",
			Attributes: []types2.EventAttribute{
				{Key: []byte("applied"), Value: []byte(hex.EncodeToString(v1.UnwrapKeyPrefix(k))), Index: true},
			},
		})
	}

	return evts, nil
}

func (ctrler *GovCtrler) Commit() ([]byte, int64, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	h0, v0, xerr := ctrler.paramsState.Commit()
	if xerr != nil {
		return nil, -1, xerr
	}
	h1, v1, xerr := ctrler.proposalState.Commit()
	if xerr != nil {
		return nil, -1, xerr
	}
	h2, v2, xerr := ctrler.frozenState.Commit()
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
