package supply

import (
	"bytes"
	"fmt"
	cfg "github.com/beatoz/beatoz-go/cmd/config"
	"github.com/beatoz/beatoz-go/ctrlers/types"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	types2 "github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	"github.com/shopspring/decimal"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmlog "github.com/tendermint/tendermint/libs/log"
	"sync"
)

type reqIssure struct {
	bctx               *types.BlockContext
	lastTotalSupply    *uint256.Int
	lastAdjustedSupply *uint256.Int
	lastAdjustedHeight int64
}
type respIssure struct {
	xerr      xerrors.XError
	newSupply *Supply
}

type SupplyCtrler struct {
	supplyState v1.IStateLedger

	lastTotalSupply    *uint256.Int
	lastAdjustedSupply *uint256.Int
	lastAdjustedHeight int64

	reqCh  chan *reqIssure
	respCh chan *respIssure

	logger tmlog.Logger
	mtx    sync.RWMutex
}

func defaultNewItem(key v1.LedgerKey) v1.ILedgerItem {
	if bytes.HasPrefix(key, v1.KeyPrefixTotalSupply) || bytes.HasPrefix(key, v1.KeyPrefixAdjustedSupply) {
		return &Supply{}
	}
	panic(fmt.Errorf("invalid key prefix:0x%x", key[0]))
}

func NewSupplyCtrler(config *cfg.Config, logger tmlog.Logger) (*SupplyCtrler, xerrors.XError) {
	lg := logger.With("module", "beatoz_SupplyCtrler")
	ledger, xerr := v1.NewStateLedger("supply", config.DBDir(), 21*2048, defaultNewItem, nil)
	if xerr != nil {
		return nil, xerr
	}

	// load supply info from ledger
	item, xerr := ledger.Get(v1.LedgerKeyTotalSupply(), true)
	if xerr != nil {
		return nil, xerr
	}
	total, _ := item.(*Supply)

	item, xerr = ledger.Get(v1.LedgerKeyAdjustedSupply(), true)
	if xerr != nil {
		return nil, xerr
	}
	adjusted, _ := item.(*Supply)

	return &SupplyCtrler{
		supplyState:        ledger,
		lastTotalSupply:    new(uint256.Int).SetBytes(total.XSupply),
		lastAdjustedSupply: new(uint256.Int).SetBytes(adjusted.XSupply),
		lastAdjustedHeight: adjusted.Height,
		reqCh:              make(chan *reqIssure, 1),
		respCh:             make(chan *respIssure, 1),
		logger:             lg,
		mtx:                sync.RWMutex{},
	}, nil
}

func (ctrler *SupplyCtrler) InitLedger(req interface{}) xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	ctrler.lastTotalSupply = types2.ToFons(350_000_000)
	ctrler.lastAdjustedSupply = types2.ToFons(350_000_000)
	ctrler.lastAdjustedHeight = 1

	// set initial total supply
	initSupply := &Supply{
		SupplyProto: SupplyProto{
			Height:  1,
			XChange: types2.ToFons(350_000_000).Bytes(),
			XSupply: types2.ToFons(350_000_000).Bytes(),
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

	go computeIssuanceAndRewardRoutine(ctrler.reqCh, ctrler.respCh)

	return nil
}
func (ctrler *SupplyCtrler) BeginBlock(bctx *types.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	if bctx.Height() > 0 && bctx.Height()%bctx.GovParams.InflationCycleBlocks() == 0 {
		ctrler.reqCh <- &reqIssure{
			bctx:               bctx,
			lastTotalSupply:    ctrler.lastTotalSupply.Clone(),
			lastAdjustedSupply: ctrler.lastAdjustedSupply.Clone(),
			lastAdjustedHeight: ctrler.lastAdjustedHeight,
		}
	}
	return nil, nil
}

func (ctrler *SupplyCtrler) EndBlock(bctx *types.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	if bctx.Height() > 0 && bctx.Height()%bctx.GovParams.InflationCycleBlocks() == 0 {
		resp := <-ctrler.respCh
		if resp.xerr != nil {
			return nil, resp.xerr
		}

		if xerr := ctrler.supplyState.Set(v1.LedgerKeyTotalSupply(), resp.newSupply, true); xerr != nil {
			ctrler.logger.Error("fail to set total supply", "error", xerr.Error())
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

func computeIssuanceAndRewardRoutine(reqCh chan *reqIssure, respCh chan *respIssure) {

	for {
		req, ok := <-reqCh
		if !ok {
			// reqCh is closed
			break
		}

		bctx := req.bctx
		lastTotalSupply := req.lastTotalSupply
		lastAdjustedSupply := req.lastAdjustedSupply
		lastAdjustedHeight := req.lastAdjustedHeight

		// 1. compute voting power weight
		wa, xerr := bctx.VPowerHandler.ComputeWeight(
			bctx.Height(),
			bctx.GovParams.RipeningBlocks(),
			bctx.GovParams.BondingBlocksWeightPermil(),
			lastTotalSupply,
		)
		if xerr != nil {
			respCh <- &respIssure{
				xerr:      xerr,
				newSupply: nil,
			}
			continue
		}
		totalSupply := Si(bctx.Height(), lastAdjustedHeight, lastAdjustedSupply, bctx.GovParams.MaxTotalSupply(), bctx.GovParams.InflationWeightPermil(), wa)
		additionalIssuance := new(uint256.Int).Sub(totalSupply, lastTotalSupply)

		//
		// 2. reward ...
		//todo: implement reward

		respCh <- &respIssure{
			xerr: nil,
			newSupply: &Supply{
				SupplyProto: SupplyProto{
					Height:  bctx.Height(),
					XSupply: totalSupply.Bytes(),
					XChange: additionalIssuance.Bytes(),
				},
			},
		}
	}

}

// Si returns the total supply amount determined by the issuance formula of block 'height'.
func Si(height, adjustedHeight int64, adjustedSupply, smax *uint256.Int, lambda int64, wa decimal.Decimal) *uint256.Int {
	if height < adjustedHeight {
		panic("the height should be greater than the adjusted height ")
	}
	_lambda := decimal.New(int64(lambda), -3)
	decLambdaAddOne := _lambda.Add(decimal.New(1, 0))
	expWHid := wa.Mul(H(height-adjustedHeight, 1))

	numer := decimal.NewFromBigInt(new(uint256.Int).Sub(smax, adjustedSupply).ToBig(), 0)
	denom := decLambdaAddOne.Pow(expWHid)

	decSmax := decimal.NewFromBigInt(smax.ToBig(), 0)
	return uint256.MustFromBig(decSmax.Sub(numer.Div(denom)).BigInt())
}

// H returns the normalized block time corresponding to the given block height.
// It calculates how far along the blockchain is relative to a predefined reference period.
// For example, if the reference period is one year, a return value of 1.0 indicates that
// exactly one reference period has elapsed.

var oneYearSeconds int64 = 31_536_000

func H(height, blockIntvSec int64) decimal.Decimal {
	return decimal.NewFromInt(height).Mul(decimal.NewFromInt(blockIntvSec)).Div(decimal.NewFromInt(oneYearSeconds))
}

var _ types.IBlockHandler = (*SupplyCtrler)(nil)
var _ types.ILedgerHandler = (*SupplyCtrler)(nil)
