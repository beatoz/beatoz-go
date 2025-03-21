package stake

import (
	"github.com/holiman/uint256"
	"github.com/shopspring/decimal"
	"sync"
)

var (
	decimalOne            = decimal.New(1, 0)
	oneYearSeconds  int64 = 31_536_000
	oneWeeksSeconds int64 = 604_800
	twoWeeksSeconds int64 = oneWeeksSeconds * 2

	powerRipeningCycle = oneYearSeconds
)

type Supplier struct {
	lastTotalSupply    decimal.Decimal
	lastAdjustedSupply decimal.Decimal
	lastAdjustedHeight int64

	//
	//maxSupply  decimal.Decimal
	//timeWeight decimal.Decimal
	//baseWeight decimal.Decimal
	mtx sync.RWMutex
}

func NewSupplier() *Supplier {
	return &Supplier{}
}

func (supplier *Supplier) SetLastAdjustedSupply(v *uint256.Int) {
	supplier.mtx.Lock()
	defer supplier.mtx.Unlock()

	supplier.lastAdjustedSupply = decimal.NewFromBigInt(v.ToBig(), 0)
}

func (supplier *Supplier) SetLastAdjustedHeight(h int64) {
	supplier.mtx.Lock()
	defer supplier.mtx.Unlock()

	supplier.lastAdjustedHeight = h
}

// Issue returns the additional issued amount at the block height.
func (supplier *Supplier) Issue(height int64, stakes []*Stake, durPermil int) *uint256.Int {
	// current voting power weight
	var vpows, vpdurs []int64
	for _, s := range stakes {
		vpows = append(vpows, s.Power)
		vpdurs = append(vpdurs, height-s.StartHeight)
	}

	// todo: Compute voting power weight `W` from `stakes`.

	// todo: Compute total supply `totalSupply` at `height` and subtract `supplier.lastTotalSupply` from it.

	return nil
}

// Sd returns the additional issuance at block `height`.
// It is computed as S(i) - S(i-C).
//
// DEPRECATED:
// The `adjustHeight` and `adjustedSupply` may be changed between block `i-C` and block `i`
// and these at block `i-C` are not known at block `i`.
// Because these at block `i-C` are not known, S(i-C) could not be accurately computed.
func Sd(height, inflationCycle, adjustedHeight int64, adjustedSupply, smax *uint256.Int, lambda string, wa, preWa decimal.Decimal) *uint256.Int {
	if height < inflationCycle {
		return uint256.NewInt(0)
	}
	si := Si(height, adjustedHeight, adjustedSupply, smax, lambda, wa)
	siC := Si(height-inflationCycle, adjustedHeight, adjustedSupply, smax, lambda, preWa)
	return new(uint256.Int).Sub(si, siC)
}

// Si returns the total supply amount determined by the issuance formula of block 'height'.
func Si(height, adjustedHeight int64, sadjusted, smax *uint256.Int, lambda string, wa decimal.Decimal) *uint256.Int {
	if height < adjustedHeight {
		panic("the height should be greater than the adjusted height ")
	}
	decLambdaAddedOne := decimal.RequireFromString(lambda).Add(decimalOne)
	expWHid := wa.Mul(H(height-adjustedHeight, 1))

	numer := decimal.NewFromBigInt(new(uint256.Int).Sub(smax, sadjusted).ToBig(), 0)
	denom := decLambdaAddedOne.Pow(expWHid)

	decSmax := decimal.NewFromBigInt(smax.ToBig(), 0)
	return uint256.MustFromBig(decSmax.Sub(numer.Div(denom)).BigInt())
}
