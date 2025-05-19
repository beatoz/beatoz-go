package supply

import (
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
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

	//
	// Process txs fee: Burn and Reward
	header := bctx.BlockInfo().Header
	sumFee := bctx.SumFee() // it's value is 0 at BeginBlock.
	if header.GetProposerAddress() != nil && sumFee.Sign() > 0 {

		// burn GovParams.BurnRate % of txs fee.
		burnAmt := new(uint256.Int).Mul(sumFee, uint256.NewInt(uint64(bctx.GovHandler.BurnRate())))
		burnAmt = new(uint256.Int).Div(burnAmt, uint256.NewInt(100))

		if xerr := bctx.AcctHandler.AddBalance(bctx.GovHandler.BurnAddress(), burnAmt, true); xerr != nil {
			return nil, xerr
		}

		//
		// this is not burning.
		// it is just to transfer to the zero address.
		//// In ctrler.burn, ctrler.lastTotalSupply is changed.
		//if xerr := ctrler.burn(bctx.Height(), burnAmt); xerr != nil {
		//	return nil, xerr
		//}

		// distribute the remaining fee to the proposer of this block.
		rwdAmt := new(uint256.Int).Sub(sumFee, burnAmt)
		if xerr := bctx.AcctHandler.AddBalance(header.GetProposerAddress(), rwdAmt, true); xerr != nil {
			return nil, xerr
		}

		ctrler.logger.Debug("txs's fee is processed", "total.fee", sumFee.Dec(), "reward", rwdAmt.Dec(), "burned", burnAmt.Dec())
	}

	//
	// Wait to finish minting...
	if bctx.Height() > 0 && bctx.Height()%bctx.GovHandler.InflationCycleBlocks() == 0 {
		start := time.Now()
		// In ctrler.waitMint, ctrler.lastTotalSupply is changed.
		_, xerr := ctrler.waitMint(bctx)
		since := time.Since(start)

		ctrler.logger.Debug("wait to process mint and reward", "delay", since)
		if xerr != nil {
			ctrler.logger.Error("fail to requestMint", "error", xerr.Error())
			return nil, xerr
		}
	}

	//
	// Set supply info to ledger
	if ctrler.lastTotalSupply.IsChanged() {
		if xerr := ctrler.supplyState.Set(v1.LedgerKeyTotalSupply(), ctrler.lastTotalSupply, true); xerr != nil {
			return nil, xerr
		}
		ctrler.lastTotalSupply.ResetChanged()
	}
	return nil, nil
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
