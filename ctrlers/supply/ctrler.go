package supply

import (
	"bytes"
	"fmt"
	cfg "github.com/beatoz/beatoz-go/cmd/config"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmlog "github.com/tendermint/tendermint/libs/log"
	"sync"
)

type reqMint struct {
	bctx               *ctrlertypes.BlockContext
	lastTotalSupply    *uint256.Int
	lastAdjustedSupply *uint256.Int
	lastAdjustedHeight int64
}
type respMint struct {
	xerr      xerrors.XError
	newSupply *Supply
	rewards   []*Reward
}

type SupplyCtrler struct {
	supplyState v1.IStateLedger

	lastTotalSupply    *uint256.Int
	lastAdjustedSupply *uint256.Int
	lastAdjustedHeight int64

	reqCh  chan *reqMint
	respCh chan *respMint

	logger tmlog.Logger
	mtx    sync.RWMutex
}

func defaultNewItem(key v1.LedgerKey) v1.ILedgerItem {
	if bytes.HasPrefix(key, v1.KeyPrefixTotalSupply) || bytes.HasPrefix(key, v1.KeyPrefixAdjustedSupply) {
		return &Supply{}
	} else if bytes.HasPrefix(key, v1.KeyPrefixReward) {
		return &Reward{}
	}
	panic(fmt.Errorf("invalid key prefix:0x%x", key[0]))
}

func NewSupplyCtrler(config *cfg.Config, logger tmlog.Logger) (*SupplyCtrler, xerrors.XError) {
	lg := logger.With("module", "beatoz_SupplyCtrler")

	ledger, xerr := v1.NewStateLedger("supply", config.DBDir(), 21*2048, defaultNewItem, lg)
	if xerr != nil {
		return nil, xerr
	}

	// load supply info from ledger
	item, xerr := ledger.Get(v1.LedgerKeyTotalSupply(), true)
	if xerr != nil && xerr != xerrors.ErrNotFoundResult {
		return nil, xerr
	}
	total, _ := item.(*Supply)
	if total == nil {
		total = &Supply{}
	}

	item, xerr = ledger.Get(v1.LedgerKeyAdjustedSupply(), true)
	if xerr != nil && xerr != xerrors.ErrNotFoundResult {
		return nil, xerr
	}
	adjusted, _ := item.(*Supply)
	if adjusted == nil {
		adjusted = &Supply{}
	}
	reqCh, respCh := make(chan *reqMint, 1), make(chan *respMint, 1)
	go computeIssuanceAndRewardRoutine(reqCh, respCh)

	return &SupplyCtrler{
		supplyState:        ledger,
		lastTotalSupply:    new(uint256.Int).SetBytes(total.XSupply),
		lastAdjustedSupply: new(uint256.Int).SetBytes(adjusted.XSupply),
		lastAdjustedHeight: adjusted.Height,
		reqCh:              reqCh,
		respCh:             respCh,
		logger:             lg,
		mtx:                sync.RWMutex{},
	}, nil
}

func (ctrler *SupplyCtrler) InitLedger(req interface{}) xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	initTotalSupply := req.(*uint256.Int)
	ctrler.lastTotalSupply = initTotalSupply
	ctrler.lastAdjustedSupply = initTotalSupply
	ctrler.lastAdjustedHeight = 1

	// set initial total supply
	initSupply := &Supply{
		SupplyProto: SupplyProto{
			Height:  1,
			XChange: initTotalSupply.Bytes(),
			XSupply: initTotalSupply.Bytes(),
		},
		key: nil,
	}
	if xerr := ctrler.supplyState.Set(v1.LedgerKeyTotalSupply(), initSupply, true); xerr != nil {
		return xerr
	}

	// set initial adjusted supply & height
	if xerr := ctrler.supplyState.Set(v1.LedgerKeyAdjustedSupply(), initSupply, true); xerr != nil {
		return xerr
	}

	return nil
}
func (ctrler *SupplyCtrler) BeginBlock(bctx *ctrlertypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	if bctx.Height() > 0 && bctx.Height()%bctx.GovParams.InflationCycleBlocks() == 0 {
		ctrler.requestMint(bctx)
	}
	return nil, nil
}

func (ctrler *SupplyCtrler) ValidateTrx(ctx *ctrlertypes.TrxContext) xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	switch ctx.Tx.GetType() {
	case ctrlertypes.TRX_WITHDRAW:
		if ctx.Tx.Amount.Sign() != 0 {
			return xerrors.ErrInvalidTrx.Wrapf("amount must be 0")
		}
		txpayload, ok := ctx.Tx.Payload.(*ctrlertypes.TrxPayloadWithdraw)
		if !ok {
			return xerrors.ErrInvalidTrxPayloadType
		}

		item, xerr := ctrler.supplyState.Get(v1.LedgerKeyReward(ctx.Tx.From), ctx.Exec)
		if xerr != nil {
			return xerr
		}

		rwd, _ := item.(*Reward)
		if txpayload.ReqAmt.Cmp(rwd.amt) > 0 {
			return xerrors.ErrInvalidTrx.Wrapf("insufficient reward")
		}

		ctx.ValidateResult = rwd
	default:
		return xerrors.ErrUnknownTrxType
	}
	return nil
}

func (ctrler *SupplyCtrler) ExecuteTrx(ctx *ctrlertypes.TrxContext) xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	switch ctx.Tx.GetType() {
	case ctrlertypes.TRX_WITHDRAW:
		return ctrler.withdrawReward(
			ctx.ValidateResult.(*Reward),
			ctx.Tx.Payload.(*ctrlertypes.TrxPayloadWithdraw).ReqAmt,
			ctx.AcctHandler,
			ctx.Exec)
	default:
		return xerrors.ErrUnknownTrxType
	}
}
func (ctrler *SupplyCtrler) EndBlock(bctx *ctrlertypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	if bctx.Height() > 0 && bctx.Height()%bctx.GovParams.InflationCycleBlocks() == 0 {
		if _, xerr := ctrler.waitMint(bctx); xerr != nil {
			ctrler.logger.Error("fail to requestMint", "error", xerr.Error())
			return nil, xerr
		}
	}

	return nil, nil
}

func (ctrler *SupplyCtrler) Commit() ([]byte, int64, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	h, v, xerr := ctrler.supplyState.Commit()
	if xerr != nil {
		return nil, 0, xerr
	}

	return h, v, nil
}

func (ctrler *SupplyCtrler) Query(qry abcitypes.RequestQuery) ([]byte, xerrors.XError) {
	//TODO implement me
	return nil, nil
}

func (ctrler *SupplyCtrler) Close() xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	if ctrler.supplyState != nil {
		if xerr := ctrler.supplyState.Close(); xerr != nil {
			ctrler.logger.Error("fail to close supplyState", "error", xerr.Error())
		}
		ctrler.supplyState = nil
	}
	if ctrler.reqCh != nil {
		close(ctrler.reqCh)
		ctrler.reqCh = nil
	}
	if ctrler.respCh != nil {
		close(ctrler.respCh)
		ctrler.respCh = nil
	}
	return nil
}
func (ctrler *SupplyCtrler) RequestMint(bctx *ctrlertypes.BlockContext) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	ctrler.requestMint(bctx)
}

func (ctrler *SupplyCtrler) requestMint(bctx *ctrlertypes.BlockContext) {
	ctrler.reqCh <- &reqMint{
		bctx:               bctx,
		lastTotalSupply:    ctrler.lastTotalSupply.Clone(),
		lastAdjustedSupply: ctrler.lastAdjustedSupply.Clone(),
		lastAdjustedHeight: ctrler.lastAdjustedHeight,
	}
}

func (ctrler *SupplyCtrler) waitMint(bctx *ctrlertypes.BlockContext) (*respMint, xerrors.XError) {
	resp, _ := <-ctrler.respCh
	if resp == nil {
		return nil, xerrors.ErrNotFoundResult.Wrapf("no minting result")
	}
	if resp.xerr != nil {
		return nil, resp.xerr
	}

	// distribute rewards
	if xerr := ctrler.addReward(resp.rewards, bctx.GovParams.RewardPoolAddress()); xerr != nil {
		return nil, xerr
	}

	if xerr := ctrler.supplyState.Set(v1.LedgerKeyTotalSupply(), resp.newSupply, true); xerr != nil {
		return nil, xerr
	}
	ctrler.lastTotalSupply = new(uint256.Int).SetBytes(resp.newSupply.XSupply)
	return resp, nil
}

func (ctrler *SupplyCtrler) Burn(bctx *ctrlertypes.BlockContext, amt *uint256.Int) xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	return ctrler.burn(amt, bctx.Height())
}

func (ctrler *SupplyCtrler) burn(amt *uint256.Int, height int64) xerrors.XError {
	adjusted := new(uint256.Int).Sub(ctrler.lastTotalSupply, amt)
	burn := &Supply{
		SupplyProto: SupplyProto{
			Height:  height,
			XSupply: adjusted.Bytes(),
			XChange: amt.Bytes(),
		},
	}
	if xerr := ctrler.supplyState.Set(v1.LedgerKeyAdjustedSupply(), burn, true); xerr != nil {
		ctrler.logger.Error("fail to set adjusted supply", "error", xerr.Error())
		return xerr
	}

	ctrler.lastTotalSupply = adjusted
	ctrler.lastAdjustedSupply = adjusted
	ctrler.lastAdjustedHeight = height
	return nil
}

var _ ctrlertypes.ISupplyHandler = (*SupplyCtrler)(nil)
var _ ctrlertypes.ITrxHandler = (*SupplyCtrler)(nil)
var _ ctrlertypes.IBlockHandler = (*SupplyCtrler)(nil)
var _ ctrlertypes.ILedgerHandler = (*SupplyCtrler)(nil)
