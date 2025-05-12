package vpower

import (
	"fmt"
	"github.com/beatoz/beatoz-go/ctrlers/mocks"
	"github.com/beatoz/beatoz-go/types"
	"github.com/shopspring/decimal"
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

	_ = mocks.InitBlockCtxWith(1, govMock, acctMock, nil, nil, ctrler)
	require.NoError(t, mocks.DoBeginBlock(ctrler))
	require.NoError(t, mocks.DoEndBlockCommit(ctrler))

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
	for h := int64(2); h <= govMock.RipeningBlocks()*5; h += govMock.InflationCycleBlocks() {

		require.NoError(t, mocks.DoBeginBlock(ctrler))

		cnt := rand.Intn(10) + 21
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

		require.NoError(t, mocks.DoEndBlockCommit(ctrler))

		// compute weight
		// WaEx64ByPowerChunks
		w_waex64pc := WaEx64ByPowerChunk(powChunks0, h, govMock.RipeningBlocks(), govMock.BondingBlocksWeightPermil(), totalSupply)
		//fmt.Println("--- WaEx64ByPowerChunk return", w_waex64pc)
		w_waex64pc = w_waex64pc.Truncate(6)

		// ComputeWeight
		weightComputed, xerr := ctrler.ComputeWeight(h, govMock.RipeningBlocks(), govMock.BondingBlocksWeightPermil(), totalSupply)
		require.NoError(t, xerr)
		//fmt.Println("--- ComputeWeight return", weightComputed.SumWeight())
		w_computed := weightComputed.SumWeight().Truncate(6)

		sumIndW := decimal.Zero
		for _, b := range weightComputed.Beneficiaries() {
			sumIndW = sumIndW.Add(b.Weight())
		}
		sumIndW = sumIndW.Truncate(6)

		require.Equal(t, w_computed.String(), sumIndW.String())
		require.True(t, w_waex64pc.LessThanOrEqual(decimalOne), "WaEx64ByPowerChunks", w_waex64pc, "height", h)
		require.True(t, w_computed.LessThanOrEqual(decimalOne), "ComputeWeight", w_computed, "height", h)
		////
		//if w_waex64pc.String() != w_computed.String() {
		//	fmt.Println("--- WaEx64ByPowerChunk return", w_waex64pc)
		//	for i, fw := range fromWals0 {
		//		tw := toWals0[i]
		//		pc := powChunks0[i]
		//		fmt.Println("currHeight", h, "from", fw.Address(), "to", tw.Address(), "power", pc.Power, "txhash", bytes.HexBytes(pc.TxHash))
		//	}
		//	fmt.Println("WaEx64ByPowerChunk power count", len(powChunks0))
		//	fmt.Println("--- ComputeWeight return", w_computed)
		//	_cnt := 0
		//	_sumW := decimal.Zero
		//	for _, val := range ctrler.lastValidators {
		//		for _, from := range val.Delegators {
		//			vpow, xerr := ctrler.readVPower(from, val.addr, true)
		//			require.NoError(t, xerr)
		//			for _, pc := range vpow.PowerChunks {
		//				fmt.Println("currHeight", h, "from", bytes.HexBytes(vpow.From), "to", vpow.to, "power", pc.Power, "txhash", bytes.HexBytes(pc.TxHash))
		//				_cnt++
		//
		//				_w := WaEx64ByPowerChunk(vpow.PowerChunks, h, govParams.RipeningBlocks(), govParams.BondingBlocksWeightPermil(), totalSupply)
		//				_sumW = _sumW.Add(_w)
		//			}
		//		}
		//	}
		//	fmt.Println("ComputeWeight power count", _cnt, "calculated weight", _sumW.String())
		//}
		require.Equal(t, w_waex64pc.String(), w_computed.String(), fmt.Sprintf("WaEx64ByPowerChunks:%v, ComputeWeight:%v, height:%v", w_waex64pc, w_computed, h))

		//fmt.Printf("Block[%v] the %v delegate txs are executed and the weight is %v <> %v\n", h, cnt, w_waex64pc, w_computed)
	}

}
