package node

import (
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"

	"github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
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
	logger  log.Logger
}

func newTrxPreparer(logger log.Logger) *TrxPreparer {
	return &TrxPreparer{
		WaitGroup:   &sync.WaitGroup{},
		chDone:      make(chan struct{}),
		chReqParams: make([]chan *requestParam, runtime.GOMAXPROCS(0)),
		logger:      logger,
	}
}

func (tp *TrxPreparer) start() {
	if atomic.CompareAndSwapUint32(&tp.started, 0, 1) {
		for i := 0; i < len(tp.chReqParams); i++ {
			tp.chReqParams[i] = make(chan *requestParam, 5000)
			go tp.trxPreparerRoutine(tp.chReqParams[i], tp.chDone, i)
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

func (tp *TrxPreparer) prepareSafe(param *requestParam, workerNo int) (ret *resultValue) {
	ret = &resultValue{
		idx:          param.idx,
		reqDeliverTx: param.req,
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			xerr := xerrors.ErrDeliverTx.Wrapf("transaction preparation failed")
			ret.txctx = nil
			ret.resDeliverTx = &abcitypes.ResponseDeliverTx{
				Code: xerr.Code(),
				Log:  xerr.Error(),
			}

			if tp.logger != nil {
				tp.logger.Error(
					"panic while preparing transaction",
					"worker", workerNo,
					"txIndex", param.idx,
					"panic", recovered,
					"stack", string(debug.Stack()),
				)
			}
		}
	}()

	ret.txctx, ret.resDeliverTx = param.onPrepare(param.req, param.idx)
	return ret
}

func (tp *TrxPreparer) trxPreparerRoutine(chReqParams chan *requestParam, done chan struct{}, no int) {
STOP:
	for {
		select {
		case param := <-chReqParams:
			ret := tp.prepareSafe(param, no)
			param.onCompleted(ret)
		case <-done:
			break STOP
		}
	}
}
