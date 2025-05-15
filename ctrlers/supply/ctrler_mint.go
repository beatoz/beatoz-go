package supply

import (
	"fmt"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	btztypes "github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	"github.com/shopspring/decimal"
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
	xerr         xerrors.XError
	sumMintedAmt *uint256.Int
	rewards      []*mintedReward
}

func (ctrler *SupplyCtrler) RequestMint(bctx *ctrlertypes.BlockContext) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	ctrler.requestMint(bctx)
}

// requestMint makes reqMint object and send it via the channel reqCh.
// it is called from BeginBlock.
func (ctrler *SupplyCtrler) requestMint(bctx *ctrlertypes.BlockContext) {
	ctrler.reqCh <- &reqMint{
		bctx:               bctx,
		lastTotalSupply:    ctrler.lastTotalSupply.GetTotalSupply(),
		lastAdjustedSupply: ctrler.lastTotalSupply.GetAdjustSupply(),
		lastAdjustedHeight: ctrler.lastTotalSupply.GetAdjustHeight(),
	}
}

// waitMint updates supplyState of ctrler.
// it is called from EndBlock.
func (ctrler *SupplyCtrler) waitMint(bctx *ctrlertypes.BlockContext) (*respMint, xerrors.XError) {

	// wait response from computeIssuanceAndRewardRoutine
	resp, _ := <-ctrler.respCh
	if resp == nil {
		return nil, xerrors.ErrNotFoundResult.Wrapf("no minting result")
	}
	if resp.xerr != nil {
		return nil, resp.xerr
	}

	// distribute rewards
	if xerr := ctrler.distReward(resp.rewards, bctx.Height(), bctx.GovHandler.RewardPoolAddress()); xerr != nil {
		return nil, xerr
	}

	ctrler.lastTotalSupply.Add(bctx.Height(), resp.sumMintedAmt)
	return resp, nil
}

// computeIssuanceAndRewardRoutine calculates additional issuance and reward amount based on voting power weights.
// And it returns the result through the response channel `respCh` because this is executed in goroutine context.
// NOTE: DO NOT ACCESS `supplyState` of SupplyCtrler in any way from this goroutine.
// The supplyState of SupplyCtrler may be updated while computeIssuanceAndRewardRoutine is executed.
// If `supplyState` is updated in this goroutine, the writing order may be not deterministic.
func computeIssuanceAndRewardRoutine(reqCh chan *reqMint, respCh chan *respMint) {
	for {
		req, ok := <-reqCh
		if !ok {
			break
		}

		bctx := req.bctx
		lastTotalSupply := req.lastTotalSupply
		lastAdjustedSupply := req.lastAdjustedSupply
		lastAdjustedHeight := req.lastAdjustedHeight

		// 1. compute voting power weight
		retWeight, xerr := bctx.VPowerHandler.ComputeWeight(
			bctx.Height(),
			bctx.GovHandler.InflationCycleBlocks(),
			bctx.GovHandler.RipeningBlocks(),
			bctx.GovHandler.BondingBlocksWeightPermil(),
			lastTotalSupply,
		)
		if xerr != nil {
			respCh <- &respMint{
				xerr: xerr,
			}
			continue
		}

		valRate := decimal.NewFromInt(int64(bctx.GovHandler.ValidatorRewardRate())).Div(decimal.NewFromInt(100))
		waAll := retWeight.SumWeight()  //.Truncate(precision) // is too expensive
		waVals := retWeight.ValWeight() //.Truncate(precision)

		totalSupply := Si(bctx.Height(), lastAdjustedHeight, lastAdjustedSupply, bctx.GovHandler.MaxTotalSupply(), bctx.GovHandler.InflationWeightPermil(), waAll).Floor()
		addedSupply := totalSupply.Sub(decimal.NewFromBigInt(lastTotalSupply.ToBig(), 0))
		if addedSupply.Sign() < 0 {
			respCh <- &respMint{
				xerr: xerrors.From(fmt.Errorf("critical error: calculated additional issuance amount must not be negative (got %v)", addedSupply)),
			}
			continue
		}

		rwdToVals := addedSupply.Mul(valRate).Floor()
		rwdToAll := addedSupply.Sub(rwdToVals)

		//
		// 2. calculate rewards ...

		beneficiaries := retWeight.Beneficiaries()
		rewards := make([]*mintedReward, len(beneficiaries))
		sumMintedAmt := uint256.NewInt(0)
		{
			remainder := decimal.Zero
			precision := int32(6)

			for i, benef := range beneficiaries {
				wi := benef.Weight() //.Truncate(precision) // Truncate is too expensive.

				// for all delegators
				rwd, _ := rwdToAll.Mul(wi).QuoRem(waAll, precision)
				// for only validators
				if benef.IsValidator() {
					_rwd, _ := rwdToVals.Mul(wi).QuoRem(waVals, precision)
					rwd = rwd.Add(_rwd)
				}

				//give `rwd` + `remainder` to `benef.Address()``
				rwd = rwd.Add(remainder)

				// Apply `benef.singW` to `rwd`
				rwd = rwd.Mul(benef.SignRate())

				rewards[i] = &mintedReward{
					addr: benef.Address(),
					amt:  uint256.MustFromBig(rwd.BigInt()),
				}

				_ = sumMintedAmt.Add(sumMintedAmt, rewards[i].amt)

				remainder = rwd.Sub(rwd.Floor())
			}
		}

		// sumMintedAmt should be equal to addedSupply.
		respCh <- &respMint{
			xerr:         nil,
			sumMintedAmt: sumMintedAmt,
			rewards:      rewards,
		}
	}
}

// Si returns the total supply amount determined by the issuance formula of block 'height'.
func Si(height, adjustedHeight int64, adjustedSupply, smax *uint256.Int, lambda int32, wa decimal.Decimal) decimal.Decimal {
	if height < adjustedHeight {
		panic("the height should be greater than the adjusted height ")
	}
	_lambda := decimal.New(int64(lambda), -3)
	decLambdaAddOne := _lambda.Add(decimal.New(1, 0))
	expWHid := wa.Mul(H(height-adjustedHeight, 1)) // todo: change block interval

	numer := decimal.NewFromBigInt(new(uint256.Int).Sub(smax, adjustedSupply).ToBig(), 0)
	denom := decLambdaAddOne.Pow(expWHid)

	decSmax := decimal.NewFromBigInt(smax.ToBig(), 0)
	return decSmax.Sub(numer.Div(denom))
}

// H returns the normalized block time corresponding to the given block height.
// It calculates how far along the blockchain is relative to a predefined reference period.
// For example, if the reference period is one year, a return value of 1.0 indicates that
// exactly one reference period has elapsed.

var oneYearSeconds int64 = 31_536_000

func H(height, blockIntvSec int64) decimal.Decimal {
	return decimal.NewFromInt(height).Mul(decimal.NewFromInt(blockIntvSec)).Div(decimal.NewFromInt(oneYearSeconds))
}
