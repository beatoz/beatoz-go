package types

import (
	"encoding/json"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	ethcore "github.com/ethereum/go-ethereum/core"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"sync"
	"time"
)

type BlockContext struct {
	Hash                bytes.HexBytes
	Height              int64
	Time                time.Time
	ProposerAddress     types.Address
	LastCommitInfo      abcitypes.LastCommitInfo
	ByzantineValidators []abcitypes.Evidence
	ChainID             string

	AppHash bytes.HexBytes

	TxsCnt        int
	TxGasLimit    uint64
	BlockGasLimit uint64
	BlockGasUsed  uint64
	blockGasPool  *ethcore.GasPool

	GovHandler   IGovHandler
	AcctHandler  IAccountHandler
	StakeHandler IStakeHandler

	ValUpdates abcitypes.ValidatorUpdates

	mtx sync.RWMutex
}

func NewBlockContext(bi abcitypes.RequestBeginBlock, g IGovHandler, a IAccountHandler, s IStakeHandler) *BlockContext {
	return &BlockContext{
		Hash:                bi.Hash,
		Height:              bi.Header.Height,
		Time:                bi.Header.Time,
		ProposerAddress:     bi.Header.ProposerAddress,
		LastCommitInfo:      bi.LastCommitInfo,
		ByzantineValidators: bi.ByzantineValidators,
		ChainID:             bi.Header.ChainID,

		AppHash:       nil,
		TxsCnt:        0,
		TxGasLimit:    g.MaxTrxGas(),
		BlockGasLimit: g.MaxBlockGas(),
		BlockGasUsed:  0,
		blockGasPool:  new(ethcore.GasPool).AddGas(g.MaxBlockGas()),
		GovHandler:    g,
		AcctHandler:   a,
		StakeHandler:  s,
		ValUpdates:    nil,
	}
}

// `NewBlockContextAs` should be used only to test.
func NewBlockContextAs(h int64, t time.Time, c string, g IGovHandler, a IAccountHandler, s IStakeHandler) *BlockContext {
	_txGasLimit := uint64(0)
	_blockGasLimit := uint64(0)
	if g != nil {
		_txGasLimit = g.MaxTrxGas()
		_blockGasLimit = g.MaxBlockGas()
	}
	return &BlockContext{
		Height:  h,
		Time:    t,
		ChainID: c,

		AppHash:       nil,
		TxsCnt:        0,
		TxGasLimit:    _txGasLimit,
		BlockGasLimit: _blockGasLimit,
		BlockGasUsed:  0,
		blockGasPool:  new(ethcore.GasPool).AddGas(_blockGasLimit),
		GovHandler:    g,
		AcctHandler:   a,
		StakeHandler:  s,
		ValUpdates:    nil,
	}
}

func ExpectedNextBlockContextOf(bctx *BlockContext, interval time.Duration) *BlockContext {
	return &BlockContext{
		Height:  bctx.Height + 1,
		Time:    bctx.Time.Add(interval),
		ChainID: bctx.ChainID,

		TxGasLimit:    bctx.TxGasLimit,
		BlockGasLimit: bctx.GovHandler.MaxBlockGas(),
		blockGasPool:  new(ethcore.GasPool).AddGas(bctx.GovHandler.MaxBlockGas()),

		GovHandler:   bctx.GovHandler,
		AcctHandler:  bctx.AcctHandler,
		StakeHandler: bctx.StakeHandler,
	}
}

func (bctx *BlockContext) SetHeight(h int64) {
	bctx.mtx.Lock()
	defer bctx.mtx.Unlock()

	bctx.Height = h
}

func (bctx *BlockContext) GetHeight() int64 {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()

	return bctx.Height
}

func (bctx *BlockContext) GetAppHash() bytes.HexBytes {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()

	return bctx.AppHash
}

func (bctx *BlockContext) SetAppHash(hash []byte) {
	bctx.mtx.Lock()
	defer bctx.mtx.Unlock()

	bctx.AppHash = hash
}

func (bctx *BlockContext) TimeNano() int64 {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()

	return bctx.Time.UnixNano()
}

// TimeSeconds returns block time in seconds
func (bctx *BlockContext) TimeSeconds() int64 {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()

	// issue #50
	// the EVM  requires the block timestamp in seconds.
	return bctx.Time.Unix()
}

func (bctx *BlockContext) ExpectedNextBlockTimeSeconds(interval time.Duration) int64 {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()

	secs := int64(interval.Seconds())
	return bctx.Time.Unix() + secs
}

func (bctx *BlockContext) GetTxsCnt() int {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()

	return bctx.TxsCnt
}

func (bctx *BlockContext) AddTxsCnt(d int) {
	bctx.mtx.Lock()
	defer bctx.mtx.Unlock()

	bctx.TxsCnt += d
}

func (bctx *BlockContext) UseGas(gas uint64) xerrors.XError {
	bctx.mtx.Lock()
	defer bctx.mtx.Unlock()

	if err := bctx.blockGasPool.SubGas(gas); err != nil {
		return xerrors.ErrOverFlow.Wrap(err)
	}
	bctx.BlockGasUsed += gas
	return nil
}

func (bctx *BlockContext) GasUsed() uint64 {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()

	return bctx.BlockGasUsed
}

func (bctx *BlockContext) GetBlockGasLimit() uint64 {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()

	return bctx.BlockGasLimit
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

func (bctx *BlockContext) AdjustTrxGasLimit(txCnt int, minCap, maxCap uint64) {
	bctx.mtx.Lock()
	defer bctx.mtx.Unlock()

	bctx.TxGasLimit = adjustTrxGasLimit(txCnt, minCap, maxCap)
}

// `SetTrxGasLimit` is used only to test.
func (bctx *BlockContext) SetTrxGasLimit(gasLimit uint64) {
	bctx.mtx.Lock()
	defer bctx.mtx.Unlock()

	bctx.TxGasLimit = gasLimit
}

func (bctx *BlockContext) GetTrxGasLimit() uint64 {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()

	return bctx.TxGasLimit
}

func (bctx *BlockContext) MarshalJSON() ([]byte, error) {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()

	_bctx := &struct {
		Hash                bytes.HexBytes           `json:"hash"`
		Height              int64                    `json:"height"`
		Time                time.Time                `json:"time"`
		ProposerAddress     types.Address            `json:"proposerAddress"`
		LastCommitInfo      abcitypes.LastCommitInfo `json:"lastCommitInfo"`
		ByzantineValidators []abcitypes.Evidence     `json:"byzantineValidators"`
		ChainID             string                   `json:"chainId"`

		AppHash       []byte `json:"appHash"`
		TxsCnt        int    `json:"txsCnt"`
		TxGasLimit    uint64 `json:"txGasLimit"`
		BlockGasLimit uint64 `json:"blockGasLimit"`
		BlockGasUsed  uint64 `json:"blockGasUsed"`
	}{
		Hash:                bctx.Hash,
		Height:              bctx.Height,
		Time:                bctx.Time,
		ProposerAddress:     bctx.ProposerAddress,
		LastCommitInfo:      bctx.LastCommitInfo,
		ByzantineValidators: bctx.ByzantineValidators,
		ChainID:             bctx.ChainID,

		AppHash:       bctx.AppHash,
		TxsCnt:        bctx.TxsCnt,
		TxGasLimit:    bctx.TxGasLimit,
		BlockGasLimit: bctx.BlockGasLimit,
		BlockGasUsed:  bctx.BlockGasUsed,
	}

	return json.Marshal(_bctx)
}

func (bctx *BlockContext) UnmarshalJSON(bz []byte) error {
	bctx.mtx.Lock()
	defer bctx.mtx.Unlock()

	_bctx := &struct {
		Hash                bytes.HexBytes           `json:"hash"`
		Height              int64                    `json:"height"`
		Time                time.Time                `json:"time"`
		ProposerAddress     types.Address            `json:"proposerAddress"`
		LastCommitInfo      abcitypes.LastCommitInfo `json:"lastCommitInfo"`
		ByzantineValidators []abcitypes.Evidence     `json:"byzantineValidators"`
		ChainID             string                   `json:"chainId"`

		AppHash       []byte `json:"appHash"`
		TxsCnt        int    `json:"txsCnt"`
		TxGasLimit    uint64 `json:"txGasLimit"`
		BlockGasLimit uint64 `json:"blockGasLimit"`
		BlockGasUsed  uint64 `json:"blockGasUsed"`
	}{}

	if err := json.Unmarshal(bz, _bctx); err != nil {
		return err
	}
	bctx.Hash = _bctx.Hash
	bctx.Height = _bctx.Height
	bctx.Time = _bctx.Time
	bctx.ProposerAddress = _bctx.ProposerAddress
	bctx.LastCommitInfo = _bctx.LastCommitInfo
	bctx.ByzantineValidators = _bctx.ByzantineValidators
	bctx.ChainID = _bctx.ChainID
	bctx.AppHash = _bctx.AppHash
	bctx.TxsCnt = _bctx.TxsCnt
	bctx.TxGasLimit = _bctx.TxGasLimit
	bctx.BlockGasLimit = _bctx.BlockGasLimit
	bctx.BlockGasUsed = _bctx.BlockGasUsed
	return nil
}

type IBlockHandler interface {
	BeginBlock(*BlockContext) ([]abcitypes.Event, xerrors.XError)
	EndBlock(*BlockContext) ([]abcitypes.Event, xerrors.XError)
}

func adjustTrxGasLimit(txCnt int, minCap, maxCap uint64) uint64 {
	// Hyperbolic Function is applied.
	// `newMaxGas = (maxCap - minCap) / (1 + TxCount) + minCap`
	return (maxCap-minCap)/uint64(1+txCnt) + minCap
}
