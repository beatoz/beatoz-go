package supply

import (
	"github.com/beatoz/beatoz-go/types/xerrors"
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
		if !wa.Equal(retWeight.SumWeight().Truncate(6)) {
			respCh <- &respMint{
				xerr: xerrors.ErrInvalidWeight,
			}
			continue
		}
		valsW := retWeight.ValWeight().Truncate(6)

		//{
		//	//
		//	// for debugging
		//	//
		//	sumWi := decimal.Zero
		//	for _, wi := range wis {
		//		sumWi = sumWi.Add(wi)
		//	}
		//	sumWi = sumWi.Truncate(6)
		//	if !sumWi.Equal(wa) {
		//		panic(fmt.Errorf("weight has error - wa:%v, sumOfWi:%v", wa, sumWi))
		//	}
		//}

		si := Si(bctx.Height(), lastAdjustedHeight, lastAdjustedSupply, bctx.GovParams.MaxTotalSupply(), bctx.GovParams.InflationWeightPermil(), wa)
		sd := si.Sub(decimal.NewFromBigInt(lastTotalSupply.ToBig(), 0))

		valRate := decimal.NewFromInt(int64(bctx.GovParams.ValidatorRewardRate())).Div(decimal.NewFromInt(100))
		sd_vals := sd.Mul(valRate)
		sd_comm := sd.Sub(sd_vals)

		//fmt.Println("compute", "wa", wa.String(), "adjustedSupply", lastAdjustedSupply, "adjustedHeight", lastAdjustedHeight, "max", bctx.GovParams.MaxTotalSupply(), "lamda", bctx.GovParams.InflationWeightPermil(), "t1", totalSupply, "t0", lastTotalSupply)

		//
		// 2. calculate rewards ...
		// 2.1. for validators
		beneficiaries := retWeight.Beneficiaries()
		rewards := make([]*Reward, len(beneficiaries))
		for i, benef := range retWeight.Beneficiaries() {
			wi := benef.Weight().Truncate(6)
			rwd := sd_comm.Mul(wi).Div(wa)
			if benef.IsValidator() {
				rwd = rwd.Add(sd_vals.Mul(wi).Div(valsW))
			}
			// give `rd` to `b`
			rewards[i] = &Reward{
				addr: benef.Address(),
				amt:  uint256.MustFromBig(rwd.BigInt()),
			}
		}

		respCh <- &respMint{
			xerr: nil,
			newSupply: &Supply{
				SupplyProto: SupplyProto{
					Height:  bctx.Height(),
					XSupply: uint256.MustFromBig(si.BigInt()).Bytes(),
					XChange: uint256.MustFromBig(sd.BigInt()).Bytes(),
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
