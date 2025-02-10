package types

import (
	"encoding/json"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	ethcore "github.com/ethereum/go-ethereum/core"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"sync"
	"time"
)

type BlockContext struct {
	blockInfo abcitypes.RequestBeginBlock
	appHash   bytes.HexBytes

	txsCnt        int
	txGasLimit    uint64
	blockGasLimit uint64
	blockGasUsed  uint64
	blockGasPool  *ethcore.GasPool

	GovHandler   IGovHandler
	AcctHandler  IAccountHandler
	StakeHandler IStakeHandler

	ValUpdates abcitypes.ValidatorUpdates

	mtx sync.RWMutex
}

func NewBlockContext(bi abcitypes.RequestBeginBlock, g IGovHandler, a IAccountHandler, s IStakeHandler) *BlockContext {
	return &BlockContext{
		blockInfo:     bi,
		appHash:       nil,
		txsCnt:        0,
		txGasLimit:    0,
		blockGasLimit: g.MaxBlockGas(),
		blockGasUsed:  0,
		blockGasPool:  new(ethcore.GasPool).AddGas(g.MaxBlockGas()),
		GovHandler:    g,
		AcctHandler:   a,
		StakeHandler:  s,
		ValUpdates:    nil,
	}
}

func (bctx *BlockContext) BlockInfo() abcitypes.RequestBeginBlock {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()

	return bctx.blockInfo
}

func (bctx *BlockContext) SetHeight(h int64) {
	bctx.mtx.Lock()
	defer bctx.mtx.Unlock()

	bctx.blockInfo.Header.Height = h
}

func (bctx *BlockContext) Height() int64 {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()

	return bctx.blockInfo.Header.Height
}

func (bctx *BlockContext) PreAppHash() bytes.HexBytes {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()

	return bctx.blockInfo.Header.GetAppHash()
}

func (bctx *BlockContext) AppHash() bytes.HexBytes {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()

	return bctx.appHash
}

func (bctx *BlockContext) SetAppHash(hash []byte) {
	bctx.mtx.Lock()
	defer bctx.mtx.Unlock()

	bctx.appHash = hash
}

func (bctx *BlockContext) TimeNano() int64 {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()

	return bctx.blockInfo.Header.GetTime().UnixNano()
}

// TimeSeconds returns block time in seconds
func (bctx *BlockContext) TimeSeconds() int64 {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()

	// issue #50
	// the EVM  requires the block timestamp in seconds.
	return bctx.blockInfo.Header.GetTime().Unix()
}

func (bctx *BlockContext) ExpectedNextBlockTimeSeconds(interval time.Duration) int64 {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()

	secs := int64(interval.Seconds())
	return bctx.blockInfo.Header.GetTime().Unix() + secs
}

func (bctx *BlockContext) TxsCnt() int {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()

	return bctx.txsCnt
}

func (bctx *BlockContext) AddTxsCnt(d int) {
	bctx.mtx.Lock()
	defer bctx.mtx.Unlock()

	bctx.txsCnt += d
}

func (bctx *BlockContext) AddGasUsed(gas uint64) {
	bctx.mtx.Lock()
	defer bctx.mtx.Unlock()

	bctx.blockGasUsed += gas
}

func (bctx *BlockContext) GasUsed() uint64 {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()

	return bctx.blockGasUsed
}

func (bctx *BlockContext) BlockGasLimit() uint64 {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()

	return bctx.blockGasLimit
}

func (bctx *BlockContext) GetValUpdates() abcitypes.ValidatorUpdates {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()

	return bctx.ValUpdates
}

func (bctx *BlockContext) SetValUpdates(valUps abcitypes.ValidatorUpdates) {
	bctx.mtx.Lock()
	defer bctx.mtx.Unlock()

	bctx.ValUpdates = valUps
}

func (bctx *BlockContext) AdjustTrxGasLimit(minCap, maxCap uint64) {
	bctx.mtx.Lock()
	defer bctx.mtx.Unlock()

	// Hyperbolic Function is applied.
	// `newMaxGas = (maxCap - minCap) / (1 + TxCount) + minCap`
	bctx.txGasLimit = (maxCap-minCap)/uint64(1+bctx.txsCnt) + minCap
}

func (bctx *BlockContext) ExpectedTrxGasLimit() uint64 {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()

	return bctx.txGasLimit
}

func (bctx *BlockContext) MarshalJSON() ([]byte, error) {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()

	_bctx := &struct {
		BlockInfo     abcitypes.RequestBeginBlock `json:"blockInfo"`
		AppHash       []byte                      `json:"appHash"`
		TxsCnt        int                         `json:"txsCnt"`
		TxGasLimit    uint64                      `json:"txGasLimit"`
		BlockGasLimit uint64                      `json:"blockGasLimit"`
		BlockGasUsed  uint64                      `json:"blockGasUsed"`
	}{
		BlockInfo:     bctx.blockInfo,
		AppHash:       bctx.appHash,
		TxsCnt:        bctx.txsCnt,
		TxGasLimit:    bctx.txGasLimit,
		BlockGasLimit: bctx.blockGasLimit,
		BlockGasUsed:  bctx.blockGasUsed,
	}

	return json.Marshal(_bctx)
}

func (bctx *BlockContext) UnmarshalJSON(bz []byte) error {
	bctx.mtx.Lock()
	defer bctx.mtx.Unlock()

	_bctx := &struct {
		BlockInfo     abcitypes.RequestBeginBlock `json:"blockInfo"`
		AppHash       []byte                      `json:"appHash"`
		TxsCnt        int                         `json:"txsCnt"`
		TxGasLimit    uint64                      `json:"txGasLimit"`
		BlockGasLimit uint64                      `json:"blockGasLimit"`
		BlockGasUsed  uint64                      `json:"blockGasUsed"`
	}{}

	if err := json.Unmarshal(bz, _bctx); err != nil {
		return err
	}
	bctx.blockInfo = _bctx.BlockInfo
	bctx.appHash = _bctx.AppHash
	bctx.txsCnt = _bctx.TxsCnt
	bctx.txGasLimit = _bctx.TxGasLimit
	bctx.blockGasLimit = _bctx.BlockGasLimit
	bctx.blockGasUsed = _bctx.BlockGasUsed
	return nil
}

type IBlockHandler interface {
	BeginBlock(*BlockContext) ([]abcitypes.Event, xerrors.XError)
	EndBlock(*BlockContext) ([]abcitypes.Event, xerrors.XError)
}
