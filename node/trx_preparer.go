package node

import (
	"github.com/beatoz/beatoz-go/ctrlers/types"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"runtime"
	"sync"
	"sync/atomic"
)

type requestParam struct {
	idx         int
	req         *abcitypes.RequestDeliverTx
	onPrepare   func(*abcitypes.RequestDeliverTx, int) (*types.TrxContext, *abcitypes.ResponseDeliverTx)
	onCompleted func(*resultValue)
}

type resultValue struct {
	idx          int
	reqDeliverTx *abcitypes.RequestDeliverTx
	resDeliverTx *abcitypes.ResponseDeliverTx
	txctx        *types.TrxContext
}

type TrxPreparer struct {
	*sync.WaitGroup

	chDone      chan struct{}
	chReqParams []chan *requestParam

	reqCount     int
	resultValues []*resultValue

	started uint32 // atomic
	stopped uint32 // atomic
	mtx     sync.RWMutex
}

func newTrxPreparer() *TrxPreparer {
	return &TrxPreparer{
		WaitGroup:   &sync.WaitGroup{},
		chDone:      make(chan struct{}),
		chReqParams: make([]chan *requestParam, runtime.GOMAXPROCS(0)),
	}
}

func (tp *TrxPreparer) start() {
	if atomic.CompareAndSwapUint32(&tp.started, 0, 1) {
		for i := 0; i < len(tp.chReqParams); i++ {
			tp.chReqParams[i] = make(chan *requestParam, 5000)
			go trxPreparerRoutine(tp.chReqParams[i], tp.chDone, i)
		}
	}
}

func (tp *TrxPreparer) stop() {
	if atomic.CompareAndSwapUint32(&tp.stopped, 0, 1) {
		close(tp.chDone)
	}
}

func (tp *TrxPreparer) reset() {
	tp.mtx.Lock()
	defer tp.mtx.Unlock()

	tp.reqCount = 0
	tp.resultValues = nil
}

func (tp *TrxPreparer) Add(req *abcitypes.RequestDeliverTx, prepareCallback func(*abcitypes.RequestDeliverTx, int) (*types.TrxContext, *abcitypes.ResponseDeliverTx)) {
	param := &requestParam{
		idx:       tp.reqCount,
		req:       req,
		onPrepare: prepareCallback,
		onCompleted: func(ret *resultValue) {
			tp.mtx.Lock()
			tp.resultValues[ret.idx] = ret
			tp.mtx.Unlock()

			tp.WaitGroup.Done()
		},
	}

	tp.mtx.Lock()
	// add an empty element
	tp.resultValues = append(tp.resultValues, (*resultValue)(nil))
	tp.reqCount++
	tp.mtx.Unlock()

	tp.WaitGroup.Add(1)
	n := param.idx % len(tp.chReqParams)
	tp.chReqParams[n] <- param
}

func (tp *TrxPreparer) resultAt(idx int) *resultValue {
	tp.mtx.RLock()
	defer tp.mtx.RUnlock()

	return tp.resultValues[idx]
}

func (tp *TrxPreparer) resultCount() int {
	tp.mtx.RLock()
	defer tp.mtx.RUnlock()

	return len(tp.resultValues)
}

func (tp *TrxPreparer) resultList() []*resultValue {
	tp.mtx.RLock()
	defer tp.mtx.RUnlock()

	return tp.resultValues
}

func trxPreparerRoutine(chReqParams chan *requestParam, done chan struct{}, no int) {
STOP:
	for {
		select {
		case param := <-chReqParams:
			_txctx, _resp := param.onPrepare(param.req, param.idx) //.paramapp.(*BeatozApp).asyncPrepareTrxContext(param.req, param.idx)
			param.onCompleted(&resultValue{param.idx, param.req, _resp, _txctx})
		case <-done:
			break STOP
		}
	}
}
