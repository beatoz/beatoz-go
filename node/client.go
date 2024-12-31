package node

import (
	"fmt"
	abcicli "github.com/tendermint/tendermint/abci/client"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/service"
	tmsync "github.com/tendermint/tendermint/libs/sync"
	tmproxy "github.com/tendermint/tendermint/proxy"
)

//----------------------------------------------------
// local proxy uses a mutex on an in-proc app

type beatozLocalClientCreator struct {
	mtx *tmsync.Mutex
	app abcitypes.Application
}

// NewLocalClientCreator returns a ClientCreator for the given app,
// which will be running locally.
func NewBeatozLocalClientCreator(app abcitypes.Application) tmproxy.ClientCreator {
	return &beatozLocalClientCreator{
		mtx: new(tmsync.Mutex),
		app: app,
	}
}

func (l *beatozLocalClientCreator) NewABCIClient() (abcicli.Client, error) {
	client := NewBeatozLocalClient(l.mtx, l.app)
	l.app.(*BeatozApp).SetLocalClient(client)
	return client, nil
}

var _ abcicli.Client = (*beatozLocalClient)(nil)

// NOTE: use defer to unlock mutex because Application might panic (e.g., in
// case of malicious tx or query). It only makes sense for publicly exposed
// methods like CheckTx (/broadcast_tx_* RPC endpoint) or Query (/abci_query
// RPC endpoint), but defers are used everywhere for the sake of consistency.
type beatozLocalClient struct {
	service.BaseService

	mtx *tmsync.Mutex
	abcitypes.Application
	abcicli.Callback

	// for parallel tx processing
	txPreparer *TrxPreparer
}

var _ abcicli.Client = (*beatozLocalClient)(nil)

// NewLocalClient creates a local web3, which will be directly calling the
// methods of the given app.
//
// Both Async and Sync methods ignore the given context.Context parameter.
func NewBeatozLocalClient(mtx *tmsync.Mutex, app abcitypes.Application) abcicli.Client {
	if mtx == nil {
		mtx = new(tmsync.Mutex)
	}
	cli := &beatozLocalClient{
		mtx:         mtx,
		Application: app,
		txPreparer:  newTrxPreparer(),
	}
	cli.BaseService = *service.NewBaseService(nil, "beatozLocalClient", cli)
	return cli
}

func (client *beatozLocalClient) OnStart() error {
	client.txPreparer.start()
	return client.Application.(*BeatozApp).Start()
}

func (client *beatozLocalClient) OnStop() {
	client.txPreparer.stop()
	client.Application.(*BeatozApp).Stop()
}

func (client *beatozLocalClient) SetResponseCallback(cb abcicli.Callback) {
	client.mtx.Lock()
	client.Callback = cb
	client.mtx.Unlock()
}

// TODO: change abcitypes.Application to include Error()?
func (client *beatozLocalClient) Error() error {
	return nil
}

func (client *beatozLocalClient) FlushAsync() *abcicli.ReqRes {
	// Do nothing
	return newLocalReqRes(abcitypes.ToRequestFlush(), nil)
}

func (client *beatozLocalClient) EchoAsync(msg string) *abcicli.ReqRes {
	client.mtx.Lock()
	defer client.mtx.Unlock()

	return client.callback(
		abcitypes.ToRequestEcho(msg),
		abcitypes.ToResponseEcho(msg),
	)
}

func (client *beatozLocalClient) InfoAsync(req abcitypes.RequestInfo) *abcicli.ReqRes {
	client.mtx.Lock()
	defer client.mtx.Unlock()

	res := client.Application.Info(req)
	return client.callback(
		abcitypes.ToRequestInfo(req),
		abcitypes.ToResponseInfo(res),
	)
}

func (client *beatozLocalClient) SetOptionAsync(req abcitypes.RequestSetOption) *abcicli.ReqRes {
	client.mtx.Lock()
	defer client.mtx.Unlock()

	res := client.Application.SetOption(req)
	return client.callback(
		abcitypes.ToRequestSetOption(req),
		abcitypes.ToResponseSetOption(res),
	)
}

func (client *beatozLocalClient) DeliverTxAsync(params abcitypes.RequestDeliverTx) *abcicli.ReqRes {
	client.mtx.Lock()
	defer client.mtx.Unlock()

	//
	// Parallel tx processing.
	// Just collect `RequestDeliverTx` at here.
	// The executions for these `RequestDeliverTx` will be done in `EncBlockSync`
	client.txPreparer.Add(&params, client.Application)

	// this return value has no meaning.
	return nil
}

/*
Original DeliverTxAsync
func (client *beatozLocalClient) DeliverTxAsync(params abcitypes.RequestDeliverTx) *abcicli.ReqRes {
	client.mtx.Lock()
	defer client.mtx.Unlock()

	res := client.Application.DeliverTx(params)
	return client.callback(
		abcitypes.ToRequestDeliverTx(params),
		abcitypes.ToResponseDeliverTx(res),
	)
}
*/

func (client *beatozLocalClient) CheckTxAsync(req abcitypes.RequestCheckTx) *abcicli.ReqRes {
	client.mtx.Lock()
	defer client.mtx.Unlock()

	res := client.Application.CheckTx(req)
	return client.callback(
		abcitypes.ToRequestCheckTx(req),
		abcitypes.ToResponseCheckTx(res),
	)
}

func (client *beatozLocalClient) QueryAsync(req abcitypes.RequestQuery) *abcicli.ReqRes {
	client.mtx.Lock()
	defer client.mtx.Unlock()

	res := client.Application.Query(req)
	return client.callback(
		abcitypes.ToRequestQuery(req),
		abcitypes.ToResponseQuery(res),
	)
}

func (client *beatozLocalClient) CommitAsync() *abcicli.ReqRes {
	client.mtx.Lock()
	defer client.mtx.Unlock()

	res := client.Application.Commit()
	return client.callback(
		abcitypes.ToRequestCommit(),
		abcitypes.ToResponseCommit(res),
	)
}

func (client *beatozLocalClient) InitChainAsync(req abcitypes.RequestInitChain) *abcicli.ReqRes {
	client.mtx.Lock()
	defer client.mtx.Unlock()

	res := client.Application.InitChain(req)
	return client.callback(
		abcitypes.ToRequestInitChain(req),
		abcitypes.ToResponseInitChain(res),
	)
}

func (client *beatozLocalClient) BeginBlockAsync(req abcitypes.RequestBeginBlock) *abcicli.ReqRes {
	client.mtx.Lock()
	defer client.mtx.Unlock()

	res := client.Application.BeginBlock(req)
	return client.callback(
		abcitypes.ToRequestBeginBlock(req),
		abcitypes.ToResponseBeginBlock(res),
	)
}

func (client *beatozLocalClient) EndBlockAsync(req abcitypes.RequestEndBlock) *abcicli.ReqRes {
	client.mtx.Lock()
	defer client.mtx.Unlock()

	res := client.Application.EndBlock(req)
	return client.callback(
		abcitypes.ToRequestEndBlock(req),
		abcitypes.ToResponseEndBlock(res),
	)
}

func (client *beatozLocalClient) ListSnapshotsAsync(req abcitypes.RequestListSnapshots) *abcicli.ReqRes {
	client.mtx.Lock()
	defer client.mtx.Unlock()

	res := client.Application.ListSnapshots(req)
	return client.callback(
		abcitypes.ToRequestListSnapshots(req),
		abcitypes.ToResponseListSnapshots(res),
	)
}

func (client *beatozLocalClient) OfferSnapshotAsync(req abcitypes.RequestOfferSnapshot) *abcicli.ReqRes {
	client.mtx.Lock()
	defer client.mtx.Unlock()

	res := client.Application.OfferSnapshot(req)
	return client.callback(
		abcitypes.ToRequestOfferSnapshot(req),
		abcitypes.ToResponseOfferSnapshot(res),
	)
}

func (client *beatozLocalClient) LoadSnapshotChunkAsync(req abcitypes.RequestLoadSnapshotChunk) *abcicli.ReqRes {
	client.mtx.Lock()
	defer client.mtx.Unlock()

	res := client.Application.LoadSnapshotChunk(req)
	return client.callback(
		abcitypes.ToRequestLoadSnapshotChunk(req),
		abcitypes.ToResponseLoadSnapshotChunk(res),
	)
}

func (client *beatozLocalClient) ApplySnapshotChunkAsync(req abcitypes.RequestApplySnapshotChunk) *abcicli.ReqRes {
	client.mtx.Lock()
	defer client.mtx.Unlock()

	res := client.Application.ApplySnapshotChunk(req)
	return client.callback(
		abcitypes.ToRequestApplySnapshotChunk(req),
		abcitypes.ToResponseApplySnapshotChunk(res),
	)
}

//-------------------------------------------------------

func (client *beatozLocalClient) FlushSync() error {
	return nil
}

func (client *beatozLocalClient) EchoSync(msg string) (*abcitypes.ResponseEcho, error) {
	return &abcitypes.ResponseEcho{Message: msg}, nil
}

func (client *beatozLocalClient) InfoSync(req abcitypes.RequestInfo) (*abcitypes.ResponseInfo, error) {
	client.mtx.Lock()
	defer client.mtx.Unlock()

	res := client.Application.Info(req)
	return &res, nil
}

func (client *beatozLocalClient) SetOptionSync(req abcitypes.RequestSetOption) (*abcitypes.ResponseSetOption, error) {
	client.mtx.Lock()
	defer client.mtx.Unlock()

	res := client.Application.SetOption(req)
	return &res, nil
}

func (client *beatozLocalClient) DeliverTxSync(req abcitypes.RequestDeliverTx) (*abcitypes.ResponseDeliverTx, error) {
	client.mtx.Lock()
	defer client.mtx.Unlock()

	res := client.Application.DeliverTx(req)
	return &res, nil
}

func (client *beatozLocalClient) CheckTxSync(req abcitypes.RequestCheckTx) (*abcitypes.ResponseCheckTx, error) {
	client.mtx.Lock()
	defer client.mtx.Unlock()

	res := client.Application.CheckTx(req)
	return &res, nil
}

func (client *beatozLocalClient) QuerySync(req abcitypes.RequestQuery) (*abcitypes.ResponseQuery, error) {
	client.mtx.Lock()
	defer client.mtx.Unlock()

	res := client.Application.Query(req)
	return &res, nil
}

func (client *beatozLocalClient) CommitSync() (*abcitypes.ResponseCommit, error) {
	client.mtx.Lock()
	defer client.mtx.Unlock()

	res := client.Application.Commit()
	return &res, nil
}

func (client *beatozLocalClient) InitChainSync(req abcitypes.RequestInitChain) (*abcitypes.ResponseInitChain, error) {
	client.mtx.Lock()
	defer client.mtx.Unlock()

	res := client.Application.InitChain(req)
	return &res, nil
}

func (client *beatozLocalClient) BeginBlockSync(req abcitypes.RequestBeginBlock) (*abcitypes.ResponseBeginBlock, error) {
	client.mtx.Lock()
	defer client.mtx.Unlock()

	res := client.Application.BeginBlock(req)
	return &res, nil
}

func (client *beatozLocalClient) EndBlockSync(req abcitypes.RequestEndBlock) (*abcitypes.ResponseEndBlock, error) {
	client.mtx.Lock()
	defer client.mtx.Unlock()

	client.txPreparer.Wait()

	// for debugging
	if client.txPreparer.resultCount() != client.Application.(*BeatozApp).nextBlockCtx.TxsCnt() {
		panic(fmt.Sprintf("error: len(client.deliverTxReqs) != txs count in block"))
	}

	// Execute every transaction in its own `TrxContext` sequentially
	for idx, param := range client.txPreparer.resultList() {
		if idx != param.idx || idx != param.txctx.TxIdx {
			panic(fmt.Sprintf("error: wrong transaction index. idx:%v, param.idx:%v, txctx.TxIdx:%v", idx, param.idx, param.txctx.TxIdx))
		}
		// `txctx` may be `nil`, which means ans error occurred in generating `TrxContext`.
		// The `ResponseDeliverTx` for this tx already exists in `deliverTxResps` and
		// it is written to blockchain as invalid tx.
		if param.txctx != nil {
			param.resDeliverTx = client.Application.(*BeatozApp).asyncExecTrxContext(param.txctx)
		}
		client.callback(
			abcitypes.ToRequestDeliverTx(*param.reqDeliverTx),
			abcitypes.ToResponseDeliverTx(*param.resDeliverTx),
		)
	}
	client.txPreparer.reset()

	res := client.Application.EndBlock(req)
	return &res, nil
}

/*
Original EndBlockSync

	func (client *beatozLocalClient) EndBlockSync(req types.RequestEndBlock) (*types.ResponseEndBlock, error) {
		client.mtx.Lock()
		defer client.mtx.Unlock()

		res := client.Application.EndBlock(req)
		return &res, nil
	}
*/
func (client *beatozLocalClient) ListSnapshotsSync(req abcitypes.RequestListSnapshots) (*abcitypes.ResponseListSnapshots, error) {
	client.mtx.Lock()
	defer client.mtx.Unlock()

	res := client.Application.ListSnapshots(req)
	return &res, nil
}

func (client *beatozLocalClient) OfferSnapshotSync(req abcitypes.RequestOfferSnapshot) (*abcitypes.ResponseOfferSnapshot, error) {
	client.mtx.Lock()
	defer client.mtx.Unlock()

	res := client.Application.OfferSnapshot(req)
	return &res, nil
}

func (client *beatozLocalClient) LoadSnapshotChunkSync(
	req abcitypes.RequestLoadSnapshotChunk) (*abcitypes.ResponseLoadSnapshotChunk, error) {
	client.mtx.Lock()
	defer client.mtx.Unlock()

	res := client.Application.LoadSnapshotChunk(req)
	return &res, nil
}

func (client *beatozLocalClient) ApplySnapshotChunkSync(
	req abcitypes.RequestApplySnapshotChunk) (*abcitypes.ResponseApplySnapshotChunk, error) {
	client.mtx.Lock()
	defer client.mtx.Unlock()

	res := client.Application.ApplySnapshotChunk(req)
	return &res, nil
}

//-------------------------------------------------------

func (client *beatozLocalClient) callback(req *abcitypes.Request, res *abcitypes.Response) *abcicli.ReqRes {
	client.Callback(req, res)
	rr := newLocalReqRes(req, res)
	rr.InvokeCallback() //rr.callbackInvoked = true
	return rr
}

func newLocalReqRes(req *abcitypes.Request, res *abcitypes.Response) *abcicli.ReqRes {
	reqRes := abcicli.NewReqRes(req)
	reqRes.Response = res
	return reqRes
}
