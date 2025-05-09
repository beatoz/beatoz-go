package vpower

import (
	"fmt"
	"github.com/beatoz/beatoz-go/ctrlers/mocks"
	"github.com/beatoz/beatoz-go/types"
	"github.com/stretchr/testify/require"
	"math/rand"
	"os"
	"testing"
)

func Test_VPowerCtrler_ComputeWeight(t *testing.T) {
	require.NoError(t, os.RemoveAll(config.RootDir))

	totalSupply := types.ToFons(uint64(350_000_000))

	ctrler, lastValUps0, valWallets0, xerr := initLedger(config)
	require.NoError(t, xerr)
	require.Equal(t, len(lastValUps0), len(valWallets0))

	_ = mocks.InitBlockCtxWith(1, acctMock, govParams, ctrler)
	require.NoError(t, mocks.DoBeginBlock(ctrler))
	require.NoError(t, mocks.DoEndBlockCommit(ctrler))

	var powChunks0 []*PowerChunkProto
	for _, val := range lastValUps0 {
		powChunks0 = append(powChunks0, &PowerChunkProto{
			Power:  val.Power,
			Height: 1,
		})
	}
	for h := int64(2); h <= govParams.RipeningBlocks()*10; h += govParams.InflationCycleBlocks() {

		require.NoError(t, mocks.DoBeginBlock(ctrler))

		cnt := rand.Intn(10) + 21
		if cnt > 0 {
			_, _, powers, _ := testRandDelegate(t, cnt, ctrler, valWallets0, h)
			for _, pow := range powers {
				powChunks0 = append(powChunks0, &PowerChunkProto{
					Power:  pow,
					Height: h,
				})
			}
		}

		require.NoError(t, mocks.DoEndBlockCommit(ctrler))

		// compute weight
		// WaEx64ByPowerChunks
		w_waex64pc := WaEx64ByPowerChunk(powChunks0, h, powerRipeningCycle, govParams.BondingBlocksWeightPermil(), totalSupply)
		w_waex64pc = w_waex64pc.Truncate(6)
		//fmt.Println("WaEx64ByPowerChunk return", w_waex64pc)

		// ComputeWeight
		w_computed, xerr := ctrler.ComputeWeight(h, govParams.RipeningBlocks(), govParams.BondingBlocksWeightPermil(), totalSupply)
		require.NoError(t, xerr)
		w_computed = w_computed.Truncate(6)

		require.True(t, w_waex64pc.LessThanOrEqual(decimalOne), "WaEx64ByPowerChunks", w_waex64pc, "height", h)
		require.True(t, w_computed.LessThanOrEqual(decimalOne), "ComputeWeight", w_computed, "height", h)
		require.True(t, w_waex64pc.Equal(w_computed), fmt.Sprintf("WaEx64ByPowerChunks:%v, ComputeWeight:%v, height:%v", w_waex64pc, w_computed, h))

		//fmt.Printf("Block[%v] the %v delegate txs are executed and the weight is %v <> %v\n", h, cnt, w_waex64pc, w_computed)
	}

}
