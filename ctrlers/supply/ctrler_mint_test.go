package supply

import (
	"fmt"
	vpowmock "github.com/beatoz/beatoz-go/ctrlers/mocks/vpower"
	"github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
	"time"
)

func Test_Mint(t *testing.T) {
	require.NoError(t, os.RemoveAll(config.RootDir))

	initSupply := types.PowerToAmount(350_000_000)
	adjustedSupply := initSupply.Clone()
	adjustedHeight := int64(1)
	ctrler, xerr := initLedger(initSupply)
	require.NoError(t, xerr)

	//
	// Use VPowerHandlerMock
	valsCnt := min(acctMock.WalletLen(), 10)
	valWals := make([]*web3.Wallet, valsCnt)
	for i := 0; i < valsCnt; i++ {
		valWals[i] = acctMock.GetWallet(i)
	}
	vpowMock := vpowmock.NewVPowerHandlerMock(valWals, len(valWals))
	totalSupply := initSupply.Clone()
	changeSupply := uint256.NewInt(0)
	fmt.Println("Test Mint using VPowerHandlerMock", "validator number", valsCnt, "total power", vpowMock.GetTotalPower())

	////
	////Use VPowerCtrler
	//fmt.Println("Test using VPowerCtrler")
	//vpowMock, xerr := vpower.NewVPowerCtrler(config, int(govMock.MaxValidatorCnt()), log.NewNopLogger())
	//require.NoError(t, xerr)
	//
	//wal := acctMock.RandWallet()
	//dgtee := vpower.NewDelegatee(wal.GetPubKey())
	//
	//vpow := vpower.NewVPower(dgtee.Address(), dgtee.Address()) // self power
	//xerr = vpowMock.BondPowerChunk(dgtee, vpow, 70_000_000, 1, bytes.RandBytes(32), true)
	//require.NoError(t, xerr)
	//
	//height0 := govMock.InflationCycleBlocks()
	//bctx := types.TempBlockContext("mint-test-chain", height0, time.Now(), govMock, acctMock, nil, nil, vpowMock)
	//
	//// before vpowCtrler.EndBlock. (vpowCtrler.lastValidators is nil)
	//// expect 0 minting
	//ctrler.requestMint(bctx)
	//result, xerr := ctrler.waitMint(bctx)
	//require.NoError(t, xerr)
	//supplyHeight := result.newSupply.Height
	//totalSupply := new(uint256.Int).SetBytes(result.newSupply.XSupply)
	//changeSupply := new(uint256.Int).SetBytes(result.newSupply.XChange)
	//
	//require.Equal(t, height0, supplyHeight)
	//require.Equal(t, initSupply.String(), totalSupply.String())
	//require.Equal(t, "0", changeSupply.String())
	//
	//_, xerr = vpowMock.EndBlock(bctx)
	//require.NoError(t, xerr)
	//// End of Use VPowerCtrler
	////

	preRewards := make(map[string]*uint256.Int)
	for currHeight := govMock.InflationCycleBlocks(); /*int64(2)*/ currHeight < govMock.InflationCycleBlocks()*1000; currHeight += govMock.InflationCycleBlocks() {
		// expect x minting
		weightInfo, xerr := vpowMock.ComputeWeight(
			currHeight,
			govMock.InflationCycleBlocks(),
			govMock.RipeningBlocks(),
			govMock.BondingBlocksWeightPermil(),
			totalSupply)
		require.NoError(t, xerr)

		wa := weightInfo.SumWeight() //.Truncate(precision)
		//wa := vpower.WaEx64ByPowerChunk(vpowMock.PowerChunks, currHeight, govMock.RipeningBlocks(), govMock.BondingBlocksWeightPermil(), totalSupply)
		//wa = wa.Truncate(precision)

		si := Si(currHeight, int64(govMock.AssumedBlockInterval()), adjustedHeight, adjustedSupply, govMock.MaxTotalSupply(), govMock.InflationWeightPermil(), wa).Floor()
		expectedTotalSupply := uint256.MustFromBig(si.BigInt())
		expectedChange := new(uint256.Int).Sub(expectedTotalSupply, totalSupply)
		//fmt.Println("expected", "height", currHeight, "wa", wa.String(), "adjustedSupply", adjustedSupply, "adjustedHeight", 1, "max", govMock.MaxTotalSupply(), "lamda", govMock.InflationWeightPermil(), "total", expectedTotalSupply, "pre.total", totalSupply, "change", expectedChange)

		bctx := types.TempBlockContext("mint-test-chain", currHeight, time.Now(), govMock, acctMock, nil, nil, vpowMock)
		ctrler.requestMint(bctx)
		result, xerr := ctrler.waitMint(bctx)
		require.NoError(t, xerr)
		totalSupply = new(uint256.Int).Add(totalSupply, result.sumMintedAmt)
		changeSupply = result.sumMintedAmt.Clone()

		require.NotEqual(t, "0", changeSupply.Dec())
		require.NotEqual(t, expectedTotalSupply.Dec(), initSupply.Dec())
		changeDiff := absDiff(expectedChange, changeSupply)
		//require.Equal(t, expectedChange.Dec(), changeSupply.Dec())
		require.LessOrEqual(t, changeDiff.Uint64(), uint64(1), "height", currHeight, "changeDiff", changeDiff.Dec())
		supplyDiff := absDiff(expectedTotalSupply, totalSupply)
		//require.Equal(t, expectedTotalSupply.Dec(), totalSupply.Dec(), "height", currHeight)
		require.LessOrEqual(t, supplyDiff.Uint64(), uint64(1), "height", currHeight, "supplyDiff", supplyDiff.Dec())

		sumMint := uint256.NewInt(0)
		for _, mintRwd := range result.rewards {
			_ = sumMint.Add(sumMint, mintRwd.amt)

			//
			// check reward amount of beneficiary
			accumRwd, xerr := ctrler.readReward(mintRwd.addr)
			require.NoError(t, xerr)
			require.Equal(t, currHeight, accumRwd.Height())
			require.Equal(t, mintRwd.amt.Dec(), accumRwd.issued.Dec())

			_preAmt, ok := preRewards[mintRwd.addr.String()]
			if !ok {
				preRewards[mintRwd.addr.String()] = accumRwd.CumulatedAmount()
			} else {
				require.Equal(t, _preAmt.Add(_preAmt, mintRwd.amt).Dec(), accumRwd.CumulatedAmount().Dec())
				preRewards[mintRwd.addr.String()] = _preAmt
			}
		}

		delta := new(uint256.Int).Sub(expectedChange, sumMint)
		require.LessOrEqual(t, new(uint256.Int).Abs(delta).Uint64(), uint64(1), delta)

		//fmt.Println("---")
		//fmt.Println("height", currHeight, "totalSupply", totalSupply.Dec(), "changeSupply", changeSupply.Dec())
		//for _, rwd := range result.rewards {
		//	fmt.Println("height", currHeight, "beneficary", rwd.addr, "reward", rwd.amt.Dec())
		//}
	}

	require.NoError(t, ctrler.Close())
	require.NoError(t, os.RemoveAll(config.RootDir))
}

func absDiff(x, y *uint256.Int) *uint256.Int {
	result := new(uint256.Int)
	switch x.Cmp(y) {
	case -1: // x < y
		result.Sub(y, x)
	case 0: // x == y
		result.SetUint64(0)
	case 1: // x > y
		result.Sub(x, y)
	}
	return result
}
