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

	var evts []abcitypes.Event

	//
	// Process txs fee: Burn and Reward
	header := bctx.BlockInfo().Header
	sumFee := bctx.SumFee() // it's value is 0 at BeginBlock.
	if header.GetProposerAddress() != nil && sumFee.Sign() > 0 {

		//
		// Reward the GovParams.TxFeeRewardRate % of txs fee to the proposer of this block.
		rwdAmt := new(uint256.Int).Mul(sumFee, uint256.NewInt(uint64(bctx.GovHandler.TxFeeRewardRate())))
		rwdAmt = new(uint256.Int).Div(rwdAmt, uint256.NewInt(100))
		if xerr := bctx.AcctHandler.AddBalance(header.GetProposerAddress(), rwdAmt, true); xerr != nil {
			return nil, xerr
		}

		//
		// Auto burn(dead): transfer the remaining fee to DEAD Address
		//
		// this is not burning.
		// it is just to transfer to the zero address.
		//// In ctrler.burn, ctrler.lastTotalSupply is changed.
		//if xerr := ctrler.burn(bctx.Height(), burnAmt); xerr != nil {
		//	return nil, xerr
		//}

		deadAmt := new(uint256.Int).Sub(sumFee, rwdAmt)
		if xerr := bctx.AcctHandler.AddBalance(bctx.GovHandler.DeadAddress(), deadAmt, true); xerr != nil {
			return nil, xerr
		}
		evts = append(evts, abcitypes.Event{
			Type: "supply.txfee",
			Attributes: []abcitypes.EventAttribute{
				{Key: []byte("dead"), Value: []byte(deadAmt.Dec()), Index: false},
				{Key: []byte("reward"), Value: []byte(rwdAmt.Dec()), Index: false},
			},
		})
		ctrler.logger.Debug("txs's fee is processed", "total.fee", sumFee.Dec(), "reward", rwdAmt.Dec(), "dead", deadAmt.Dec())
	}

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
