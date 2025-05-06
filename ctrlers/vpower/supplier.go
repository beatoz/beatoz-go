package vpower

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

func NewSupplierWith(lastTotalSupply, lastAdjustedSupply *uint256.Int, lastAdjustedHeight int64) *Supplier {
	return &Supplier{
		lastTotalSupply:    decimal.NewFromBigInt(lastTotalSupply.ToBig(), 0),
		lastAdjustedSupply: decimal.NewFromBigInt(lastAdjustedSupply.ToBig(), 0),
		lastAdjustedHeight: lastAdjustedHeight,
	}
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
func (supplier *Supplier) Issue(height int64, vpows []*VPower, durPermil int) *uint256.Int {
	// todo: Compute voting power weight `W` from `stakes`.

	// todo: Compute total supply `totalSupply` at `height` and subtract `supplier.lastTotalSupply` from it.

	return nil
}

// Sd returns the additional issuance at block `height`.
// It is computed as S(i) - S(i-C).
//
// DEPRECATED:
// The `adjustedHeight` and `adjustedSupply` may be changed between `block[height - inflationCycle]` and `block[height]`.
// The following `adjustedHeight` and `adjustedSupply` are valid at `block[height]` but not at `block[height - inflationCycle]`.
// Because of that, Si(height - inflationCycle, adjustedHeight, adjustedSupply,...) can not accurately be computed.
// Sd can be accurately computed only when the adjustedXXX values did not change
// between `block[height - inflationCycle]` and `block[height]`.
func Sd(height, inflationCycle, adjustedHeight int64, adjustedSupply, smax *uint256.Int, lambda string, wa, preWa decimal.Decimal) *uint256.Int {
	if height < inflationCycle {
		return uint256.NewInt(0)
	}
	si := Si(height, adjustedHeight, adjustedSupply, smax, lambda, wa)
	siC := Si(height-inflationCycle, adjustedHeight, adjustedSupply, smax, lambda, preWa)
	return new(uint256.Int).Sub(si, siC)
}

// Si returns the total supply amount determined by the issuance formula of block 'height'.
func Si(height, adjustedHeight int64, adjustedSupply, smax *uint256.Int, lambda string, wa decimal.Decimal) *uint256.Int {
	if height < adjustedHeight {
		panic("the height should be greater than the adjusted height ")
	}
	decLambdaAddOne := decimal.RequireFromString(lambda).Add(decimalOne)
	expWHid := wa.Mul(H(height-adjustedHeight, 1))

	numer := decimal.NewFromBigInt(new(uint256.Int).Sub(smax, adjustedSupply).ToBig(), 0)
	denom := decLambdaAddOne.Pow(expWHid)

	decSmax := decimal.NewFromBigInt(smax.ToBig(), 0)
	return uint256.MustFromBig(decSmax.Sub(numer.Div(denom)).BigInt())
}
