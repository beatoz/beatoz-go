package node

import (
	"fmt"
	"github.com/beatoz/beatoz-go/ctrlers/types"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"runtime"
	"sync"
)

type ReqParam struct {
	idx      int
	req      *abcitypes.RequestDeliverTx
	Callback func(ret *RetParam)
}

type RetParam struct {
	idx          int
	reqDeliverTx *abcitypes.RequestDeliverTx
	resDeliverTx *abcitypes.ResponseDeliverTx
	txctx        *types.TrxContext
}

type TrxPreparer struct {
	*sync.WaitGroup
	chDone      chan struct{}
	chReqParams []chan *ReqParam
}

func newTrxPreparer() *TrxPreparer {
	return &TrxPreparer{
		WaitGroup:   &sync.WaitGroup{},
		chDone:      make(chan struct{}),
		chReqParams: make([]chan *ReqParam, runtime.GOMAXPROCS(0)),
	}
}

func (tp *TrxPreparer) start(app abcitypes.Application) {
	for i := 0; i < len(tp.chReqParams); i++ {
		tp.chReqParams[i] = make(chan *ReqParam, 5000)
		go trxPreparerRoutine(tp.chReqParams[i], tp.chDone, app)
	}
}

func (tp *TrxPreparer) stop() {
	close(tp.chDone)
}

func (tp *TrxPreparer) Add(param *ReqParam) {
	tp.WaitGroup.Add(1)
	n := param.idx % len(tp.chReqParams)
	tp.chReqParams[n] <- param
}

func trxPreparerRoutine(chReqParams chan *ReqParam, done chan struct{}, app abcitypes.Application) {
	fmt.Println("*************************** START trxPreparerRoutine")

STOP:
	for {
		select {
		case param := <-chReqParams:
			_txctx, _resp := app.(*BeatozApp).asyncPrepareTrxContext(param.req, param.idx)
			param.Callback(&RetParam{param.idx, param.req, _resp, _txctx})
		case <-done:
			break STOP
		}
	}

	fmt.Println("*************************** EXIT trxPreparerRoutine")
}
