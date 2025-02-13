package types

import (
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	ethcore "github.com/ethereum/go-ethereum/core"
	"github.com/holiman/uint256"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"sync"
	"time"
)

type BlockContext struct {
	Hash                bytes.HexBytes           `json:"hash"`
	Height              int64                    `json:"height"`
	Time                time.Time                `json:"time"`
	ProposerAddress     types.Address            `json:"proposerAddress"`
	LastCommitInfo      abcitypes.LastCommitInfo `json:"lastCommitInfo"`
	ByzantineValidators []abcitypes.Evidence     `json:"byzantineValidators"`
	ChainID             string                   `json:"chainId"`

	AppHash bytes.HexBytes `json:"appHash"`

	TxsCnt        int    `json:"txsCnt"`
	TxGasLimit    uint64 `json:"txGasLimit"`
	BlockGasLimit uint64 `json:"blockGasLimit"`
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
		blockGasPool:  new(ethcore.GasPool).AddGas(g.MaxBlockGas()),
		GovHandler:    g,
		AcctHandler:   a,
		StakeHandler:  s,
		ValUpdates:    nil,
	}
}

// 'NewBlockContextAs' should be used for testing purposes only.
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
	return nil
}

func (bctx *BlockContext) RefundGas(gas uint64) xerrors.XError {
	bctx.mtx.Lock()
	defer bctx.mtx.Unlock()

	if bctx.blockGasPool.Gas()+gas > bctx.BlockGasLimit {
		xerrors.ErrOverFlow.Wrapf("gas refund causes overflow: gas limit: %d, gas pool: %d+%d",
			bctx.BlockGasLimit, bctx.blockGasPool, gas)
	}
	_ = bctx.blockGasPool.AddGas(gas)
	return nil
}

func (bctx *BlockContext) GasUsed() uint64 {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()

	return bctx.BlockGasLimit - bctx.blockGasPool.Gas()
}

func (bctx *BlockContext) FeeUsed() *uint256.Int {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()

	return GasToFee(bctx.GasUsed(), bctx.GovHandler.GasPrice())
}

func (bctx *BlockContext) BlockGasRemained() uint64 {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()

	return bctx.blockGasPool.Gas()
}

func (bctx *BlockContext) GetBlockGasPool() *ethcore.GasPool {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()

	return bctx.blockGasPool
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

func (bctx *BlockContext) GetTrxGasLimit() uint64 {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()

	return bctx.TxGasLimit
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
