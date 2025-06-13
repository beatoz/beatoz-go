package supply

import (
	"fmt"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/libs/fxnum"
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
		lastAdjustedHeight := req.lastAdjustedHeight

		// 1. compute voting power weight
		retWeight, xerr := bctx.VPowerHandler.ComputeWeight(
			bctx.Height(),
			bctx.GovHandler.InflationCycleBlocks(),
			bctx.GovHandler.RipeningBlocks(),
			bctx.GovHandler.BondingBlocksWeightPermil(),
			lastTotalSupply, // todo: Decide whether to use `lastTotalSupply` or the `lastAdjustedSupply`.
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

		addedSupply := Sd(
			heightYears(bctx.Height()-lastAdjustedHeight, bctx.GovHandler.AssumedBlockInterval()),
			lastTotalSupply,
			bctx.GovHandler.MaxTotalSupply(), bctx.GovHandler.InflationWeightPermil(),
			waAll,
		)
		if addedSupply.Sign() < 0 {
			respCh <- &respMint{
				xerr: xerrors.From(fmt.Errorf("critical error: calculated additional issuance amount must not be negative (got %v)", addedSupply)),
			}
			continue
		}

		decWaAll, _ := waAll.ToDecimal()
		decWaVals, _ := waVals.ToDecimal()
		rwdToVals := addedSupply.Mul(valRate).Floor()
		rwdToAll := addedSupply.Sub(rwdToVals)

		//
		// 2. calculate rewards ...

		beneficiaries := retWeight.Beneficiaries()
		rewards := make([]*mintedReward, len(beneficiaries))
		sumMintedAmt := uint256.NewInt(0)
		{
			remainder := decimal.Zero

			for i, benef := range beneficiaries {
				//.Truncate(precision) // Truncate is too expensive.
				decWi, _ := benef.Weight().ToDecimal()

				// for all delegators
				rwd, _ := rwdToAll.Mul(decWi).QuoRem(decWaAll, int32(decimal.DivisionPrecision))
				// for only validators
				if benef.IsValidator() {
					_rwd, _ := rwdToVals.Mul(decWi).QuoRem(decWaVals, int32(decimal.DivisionPrecision))
					rwd = rwd.Add(_rwd)
				}

				//give `rwd` + `remainder` to `benef.Address()``
				rwd = rwd.Add(remainder)

				// Apply `benef.singW` to `rwd`
				sw, _ := benef.SignRate().ToDecimal()
				rwd = rwd.Mul(sw)

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

// DEPRECATED: adjustHeight returns the estimated height when the total supply was `si`.
// return_height = {(si * YearSeconds) / (vpAmt * blockIntv)} * {ln((smax - preSi)/(smax-si)) / ln(1+lambda)}
func adjustHeight(si, preSi, smax *uint256.Int, vp int64, lambda, blockIntv int32) int64 {
	dLambdaAddOne := decimal.New(int64(lambda), -3)
	dLambdaAddOne = dLambdaAddOne.Add(decimal.NewFromInt(1))
	dsi := decimal.NewFromBigInt(si.ToBig(), 0)
	d0 := decimal.NewFromInt(ctrlertypes.YearSeconds).Mul(dsi)
	d0 = d0.Div(decimal.New(vp, 18).Mul(decimal.NewFromInt(int64(blockIntv))))

	var err error
	dlastSi := decimal.NewFromBigInt(preSi.ToBig(), 0)
	dlog := decimal.NewFromBigInt(smax.ToBig(), 0).Sub(dlastSi)
	dlog = dlog.Div(decimal.NewFromBigInt(smax.ToBig(), 0).Sub(dsi))
	dlog, err = dlog.Ln(int32(decimal.DivisionPrecision))
	if err != nil {
		panic(err)
	}
	dLambdaAddOne, err = dLambdaAddOne.Ln(int32(decimal.DivisionPrecision))
	if err != nil {
		panic(err)
	}
	dlog = dlog.Div(dLambdaAddOne)

	h := d0.Mul(dlog)
	return h.IntPart()
}

// scaledHeight returns the normalized block time corresponding to the given block height.
// (`ret = current_height * block_interval_sec / one_year_seconds`)
// It calculates how far along the blockchain is relative to a predefined reference period.
// For example, if the reference period is one year, a return value of 1.0 indicates that
// exactly one reference period has elapsed.
func scaledHeight(height, base int64) fxnum.FxNum {
	return fxnum.FromInt(height).Div(fxnum.FromInt(base))
}

func heightYears(height int64, intval int32) fxnum.FxNum {
	return scaledHeight(height*int64(intval), ctrlertypes.YearSeconds)
}

func Sd(scaledHeight fxnum.FxNum, lastSupply, smax *uint256.Int, lambda int32, wa fxnum.FxNum) decimal.Decimal {
	return decimalSd(scaledHeight, lastSupply, smax, lambda, wa)
}

func decimalSd(scaledHeight fxnum.FxNum, lastSupply, smax *uint256.Int, lambda int32, wa fxnum.FxNum) decimal.Decimal {
	decLambdaAddOne := decimal.New(int64(lambda), -3)
	decLambdaAddOne = decLambdaAddOne.Add(decimal.New(1, 0))
	decScaledH, _ := scaledHeight.ToDecimal()
	decWa, _ := wa.ToDecimal()
	decExp := decWa.Mul(decScaledH)

	part1 := decimal.NewFromInt(1).Sub(decLambdaAddOne.Pow(decExp.Neg()))
	part0 := decimal.NewFromBigInt(new(uint256.Int).Sub(smax, lastSupply).ToBig(), 0)
	return part0.Mul(part1)
}

// DEPRECATED
func Sd2(scaledHeight fxnum.FxNum, lastSupply, smax *uint256.Int, inflaCycle int64, blockIntv, lambda int32, wa fxnum.FxNum) decimal.Decimal {
	return decimalSd2(scaledHeight, lastSupply, smax, inflaCycle, blockIntv, lambda, wa)
}

// DEPRECATED
func decimalSd2(scaledHeight fxnum.FxNum, sadjusted, smax *uint256.Int, inflaCycle int64, blockIntv, lambda int32, wa fxnum.FxNum) decimal.Decimal {
	decLambdaAddOne := decimal.New(int64(lambda), -3)
	decLambdaAddOne = decLambdaAddOne.Add(decimal.New(1, 0))
	decScaledH, _ := scaledHeight.ToDecimal()
	decScaledC, _ := heightYears(inflaCycle, blockIntv).ToDecimal()
	decWa, _ := wa.ToDecimal()
	decExp0 := decWa.Mul(decScaledH)
	decExp1 := decWa.Mul(decScaledC)

	part0 := decimal.NewFromBigInt(new(uint256.Int).Sub(smax, sadjusted).ToBig(), 0).Div(decLambdaAddOne.Pow(decExp0))
	part1 := decLambdaAddOne.Pow(decExp1).Sub(decimal.NewFromInt(1))
	return part0.Mul(part1)
}

// DEPRECATED
// Si returns the total supply amount determined by the issuance formula of block 'height'.
func Si(scaledHeight fxnum.FxNum, adjustSupply, smax *uint256.Int, lambda int32, wa fxnum.FxNum) decimal.Decimal {
	return decimalSi(scaledHeight, adjustSupply, smax, lambda, wa)
}

// DEPRECATED
func decimalSi(scaledHeight fxnum.FxNum, adjustSupply, smax *uint256.Int, lambda int32, wa fxnum.FxNum) decimal.Decimal {
	_lambda := decimal.New(int64(lambda), -3)
	decLambdaAddOne := _lambda.Add(decimal.New(1, 0))
	decScaledH, _ := scaledHeight.ToDecimal()
	decWa, _ := wa.ToDecimal()
	decNumer := decimal.NewFromBigInt(new(uint256.Int).Sub(smax, adjustSupply).ToBig(), 0)

	decWHid := decWa.Mul(decScaledH)
	decDenom := decLambdaAddOne.Pow(decWHid)
	decSmax := decimal.NewFromBigInt(smax.ToBig(), 0)
	return decSmax.Sub(decNumer.Div(decDenom))
}

// DEPRECATED
func fxnumSi(scaledHeight fxnum.FxNum, adjustSupply, smax *uint256.Int, lambda int32, wa fxnum.FxNum) decimal.Decimal {
	_lambda := fxnum.Permil(int(lambda))
	fxLambdaAddOne := _lambda.Add(fxnum.ONE)

	fxWHid := wa.Mul(scaledHeight)
	fxDenom := fxLambdaAddOne.Pow(fxWHid)

	decDenom, _ := fxDenom.ToDecimal()
	decNumer := decimal.NewFromBigInt(new(uint256.Int).Sub(smax, adjustSupply).ToBig(), 0)
	decSmax := decimal.NewFromBigInt(smax.ToBig(), 0)
	return decSmax.Sub(decNumer.Div(decDenom))
}
