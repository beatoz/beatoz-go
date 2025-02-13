package mocks

import (
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"time"
)

var lastBlockCtx *ctrlertypes.BlockContext

func InitBlockCtxWith(h int64, chainId string, g ctrlertypes.IGovHandler, a ctrlertypes.IAccountHandler, s ctrlertypes.IStakeHandler) *ctrlertypes.BlockContext {
	bctx := ctrlertypes.NewBlockContextAs(h, time.Now(), chainId, g, a, s)
	lastBlockCtx = bctx
	return bctx
}

func InitBlockCtx(bctx *ctrlertypes.BlockContext) {
	lastBlockCtx = bctx
}

func NextBlockCtx() *ctrlertypes.BlockContext {
	if lastBlockCtx == nil {
		panic("lastBlockCtx is nil - Run InitBlockCtxWith")
	}
	lastBlockCtx = NextBlockCtxOf(lastBlockCtx)
	return lastBlockCtx
}

func NextBlockCtxOf(bctx *ctrlertypes.BlockContext) *ctrlertypes.BlockContext {
	if lastBlockCtx == nil {
		panic("lastBlockCtx is nil - Run InitBlockCtxWith")
	}
	lastBlockCtx = InitBlockCtxWith(bctx.GetHeight()+1, bctx.ChainID, bctx.GovHandler, bctx.AcctHandler, bctx.StakeHandler)
	return lastBlockCtx
}

func LastBlockCtx() *ctrlertypes.BlockContext {
	return lastBlockCtx
}

func LastBlockHeight() int64 {
	if lastBlockCtx == nil {
		return 0
	}
	return lastBlockCtx.GetHeight()
}

func DoBeginBlock(ctrler ctrlertypes.IBlockHandler) error {
	bctx := LastBlockCtx() //mocks.NextBlockCtx()
	//fmt.Println("DoBeginBlock for", bctx.GetHeight())
	_, err := ctrler.BeginBlock(bctx)
	return err
}
func DoEndBlock(ctrler ctrlertypes.IBlockHandler) error {
	bctx := LastBlockCtx()
	//fmt.Println("DoEndBlock for", bctx.GetHeight())
	if _, err := ctrler.EndBlock(bctx); err != nil {
		return err
	}
	return nil
}
func DoCommitBlock(ctrler ctrlertypes.ILedgerHandler) error {
	if _, v, err := ctrler.Commit(); err != nil {
		return err
	} else {
		LastBlockCtx().Height = v
	}
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
