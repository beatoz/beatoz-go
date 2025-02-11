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

	appHash bytes.HexBytes

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
		Hash:                bi.Hash,
		Height:              bi.Header.Height,
		Time:                bi.Header.Time,
		ProposerAddress:     bi.Header.ProposerAddress,
		LastCommitInfo:      bi.LastCommitInfo,
		ByzantineValidators: bi.ByzantineValidators,

		appHash:       nil,
		txsCnt:        0,
		txGasLimit:    g.MaxTrxGas(),
		blockGasLimit: g.MaxBlockGas(),
		blockGasUsed:  0,
		blockGasPool:  new(ethcore.GasPool).AddGas(g.MaxBlockGas()),
		GovHandler:    g,
		AcctHandler:   a,
		StakeHandler:  s,
		ValUpdates:    nil,
	}
}

func ExpectedNextBlockContextOf(bctx *BlockContext, interval time.Duration) *BlockContext {
	return &BlockContext{
		Height: bctx.Height + 1,
		Time:   bctx.Time.Add(interval),

		txGasLimit:    bctx.txGasLimit,
		blockGasLimit: bctx.GovHandler.MaxBlockGas(),
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

	return bctx.txsCnt
}

func (bctx *BlockContext) AddTxsCnt(d int) {
	bctx.mtx.Lock()
	defer bctx.mtx.Unlock()

	bctx.txsCnt += d
}

func (bctx *BlockContext) UseGas(gas uint64) xerrors.XError {
	bctx.mtx.Lock()
	defer bctx.mtx.Unlock()

	if err := bctx.blockGasPool.SubGas(gas); err != nil {
		return xerrors.ErrOverFlow.Wrap(err)
	}
	bctx.blockGasUsed += gas
	return nil
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

func (bctx *BlockContext) AdjustTrxGasLimit(txCnt int, minCap, maxCap uint64) {
	bctx.mtx.Lock()
	defer bctx.mtx.Unlock()

	bctx.txGasLimit = adjustTrxGasLimit(txCnt, minCap, maxCap)
}

// `SetTrxGasLimit` is used only to test.
func (bctx *BlockContext) SetTrxGasLimit(gasLimit uint64) {
	bctx.mtx.Lock()
	defer bctx.mtx.Unlock()

	bctx.txGasLimit = gasLimit
}

func (bctx *BlockContext) GetTrxGasLimit() uint64 {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()

	return bctx.txGasLimit
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

func adjustTrxGasLimit(txCnt int, minCap, maxCap uint64) uint64 {
	// Hyperbolic Function is applied.
	// `newMaxGas = (maxCap - minCap) / (1 + TxCount) + minCap`
	return (maxCap-minCap)/uint64(1+txCnt) + minCap
}
