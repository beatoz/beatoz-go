package evm

import (
	"encoding/hex"
	"fmt"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/ethereum/go-ethereum/common"
	ethcore "github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	ethvm "github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmlog "github.com/tendermint/tendermint/libs/log"
	tmdb "github.com/tendermint/tm-db"
	"math/big"
	"strconv"
	"strings"
	"sync"
)

var (
	lastBlockHeightKey                = []byte("lbh")
	BEATOZTestnetEVMCtrlerChainConfig = &params.ChainConfig{big.NewInt(220818), big.NewInt(0), nil, false, big.NewInt(0), common.Hash{}, big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), nil, nil, nil, nil, false, new(params.EthashConfig), nil}
	BEATOZMainnetEVMCtrlerChainConfig = &params.ChainConfig{big.NewInt(220819), big.NewInt(0), nil, false, big.NewInt(0), common.Hash{}, big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), nil, nil, nil, nil, false, new(params.EthashConfig), nil}
)

func blockKey(h int64) []byte {
	return []byte(fmt.Sprintf("bn%v", h))
}

type EVMCtrler struct {
	vmevm          *ethvm.EVM
	ethChainConfig *params.ChainConfig
	ethDB          ethdb.Database
	stateDBWrapper *StateDBWrapper
	acctHandler    ctrlertypes.IAccountHandler
	blockGasPool   *ethcore.GasPool

	metadb          tmdb.DB
	lastRootHash    bytes.HexBytes
	lastBlockHeight int64

	logger tmlog.Logger
	mtx    sync.RWMutex
}

func NewEVMCtrler(path string, acctHandler ctrlertypes.IAccountHandler, logger tmlog.Logger) *EVMCtrler {
	metadb, err := tmdb.NewDB("heightRootHash", "goleveldb", path)
	if err != nil {
		panic(err)
	}
	val, err := metadb.Get(lastBlockHeightKey)
	if err != nil {
		panic(err)
	}

	bn := int64(0)
	if val != nil {
		bn, err = strconv.ParseInt(string(val), 10, 64)
		if err != nil {
			panic(err)
		}
	}

	hash, err := metadb.Get(blockKey(bn))
	if err != nil {
		panic(err)
	}

	//rawDB, err := rawdb.NewLevelDBDatabaseWithFreezer(path, 0, 0, path, "", false)
	db, err := rawdb.NewLevelDBDatabase(path, 128, 128, "", false)
	if err != nil {
		panic(err)
	}

	logger = logger.With("module", "beatoz_EVMCtrler")

	return &EVMCtrler{
		ethChainConfig:  BEATOZMainnetEVMCtrlerChainConfig,
		ethDB:           db,
		metadb:          metadb,
		acctHandler:     acctHandler,
		lastRootHash:    hash,
		lastBlockHeight: bn,
		logger:          logger,
	}
}

func (ctrler *EVMCtrler) InitLedger(req interface{}) xerrors.XError {
	// Handle `lastRoot` at here
	return nil
}

func (ctrler *EVMCtrler) BeginBlock(ctx *ctrlertypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	if ctrler.lastBlockHeight+1 != ctx.Height() {
		return nil, xerrors.ErrBeginBlock.Wrapf("wrong block height - expected: %v, actual: %v", ctrler.lastBlockHeight+1, ctx.Height())
	}

	stdb, err := NewStateDBWrapper(ctrler.ethDB, ctrler.lastRootHash, ctx.AcctHandler, ctrler.logger)
	if err != nil {
		return nil, xerrors.From(err)
	}

	ctrler.stateDBWrapper = stdb
	ctrler.blockGasPool = new(ethcore.GasPool).AddGas(gasLimit)

	beneficiary := bytes.HexBytes(ctx.BlockInfo().Header.ProposerAddress).Array20()
	blockContext := evmBlockContext(beneficiary, ctx.Height(), ctx.TimeSeconds())
	ctrler.vmevm = ethvm.NewEVM(blockContext, ethvm.TxContext{}, ctrler.stateDBWrapper, ctrler.ethChainConfig, ethvm.Config{NoBaseFee: true})

	return nil, nil
}

func (ctrler *EVMCtrler) ValidateTrx(ctx *ctrlertypes.TrxContext) xerrors.XError {
	if ctx.Tx.GetType() != ctrlertypes.TRX_CONTRACT && ctx.Receiver.Code == nil {
		return xerrors.ErrUnknownTrxType
	}

	inputData := []byte(nil)
	payload, ok := ctx.Tx.Payload.(*ctrlertypes.TrxPayloadContract)
	if ok {
		inputData = payload.Data
	}

	//payload, ok := ctx.Tx.Payload.(*ctrlertypes.TrxPayloadContract)
	//if !ok {
	//	return xerrors.ErrInvalidTrxPayloadType
	//}
	//if payload.Data == nil || len(payload.Data) == 0 {
	//	return xerrors.ErrInvalidTrxPayloadParams
	//}

	// Check intrinsic gas if everything is correct
	bn := big.NewInt(ctx.Height)
	gas, err := ethcore.IntrinsicGas(inputData, nil, types.IsZeroAddress(ctx.Tx.To), ctrler.ethChainConfig.IsHomestead(bn), ctrler.ethChainConfig.IsIstanbul(bn))
	if err != nil {
		return xerrors.From(err)
	}

	if ctx.Tx.Gas < gas {
		return xerrors.ErrInvalidGas
	}

	return nil
}

func (ctrler *EVMCtrler) ExecuteTrx(ctx *ctrlertypes.TrxContext) xerrors.XError {
	if ctx.Exec == false {
		// issue #71
		// Execute a contract transaction only on `deliveryTx`
		return nil
	}
	if ctx.Tx.GetType() != ctrlertypes.TRX_CONTRACT && ctx.Receiver.Code == nil {
		return xerrors.ErrUnknownTrxType
	}

	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	// issue #69 - in order to pass `snap` to `Prepare`, call `Snapshot` before `Prepare`
	snap := ctrler.stateDBWrapper.Snapshot()
	// issue #48 - prepare hash and index of tx
	ctrler.stateDBWrapper.Prepare(ctx.TxHash, ctx.TxIdx, ctx.Tx.From, ctx.Tx.To, snap, ctx.Exec)

	inputData := []byte(nil)
	payload, ok := ctx.Tx.Payload.(*ctrlertypes.TrxPayloadContract)
	if ok {
		inputData = payload.Data
	}

	evmResult, xerr := ctrler.execVM(
		ctx.Tx.From,
		ctx.Tx.To,
		ctx.Tx.Nonce,
		ctx.Tx.Gas,
		ctx.GovHandler.GasPrice(),
		ctx.Tx.Amount,
		inputData,
		ctx.Exec,
	)
	if xerr != nil {
		ctrler.stateDBWrapper.RevertToSnapshot(snap)
		ctrler.stateDBWrapper.Finish()
		return xerr
	}

	if evmResult.Failed() {
		ctrler.stateDBWrapper.RevertToSnapshot(snap)
		ctrler.stateDBWrapper.Finish()
		ctx.RetData = evmResult.ReturnData
		return xerrors.From(evmResult.Err)
	}

	ctrler.stateDBWrapper.Finish()

	// Update the state with pending changes.
	blockNumber := uint256.NewInt(uint64(ctx.Height)).ToBig()
	if ctrler.ethChainConfig.IsByzantium(blockNumber) {
		ctrler.stateDBWrapper.Finalise(true)
	} else {
		ctrler.lastRootHash = ctrler.stateDBWrapper.IntermediateRoot(ctrler.ethChainConfig.IsEIP158(blockNumber)).Bytes()
	}

	// Gas is already applied to accounts by buyGas and refundGas of EVM.
	// the `EVM` handles nonce, amount and gas.
	ctx.GasUsed = evmResult.UsedGas

	ctx.RetData = evmResult.ReturnData

	//
	// Add events from evm.
	var attrs []abcitypes.EventAttribute
	if ctx.Tx.To == nil || types.IsZeroAddress(ctx.Tx.To) {
		// contract 생성.
		createdAddr := crypto.CreateAddress(ctx.Tx.From.Array20(), ctx.Tx.Nonce)
		ctrler.logger.Debug("Create contract", "address", createdAddr)

		// Account.Code 에 현재 Tx(Contract 생성) 의 Hash 를 기록.
		contAcct := ctx.AcctHandler.FindAccount(createdAddr[:], ctx.Exec)
		contAcct.SetCode(ctx.TxHash)
		if xerr := ctx.AcctHandler.SetAccount(contAcct, ctx.Exec); xerr != nil {
			return xerr
		}

		// EVM은 ReturnData 에 deployed code 를 리턴한다.
		// contract 생성 주소를 계산하여 리턴.
		ctx.RetData = createdAddr[:]

		// Event 로도 컨트랙트 주소 리턴
		attrs = []abcitypes.EventAttribute{
			{
				Key:   []byte("contractAddress"),
				Value: []byte(bytes.HexBytes(ctx.RetData).String()),
				Index: false,
			},
		}
	}

	logs := ctrler.stateDBWrapper.GetLogs(ctx.TxHash.Array32(), common.Hash{})
	if logs != nil && len(logs) > 0 {
		for _, l := range logs {
			// Contract Address
			strVal := hex.EncodeToString(l.Address[:])
			attrs = append(attrs, abcitypes.EventAttribute{
				Key:   []byte("contract"),
				Value: []byte(strVal),
				Index: true,
			})

			// Topics (indexed)
			for i, t := range l.Topics {
				strVal = hex.EncodeToString(t.Bytes())
				attrs = append(attrs, abcitypes.EventAttribute{
					Key:   []byte(fmt.Sprintf("topic.%d", i)),
					Value: []byte(strings.ToUpper(strVal)),
					Index: true,
				})
			}

			// Data (not indexed)
			if l.Data != nil && len(l.Data) > 0 {
				strVal = hex.EncodeToString(l.Data)
				attrs = append(attrs, abcitypes.EventAttribute{
					Key:   []byte("data"),
					Value: []byte(strVal),
					Index: false,
				})
			}

			// Removed
			strVal = "false"
			if l.Removed {
				strVal = "true"
			}
			attrs = append(attrs, abcitypes.EventAttribute{
				Key:   []byte("removed"),
				Value: []byte(strVal),
				Index: false,
			})
		}
	}
	ctx.Events = append(ctx.Events, abcitypes.Event{
		Type:       "evm",
		Attributes: attrs,
	})

	return nil
}

func (ctrler *EVMCtrler) execVM(from, to types.Address, nonce, gas uint64, gasPrice, amt *uint256.Int, data []byte, exec bool) (*ethcore.ExecutionResult, xerrors.XError) {
	var toAddr *common.Address
	if to != nil && !types.IsZeroAddress(to) {
		toAddr = new(common.Address)
		copy(toAddr[:], to)
	}

	vmmsg := evmMessage(from.Array20(), toAddr, nonce, gas, gasPrice, amt, data, false)
	txContext := ethcore.NewEVMTxContext(vmmsg)
	ctrler.vmevm.Reset(txContext, ctrler.stateDBWrapper)

	result, err := ethcore.ApplyMessage(ctrler.vmevm, vmmsg, ctrler.blockGasPool)
	if err != nil {
		return nil, xerrors.From(err)
	}

	return result, nil
}

func (ctrler *EVMCtrler) EndBlock(context *ctrlertypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	return nil, nil
}

func (ctrler *EVMCtrler) Commit() ([]byte, int64, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	rootHash, err := ctrler.stateDBWrapper.Commit(true)
	if err != nil {
		panic(err)
	}
	if err := ctrler.stateDBWrapper.Database().TrieDB().Commit(rootHash, true, nil); err != nil {
		panic(err)
	}
	ctrler.lastBlockHeight++
	ctrler.lastRootHash = rootHash[:]

	batch := ctrler.metadb.NewBatch()
	batch.Set(lastBlockHeightKey, []byte(strconv.FormatInt(ctrler.lastBlockHeight, 10)))
	batch.Set(blockKey(ctrler.lastBlockHeight), ctrler.lastRootHash)
	batch.WriteSync()
	batch.Close()

	stdb, err := NewStateDBWrapper(ctrler.ethDB, ctrler.lastRootHash, ctrler.acctHandler, ctrler.logger)
	if err != nil {
		panic(err)
	}

	ctrler.stateDBWrapper = stdb

	return rootHash[:], ctrler.lastBlockHeight, nil
}

func (ctrler *EVMCtrler) Close() xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	if ctrler.metadb != nil {
		if err := ctrler.metadb.Close(); err != nil {
			return xerrors.From(err)
		}
		ctrler.metadb = nil
	}

	if ctrler.stateDBWrapper != nil {
		if err := ctrler.stateDBWrapper.Close(); err != nil {
			return xerrors.From(err)
		}
		ctrler.stateDBWrapper = nil
	}

	return nil
}

// MemStateAt returns the ledger of EVM and AcctCtrler with the state values at the `height`.
// THIS LEDGER MUST BE NOT COMMITED.
// MemStateAt is called from `QueryCode` and `callVM`.
// When it is called from `QueryCode`, this ledger is only read (not updated).
// In this case, the ledger can be immutable.
// When it is called from `callVM`, this ledger may be updated.
// In this case, the ledger should not be immutable.
// In both cases, the ledger SHOULD NOT BE COMMITTED.
// To satisfy all conditions, MemStateAt returns the mempool ledger which can be updated but not committed.
func (ctrler *EVMCtrler) MemStateAt(height int64) (*StateDBWrapper, xerrors.XError) {
	hash, err := ctrler.metadb.Get(blockKey(height))
	if err != nil {
		return nil, xerrors.From(err)
	}

	stateDB, err := state.New(bytes.HexBytes(hash).Array32(), state.NewDatabase(ctrler.ethDB), nil)
	if err != nil {
		return nil, xerrors.From(err)
	}

	memAcctHandler, xerr := ctrler.acctHandler.MempoolAcctCtrlerAt(height)
	if xerr != nil {
		return nil, xerr
	}
	return &StateDBWrapper{
		StateDB:          stateDB,
		acctHandler:      memAcctHandler,
		accessedObjAddrs: make(map[common.Address]int),
		exec:             false,
		logger:           ctrler.logger,
	}, nil
}

var _ ctrlertypes.ILedgerHandler = (*EVMCtrler)(nil)
var _ ctrlertypes.ITrxHandler = (*EVMCtrler)(nil)
var _ ctrlertypes.IBlockHandler = (*EVMCtrler)(nil)
