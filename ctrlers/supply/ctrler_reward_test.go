package supply

import (
	"fmt"
	vpowmock "github.com/beatoz/beatoz-go/ctrlers/mocks/vpower"
	"github.com/beatoz/beatoz-go/ctrlers/types"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	types2 "github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
	"time"
)

func Test_Withdraw(t *testing.T) {
	require.NoError(t, os.RemoveAll(config.RootDir))

	initSupply := types2.PowerToAmount(350_000_000)
	ctrler, xerr := initLedger(initSupply)
	require.NoError(t, xerr)

	//
	// Use VPowerHandlerMock
	valsCnt := min(acctMock.WalletLen(), 21)
	valWals := make([]*web3.Wallet, valsCnt)
	for i := 0; i < valsCnt; i++ {
		valWals[i] = acctMock.GetWallet(i)
	}
	vpowMock := vpowmock.NewVPowerHandlerMock(valWals, len(valWals))
	fmt.Println("Test Withdraw using VPowerHandlerMock", "validator number", valsCnt, "total power", vpowMock.GetTotalPower())

	//
	// generate rewards
	preRewards := make(map[string]*uint256.Int)
	for currHeight := int64(2); currHeight < govMock.InflationCycleBlocks()*30; currHeight += govMock.InflationCycleBlocks() {
		bctx := types.TempBlockContext("mint-test-chain", currHeight, time.Now(), govMock, acctMock, nil, nil, vpowMock)
		ctrler.requestMint(bctx)
		result, xerr := ctrler.waitMint(bctx)
		require.NoError(t, xerr)

		for _, mintRwd := range result.rewards {
			// check reward amount of beneficiary
			accumRwd, xerr := ctrler.readReward(mintRwd.addr)
			require.NoError(t, xerr)
			require.EqualValues(t, mintRwd.addr, accumRwd.Address())

			preRwdAmt, ok := preRewards[mintRwd.addr.String()]
			if !ok {
				preRewards[mintRwd.addr.String()] = accumRwd.CumulatedAmount()
			} else {
				_ = preRwdAmt.Add(preRwdAmt, mintRwd.amt)
				require.Equal(t, preRwdAmt.Dec(), accumRwd.CumulatedAmount().Dec())
				preRewards[mintRwd.addr.String()] = preRwdAmt

				// 	withdarw
				wal := acctMock.FindWallet(mintRwd.addr)
				require.NotNil(t, wal)
				beforeBal := wal.GetBalance()
				beforeWithdrawn := accumRwd.WithdrawnAmount()
				beforeCummAmt := accumRwd.CumulatedAmount()

				item, xerr := ctrler.supplyState.Get(v1.LedgerKeyReward(mintRwd.addr), true)
				require.NoError(t, xerr)

				rwd := item.(*Reward)
				ramt := bytes.RandU256IntN(accumRwd.CumulatedAmount())
				require.NoError(t, ctrler.withdrawReward(rwd, ramt, currHeight, acctMock, true))

				expectedBalance := new(uint256.Int).Add(beforeBal, ramt)
				afaterBal := wal.GetBalance()
				require.Equal(t, expectedBalance.Dec(), afaterBal.Dec())

				expectedWithdraw := new(uint256.Int).Add(beforeWithdrawn, ramt)
				expectedCummAmt := new(uint256.Int).Sub(beforeCummAmt, ramt)

				accumRwd1, xerr := ctrler.readReward(mintRwd.addr)
				require.NoError(t, xerr)
				require.Equal(t, expectedWithdraw.Dec(), accumRwd1.WithdrawnAmount().Dec())
				require.Equal(t, expectedCummAmt.Dec(), accumRwd1.CumulatedAmount().Dec())

				preRewards[mintRwd.addr.String()] = expectedCummAmt
			}
		}
	}
	require.NoError(t, ctrler.Close())
	require.NoError(t, os.RemoveAll(config.RootDir))
}
