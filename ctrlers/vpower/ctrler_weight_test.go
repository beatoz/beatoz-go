package vpower

import (
	"fmt"
	"github.com/beatoz/beatoz-go/ctrlers/mocks"
	"github.com/beatoz/beatoz-go/libs/fxnum"
	"github.com/stretchr/testify/require"
	"math/rand"
	"os"
	"testing"
)

func Test_VPowerCtrler_ComputeWeight(t *testing.T) {
	require.NoError(t, os.RemoveAll(config.RootDir))

	ctrler, lastValUps0, valWallets0, xerr := initLedger(config)
	require.NoError(t, xerr)
	require.Equal(t, len(lastValUps0), len(valWallets0))

	_ = mocks.InitBlockCtxWith(config.ChainID, 1, govMock, acctMock, nil, nil, ctrler)
	require.NoError(t, mocks.DoBeginBlock(ctrler))
	require.NoError(t, mocks.DoEndBlockAndCommit(ctrler))

	//var fromWals0, toWals0 []*web3.Wallet
	var powChunks0 []*PowerChunkProto
	for _, val := range lastValUps0 {
		//fromWals0 = append(fromWals0, valWallets0[i])
		//toWals0 = append(toWals0, valWallets0[i])
		powChunks0 = append(powChunks0, &PowerChunkProto{
			Power:  val.Power,
			Height: 1,
		})
	}
	for h := int64(2); h <= govMock.RipeningBlocks()*2; h += govMock.InflationCycleBlocks() {
		for i := mocks.CurrBlockCtx().Height(); i < h; i++ {
			require.NoError(t, mocks.DoCommit(ctrler))
		}

		// processing random staking(delegating) txs
		cnt := rand.Intn(100)
		if cnt > 0 {
			froms, tos, powers, txhashes := testRandDelegate(t, cnt, ctrler, valWallets0, h)
			require.Equal(t, len(froms), len(tos), "len(froms)", len(froms), "len(tos)", len(tos))
			require.Equal(t, len(froms), len(powers), "len(froms)", len(froms), "len(powers)", len(powers))
			require.Equal(t, len(froms), len(txhashes), "len(froms)", len(froms), "len(txhashes)", len(txhashes))
			for i, pow := range powers {
				powChunks0 = append(powChunks0, &PowerChunkProto{
					Power:  pow,
					Height: h,
					TxHash: txhashes[i],
				})
				//fmt.Println("new tx", bytes.HexBytes(txhashes[i]))
			}
			//fromWals0 = append(fromWals0, froms...)
			//toWals0 = append(toWals0, tos...)
		}

		require.NoError(t, mocks.DoEndBlockAndCommit(ctrler))

		// compute weight
		// fxnumWeightOfPowerChunks
		w_waex64pc := fxnumWeightOfPowerChunks(powChunks0, h, govMock.RipeningBlocks(), govMock.BondingBlocksWeightPermil(), totalSupply)
		//fmt.Println("--- decimalWeightOfPowerChunks return", w_waex64pc)
		w_waex64pc = w_waex64pc.Truncate(fxnum.GetWantPrecision())

		// ComputeWeight
		weightComputed, xerr := ctrler.ComputeWeight(
			h,
			govMock.InflationCycleBlocks(),
			govMock.RipeningBlocks(),
			govMock.BondingBlocksWeightPermil(),
			totalSupply)
		require.NoError(t, xerr)

		expectedSumWeights, expectedValsSumWeights := fxnum.ZERO, fxnum.ZERO
		for _, benef := range weightComputed.Beneficiaries() {
			expectedSumWeights = expectedSumWeights.Add(benef.Weight())
			if benef.IsValidator() {
				expectedValsSumWeights = expectedValsSumWeights.Add(benef.Weight())
			}
		}
		require.Equal(t, expectedSumWeights, weightComputed.SumWeight(), expectedSumWeights.String(), weightComputed.SumWeight().String())
		require.Equal(t, expectedValsSumWeights, weightComputed.ValWeight(), expectedValsSumWeights.String(), weightComputed.ValWeight().String())

		//fmt.Println("--- ComputeWeight return", weightComputed.SumWeight())
		w_computed := weightComputed.SumWeight().Truncate(fxnum.GetWantPrecision())

		sumIndW := fxnum.New(0, 0)
		for _, b := range weightComputed.Beneficiaries() {
			sumIndW = sumIndW.Add(b.Weight())
		}
		sumIndW = sumIndW.Truncate(fxnum.GetWantPrecision())

		require.True(t, w_waex64pc.LessThanOrEqual(fxnum.ONE), "fxnumWeightOfPowerChunks", w_waex64pc, "height", h)
		require.True(t, w_computed.LessThanOrEqual(fxnum.ONE), "ComputeWeight", w_computed, "height", h)
		require.Equal(t, w_computed.String(), sumIndW.String())
		require.Equal(t, w_waex64pc.String(), w_computed.String(), fmt.Sprintf("fxnumWeightOfPowerChunks:%v, ComputeWeight:%v, height:%v", w_waex64pc, w_computed, h))

		//fmt.Printf("Block[%v] the %v delegate txs are executed and the weight is %v <> %v\n", h, cnt, w_waex64pc, w_computed)
	}
	require.NoError(t, ctrler.Close())
	require.NoError(t, os.RemoveAll(config.DBDir()))
}
