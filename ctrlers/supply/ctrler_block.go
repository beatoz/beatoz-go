package supply

import (
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types/xerrors"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"time"
)

func (ctrler *SupplyCtrler) BeginBlock(bctx *ctrlertypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	//
	// Request to mint & reward
	if bctx.Height() > 0 && bctx.Height()%bctx.GovHandler.InflationCycleBlocks() == 0 {
		ctrler.requestMint(bctx)
	}

	return nil, nil
}

func (ctrler *SupplyCtrler) EndBlock(bctx *ctrlertypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	var evts []abcitypes.Event

	//
	// Wait to finish minting...
	if bctx.Height() > 0 && bctx.Height()%bctx.GovHandler.InflationCycleBlocks() == 0 {
		start := time.Now()
		// In ctrler.waitMint, ctrler.lastTotalSupply is changed.
		resp, xerr := ctrler.waitMint(bctx)
		since := time.Since(start)

		ctrler.logger.Debug("wait to process mint and reward", "delay", since)
		if xerr != nil {
			ctrler.logger.Error("waitMint returns", "error", xerr.Error())
			return nil, xerr
		}

		evts = append(evts, abcitypes.Event{
			Type: "supply.mint",
			Attributes: []abcitypes.EventAttribute{
				{Key: []byte("mint"), Value: []byte(resp.sumMintedAmt.Dec()), Index: false},
				{Key: []byte("total.supply"), Value: []byte(ctrler.lastTotalSupply.totalSupply.Dec()), Index: false},
			},
		})
	}

	//
	// Set supply info to ledger
	if ctrler.lastTotalSupply.IsChanged() {
		if xerr := ctrler.supplyState.Set(v1.LedgerKeyTotalSupply(), ctrler.lastTotalSupply, true); xerr != nil {
			return nil, xerr
		}
		ctrler.lastTotalSupply.ResetChanged()
	}
	return evts, nil
}

func (ctrler *SupplyCtrler) Commit() ([]byte, int64, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	h, v, xerr := ctrler.supplyState.Commit()
	if xerr != nil {
		return nil, 0, xerr
	}

	return h, v, nil
}
