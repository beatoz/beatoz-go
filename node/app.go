package node

import (
	"fmt"
	cfg "github.com/beatoz/beatoz-go/cmd/config"
	"github.com/beatoz/beatoz-go/cmd/version"
	"github.com/beatoz/beatoz-go/ctrlers/account"
	"github.com/beatoz/beatoz-go/ctrlers/gov"
	"github.com/beatoz/beatoz-go/ctrlers/supply"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/ctrlers/vm/evm"
	"github.com/beatoz/beatoz-go/ctrlers/vpower"
	"github.com/beatoz/beatoz-go/genesis"
	"github.com/beatoz/beatoz-go/libs/jsonx"
	types2 "github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/crypto"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	abcicli "github.com/tendermint/tendermint/abci/client"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tmtypes "github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
	tmver "github.com/tendermint/tendermint/version"
	"strconv"
	"sync"
	"time"
)

var _ abcitypes.Application = (*BeatozApp)(nil)

type BeatozApp struct {
	abcitypes.BaseApplication

	lastBlockCtx *ctrlertypes.BlockContext
	currBlockCtx *ctrlertypes.BlockContext

	metaDB       *MetaDB
	acctCtrler   *account.AcctCtrler
	govCtrler    *gov.GovCtrler
	vpowCtrler   *vpower.VPowerCtrler
	supplyCtrler *supply.SupplyCtrler
	vmCtrler     *evm.EVMCtrler
	txExecutor   *TrxExecutor

	localClient abcicli.Client
	rootConfig  *cfg.Config

	started int32
	logger  log.Logger
	mtx     sync.Mutex
}

func NewBeatozApp(config *cfg.Config, logger log.Logger) *BeatozApp {
	metaDB, err := OpenMetaDB("beatoz_app", config.DBDir())
	if err != nil {
		panic(err)
	}

	govCtrler, err := gov.NewGovCtrler(config, logger)
	if err != nil {
		panic(err)
	}

	acctCtrler, err := account.NewAcctCtrler(config, logger)
	if err != nil {
		panic(err)
	}

	vpowCtrler, err := vpower.NewVPowerCtrler(config, int(govCtrler.MaxValidatorCnt()), logger)
	if err != nil {
		panic(err)
	}

	supplyCtrler, err := supply.NewSupplyCtrler(config, logger)
	if err != nil {
		panic(err)
	}

	vmCtrler := evm.NewEVMCtrler(config.DBDir(), acctCtrler, logger)

	txExecutor := NewTrxExecutor(logger)

	return &BeatozApp{
		metaDB:       metaDB,
		acctCtrler:   acctCtrler,
		govCtrler:    govCtrler,
		vpowCtrler:   vpowCtrler,
		supplyCtrler: supplyCtrler,
		vmCtrler:     vmCtrler,
		txExecutor:   txExecutor,
		rootConfig:   config,
		logger:       logger,
	}
}

func (ctrler *BeatozApp) Start() error {
	ctrler.txExecutor.start()
	return nil
}

func (ctrler *BeatozApp) Stop() error {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	ctrler.txExecutor.stop()

	if err := ctrler.acctCtrler.Close(); err != nil {
		return err
	}
	if err := ctrler.govCtrler.Close(); err != nil {
		return err
	}
	if err := ctrler.vmCtrler.Close(); err != nil {
		return err
	}
	if err := ctrler.metaDB.Close(); err != nil {
		return err
	}
	return nil
}

func (ctrler *BeatozApp) SetLocalClient(client abcicli.Client) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	ctrler.localClient = client
}
func (ctrler *BeatozApp) Info(info abcitypes.RequestInfo) abcitypes.ResponseInfo {
	ctrler.logger.Info("Info", "version", tmver.ABCIVersion, "AppVersion", version.String())

	var appHash bytes.HexBytes
	var lastHeight int64
	ctrler.lastBlockCtx = ctrler.metaDB.LastBlockContext()
	if ctrler.lastBlockCtx == nil {
		ctrler.lastBlockCtx = ctrlertypes.NewBlockContext(
			abcitypes.RequestBeginBlock{
				Header: tmproto.Header{
					Height: lastHeight,
					Time:   tmtime.Canonical(time.Now()),
				},
			},
			ctrler.govCtrler, ctrler.acctCtrler, ctrler.vmCtrler, ctrler.supplyCtrler, ctrler.vpowCtrler,
		)
		ctrler.lastBlockCtx.SetAppHash(appHash)
	} else {
		ctrler.lastBlockCtx.GovHandler = ctrler.govCtrler
		ctrler.lastBlockCtx.AcctHandler = ctrler.acctCtrler
		ctrler.lastBlockCtx.EVMHandler = ctrler.vmCtrler
		ctrler.lastBlockCtx.SupplyHandler = ctrler.supplyCtrler
		ctrler.lastBlockCtx.VPowerHandler = ctrler.vpowCtrler
		lastHeight = ctrler.lastBlockCtx.Height()
		appHash = ctrler.lastBlockCtx.AppHash()
	}

	// get chain_id
	ctrler.rootConfig.ChainID = ctrler.lastBlockCtx.ChainID()

	ctrler.logger.Info("last block information",
		"chainID", ctrler.lastBlockCtx.ChainID(),
		"height", ctrler.lastBlockCtx.Height(),
		"appHash", ctrler.lastBlockCtx.AppHash(),
		"blockSizeLimit", ctrler.lastBlockCtx.GetBlockSizeLimit(),
		"blockGasLimit", ctrler.lastBlockCtx.GetBlockGasLimit())

	return abcitypes.ResponseInfo{
		Data:             "",
		Version:          tmver.ABCIVersion,
		AppVersion:       version.Major(),
		LastBlockHeight:  lastHeight,
		LastBlockAppHash: appHash,
	}
}

// InitChain is called only when the ResponseInfo::LastBlockHeight which is returned in Info() is 0.
func (ctrler *BeatozApp) InitChain(req abcitypes.RequestInitChain) abcitypes.ResponseInitChain {
	// set and put chain_id
	if req.GetChainId() == "" {
		panic("there is no chain_id")
	}

	appState, initTotalSupply, xerr := checkRequestInitChain(req)
	if xerr != nil {
		ctrler.logger.Error("wrong request", "error", xerr)
		panic(xerr)
	}

	if xerr := ctrler.govCtrler.InitLedger(appState); xerr != nil {
		ctrler.logger.Error("fail to initialize governance controller", "error", xerr)
		panic(xerr)
	}
	if xerr := ctrler.acctCtrler.InitLedger(appState); xerr != nil {
		ctrler.logger.Error("fail to initialize account controller", "error", xerr)
		panic(xerr)
	}

	if xerr := ctrler.supplyCtrler.InitLedger(initTotalSupply); xerr != nil {
		ctrler.logger.Error("fail to initialize supply controller", "error", xerr)
	}

	if xerr := ctrler.vpowCtrler.InitLedger(req.Validators); xerr != nil {
		ctrler.logger.Error("fail to initialize voting power controller", "error", xerr)
		panic(xerr)
	}

	appHash, err := appState.Hash()
	if err != nil {
		panic(err)
	}

	// set initial block info
	ctrler.lastBlockCtx.SetChainID(req.GetChainId())
	ctrler.lastBlockCtx.SetBlockSizeLimit(req.ConsensusParams.Block.MaxBytes)
	ctrler.lastBlockCtx.SetBlockGasLimit(req.ConsensusParams.Block.MaxGas)
	ctrler.lastBlockCtx.SetAppHash(appHash)
	ctrler.rootConfig.ChainID = req.GetChainId()

	ctrler.logger.Info("InitChain",
		"chainID", ctrler.lastBlockCtx.ChainID(),
		"height", ctrler.lastBlockCtx.Height(),
		"appHash", ctrler.lastBlockCtx.AppHash(),
		"blockSizeLimit", ctrler.lastBlockCtx.GetBlockSizeLimit(),
		"blockGasLimit", ctrler.lastBlockCtx.GetBlockGasLimit())

	// these values will be saved as state of the consensus engine.
	return abcitypes.ResponseInitChain{
		AppHash: appHash,
	}
}

func (ctrler *BeatozApp) CheckTx(req abcitypes.RequestCheckTx) abcitypes.ResponseCheckTx {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	switch req.Type {
	case abcitypes.CheckTxType_New:
		_bctx := ctrlertypes.ExpectNextBlockContext(ctrler.lastBlockCtx, time.Duration(ctrler.govCtrler.AssumedBlockInterval())*time.Second)
		txctx, xerr := ctrlertypes.NewTrxContext(
			req.Tx,
			_bctx,
			false,
		)
		if xerr != nil {
			xerr = xerrors.ErrCheckTx.Wrap(xerr)
			ctrler.logger.Error("CheckTx", "error", xerr)
			return abcitypes.ResponseCheckTx{
				Code: xerr.Code(),
				Log:  xerr.Error(),
			}
		}

		xerr = ctrler.txExecutor.ExecuteSync(txctx)
		if xerr != nil {
			xerr = xerrors.ErrCheckTx.Wrap(xerr)
			ctrler.logger.Error("CheckTx", "error", xerr)
			return abcitypes.ResponseCheckTx{
				Code:      xerr.Code(),
				Log:       xerr.Error(),
				Data:      txctx.RetData, // in case of evm, there may be return data when tx is failed.
				GasWanted: txctx.Tx.Gas,
			}
		}

		return abcitypes.ResponseCheckTx{
			Code:      abcitypes.CodeTypeOK,
			GasWanted: txctx.Tx.Gas,
			GasUsed:   txctx.GasUsed,
			Log:       "",
			Data:      txctx.RetData,
		}
	case abcitypes.CheckTxType_Recheck:
		// do Tx validation minimally
		// validate amount and nonce of sender, which may have been changed.
		tx := &ctrlertypes.Trx{}
		if xerr := tx.Decode(req.Tx); xerr != nil {
			xerr = xerrors.ErrCheckTx.Wrap(xerr)
			ctrler.logger.Error("ReCheckTx", "error", xerr)
			return abcitypes.ResponseCheckTx{
				Code: xerr.Code(),
				Log:  xerr.Error(),
			}
		}

		sender := ctrler.acctCtrler.FindAccount(tx.From, false)
		if sender == nil {
			xerr := xerrors.ErrCheckTx.Wrap(xerrors.ErrNotFoundAccount.Wrapf("sender address: %v", tx.From))
			ctrler.logger.Error("ReCheckTx", "error", xerr)
			return abcitypes.ResponseCheckTx{
				Code:      xerr.Code(),
				Log:       xerr.Error(),
				GasWanted: tx.Gas,
			}
		}

		// check balance
		feeAmt := new(uint256.Int).Mul(tx.GasPrice, uint256.NewInt(uint64(tx.Gas)))
		needAmt := new(uint256.Int).Add(feeAmt, tx.Amount)
		if xerr := sender.CheckBalance(needAmt); xerr != nil {
			xerr = xerrors.ErrCheckTx.Wrap(xerr)
			ctrler.logger.Error("ReCheckTx", "error", xerr)
			return abcitypes.ResponseCheckTx{
				Code:      xerr.Code(),
				Log:       xerr.Error(),
				GasWanted: tx.Gas,
			}
		}

		// check nonce
		if xerr := sender.CheckNonce(tx.Nonce); xerr != nil {
			xerr = xerr.Wrap(fmt.Errorf("ledger: %v, tx:%v, address: %v, txhash: %X", sender.GetNonce(), tx.Nonce, sender.Address, tmtypes.Tx(req.Tx).Hash()))
			ctrler.logger.Error("ReCheckTx", "error", xerr)
			return abcitypes.ResponseCheckTx{
				Code:      xerr.Code(),
				Log:       xerr.Error(),
				GasWanted: tx.Gas,
			}
		}

		// update sender account
		if xerr := sender.SubBalance(feeAmt); xerr != nil {
			xerr = xerrors.ErrCheckTx.Wrap(xerr)
			ctrler.logger.Error("ReCheckTx", "error", xerr)
			return abcitypes.ResponseCheckTx{
				Code:      xerr.Code(),
				Log:       xerr.Error(),
				GasWanted: tx.Gas,
			}
		}
		sender.AddNonce()

		if xerr := ctrler.acctCtrler.SetAccount(sender, false); xerr != nil {
			xerr = xerrors.ErrCheckTx.Wrap(xerr)
			ctrler.logger.Error("ReCheckTx", "error", xerr)
			return abcitypes.ResponseCheckTx{
				Code:      xerr.Code(),
				Log:       xerr.Error(),
				GasWanted: tx.Gas,
			}
		}
		return abcitypes.ResponseCheckTx{
			Code:      abcitypes.CodeTypeOK,
			GasWanted: tx.Gas,
			GasUsed:   tx.Gas,
		}
	}
	return abcitypes.ResponseCheckTx{Code: abcitypes.CodeTypeOK}
}

func (ctrler *BeatozApp) BeginBlock(req abcitypes.RequestBeginBlock) abcitypes.ResponseBeginBlock {
	if req.Header.Height != ctrler.lastBlockCtx.Height()+1 {
		panic(fmt.Errorf("error block height: expected(%v), actual(%v)", ctrler.lastBlockCtx.Height()+1, req.Header.Height))
	}
	ctrler.logger.Debug("BeatozApp::BeginBlock",
		"height", req.Header.Height,
		"hash", req.Hash,
		"prev.hash", req.Header.LastBlockId.Hash)

	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	ctrler.currBlockCtx = ctrlertypes.NewBlockContext(
		req,
		ctrler.govCtrler,
		ctrler.acctCtrler,
		ctrler.vmCtrler,
		ctrler.supplyCtrler,
		ctrler.vpowCtrler,
	)
	ctrler.currBlockCtx.SetBlockSizeLimit(ctrler.lastBlockCtx.GetBlockSizeLimit())
	ctrler.currBlockCtx.SetBlockGasLimit(ctrler.lastBlockCtx.GetBlockGasLimit())

	var beginBlockEvents []abcitypes.Event

	evs, xerr := ctrler.govCtrler.BeginBlock(ctrler.currBlockCtx)
	if xerr != nil {
		ctrler.logger.Error("failed to execute BeginBlock of govCtrler", "error", xerr)
		panic(xerr)
	}
	beginBlockEvents = append(beginBlockEvents, evs...)

	evs, xerr = ctrler.acctCtrler.BeginBlock(ctrler.currBlockCtx)
	if xerr != nil {
		ctrler.logger.Error("failed to execute BeginBlock of acctCtrler", "error", xerr)
		panic(xerr)
	}
	beginBlockEvents = append(beginBlockEvents, evs...)

	evs, xerr = ctrler.supplyCtrler.BeginBlock(ctrler.currBlockCtx)
	if xerr != nil {
		ctrler.logger.Error("failed to execute BeginBlock of supplyCtrler", "error", xerr)
		panic(xerr)
	}
	beginBlockEvents = append(beginBlockEvents, evs...)

	evs, xerr = ctrler.vpowCtrler.BeginBlock(ctrler.currBlockCtx)
	if xerr != nil {
		ctrler.logger.Error("failed to execute BeginBlock of vpowCtrler", "error", xerr)
		panic(xerr)
	}
	beginBlockEvents = append(beginBlockEvents, evs...)

	evs, xerr = ctrler.vmCtrler.BeginBlock(ctrler.currBlockCtx)
	if xerr != nil {
		ctrler.logger.Error("failed to execute BeginBlock of vmCtrler", "error", xerr)
		panic(xerr)
	}
	beginBlockEvents = append(beginBlockEvents, evs...)

	return abcitypes.ResponseBeginBlock{
		Events: beginBlockEvents,
	}
}

// DEPRECATED
func (ctrler *BeatozApp) deliverTxSync(req abcitypes.RequestDeliverTx) abcitypes.ResponseDeliverTx {

	txctx, xerr := ctrlertypes.NewTrxContext(req.Tx,
		ctrler.currBlockCtx,
		true)
	if xerr != nil {
		xerr = xerrors.ErrDeliverTx.Wrap(xerr)
		ctrler.logger.Error("deliverTxSync", "error", xerr)

		var events []abcitypes.Event
		if txctx != nil && txctx.Tx != nil {
			// add event
			events = append(events, abcitypes.Event{
				Type: "tx",
				Attributes: []abcitypes.EventAttribute{
					{Key: []byte(ctrlertypes.EVENT_ATTR_TXTYPE), Value: []byte(txctx.Tx.TypeString()), Index: true},
					{Key: []byte(ctrlertypes.EVENT_ATTR_TXSENDER), Value: []byte(txctx.Tx.From.String()), Index: true},
					{Key: []byte(ctrlertypes.EVENT_ATTR_TXSTATUS), Value: []byte(strconv.Itoa(int(xerr.Code()))), Index: false},
				},
			})
		}

		return abcitypes.ResponseDeliverTx{
			Code:   xerr.Code(),
			Log:    xerr.Error(),
			Events: events,
		}

	}
	ctrler.currBlockCtx.AddTxsCnt(1)

	xerr = ctrler.txExecutor.ExecuteSync(txctx)
	if xerr != nil {
		xerr = xerrors.ErrDeliverTx.Wrap(xerr)
		ctrler.logger.Error("deliverTxSync", "error", xerr)

		// add event
		txctx.Events = append(txctx.Events, abcitypes.Event{
			Type: "tx",
			Attributes: []abcitypes.EventAttribute{
				{Key: []byte(ctrlertypes.EVENT_ATTR_TXTYPE), Value: []byte(txctx.Tx.TypeString()), Index: true},
				{Key: []byte(ctrlertypes.EVENT_ATTR_TXSENDER), Value: []byte(txctx.Tx.From.String()), Index: true},
				{Key: []byte(ctrlertypes.EVENT_ATTR_TXSTATUS), Value: []byte(strconv.Itoa(int(xerr.Code()))), Index: false},
			},
		})

		return abcitypes.ResponseDeliverTx{
			Code:   xerr.Code(),
			Log:    xerr.Error(),
			Data:   txctx.RetData, // in case of evm, there may be return data when tx is failed.
			Events: txctx.Events,
		}
	} else {

		ctrler.currBlockCtx.AddFee(types2.GasToFee(txctx.GasUsed, ctrler.govCtrler.GasPrice()))

		// add event
		txctx.Events = append(txctx.Events, abcitypes.Event{
			Type: "tx",
			Attributes: []abcitypes.EventAttribute{
				{Key: []byte(ctrlertypes.EVENT_ATTR_TXTYPE), Value: []byte(txctx.Tx.TypeString()), Index: true},
				{Key: []byte(ctrlertypes.EVENT_ATTR_TXSENDER), Value: []byte(txctx.Tx.From.String()), Index: true},
				{Key: []byte(ctrlertypes.EVENT_ATTR_TXRECVER), Value: []byte(txctx.Tx.To.String()), Index: true},
				{Key: []byte(ctrlertypes.EVENT_ATTR_ADDRPAIR), Value: []byte(txctx.Tx.From.String() + txctx.Tx.To.String()), Index: true},
				{Key: []byte(ctrlertypes.EVENT_ATTR_AMOUNT), Value: []byte(txctx.Tx.Amount.Dec()), Index: false},
				{Key: []byte(ctrlertypes.EVENT_ATTR_TXSTATUS), Value: []byte("0"), Index: false},
			},
		})

		return abcitypes.ResponseDeliverTx{
			Code:      abcitypes.CodeTypeOK,
			GasWanted: txctx.Tx.Gas,
			GasUsed:   txctx.GasUsed,
			Data:      txctx.RetData,
			Events:    txctx.Events,
		}
	}
}

// DEPRECATED
func (ctrler *BeatozApp) DeliverTxSync(req abcitypes.RequestDeliverTx) abcitypes.ResponseDeliverTx {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	return ctrler.deliverTxSync(req)
}

func (ctrler *BeatozApp) DeliverTx(req abcitypes.RequestDeliverTx) abcitypes.ResponseDeliverTx {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	//
	// Parallel tx processing.
	// Just request to create `TrxContext` with `req RequestDeliverTx`.
	// The executions for this `req RequestDeliverTx` will be done in `EncBlockSync`
	ctrler.txExecutor.Add(
		&req,
		func(req *abcitypes.RequestDeliverTx, idx int) (*ctrlertypes.TrxContext, *abcitypes.ResponseDeliverTx) {
			txctx, xerr := ctrlertypes.NewTrxContext(req.Tx,
				ctrler.currBlockCtx,
				true)
			if xerr != nil {
				xerr = xerrors.ErrDeliverTx.Wrap(xerr)
				ctrler.logger.Error("asyncPrepareTrxContext", "error", xerr)

				return nil, &abcitypes.ResponseDeliverTx{
					Code: xerr.Code(),
					Log:  xerr.Error(),
				}
			}

			ctrler.currBlockCtx.AddTxsCnt(1)
			return txctx, nil
		},
	)

	// this return value has no meaning.
	return abcitypes.ResponseDeliverTx{}
}

// asyncExecTrxContext is called in parallel tx processing
func (ctrler *BeatozApp) asyncExecTrxContext(txctx *ctrlertypes.TrxContext) *abcitypes.ResponseDeliverTx {
	xerr := ctrler.txExecutor.ExecuteSync(txctx)
	if xerr != nil {
		xerr = xerrors.ErrDeliverTx.Wrap(xerr)
		ctrler.logger.Error("asyncExecTrxContext", "error", xerr)

		// add event
		txctx.Events = append(txctx.Events, abcitypes.Event{
			Type: "tx",
			Attributes: []abcitypes.EventAttribute{
				{Key: []byte(ctrlertypes.EVENT_ATTR_TXTYPE), Value: []byte(txctx.Tx.TypeString()), Index: true},
				{Key: []byte(ctrlertypes.EVENT_ATTR_TXSENDER), Value: []byte(txctx.Tx.From.String()), Index: true},
				{Key: []byte(ctrlertypes.EVENT_ATTR_TXSTATUS), Value: []byte(strconv.Itoa(int(xerr.Code()))), Index: false},
			},
		})

		return &abcitypes.ResponseDeliverTx{
			Code:      xerr.Code(),
			Log:       xerr.Error(),
			GasWanted: txctx.Tx.Gas,
			GasUsed:   txctx.GasUsed,
			Data:      txctx.RetData, // in case of evm, there may be return data when tx is failed.
			Events:    txctx.Events,
		}
	} else {

		ctrler.currBlockCtx.AddFee(types2.GasToFee(txctx.GasUsed, ctrler.govCtrler.GasPrice()))

		// add event
		txctx.Events = append(txctx.Events, abcitypes.Event{
			Type: "tx",
			Attributes: []abcitypes.EventAttribute{
				{Key: []byte(ctrlertypes.EVENT_ATTR_TXTYPE), Value: []byte(txctx.Tx.TypeString()), Index: true},
				{Key: []byte(ctrlertypes.EVENT_ATTR_TXSENDER), Value: []byte(txctx.Tx.From.String()), Index: true},
				{Key: []byte(ctrlertypes.EVENT_ATTR_TXRECVER), Value: []byte(txctx.Tx.To.String()), Index: true},
				{Key: []byte(ctrlertypes.EVENT_ATTR_ADDRPAIR), Value: []byte(txctx.Tx.From.String() + txctx.Tx.To.String()), Index: true},
				{Key: []byte(ctrlertypes.EVENT_ATTR_AMOUNT), Value: []byte(txctx.Tx.Amount.Dec()), Index: false},
				{Key: []byte(ctrlertypes.EVENT_ATTR_TXSTATUS), Value: []byte("0"), Index: false},
			},
		})

		return &abcitypes.ResponseDeliverTx{
			Code:      abcitypes.CodeTypeOK,
			GasWanted: txctx.Tx.Gas,
			GasUsed:   txctx.GasUsed,
			Data:      txctx.RetData,
			Events:    txctx.Events,
		}
	}
}

func (ctrler *BeatozApp) EndBlock(req abcitypes.RequestEndBlock) abcitypes.ResponseEndBlock {
	ctrler.logger.Debug("BeatozApp::EndBlock",
		"height", req.Height)

	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	{
		//
		// Execute all transactions
		//
		client := ctrler.localClient.(*beatozLocalClient)

		ctrler.txExecutor.TrxPreparer.Wait()
		// for debugging
		if ctrler.txExecutor.TrxPreparer.resultCount() != ctrler.currBlockCtx.TxsCnt() {
			panic(fmt.Sprintf("error: len(client.deliverTxReqs)(%v) != txs count in block(%v)",
				ctrler.txExecutor.TrxPreparer.resultCount(), ctrler.currBlockCtx.TxsCnt()))
		}

		// Execute every transaction in its own `TrxContext` sequentially
		for idx, ret := range ctrler.txExecutor.TrxPreparer.resultList() {
			// for debugging
			if ret == nil {
				panic(fmt.Sprintf("error: ret[%v] is nil. total result count: %v", idx, ctrler.txExecutor.TrxPreparer.resultCount()))
			} else if idx != ret.idx {
				panic(fmt.Sprintf("error: wrong transaction index. idx:%v, param.idx:%v", idx, ret.idx))
			}

			// `param.txctx` may be `nil`, which means an error occurred in generating `TrxContext`.
			// The `ResponseDeliverTx` with the error for this tx (`param.reqDeliverTx`)
			// already exists in `param.resDeliverTx` and it is written to blockchain as invalid tx.
			if ret.txctx != nil {
				ret.resDeliverTx = ctrler.asyncExecTrxContext(ret.txctx)
			}

			// the `client.Callback` will be called.
			// this callback function is set before calling `client.DeliverTxAsync`
			// in `execBlockOnProxyApp`(`github.com/tendermint/tendermint/state/execution.go`).
			client.callback(
				abcitypes.ToRequestDeliverTx(*ret.reqDeliverTx),
				abcitypes.ToResponseDeliverTx(*ret.resDeliverTx),
			)
		}
		ctrler.txExecutor.TrxPreparer.reset()
	}

	var beginBlockEvents []abcitypes.Event

	evts, xerr := ctrler.govCtrler.EndBlock(ctrler.currBlockCtx)
	if xerr != nil {
		ctrler.logger.Error("fail to execute EndBlock of govCtrler", "error", xerr)
		panic(xerr)
	}
	beginBlockEvents = append(beginBlockEvents, evts...)

	evts, xerr = ctrler.acctCtrler.EndBlock(ctrler.currBlockCtx)
	if xerr != nil {
		ctrler.logger.Error("fail to execute EndBlock of acctCtrler", "error", xerr)
		panic(xerr)
	}
	beginBlockEvents = append(beginBlockEvents, evts...)

	//
	// NOTE:
	// supplyCtrler.EncBlock should be called before vpowCtrler.EndBlock
	// to mint additional issuance based on the vpowCtrler.lastValidators of the previous block.
	// If you want to mint based on the lastValidators updated by transactions in the current block,
	// call it after vpowCtrler.EndBlock.
	evts, xerr = ctrler.supplyCtrler.EndBlock(ctrler.currBlockCtx)
	if xerr != nil {
		ctrler.logger.Error("fail to execute EndBlock of supplyCtrler", "error", xerr)
		panic(xerr)
	}
	beginBlockEvents = append(beginBlockEvents, evts...)

	evts, xerr = ctrler.vpowCtrler.EndBlock(ctrler.currBlockCtx)
	if xerr != nil {
		ctrler.logger.Error("fail to execute EndBlock of vpowCtrler", "error", xerr)
		panic(xerr)
	}
	beginBlockEvents = append(beginBlockEvents, evts...)

	evts, xerr = ctrler.vmCtrler.EndBlock(ctrler.currBlockCtx)
	if xerr != nil {
		ctrler.logger.Error("fail to execute EndBlock of vmCtrler", "error", xerr)
		panic(xerr)
	}
	beginBlockEvents = append(beginBlockEvents, evts...)

	//
	// adjust block gas limit
	newBlockGasLimit := ctrlertypes.AdjustBlockGasLimit(
		ctrler.currBlockCtx.GetBlockGasLimit(),
		ctrler.currBlockCtx.GetBlockGasUsed(),
		ctrler.govCtrler.MinBlockGasLimit(), // minimum block gas limit
		ctrler.govCtrler.MaxBlockGasLimit(),
	)

	var consensusParams *abcitypes.ConsensusParams
	if newBlockGasLimit != ctrler.currBlockCtx.GetBlockGasLimit() {
		consensusParams = &abcitypes.ConsensusParams{
			Block: &abcitypes.BlockParams{
				MaxBytes: ctrler.currBlockCtx.GetBlockSizeLimit(),
				MaxGas:   newBlockGasLimit,
			},
		}

		ctrler.logger.Info("Update block gas limit",
			"height", req.Height,
			"used", ctrler.currBlockCtx.GetBlockGasUsed(),
			"origin", ctrler.currBlockCtx.GetBlockGasLimit(),
			"ratio(%)", ctrler.currBlockCtx.GetBlockGasUsed()*100/ctrler.currBlockCtx.GetBlockGasLimit(),
			"new", newBlockGasLimit)
	}

	return abcitypes.ResponseEndBlock{
		ValidatorUpdates:      ctrler.currBlockCtx.ValUpdates,
		ConsensusParamUpdates: consensusParams,
		Events:                beginBlockEvents,
	}
}

func (ctrler *BeatozApp) Commit() abcitypes.ResponseCommit {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	ctrler.logger.Debug("BeatozApp::Commit", "height", ctrler.currBlockCtx.Height())

	ver0 := int64(0)
	hasher := crypto.DefaultHasher()
	ctrlers := []ctrlertypes.ILedgerHandler{
		ctrler.govCtrler,
		ctrler.acctCtrler,
		ctrler.supplyCtrler,
		ctrler.vpowCtrler,
		ctrler.vmCtrler,
	}

	for _, ctr := range ctrlers {
		hash, ver, xerr := ctr.Commit()
		if xerr != nil {
			panic(xerr)
		}
		if ver0 == 0 {
			ver0 = ver
		} else if ver != ver0 {
			panic(fmt.Sprintf("Not same versions: expected: %v, actual: %v", ver0, ver))
		}
		_, _ = hasher.Write(hash)
	}

	appHash := hasher.Sum(nil)

	ctrler.currBlockCtx.SetAppHash(appHash)
	ctrler.logger.Debug("Finish BeatozApp::Commit",
		"height", ver0,
		"txs", ctrler.currBlockCtx.TxsCnt(),
		"appHash", ctrler.currBlockCtx.AppHash())
	_ = ctrler.metaDB.PutLastBlockContext(ctrler.currBlockCtx)
	ctrler.lastBlockCtx = ctrler.currBlockCtx
	ctrler.currBlockCtx = nil

	if ctrler.rootConfig.RPC.ListenAddress != "" {
		if ctrler.lastBlockCtx.TxsCnt() > 0 {
			txn := ctrler.metaDB.Txn() + uint64(ctrler.lastBlockCtx.TxsCnt())
			_ = ctrler.metaDB.PutTxn(txn)
		}
		if ctrler.lastBlockCtx.SumFee().Sign() > 0 {
			feeTotal := new(uint256.Int).Add(ctrler.metaDB.TotalTxFee(), ctrler.lastBlockCtx.SumFee())
			_ = ctrler.metaDB.PutTotalTxFee(feeTotal)
		}
	}

	return abcitypes.ResponseCommit{
		Data: appHash[:],
	}
}

func checkRequestInitChain(req abcitypes.RequestInitChain) (*genesis.GenesisAppState, *uint256.Int, error) {
	//
	// genesis voting power
	genVotinPower := int64(0)
	for _, val := range req.Validators {
		genVotinPower += val.Power
	}
	genVotingPowerAmt := types2.PowerToAmount(genVotinPower)

	genAppState := &genesis.GenesisAppState{}
	if err := jsonx.Unmarshal(req.AppStateBytes, genAppState); err != nil {
		return nil, nil, err
	}

	//
	// initial supply
	genTotalSupply := genVotingPowerAmt.Clone()
	for _, holder := range genAppState.AssetHolders {
		_ = genTotalSupply.Add(genTotalSupply, holder.Balance)
	}

	govParams := genAppState.GovParams
	maxTotalSupply := govParams.MaxTotalSupply()
	if genTotalSupply.Cmp(maxTotalSupply) > 0 {
		return nil, nil, fmt.Errorf("error: initial supply (%d) cannot exceed max total supply (%d)", genTotalSupply, maxTotalSupply)
	}
	return genAppState, genTotalSupply, nil
}
