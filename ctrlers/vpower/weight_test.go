package vpower

import (
	"fmt"
	"github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_Weight(t *testing.T) {
	cases := []*struct {
		powerChunks    []*PowerChunkProto
		atHeight       int64
		expectedWeight string
	}{
		{
			powerChunks: []*PowerChunkProto{
				{Power: 70_000_000, Height: 1},
				{Power: 1_000_000, Height: 10},
				{Power: 1_000_000, Height: 23},
			},
			atHeight:       31,
			expectedWeight: "0.1506577",
		},
		{
			powerChunks: []*PowerChunkProto{
				{Power: 70_000_000, Height: 1},
				{Power: 1_000_000, Height: 10},
				{Power: 1_000_000, Height: 23},
				{Power: 1_000_000, Height: 56},
				{Power: 1_000_000, Height: 78},
				{Power: 1_000_000, Height: 109},
			},
			atHeight:       191,
			expectedWeight: "0.2140903",
		},
		//{
		//	powerChunks: []*PowerChunkProto{
		//		{Power: 1000_000, Height: 1},
		//	},
		//	atHeight:       21,
		//	expectedWeight: "0.0021714",
		//},
		//{
		//	powerChunks: []*PowerChunkProto{
		//		{Power: 1000_000, Height: 1},
		//	},
		//	atHeight:       31,
		//	expectedWeight: "0.0022571",
		//},
	}

	ripeningBlock := int64(100)
	tau := int32(380)
	_totalSupply := types.PowerToAmount(int64(350_000_000))
	for _, c := range cases {
		w := fxnumWeightOfPowerChunks(c.powerChunks, c.atHeight, ripeningBlock, tau, _totalSupply)
		require.Equal(t, c.expectedWeight, w.String(), fmt.Sprintf("expected:%v, actual:%v\n", c.expectedWeight, w.String()))
	}
}
