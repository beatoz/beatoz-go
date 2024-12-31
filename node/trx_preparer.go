package node

import (
	"github.com/beatoz/beatoz-go/ctrlers/types"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"runtime"
	"sync"
)

type ReqParam struct {
	idx      int
	req      *abcitypes.RequestDeliverTx
	onResult func(ret *RetParam)
	app      abcitypes.Application
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

	reqIdx    int
	retParams []*RetParam
}

func newTrxPreparer() *TrxPreparer {
	return &TrxPreparer{
		WaitGroup:   &sync.WaitGroup{},
		chDone:      make(chan struct{}),
		chReqParams: make([]chan *ReqParam, runtime.GOMAXPROCS(0)),
	}
}

func (tp *TrxPreparer) start() {
	for i := 0; i < len(tp.chReqParams); i++ {
		tp.chReqParams[i] = make(chan *ReqParam, 5000)
		go trxPreparerRoutine(tp.chReqParams[i], tp.chDone, i)
	}
}

func (tp *TrxPreparer) stop() {
	close(tp.chDone)
}

func (tp *TrxPreparer) reset() {
	tp.reqIdx = 0
	tp.retParams = nil
}

func (tp *TrxPreparer) Add(req *abcitypes.RequestDeliverTx, app abcitypes.Application) {
	tp.retParams = append(tp.retParams, nil)
	param := &ReqParam{
		idx: tp.reqIdx,
		req: req,
		app: app,
		onResult: func(ret *RetParam) {
			tp.retParams[ret.idx] = ret
			tp.WaitGroup.Done()
		},
	}
	tp.reqIdx++

	tp.WaitGroup.Add(1)
	n := param.idx % len(tp.chReqParams)
	tp.chReqParams[n] <- param
}

func (tp *TrxPreparer) resultAt(idx int) *RetParam {
	return tp.retParams[idx]
}

func (tp *TrxPreparer) resultCount() int {
	return len(tp.retParams)
}

func (tp *TrxPreparer) resultList() []*RetParam {
	return tp.retParams
}

func trxPreparerRoutine(chReqParams chan *ReqParam, done chan struct{}, no int) {
	//fmt.Println("*************************** START trxPreparerRoutine:", no)

STOP:
	for {
		select {
		case param := <-chReqParams:
			_txctx, _resp := param.app.(*BeatozApp).asyncPrepareTrxContext(param.req, param.idx)
			param.onResult(&RetParam{param.idx, param.req, _resp, _txctx})
		case <-done:
			break STOP
		}
	}

	//fmt.Println("*************************** EXIT trxPreparerRoutine:", no)
}
