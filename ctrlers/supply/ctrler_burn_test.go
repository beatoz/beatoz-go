package supply

import (
	"fmt"
	"github.com/beatoz/beatoz-go/ctrlers/mocks"
	vpowmock "github.com/beatoz/beatoz-go/ctrlers/mocks/vpower"
	"github.com/beatoz/beatoz-go/ctrlers/types"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	btztypes "github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

// todo: test when the burning occurs multiple times in a block.
func Test_TxFeeProcessing(t *testing.T) {
	require.NoError(t, os.RemoveAll(config.RootDir))

	initSupply := types.PowerToAmount(350_000_000)

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
	fmt.Println("Test TxFeeProcessing using VPowerHandlerMock", "validator number", valsCnt, "total power", vpowMock.GetTotalPower())

	_ = mocks.InitBlockCtxWith("", 1, govMock, acctMock, nil, nil, vpowMock)
	require.NoError(t, mocks.DoBeginBlock(ctrler))
	require.NoError(t, mocks.DoEndBlockAndCommit(ctrler))

	require.Equal(t, initSupply, ctrler.lastTotalSupply.totalSupply)
	require.Equal(t, initSupply, ctrler.lastTotalSupply.adjustSupply)
	require.Equal(t, int64(1), ctrler.lastTotalSupply.GetAdjustHeight())

	// Use a short InflationCycleBlocks value for faster testing.
	// The original value should be restored afterward to avoid affecting other tests.
	orgInflCycleBlocks := govMock.InflationCycleBlocks()
	govMock.GetValues().InflationCycleBlocks = 10
	defer func() { govMock.GetValues().InflationCycleBlocks = orgInflCycleBlocks }()

	for currHeight := int64(2); currHeight < govMock.InflationCycleBlocks()+10; currHeight++ {
		//fmt.Println("---- block", currHeight)

		expectedTotalSupply := ctrler.lastTotalSupply.GetTotalSupply()
		expectedAdjustedSupply := ctrler.lastTotalSupply.GetAdjustSupply()
		expectedAdjustedHeight := ctrler.lastTotalSupply.GetAdjustHeight()

		proposer := btztypes.RandAddress()
		mocks.CurrBlockCtx().SetProposerAddress(proposer)

		expectedBurned, expectedRwdFee, expectedProposerBal := uint256.NewInt(0), uint256.NewInt(0), uint256.NewInt(0)
		if bytes.RandInt64N(2) == 0 {
			// when the burning occurs.

			gas := bytes.RandInt64N(500_000) + govMock.MinTrxGas()
			fee := types.GasToFee(gas, govMock.GasPrice())
			mocks.CurrBlockCtx().AddFee(fee)
			expectedBurned = new(uint256.Int).Mul(fee, uint256.NewInt(uint64(100-govMock.TxFeeRewardRate())))
			expectedBurned = new(uint256.Int).Div(expectedBurned, uint256.NewInt(100))
			expectedRwdFee = new(uint256.Int).Sub(fee, expectedBurned)

			if acct := acctMock.FindAccount(proposer, true); acct != nil {
				expectedProposerBal = acct.GetBalance()
			}
			//fmt.Println("tx in block", currHeight)
		}

		require.NoError(t, mocks.DoBeginBlock(ctrler))

		// nothing is changed after BeginBlock
		require.Equal(t, expectedTotalSupply, ctrler.lastTotalSupply.totalSupply)
		require.Equal(t, expectedAdjustedSupply, ctrler.lastTotalSupply.adjustSupply)
		require.Equal(t, expectedAdjustedHeight, ctrler.lastTotalSupply.GetAdjustHeight())

		//
		// Compute expected values.
		//
		{
			// burn & fee reward
			// expect that the treasury address(zero address)'s balance is increased
			// and the proposer's balance is increased too.
			// But the total supply is not changed.
			// This burn operation does not reduce the total supply.
			expectedTotalSupply = ctrler.lastTotalSupply.GetTotalSupply()
			expectedAdjustedSupply = ctrler.lastTotalSupply.GetAdjustSupply()
			expectedAdjustedHeight = ctrler.lastTotalSupply.GetAdjustHeight()
			expectedProposerBal = new(uint256.Int).Add(expectedProposerBal, expectedRwdFee)
			//fmt.Println("expected burn", t0, "-", expectedBurned, "=", expectedTotalSupply, "bctx.sumfee", mocks.CurrBlockCtx().SumFee())

			// apply inflation
			if currHeight%govMock.InflationCycleBlocks() == 0 {
				_totalSupplyAmt := ctrler.lastTotalSupply.GetTotalSupply()
				_adjustSupplyAmt := ctrler.lastTotalSupply.GetAdjustSupply()
				_adjustHeight := ctrler.lastTotalSupply.GetAdjustHeight()

				weightInfo, xerr := vpowMock.ComputeWeight(
					currHeight,
					govMock.InflationCycleBlocks(),
					govMock.RipeningBlocks(),
					govMock.BondingBlocksWeightPermil(),
					_totalSupplyAmt)
				require.NoError(t, xerr)

				wa := weightInfo.SumWeight() //.Truncate(precision)
				//wa := vpower.WaEx64ByPowerChunk(vpowMock.PowerChunks, currHeight, govMock.RipeningBlocks(), govMock.BondingBlocksWeightPermil(), totalSupply)
				//wa = wa.Truncate(precision)

				si := Si(currHeight, int64(govMock.AssumedBlockInterval()), _adjustHeight, _adjustSupplyAmt, govMock.MaxTotalSupply(), govMock.InflationWeightPermil(), wa).Floor()
				//expectedTotalSupply = uint256.MustFromBig(si.BigInt())
				newTotalSupply := uint256.MustFromBig(si.BigInt())
				addedAmt := new(uint256.Int).Sub(newTotalSupply, _totalSupplyAmt)
				expectedTotalSupply = new(uint256.Int).Add(expectedTotalSupply, addedAmt)
				//fmt.Println("expected inflation amount", expectedTotalSupply, "last.total", ctrler.lastTotalSupply.totalSupply, "last.adjust", ctrler.lastTotalSupply.adjustSupply, "adjust.height", ctrler.lastTotalSupply.GetAdjustHeight())
			}
		}

		//
		// If inflation orrcurs, ctrler.lastTotalSupply is changed in EndBlock
		require.NoError(t, mocks.DoEndBlock(ctrler))

		diff := absDiff(ctrler.lastTotalSupply.totalSupply, expectedTotalSupply)
		require.LessOrEqual(t, diff.Uint64(), uint64(1), diff.Uint64()) // there may be an error from calculating weight.
		expectedTotalSupply = ctrler.lastTotalSupply.GetTotalSupply()

		item, xerr := ctrler.supplyState.Get(v1.LedgerKeyTotalSupply(), true)
		require.NoError(t, xerr)
		_supplyInfo, ok := item.(*Supply)
		require.True(t, ok)
		require.Equal(t, expectedTotalSupply, _supplyInfo.totalSupply)
		require.Equal(t, expectedAdjustedSupply, _supplyInfo.adjustSupply)
		require.Equal(t, expectedAdjustedHeight, _supplyInfo.GetAdjustHeight())

		require.Equal(t, expectedTotalSupply, ctrler.lastTotalSupply.totalSupply)
		require.Equal(t, expectedAdjustedSupply, ctrler.lastTotalSupply.adjustSupply)
		require.Equal(t, expectedAdjustedHeight, ctrler.lastTotalSupply.GetAdjustHeight())

		if expectedProposerBal.Sign() > 0 {
			acct := acctMock.FindAccount(mocks.CurrBlockCtx().ProposerAddress(), true)
			require.NotNil(t, acct)
			require.Equal(t, expectedProposerBal, acct.GetBalance())
		}

		require.NoError(t, mocks.DoCommit(ctrler))

		require.Equal(t, expectedTotalSupply, ctrler.lastTotalSupply.totalSupply)
		require.Equal(t, expectedAdjustedSupply, ctrler.lastTotalSupply.adjustSupply)
		require.Equal(t, expectedAdjustedHeight, ctrler.lastTotalSupply.GetAdjustHeight())

		item, xerr = ctrler.supplyState.Get(v1.LedgerKeyTotalSupply(), true)
		require.NoError(t, xerr)
		_supplyInfo, ok = item.(*Supply)
		require.True(t, ok)
		require.Equal(t, expectedTotalSupply, _supplyInfo.totalSupply)
		require.Equal(t, expectedAdjustedSupply, _supplyInfo.adjustSupply)
		require.Equal(t, expectedAdjustedHeight, _supplyInfo.GetAdjustHeight())

	}
}
