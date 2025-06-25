package account

import (
	btztypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

func (ctrler *AcctCtrler) BeginBlock(*btztypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	// do nothing
	return nil, nil
}

func (ctrler *AcctCtrler) EndBlock(bctx *btztypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	var evts []abcitypes.Event

	//
	// Process txs fee: Reward and Auto Fee Draining
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
		// Fee Draining: transfer the remaining fee to DEAD Address
		// this is not burning. it is just to transfer to the zero address.
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
	return evts, nil
}

func (ctrler *AcctCtrler) Commit() ([]byte, int64, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	clear(ctrler.newbiesCheck)
	clear(ctrler.newbiesDeliver)

	h, v, xerr := ctrler.acctState.Commit()
	return h, v, xerr
}
