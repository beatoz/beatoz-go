package mocks

import (
	"fmt"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

var lastBlockCtx *ctrlertypes.BlockContext
var currBlockCtx *ctrlertypes.BlockContext

func InitBlockCtxWith(h int64, g ctrlertypes.IGovHandler, a ctrlertypes.IAccountHandler, e ctrlertypes.IEVMHandler, su ctrlertypes.ISupplyHandler, vp ctrlertypes.IVPowerHandler) *ctrlertypes.BlockContext {
	bctx := ctrlertypes.NewBlockContext(abcitypes.RequestBeginBlock{}, g, a, e, su, vp)
	bctx.SetHeight(h)

	currBlockCtx = bctx
	return bctx
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
	return InitBlockCtxWith(bctx.Height()+1, bctx.GovHandler, bctx.AcctHandler, bctx.EVMHandler, bctx.SupplyHandler, bctx.VPowerHandler)
}

func DoBeginBlock(ctrler ctrlertypes.IBlockHandler) error {
	bctx := CurrBlockCtx() //mocks.NextBlockCtx()
	//fmt.Println("DoBeginBlock for", bctx.Height())
	_, err := ctrler.BeginBlock(bctx)
	return err
}
func DoEndBlock(ctrler ctrlertypes.IBlockHandler) error {
	bctx := CurrBlockCtx()
	//fmt.Println("DoEndBlock for", bctx.Height())
	if _, err := ctrler.EndBlock(bctx); err != nil {
		return err
	}
	return nil
}
func DoCommitBlock(ctrler ctrlertypes.ILedgerHandler) error {
	if _, v, err := ctrler.Commit(); err != nil {
		return err
	} else if v != currBlockCtx.Height() {
		panic(fmt.Errorf("different height between ledger(%v) and currBlockCtx(%v)", v, currBlockCtx.Height()))
	} else {
		lastBlockCtx = currBlockCtx
		currBlockCtx = NextBlockCtxOf(lastBlockCtx)
	}
	//fmt.Printf("DoCommitBlock - last: %v, curr: %v\n", lastBlockCtx.Height(), currBlockCtx.Height())
	return nil
}

func DoEndBlockCommit(ctrler interface{}) error {
	if err := DoEndBlock(ctrler.(ctrlertypes.IBlockHandler)); err != nil {
		return err
	}
	if err := DoCommitBlock(ctrler.(ctrlertypes.ILedgerHandler)); err != nil {
		return err
	}
	return nil
}
