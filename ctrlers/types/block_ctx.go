package types

import (
	"encoding/json"
	"fmt"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	ethcore "github.com/ethereum/go-ethereum/core"
	"github.com/holiman/uint256"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"sync"
	"time"
)

type BlockContext struct {
	blockInfo      abcitypes.RequestBeginBlock
	blockSizeLimit int64
	blockGasLimit  uint64
	blockGasPool   *ethcore.GasPool
	feeSum         *uint256.Int
	txsCnt         int
	evmTxsCnt      int
	appHash        bytes.HexBytes

	GovHandler   IGovHandler
	AcctHandler  IAccountHandler
	StakeHandler IStakeHandler

	ValUpdates abcitypes.ValidatorUpdates

	mtx sync.RWMutex
}

func NewBlockContext(bi abcitypes.RequestBeginBlock, g IGovHandler, a IAccountHandler, s IStakeHandler) *BlockContext {
	ret := &BlockContext{
		blockInfo:    bi,
		feeSum:       uint256.NewInt(0),
		txsCnt:       0,
		evmTxsCnt:    0,
		appHash:      nil,
		GovHandler:   g,
		AcctHandler:  a,
		StakeHandler: s,
		ValUpdates:   nil,
	}
	if g != nil {
		ret.setBlockGasLimit(g.MaxBlockGas())
	}
	return ret
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

func (bctx *BlockContext) EVMTxsCnt() int {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()

	return bctx.evmTxsCnt
}

func (bctx *BlockContext) AddTxsCnt(d int, isEVMTx bool) {
	bctx.mtx.Lock()
	defer bctx.mtx.Unlock()

	bctx.txsCnt += d
	if isEVMTx {
		bctx.evmTxsCnt += d
	}
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

func (bctx *BlockContext) GetBlockSizeLimit() int64 {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()
	return bctx.blockSizeLimit
}

func (bctx *BlockContext) SetBlockSizeLimit(limit int64) {
	bctx.mtx.Lock()
	defer bctx.mtx.Unlock()
	bctx.blockSizeLimit = limit
}

func (bctx *BlockContext) GetBlockGasLimit() uint64 {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()

	return bctx.blockGasLimit
}

func (bctx *BlockContext) SetBlockGasLimit(gasLimit uint64) {
	bctx.mtx.Lock()
	defer bctx.mtx.Unlock()

	bctx.setBlockGasLimit(gasLimit)
}

func (bctx *BlockContext) setBlockGasLimit(gasLimit uint64) {
	bctx.blockGasLimit = gasLimit
	bctx.blockGasPool = new(ethcore.GasPool).AddGas(gasLimit)
}

func (bctx *BlockContext) GetBlockGasUsed() uint64 {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()
	return bctx.getBlockGasUsed()
}

func (bctx *BlockContext) getBlockGasUsed() uint64 {
	return bctx.blockGasLimit - bctx.blockGasPool.Gas()
}

func (bctx *BlockContext) UseBlockGas(gas uint64) xerrors.XError {
	bctx.mtx.Lock()
	defer bctx.mtx.Unlock()

	if err := bctx.blockGasPool.SubGas(gas); err != nil {
		return xerrors.ErrInvalidGas.Wrap(err)
	}
	return nil
}

func (bctx *BlockContext) RefundBlockGas(gas uint64) {
	bctx.mtx.Lock()
	defer bctx.mtx.Unlock()

	// for debug
	_gasPool0 := bctx.blockGasPool.Gas()

	_ = bctx.blockGasPool.AddGas(gas)

	//
	// for debug
	_gasPool1 := bctx.blockGasPool.Gas()
	if _gasPool1 > bctx.blockGasLimit {
		panic(fmt.Sprintf("before gas pool(%v), gas(%v), after gas pool(%v), gas limit(%v)", _gasPool0, gas, _gasPool1, bctx.blockGasLimit))
	}
	//
	//
}

func (bctx *BlockContext) GetBlockGasPool() *ethcore.GasPool {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()
	return bctx.blockGasPool
}

func (bctx *BlockContext) MarshalJSON() ([]byte, error) {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()

	_bctx := &struct {
		BlockInfo     abcitypes.RequestBeginBlock `json:"blockInfo"`
		BlockGasLimit uint64                      `json:"blockGasLimit"`
		BlockGasUsed  uint64                      `json:"blockGasUsed"`
		FeeSum        *uint256.Int                `json:"feeSum"`
		TxsCnt        int                         `json:"txsCnt"`
		EVMTxsCnt     int                         `json:"evmTxsCnt"`
		AppHash       []byte                      `json:"appHash"`
	}{
		BlockInfo:     bctx.blockInfo,
		BlockGasLimit: bctx.blockGasLimit,
		BlockGasUsed:  bctx.GetBlockGasUsed(),
		FeeSum:        bctx.feeSum,
		TxsCnt:        bctx.txsCnt,
		EVMTxsCnt:     bctx.evmTxsCnt,
		AppHash:       bctx.appHash,
	}

	return json.Marshal(_bctx)
}

func (bctx *BlockContext) UnmarshalJSON(bz []byte) error {
	bctx.mtx.Lock()
	defer bctx.mtx.Unlock()

	_bctx := &struct {
		BlockInfo     abcitypes.RequestBeginBlock `json:"blockInfo"`
		BlockGasLimit uint64                      `json:"blockGasLimit"`
		BlockGasUsed  uint64                      `json:"blockGasUsed"`
		FeeSum        *uint256.Int                `json:"feeSum"`
		TxsCnt        int                         `json:"txsCnt"`
		EVMTxsCnt     int                         `json:"evmTxsCnt"`
		AppHash       []byte                      `json:"appHash"`
	}{}

	if err := json.Unmarshal(bz, _bctx); err != nil {
		return err
	}
	bctx.blockInfo = _bctx.BlockInfo
	bctx.blockGasLimit = _bctx.BlockGasLimit
	bctx.blockGasPool = new(ethcore.GasPool).AddGas(bctx.blockGasLimit - _bctx.BlockGasUsed)
	bctx.feeSum = _bctx.FeeSum
	bctx.txsCnt = _bctx.TxsCnt
	bctx.evmTxsCnt = _bctx.EVMTxsCnt
	bctx.appHash = _bctx.AppHash
	return nil
}

func AdjustBlockGasLimit(preBlockGasLimit, preBlockGasUsed, min, max uint64) uint64 {
	if preBlockGasUsed == 0 {
		return preBlockGasLimit
	}

	blockGasLimit := preBlockGasLimit
	upperThreshold := blockGasLimit - (blockGasLimit / 10) // 90%
	lowerThreshold := blockGasLimit / 100                  // 1%
	if preBlockGasUsed > upperThreshold {
		// increase gas limit
		blockGasLimit = blockGasLimit + (blockGasLimit / 10) // increase 10%

	} else if preBlockGasUsed < lowerThreshold {
		// decrease gas limit
		blockGasLimit = blockGasLimit - (blockGasLimit / 100) // decrease 1%
	}

	if blockGasLimit > max {
		blockGasLimit = max
	} else if blockGasLimit < min {
		blockGasLimit = min
	}
	return blockGasLimit
}

type IBlockHandler interface {
	BeginBlock(*BlockContext) ([]abcitypes.Event, xerrors.XError)
	EndBlock(*BlockContext) ([]abcitypes.Event, xerrors.XError)
}
