package stake

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

	btz, _ := types.FromFons(totalSupply)
	w0 := votingPowerObj{
		vpow: int64(btz),
		vdur: 0,
	}
	Wall := decimal.Zero

	sumAdded := uint256.NewInt(0)
	inflationCyle := twoWeeksSeconds
	for i := inflationCyle + adjustedHeight; i <= 100*oneYearSeconds; i += inflationCyle {
		preWall := Wall

		w0.vdur = i
		Wall = Wi(w0.vpow, w0.vdur, decimal.NewFromBigInt(totalSupply.ToBig(), 0), 200).Truncate(3)
		require.True(t, Wall.LessThanOrEqual(decimalOne))

		sd := Sd(i, inflationCyle, adjustedHeight, adjustedSupply, maxSupply, "0.3", Wall, preWall)
		if i == 0 {
			require.Equal(t, uint256.NewInt(0), sd)
		}

		sumAdded = new(uint256.Int).Add(sumAdded, sd)
		totalSupply = new(uint256.Int).Add(totalSupply, sd)
		require.True(t, totalSupply.Cmp(maxSupply) <= 0, totalSupply)

		_added, _ := types.FromFons(sd)
		_totsup, _ := types.FromFons(totalSupply)
		_sumadded, _ := types.FromFons(sumAdded)
		fmt.Println("year", 1+i/oneYearSeconds, "week", i/oneWeeksSeconds, "W", Wall, "added", _added, "cumu", _sumadded, "total", _totsup)

		//// deflation
		//if i%(oneYearSeconds*10) == 0 {
		//	adjustedHeight = i
		//	decay := new(uint256.Int).Div(totalSupply, uint256.NewInt(10))
		//	totalSupply.Sub(totalSupply, decay)
		//	adjustedSupply.Set(totalSupply)
		//}
	}
}
