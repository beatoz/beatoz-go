package vpower

import (
	"fmt"
	"github.com/beatoz/beatoz-go/types"
	"github.com/holiman/uint256"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"math/rand/v2"
	"testing"
)

func init() {
	decimal.DivisionPrecision = 9
}

func Test_Si(t *testing.T) {
	maxSupply := types.ToFons(700_000_000)
	adjustedSupply := types.ToFons(350_000_000)
	adjustedHeight := rand.Int64()

	// height < adjustedHeight : expected panic
	height := adjustedHeight - 1
	require.Panics(t, func() { _ = Si(height, adjustedHeight, adjustedSupply, maxSupply, "0.3", decimalOne) })

	// height == adjustedHeight : expected `adjustedSupply`
	height = adjustedHeight
	r := Si(height, adjustedHeight, adjustedSupply, maxSupply, "0.3", decimalOne)
	require.Equal(t, adjustedSupply, r)

	// height > adjustedHeight : expected >`adjustedSupply`
	height = adjustedHeight + 1
	r = Si(height, adjustedHeight, adjustedSupply, maxSupply, "0.3", decimalOne)
	require.True(t, r.Cmp(adjustedSupply) > 0)
}

func Test_AnnualSd(t *testing.T) {
	maxSupply := types.ToFons(700_000_000)
	totalSupply := types.ToFons(350_000_000)
	adjustedSupply := types.ToFons(350_000_000)
	adjustedHeight := int64(1)

	btz := 100_000_000 //types.FromFons(totalSupply)
	w0 := testPowObj{
		vpow: int64(btz),
		vdur: 0,
	}
	Wall := decimal.Zero

	sumAdded := uint256.NewInt(0)
	inflationCyle := oneYearSeconds //twoWeeksSeconds
	for i := inflationCyle + adjustedHeight; i <= 100*oneYearSeconds+1; i += inflationCyle {
		preWall := Wall

		w0.vdur = i
		Wall = Wi(w0.vpow, w0.vdur, powerRipeningCycle, decimal.NewFromBigInt(totalSupply.ToBig(), 0), 200).Truncate(3)
		require.True(t, Wall.LessThanOrEqual(decimalOne))

		sd := Sd(i, inflationCyle, adjustedHeight, adjustedSupply, maxSupply, "0.3", Wall, preWall)
		if i == 0 {
			require.Equal(t, uint256.NewInt(0), sd)
		}

		// reward rate  = 100*fromFons(sd)/w0.vpow
		_sd, _ := types.FromFons(sd)
		rwdRate := decimal.NewFromUint64(_sd).Div(decimal.NewFromInt(w0.vpow))
		inflRate := decimal.NewFromBigInt(sd.ToBig(), 0).Div(decimal.NewFromBigInt(maxSupply.ToBig(), 0))

		sumAdded = new(uint256.Int).Add(sumAdded, sd)
		totalSupply = new(uint256.Int).Add(totalSupply, sd)
		require.True(t, totalSupply.Cmp(maxSupply) <= 0, totalSupply)

		_added, _ := types.FromFons(sd)
		_totsup, _ := types.FromFons(totalSupply)
		_sumadded, _ := types.FromFons(sumAdded)
		//fmt.Println("year", 1+i/oneYearSeconds, "week", i/oneWeeksSeconds, "W", Wall, "added", _added, "cumu", _sumadded, "total", _totsup, "reward rate", rwdRate.Truncate(2))
		fmt.Printf("year: %3v (%4vw), total supply: %9v, added: %9v, sumAdded: %9v, rwdRate: % v, inflRate: %v, W: %v\n",
			i/oneYearSeconds, i/oneWeeksSeconds, _totsup, _added, _sumadded, rwdRate.StringFixed(2), inflRate.StringFixed(2), Wall)

		//// deflation
		//if i%(oneYearSeconds*10) == 0 {
		//	adjustedHeight = i
		//	decay := new(uint256.Int).Div(totalSupply, uint256.NewInt(10))
		//	totalSupply.Sub(totalSupply, decay)
		//	adjustedSupply.Set(totalSupply)
		//}
	}
}
