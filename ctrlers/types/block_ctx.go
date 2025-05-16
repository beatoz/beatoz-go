package types

import (
	"encoding/json"
	"fmt"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	ethcore "github.com/ethereum/go-ethereum/core"
	"github.com/holiman/uint256"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmprototypes "github.com/tendermint/tendermint/proto/tendermint/types"
	"sync"
	"time"
)

type BlockContext struct {
	blockInfo      abcitypes.RequestBeginBlock
	blockSizeLimit int64
	blockGasLimit  int64
	blockGasPool   *ethcore.GasPool
	feeSum         *uint256.Int
	txsCnt         int
	evmTxsCnt      int
	appHash        bytes.HexBytes

	GovHandler  IGovHandler
	AcctHandler IAccountHandler
	EVMHandler  IEVMHandler

	// DEPRECATED
	//StakeHandler  IStakeHandler

	SupplyHandler ISupplyHandler
	VPowerHandler IVPowerHandler

	ValUpdates abcitypes.ValidatorUpdates

	mtx sync.RWMutex
}

func NewBlockContext(bi abcitypes.RequestBeginBlock, g IGovHandler, a IAccountHandler, e IEVMHandler, su ISupplyHandler, vp IVPowerHandler) *BlockContext {

	// all handlers should implement ITrxHandler and IBlockHandler
	for _, handler := range []interface{}{g, a, e, su, vp} {
		if handler != nil {
			_ = handler.(ITrxHandler)
			_ = handler.(IBlockHandler)
		}

	}

	ret := &BlockContext{
		blockInfo:   bi,
		feeSum:      uint256.NewInt(0),
		txsCnt:      0,
		evmTxsCnt:   0,
		appHash:     nil,
		GovHandler:  g,
		AcctHandler: a,
		EVMHandler:  e,
		//StakeHandler:  s,
		SupplyHandler: su,
		VPowerHandler: vp, // todo: perfectly implement temp code
		ValUpdates:    nil,
	}
	if g != nil {
		ret.setBlockGasLimit(g.MaxBlockGas())
	}
	return ret
}

func TempBlockContext(chainId string, height int64, btime time.Time, g IGovHandler, a IAccountHandler, e IEVMHandler, su ISupplyHandler, vp IVPowerHandler) *BlockContext {
	next := NewBlockContext(
		abcitypes.RequestBeginBlock{
			Header: tmprototypes.Header{
				ChainID: chainId,
				Height:  height,
				Time:    btime,
			},
		},
		g, a, e, su, vp,
	)
	return next
}

func ExpectNextBlockContext(last *BlockContext, blockIntval time.Duration) *BlockContext {
	tm := last.BlockInfo().Header.Time.Add(blockIntval * time.Second)
	next := NewBlockContext(
		abcitypes.RequestBeginBlock{
			Header: tmprototypes.Header{
				ChainID: last.ChainID(),
				Height:  last.Height() + 1,
				Time:    tm,
			},
		},
		last.GovHandler,
		last.AcctHandler,
		last.EVMHandler,
		last.SupplyHandler,
		last.VPowerHandler,
	)
	return next
}

func (bctx *BlockContext) BlockInfo() abcitypes.RequestBeginBlock {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()

	return bctx.blockInfo
}

func (bctx *BlockContext) ChainID() string {
	bctx.mtx.Lock()
	defer bctx.mtx.Unlock()

	return bctx.blockInfo.Header.ChainID
}

func (bctx *BlockContext) Height() int64 {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()

	return bctx.blockInfo.Header.Height
}

func (bctx *BlockContext) ProposerAddress() types.Address {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()
	return bctx.blockInfo.Header.ProposerAddress
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

func (bctx *BlockContext) GetBlockGasLimit() int64 {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()

	return bctx.blockGasLimit
}

func (bctx *BlockContext) SetBlockGasLimit(gasLimit int64) {
	bctx.mtx.Lock()
	defer bctx.mtx.Unlock()

	bctx.setBlockGasLimit(gasLimit)
}

func (bctx *BlockContext) setBlockGasLimit(gasLimit int64) {
	bctx.blockGasLimit = gasLimit
	bctx.blockGasPool = new(ethcore.GasPool).AddGas(uint64(gasLimit))
}

func (bctx *BlockContext) GetBlockGasUsed() int64 {
	bctx.mtx.RLock()
	defer bctx.mtx.RUnlock()
	return bctx.getBlockGasUsed()
}

func (bctx *BlockContext) getBlockGasUsed() int64 {
	return bctx.blockGasLimit - int64(bctx.blockGasPool.Gas())
}

func (bctx *BlockContext) UseBlockGas(gas int64) xerrors.XError {
	bctx.mtx.Lock()
	defer bctx.mtx.Unlock()

	if err := bctx.blockGasPool.SubGas(uint64(gas)); err != nil {
		return xerrors.ErrInvalidGas.Wrap(err)
	}
	return nil
}

func (bctx *BlockContext) RefundBlockGas(gas int64) {
	bctx.mtx.Lock()
	defer bctx.mtx.Unlock()

	// for debug
	_gasPool0 := bctx.blockGasPool.Gas()

	_ = bctx.blockGasPool.AddGas(uint64(gas))

	//
	// for debug
	_gasPool1 := int64(bctx.blockGasPool.Gas())
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
		BlockGasLimit int64                       `json:"blockGasLimit"`
		BlockGasUsed  int64                       `json:"blockGasUsed"`
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
		BlockGasLimit int64                       `json:"blockGasLimit"`
		BlockGasUsed  int64                       `json:"blockGasUsed"`
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
	bctx.blockGasPool = new(ethcore.GasPool).AddGas(uint64(bctx.blockGasLimit - _bctx.BlockGasUsed))
	bctx.feeSum = _bctx.FeeSum
	bctx.txsCnt = _bctx.TxsCnt
	bctx.evmTxsCnt = _bctx.EVMTxsCnt
	bctx.appHash = _bctx.AppHash
	return nil
}

func AdjustBlockGasLimit(preBlockGasLimit, preBlockGasUsed, min, max int64) int64 {
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

// DEPRECATED: Use for test only
func (bctx *BlockContext) SetHeight(h int64) {
	bctx.mtx.Lock()
	defer bctx.mtx.Unlock()

	bctx.blockInfo.Header.Height = h
}

// DEPRECATED: Use for test only
func (bctx *BlockContext) SetProposerAddress(addr types.Address) {
	bctx.mtx.Lock()
	defer bctx.mtx.Unlock()

	bctx.blockInfo.Header.ProposerAddress = addr
}

// DEPRECATED: Use for test only
func (bctx *BlockContext) SetEvidance(evidences []abcitypes.Evidence) {
	bctx.mtx.Lock()
	defer bctx.mtx.Unlock()

	bctx.blockInfo.ByzantineValidators = append(bctx.blockInfo.ByzantineValidators, evidences...)
}
