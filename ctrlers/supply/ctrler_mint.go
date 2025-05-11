package supply

import (
	"github.com/holiman/uint256"
	"github.com/shopspring/decimal"
)

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
			bctx.GovParams.RipeningBlocks(),
			bctx.GovParams.BondingBlocksWeightPermil(),
			lastTotalSupply,
		)
		if xerr != nil {
			respCh <- &respMint{
				xerr:      xerr,
				newSupply: nil,
			}
			continue
		}

		wa := retWeight.SumWeight().Truncate(6)
		totalSupply := Si(bctx.Height(), lastAdjustedHeight, lastAdjustedSupply, bctx.GovParams.MaxTotalSupply(), bctx.GovParams.InflationWeightPermil(), wa)
		addedSupply := totalSupply.Sub(decimal.NewFromBigInt(lastTotalSupply.ToBig(), 0))

		valRate := decimal.NewFromInt(int64(bctx.GovParams.ValidatorRewardRate())).Div(decimal.NewFromInt(100))
		mintedVals := addedSupply.Mul(valRate)
		mintedAlls := addedSupply.Sub(mintedVals)

		//
		// 2. calculate rewards ...
		rewards := calculateRewards(retWeight, mintedAlls, mintedVals)

		respCh <- &respMint{
			xerr: nil,
			newSupply: &Supply{
				SupplyProto: SupplyProto{
					Height:  bctx.Height(),
					XSupply: uint256.MustFromBig(totalSupply.BigInt()).Bytes(),
					XChange: uint256.MustFromBig(addedSupply.BigInt()).Bytes(),
				},
			},
			rewards: rewards,
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
	expWHid := wa.Mul(H(height-adjustedHeight, 1))

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
