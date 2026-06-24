package supply

import (
	"bytes"
	cfg "github.com/beatoz/beatoz-go/cmd/config"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	v2 "github.com/beatoz/beatoz-go/ledger/v2"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	tmlog "github.com/tendermint/tendermint/libs/log"
	"sync"
)

type SupplyCtrler struct {
	supplyState v2.IStateLedger

	lastTotalSupply *Supply

	reqCh  chan *reqMint
	respCh chan *respMint

	logger tmlog.Logger
	mtx    sync.RWMutex
}

func defaultNewItem(key v2.LedgerKey) v2.ILedgerItem {
	if bytes.HasPrefix(key, v2.KeyPrefixTotalSupply) {
		return &Supply{}
	} else if bytes.HasPrefix(key, v2.KeyPrefixReward) {
		return &Reward{}
	}
	return v2.NewInvalidLedgerItem("unknown supply ledger key prefix: 0x%x", []byte(key))
}

func NewSupplyCtrler(config *cfg.Config, logger tmlog.Logger) (*SupplyCtrler, xerrors.XError) {
	lg := logger.With("module", "beatoz_SupplyCtrler")

	ledger, xerr := v2.NewStateLedger("supply", config.DBDir(), 21*1000, defaultNewItem, lg)
	if xerr != nil {
		return nil, xerr
	}

	// load supply info from ledger
	item, xerr := ledger.Get(v2.LedgerKeyTotalSupply(), true)
	if xerr != nil && !xerr.Contains(xerrors.ErrNotFoundResult) {
		return nil, xerr
	}
	total, _ := item.(*Supply)
	if total == nil {
		total = NewSupply() // at genesis time.
	}

	reqCh, respCh := make(chan *reqMint, 1), make(chan *respMint, 1)
	go computeIssuanceAndRewardRoutine(reqCh, respCh)

	return &SupplyCtrler{
		supplyState:     ledger,
		lastTotalSupply: total,
		reqCh:           reqCh,
		respCh:          respCh,
		logger:          lg,
		mtx:             sync.RWMutex{},
	}, nil
}

func (ctrler *SupplyCtrler) CacheContext(exec bool) (*SupplyCtrler, func() xerrors.XError) {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	cachedState, writeCache := ctrler.supplyState.CacheContext(exec)
	return &SupplyCtrler{
		supplyState:     cachedState,
		lastTotalSupply: ctrler.lastTotalSupply,
		reqCh:           ctrler.reqCh,
		respCh:          ctrler.respCh,
		logger:          ctrler.logger,
	}, writeCache
}

func (ctrler *SupplyCtrler) CacheHandlerContext(exec bool) (ctrlertypes.ISupplyHandler, func() xerrors.XError) {
	return ctrler.CacheContext(exec)
}

func (ctrler *SupplyCtrler) InitLedger(req interface{}) xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	// it will be saved at Commit
	ctrler.lastTotalSupply.AdjustAdd(1, req.(*uint256.Int))
	return nil
}

func (ctrler *SupplyCtrler) TotalSupply() *uint256.Int {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	return ctrler.lastTotalSupply.GetTotalSupply()
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

		item, xerr := ctrler.supplyState.Get(v2.LedgerKeyReward(ctx.Tx.From), ctx.Exec)
		if xerr != nil {
			return xerr
		}

		rwd, _ := item.(*Reward)
		if txpayload.ReqAmt.Cmp(rwd.CumulatedAmount()) > 0 {
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
			ctx.Height(),
			ctx.AcctHandler,
			ctx.Exec)
	default:
		return xerrors.ErrUnknownTrxType
	}
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

var _ ctrlertypes.ISupplyHandler = (*SupplyCtrler)(nil)
var _ ctrlertypes.ITrxHandler = (*SupplyCtrler)(nil)
var _ ctrlertypes.IBlockHandler = (*SupplyCtrler)(nil)
var _ ctrlertypes.ILedgerHandler = (*SupplyCtrler)(nil)
