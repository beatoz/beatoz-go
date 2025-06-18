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
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
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

	db, err := rawdb.NewLevelDBDatabase(path, 128, 128, "", false)
	if err != nil {
		panic(err)
	}

	lg := logger.With("module", "beatoz_EVMCtrler")

	return &EVMCtrler{
		ethChainConfig:  BEATOZMainnetEVMCtrlerChainConfig,
		ethDB:           db,
		metadb:          metadb,
		acctHandler:     acctHandler,
		lastRootHash:    hash,
		lastBlockHeight: bn,
		logger:          lg,
	}
}

func (ctrler *EVMCtrler) InitLedger(req interface{}) xerrors.XError {
	// Handle `lastRoot` at here
	return nil
}

func (ctrler *EVMCtrler) BeginBlock(bctx *ctrlertypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	if ctrler.lastBlockHeight+1 != bctx.Height() {
		return nil, xerrors.ErrBeginBlock.Wrapf("wrong block height - expected: %v, actual: %v", ctrler.lastBlockHeight+1, bctx.Height())
	}

	stdb, err := NewStateDBWrapper(ctrler.ethDB, ctrler.lastRootHash, bctx.AcctHandler, ctrler.logger)
	if err != nil {
		return nil, xerrors.From(err)
	}

	beneficiary := bytes.HexBytes(bctx.BlockInfo().Header.ProposerAddress).Array20()
	blockContext := evmBlockContext(beneficiary, bctx.Height(), bctx.TimeSeconds(), bctx.GetBlockGasLimit())
	ctrler.vmevm = ethvm.NewEVM(blockContext, ethvm.TxContext{}, stdb, ctrler.ethChainConfig, ethvm.Config{NoBaseFee: true})
	ctrler.stateDBWrapper = stdb
	ctrler.blockGasPool = bctx.GetBlockGasPool()

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

	// Check intrinsic gas if everything is correct
	bn := big.NewInt(ctx.Height())
	gas, err := ethcore.IntrinsicGas(inputData, nil, types.IsZeroAddress(ctx.Tx.To), ctrler.ethChainConfig.IsHomestead(bn), ctrler.ethChainConfig.IsIstanbul(bn))
	if err != nil {
		return xerrors.From(err)
	}

	if uint64(ctx.Tx.Gas) < gas {
		return xerrors.ErrInvalidGas
	}

	if ctx.Exec == false {
		// `GasUsed` will be used to simulate in `ExecuteTrx`.
		ctx.GasUsed = int64(gas)
	}

	return nil
}

func (ctrler *EVMCtrler) ExecuteTrx(ctx *ctrlertypes.TrxContext) xerrors.XError {
	if ctx.Exec == false {
		// Only in the 'DeliveryTx' phase, the contract transaction is fully executed,
		// and in the 'CheckTx' phase it is minimally executed.

		// update balance
		feeAmt := new(uint256.Int).Mul(ctx.Tx.GasPrice, uint256.NewInt(uint64(ctx.GasUsed)))
		needAmt := new(uint256.Int).Add(feeAmt, ctx.Tx.Amount)
		if xerr := ctx.Sender.SubBalance(needAmt); xerr != nil {
			return xerr
		}

		// update nonce
		ctx.Sender.AddNonce()

		// update account ledger
		if xerr := ctx.AcctHandler.SetAccount(ctx.Sender, ctx.Exec); xerr != nil {
			return xerr
		}
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
		ctx.Tx.GasPrice,
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
	blockNumber := uint256.NewInt(uint64(ctx.Height())).ToBig()
	if ctrler.ethChainConfig.IsByzantium(blockNumber) {
		ctrler.stateDBWrapper.Finalise(true)
	} else {
		ctrler.lastRootHash = ctrler.stateDBWrapper.IntermediateRoot(ctrler.ethChainConfig.IsEIP158(blockNumber)).Bytes()
	}

	// Gas is already applied to accounts and gas pool by buyGas and refundGas in EVM
	// the `EVM` handles nonce, amount and gas.
	ctx.GasUsed = int64(evmResult.UsedGas)
	ctx.RetData = evmResult.ReturnData

	//
	// Add events from evm logs.
	evmEvts := ctrler.evmLogsToEvent(ctx.TxHash.Array32())

	if ctx.Tx.To == nil || types.IsZeroAddress(ctx.Tx.To) {
		// When the new contract is created.
		createdAddr := ethcrypto.CreateAddress(ctx.Tx.From.Array20(), uint64(ctx.Tx.Nonce))
		ctrler.logger.Debug("Create contract", "address", createdAddr)

		// Account.Code 에 현재 Tx(Contract 생성) 의 Hash 를 기록.
		contAcct := ctx.AcctHandler.FindAccount(createdAddr[:], ctx.Exec)
		contAcct.SetCode(ctx.TxHash)
		if xerr := ctx.AcctHandler.SetAccount(contAcct, ctx.Exec); xerr != nil {
			return xerr
		}

		// When creating a contract,
		// the original evm returns deployed code (via evmResult.ReturnData),
		// and `ctx.RetData` currently points to it.
		// But should the deployed code really be returned?
		// Instead, let `ctx.RetData` have the deployed contract address.
		ctx.RetData = createdAddr[:]

		if len(evmEvts) == 0 {
			// If there is one or more events in `evmEvts`,
			// the `contractAddress` attribute already exists.
			evt := abcitypes.Event{
				Type: "evm",
				Attributes: []abcitypes.EventAttribute{
					{
						Key:   []byte("contractAddress"),
						Value: []byte(hex.EncodeToString(ctx.RetData)),
						Index: false,
					},
				},
			}
			evmEvts = append(evmEvts, evt)
		}
	}

	ctx.Events = append(ctx.Events, evmEvts...)

	return nil
}

func (ctrler *EVMCtrler) execVM(from, to types.Address, nonce, gas int64, gasPrice, amt *uint256.Int, data []byte, exec bool) (*ethcore.ExecutionResult, xerrors.XError) {
	var toAddr *common.Address
	if to != nil && !types.IsZeroAddress(to) {
		toAddr = new(common.Address)
		copy(toAddr[:], to)
	}

	vmmsg := evmMessage(from.Array20(), toAddr, nonce, gas, gasPrice, amt, data, false)
	txContext := ethcore.NewEVMTxContext(vmmsg)
	ctrler.vmevm.Reset(txContext, ctrler.stateDBWrapper)

	result, err := NewVMStateTransition(ctrler.vmevm, vmmsg, ctrler.blockGasPool).TransitionDb()
	if err != nil {
		return nil, xerrors.From(err)
	}

	return result, nil
}

func (ctrler *EVMCtrler) evmLogsToEvent(txHash common.Hash) []abcitypes.Event {
	var evts []abcitypes.Event // log : event = 1 : 1
	logs := ctrler.stateDBWrapper.GetLogs(txHash, common.Hash{})
	if logs != nil && len(logs) > 0 {
		for _, l := range logs {
			evt := abcitypes.Event{
				Type: "evm",
			}

			// Contract Address
			strVal := hex.EncodeToString(l.Address[:])
			evt.Attributes = append(evt.Attributes, abcitypes.EventAttribute{
				Key:   []byte("contractAddress"),
				Value: []byte(strVal),
				Index: false,
			})

			// Topics (indexed)
			for i, t := range l.Topics {
				strVal = hex.EncodeToString(t.Bytes())
				evt.Attributes = append(evt.Attributes, abcitypes.EventAttribute{
					Key:   []byte(fmt.Sprintf("topic.%d", i)),
					Value: []byte(strings.ToUpper(strVal)),
					Index: true,
				})
			}

			// Data (not indexed)
			if l.Data != nil && len(l.Data) > 0 {
				strVal = hex.EncodeToString(l.Data)
				evt.Attributes = append(evt.Attributes, abcitypes.EventAttribute{
					Key:   []byte("data"),
					Value: []byte(strVal),
					Index: false,
				})
			}

			// block height
			evt.Attributes = append(evt.Attributes, abcitypes.EventAttribute{
				Key:   []byte("blockNumber"),
				Value: []byte(strconv.FormatUint(l.BlockNumber, 10)),
			})

			// Removed
			strVal = "false"
			if l.Removed {
				strVal = "true"
			}
			evt.Attributes = append(evt.Attributes, abcitypes.EventAttribute{
				Key:   []byte("removed"),
				Value: []byte(strVal),
				Index: false,
			})

			evts = append(evts, evt)
		}
	}

	return evts
}

func (ctrler *EVMCtrler) EndBlock(bctx *ctrlertypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
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

	if ctrler.ethDB != nil {
		if err := ctrler.ethDB.Close(); err != nil {
			return xerrors.From(err)
		}
		ctrler.ethDB = nil
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

	memAcctHandler, xerr := ctrler.acctHandler.SimuAcctCtrlerAt(height)
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
