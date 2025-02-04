package types

import (
	"encoding/json"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"sync"
	"time"
)

type BlockContext struct {
	blockInfo    abcitypes.RequestBeginBlock
	feeSum       *uint256.Int
	txsCnt       int
	maxGasPerTrx uint64
	appHash      bytes.HexBytes

	GovHandler   IGovHandler
	AcctHandler  IAccountHandler
	StakeHandler IStakeHandler

	ValUpdates abcitypes.ValidatorUpdates

	mtx sync.RWMutex
}

func NewBlockContext(bi abcitypes.RequestBeginBlock, g IGovHandler, a IAccountHandler, s IStakeHandler) *BlockContext {
	return &BlockContext{
		blockInfo:    bi,
		feeSum:       uint256.NewInt(0),
		txsCnt:       0,
		appHash:      nil,
		GovHandler:   g,
		AcctHandler:  a,
		StakeHandler: s,
		ValUpdates:   nil,
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

func (bctx *BlockContext) SumFee() *uint256.Int {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()

	return bctx.feeSum.Clone()
}

func (bctx *BlockContext) AddFee(fee *uint256.Int) {
	bctx.mtx.Lock()
	defer bctx.mtx.Unlock()

	_ = bctx.feeSum.Add(bctx.feeSum, fee)
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

func (bctx *BlockContext) AdjustMaxGasPerTrx(minCap, maxCap uint64) {
	bctx.mtx.Lock()
	defer bctx.mtx.Unlock()

	// Hyperbolic Function is applied.
	// `newMaxGas = (maxCap - minCap) / (1 + TxCount) + minCap`
	bctx.maxGasPerTrx = (maxCap-minCap)/uint64(1+bctx.txsCnt) + minCap
}

func (bctx *BlockContext) MaxGasPerTrx() uint64 {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()

	return bctx.maxGasPerTrx
}

func (bctx *BlockContext) MarshalJSON() ([]byte, error) {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()

	_bctx := &struct {
		BlockInfo    abcitypes.RequestBeginBlock `json:"blockInfo"`
		GasSum       *uint256.Int                `json:"feeSum"`
		TxsCnt       int                         `json:"txsCnt"`
		MaxGasPerTrx uint64                      `json:"maxGasPerTrx"`
		AppHash      []byte                      `json:"appHash"`
	}{
		BlockInfo:    bctx.blockInfo,
		GasSum:       bctx.feeSum,
		TxsCnt:       bctx.txsCnt,
		MaxGasPerTrx: bctx.maxGasPerTrx,
		AppHash:      bctx.appHash,
	}

	return json.Marshal(_bctx)
}

func (bctx *BlockContext) UnmarshalJSON(bz []byte) error {
	bctx.mtx.Lock()
	defer bctx.mtx.Unlock()

	_bctx := &struct {
		BlockInfo    abcitypes.RequestBeginBlock `json:"blockInfo"`
		GasSum       *uint256.Int                `json:"feeSum"`
		TxsCnt       int                         `json:"txsCnt"`
		MaxGasPerTrx uint64                      `json:"maxGasPerTrx"`
		AppHash      []byte                      `json:"appHash"`
	}{}

	if err := json.Unmarshal(bz, _bctx); err != nil {
		return err
	}
	bctx.blockInfo = _bctx.BlockInfo
	bctx.feeSum = _bctx.GasSum
	bctx.txsCnt = _bctx.TxsCnt
	bctx.maxGasPerTrx = _bctx.MaxGasPerTrx
	bctx.appHash = _bctx.AppHash
	return nil
}

type IBlockHandler interface {
	BeginBlock(*BlockContext) ([]abcitypes.Event, xerrors.XError)
	EndBlock(*BlockContext) ([]abcitypes.Event, xerrors.XError)
}
