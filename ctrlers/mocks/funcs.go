package mocks

import (
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

var lastBlockCtx *ctrlertypes.BlockContext

func InitBlockCtxWith(h int64, a ctrlertypes.IAccountHandler, g ctrlertypes.IGovParams, s ctrlertypes.IStakeHandler) *ctrlertypes.BlockContext {
	bctx := ctrlertypes.NewBlockContext(abcitypes.RequestBeginBlock{}, g, a, s)
	bctx.SetHeight(h)

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
	lastBlockCtx = InitBlockCtxWith(lastBlockCtx.Height()+1, lastBlockCtx.AcctHandler, lastBlockCtx.GovParams, lastBlockCtx.StakeHandler)
	return lastBlockCtx
}

func NextBlockCtxOf(bctx *ctrlertypes.BlockContext) *ctrlertypes.BlockContext {
	if lastBlockCtx == nil {
		panic("lastBlockCtx is nil - Run InitBlockCtxWith")
	}
	lastBlockCtx = InitBlockCtxWith(bctx.Height()+1, bctx.AcctHandler, bctx.GovParams, bctx.StakeHandler)
	return lastBlockCtx
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

func DoBeginBlock(ctrler ctrlertypes.IBlockHandler) error {
	bctx := LastBlockCtx() //mocks.NextBlockCtx()
	//fmt.Println("DoBeginBlock for", bctx.Height())
	_, err := ctrler.BeginBlock(bctx)
	return err
}
func DoEndBlock(ctrler ctrlertypes.IBlockHandler) error {
	bctx := LastBlockCtx()
	//fmt.Println("DoEndBlock for", bctx.Height())
	if _, err := ctrler.EndBlock(bctx); err != nil {
		return err
	}
	return nil
}
func DoCommitBlock(ctrler ctrlertypes.ILedgerHandler) error {
	if _, v, err := ctrler.Commit(); err != nil {
		return err
	} else {
		LastBlockCtx().SetHeight(v)
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
