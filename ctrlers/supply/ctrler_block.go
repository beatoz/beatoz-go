package supply

import (
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

func (ctrler *SupplyCtrler) BeginBlock(bctx *ctrlertypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	//
	// issue additional supply & reward
	if bctx.Height() > 0 && bctx.Height()%bctx.GovHandler.InflationCycleBlocks() == 0 {
		ctrler.requestMint(bctx)
	}
	return nil, nil
}

func (ctrler *SupplyCtrler) EndBlock(bctx *ctrlertypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	if bctx.Height() > 0 && bctx.Height()%bctx.GovHandler.InflationCycleBlocks() == 0 {
		if _, xerr := ctrler.waitMint(bctx); xerr != nil {
			ctrler.logger.Error("fail to requestMint", "error", xerr.Error())
			return nil, xerr
		}
	}

	//
	// Process transactions fee
	header := bctx.BlockInfo().Header
	sumFee := bctx.SumFee()
	if header.GetProposerAddress() != nil && sumFee.Sign() > 0 {

		// burn GovParams.BurnRate % of txs fee.
		burnAmt := new(uint256.Int).Mul(sumFee, uint256.NewInt(uint64(bctx.GovHandler.BurnRate())))
		burnAmt = new(uint256.Int).Div(burnAmt, uint256.NewInt(100))
		if xerr := ctrler.burn(bctx.Height(), burnAmt); xerr != nil {
			return nil, xerr
		}
		if xerr := bctx.AcctHandler.AddBalance(bctx.GovHandler.BurnAddress(), burnAmt, true); xerr != nil {
			return nil, xerr
		}

		// distribute the remaining fee to the proposer of this block.
		rwdAmt := new(uint256.Int).Sub(sumFee, burnAmt)
		if xerr := bctx.AcctHandler.AddBalance(header.GetProposerAddress(), rwdAmt, true); xerr != nil {
			return nil, xerr
		}

		ctrler.logger.Debug("txs's fee is processed", "total.fee", sumFee.Dec(), "reward", rwdAmt.Dec(), "burned", burnAmt.Dec())

		return nil, nil
	}

	return nil, nil
}

func (ctrler *SupplyCtrler) Commit() ([]byte, int64, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	if len(ctrler.burnedSupply) > 0 {
		newTotalSupply := ctrler.lastTotalSupply.Clone()
		adjustedHeight := ctrler.lastAdjustedHeight
		burnAmt := uint256.NewInt(0)
		for _, burn := range ctrler.burnedSupply {
			adjustedHeight = burn.Height()
			_ = burnAmt.Add(burnAmt, burn.change)
			_ = newTotalSupply.Sub(newTotalSupply, burn.change)
		}
		newSupply := NewSupply(
			adjustedHeight,
			newTotalSupply,
			burnAmt, // change
			false,
		)
		if xerr := ctrler.supplyState.Set(v1.LedgerKeyTotalSupply(), newSupply, true); xerr != nil {
			ctrler.logger.Error("fail to set total supply", "error", xerr.Error())
			return nil, 0, xerr
		}
		if xerr := ctrler.supplyState.Set(v1.LedgerKeyAdjustedSupply(), newSupply, true); xerr != nil {
			ctrler.logger.Error("fail to set adjusted supply", "error", xerr.Error())
			return nil, 0, xerr
		}
	}

	h, v, xerr := ctrler.supplyState.Commit()
	if xerr != nil {
		return nil, 0, xerr
	}

	return h, v, nil
}
