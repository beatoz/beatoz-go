package mocks

import (
	"fmt"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	types2 "github.com/tendermint/tendermint/proto/tendermint/types"
	"time"
)

var lastBlockCtx *ctrlertypes.BlockContext
var currBlockCtx *ctrlertypes.BlockContext

func InitBlockCtxWith(chainId string, h int64, g ctrlertypes.IGovHandler, a ctrlertypes.IAccountHandler, e ctrlertypes.IEVMHandler, su ctrlertypes.ISupplyHandler, vp ctrlertypes.IVPowerHandler) *ctrlertypes.BlockContext {
	currBlockCtx = ctrlertypes.NewBlockContext(abcitypes.RequestBeginBlock{
		Header: types2.Header{
			ChainID: chainId,
			Height:  h,
			Time:    time.Now(),
		},
	}, g, a, e, su, vp)
	return currBlockCtx
}

func InitBlockCtx(bctx *ctrlertypes.BlockContext) {
	currBlockCtx = bctx
}

func CurrBlockCtx() *ctrlertypes.BlockContext {
	return currBlockCtx
}

func CurrBlockHeight() int64 {
	if currBlockCtx == nil {
		return 0
	}
	return currBlockCtx.Height()
}

func SetCurrBlockCtx(bctx *ctrlertypes.BlockContext) {
	currBlockCtx = bctx
}

func LastBlockCtx() *ctrlertypes.BlockContext {
	return lastBlockCtx
}

func LastBlockHeight() int64 {
	if lastBlockCtx == nil {
		return 0
	}
	return lastBlockCtx.Height()
}

func SetLastBlockCtx(bctx *ctrlertypes.BlockContext) {
	lastBlockCtx = bctx
}

func NextBlockCtxOf(bctx *ctrlertypes.BlockContext) *ctrlertypes.BlockContext {
	if bctx == nil {
		bctx = lastBlockCtx
	}
	return InitBlockCtxWith(bctx.ChainID(), bctx.Height()+1, bctx.GovHandler, bctx.AcctHandler, bctx.EVMHandler, bctx.SupplyHandler, bctx.VPowerHandler)
}

func DoBeginBlock(ctrlers ...ctrlertypes.IBlockHandler) error {
	bctx := CurrBlockCtx()
	for _, ctr := range ctrlers {
		_, err := ctr.BeginBlock(bctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func DoRunTrx(ctrler ctrlertypes.ITrxHandler, txctxs ...*ctrlertypes.TrxContext) error {
	for _, tctx := range txctxs {
		tctx.BlockContext = CurrBlockCtx()
		if xerr := ctrler.ValidateTrx(tctx); xerr != nil {
			return xerr
		}
		if xerr := ctrler.ExecuteTrx(tctx); xerr != nil {
			return xerr
		}
	}
	return nil
}

func DoEndBlock(ctrlers ...ctrlertypes.IBlockHandler) error {
	bctx := CurrBlockCtx()
	for _, ctr := range ctrlers {
		_, err := ctr.EndBlock(bctx)
		if err != nil {
			return err
		}
	}
	return nil
}
func DoCommit(ctrlers ...ctrlertypes.IBlockHandler) error {

	for _, ctr := range ctrlers {
		if _, v, err := ctr.Commit(); err != nil {
			return err
		} else if v != currBlockCtx.Height() {
			return fmt.Errorf("different height between ledger(%v) and currBlockCtx(%v)", v, currBlockCtx.Height())
		}
	}
	lastBlockCtx = currBlockCtx
	currBlockCtx = NextBlockCtxOf(lastBlockCtx)
	return nil
}

func DoEndBlockAndCommit(ctrler ...ctrlertypes.IBlockHandler) error {
	if err := DoEndBlock(ctrler...); err != nil {
		return err
	}
	if err := DoCommit(ctrler...); err != nil {
		return err
	}

	return nil
}

func DoAllProcess(ctrlers ...ctrlertypes.IBlockHandler) error {
	if err := DoBeginBlock(ctrlers...); err != nil {
		return err
	}
	if err := DoEndBlock(ctrlers...); err != nil {
		return err
	}
	if err := DoCommit(ctrlers...); err != nil {
		return err
	}
	return nil
}

func DoAllProcessTo(height int64, ctrlers ...ctrlertypes.IBlockHandler) error {
	for i := CurrBlockHeight(); i < height; i++ {
		if err := DoAllProcess(ctrlers...); err != nil {
			return err
		}
	}
	fmt.Println("DoAllProcessTo - current height:", currBlockCtx.Height())
	return nil
}
