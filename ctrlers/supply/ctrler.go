package supply

import (
	"bytes"
	"fmt"
	cfg "github.com/beatoz/beatoz-go/cmd/config"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	btztypes "github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	tmlog "github.com/tendermint/tendermint/libs/log"
	"sync"
)

type mintedReward struct {
	addr btztypes.Address
	amt  *uint256.Int
}
type reqMint struct {
	bctx               *ctrlertypes.BlockContext
	lastTotalSupply    *uint256.Int
	lastAdjustedSupply *uint256.Int
	lastAdjustedHeight int64
}
type respMint struct {
	xerr      xerrors.XError
	newSupply *Supply
	rewards   []*mintedReward
}

type SupplyCtrler struct {
	supplyState v1.IStateLedger

	lastTotalSupply    *uint256.Int
	lastAdjustedSupply *uint256.Int
	lastAdjustedHeight int64

	mintedSupply []*Supply
	burnedSupply []*Supply

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
	if xerr != nil && !xerr.Contains(xerrors.ErrNotFoundResult) {
		return nil, xerr
	}
	total, _ := item.(*Supply)
	if total == nil {
		total = NewSupply(0, uint256.NewInt(0), uint256.NewInt(0))
	}

	item, xerr = ledger.Get(v1.LedgerKeyAdjustedSupply(), true)
	if xerr != nil && !xerr.Contains(xerrors.ErrNotFoundResult) {
		return nil, xerr
	}
	adjusted, _ := item.(*Supply)
	if adjusted == nil {
		adjusted = NewSupply(0, uint256.NewInt(0), uint256.NewInt(0))
	}
	reqCh, respCh := make(chan *reqMint, 1), make(chan *respMint, 1)
	go computeIssuanceAndRewardRoutine(reqCh, respCh)

	return &SupplyCtrler{
		supplyState:        ledger,
		lastTotalSupply:    total.Supply(),
		lastAdjustedSupply: adjusted.Supply(),
		lastAdjustedHeight: adjusted.Height(),
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
	initSupply := NewSupply(1, initTotalSupply, initTotalSupply)
	if xerr := ctrler.supplyState.Set(v1.LedgerKeyTotalSupply(), initSupply, true); xerr != nil {
		return xerr
	}

	// set initial adjusted supply & height
	if xerr := ctrler.supplyState.Set(v1.LedgerKeyAdjustedSupply(), initSupply, true); xerr != nil {
		return xerr
	}

	return nil
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
