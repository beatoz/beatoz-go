package account

import (
	btzcfg "github.com/beatoz/beatoz-go/cmd/config"
	"github.com/beatoz/beatoz-go/ctrlers/mocks"
	govmock "github.com/beatoz/beatoz-go/ctrlers/mocks/gov"
	"github.com/beatoz/beatoz-go/ctrlers/types"
	btztypes "github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	tmlog "github.com/tendermint/tendermint/libs/log"
	"os"
	"path/filepath"
	"testing"
)

func Test_TxFeeProcessing(t *testing.T) {
	rootDir := filepath.Join(os.TempDir(), "supply-test")
	config := btzcfg.DefaultConfig()
	config.SetRoot(rootDir)
	require.NoError(t, os.RemoveAll(config.RootDir))

	govMock := govmock.NewGovHandlerMock(types.NewGovParams(1))
	ctrler, xerr := NewAcctCtrler(config, tmlog.NewNopLogger())
	require.NoError(t, xerr)

	_ = mocks.InitBlockCtxWith("", 1, govMock, ctrler, nil, nil, nil)
	require.NoError(t, mocks.DoBeginBlock(ctrler))
	require.NoError(t, mocks.DoEndBlockAndCommit(ctrler))

	for currHeight := int64(2); currHeight < 500; currHeight++ {
		//fmt.Println("---- block", currHeight)

		deadAddr := govMock.DeadAddress()
		proposer := btztypes.RandAddress()
		mocks.CurrBlockCtx().SetProposerAddress(proposer)

		expectedDrained, expectedRwdFee, expectedDeadBal, expectedProposerBal := uint256.NewInt(0), uint256.NewInt(0), uint256.NewInt(0), uint256.NewInt(0)
		if bytes.RandInt64N(2) == 0 {
			// when the fee-to-dead occurs.

			gas := bytes.RandInt64N(500_000) + govMock.MinTrxGas()
			fee := btztypes.GasToFee(gas, govMock.GasPrice())
			mocks.CurrBlockCtx().AddFee(fee)
			expectedDrained = new(uint256.Int).Mul(fee, uint256.NewInt(uint64(100-govMock.TxFeeRewardRate())))
			expectedDrained = new(uint256.Int).Div(expectedDrained, uint256.NewInt(100))
			expectedRwdFee = new(uint256.Int).Sub(fee, expectedDrained)

			if _acct := ctrler.FindAccount(deadAddr, true); _acct != nil {
				expectedDeadBal = _acct.GetBalance()
			}
			_ = expectedDeadBal.Add(expectedDeadBal, expectedDrained)

			if _acct := ctrler.FindAccount(proposer, true); _acct != nil {
				expectedProposerBal = _acct.GetBalance()
			}
			_ = expectedProposerBal.Add(expectedProposerBal, expectedRwdFee)
		}

		require.NoError(t, mocks.DoBeginBlock(ctrler))

		// fee-to-dead occurs
		require.NoError(t, mocks.DoEndBlock(ctrler))

		if expectedDeadBal.Sign() > 0 {
			acct := ctrler.FindAccount(deadAddr, true)
			require.NotNil(t, acct, deadAddr)
			require.Equal(t, expectedDeadBal, acct.GetBalance(), "wrong dead balance", "height", currHeight)
		}

		if expectedProposerBal.Sign() > 0 {
			acct := ctrler.FindAccount(mocks.CurrBlockCtx().ProposerAddress(), true)
			require.NotNil(t, acct, mocks.CurrBlockCtx().ProposerAddress())
			require.Equal(t, expectedProposerBal, acct.GetBalance(), "wrong proposer balance", "height", currHeight)
		}

		require.NoError(t, mocks.DoCommit(ctrler))
	}
}
