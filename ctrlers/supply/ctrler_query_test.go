package supply

import (
	"fmt"
	"github.com/beatoz/beatoz-go/ctrlers/mocks/gov"
	vpowmock "github.com/beatoz/beatoz-go/ctrlers/mocks/vpower"
	"github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	tmjson "github.com/tendermint/tendermint/libs/json"
	"os"
	"testing"
	"time"
)

func Test_Query_Reward(t *testing.T) {
	require.NoError(t, os.RemoveAll(config.RootDir))

	localGovMock := gov.NewGovHandlerMock(types.DefaultGovParams())
	protoVal := localGovMock.GetValues()
	protoVal.InflationCycleBlocks = 10

	initSupply := types.PowerToAmount(350_000_000)
	ctrler, xerr := initLedger(initSupply)
	require.NoError(t, xerr)

	//
	// Use VPowerHandlerMock
	valsCnt := min(acctMock.WalletLen(), 21)
	valWals := make([]*web3.Wallet, valsCnt)
	for i := 0; i < valsCnt; i++ {
		valWals[i] = acctMock.GetWallet(i)
	}
	vpowMock := vpowmock.NewVPowerHandlerMock(valWals)
	fmt.Println("Test Withdraw using VPowerHandlerMock", "validator number", valsCnt, "total power", vpowMock.GetTotalPower())

	_, _, xerr = ctrler.Commit()
	require.NoError(t, xerr)

	for currHeight := int64(2); currHeight < localGovMock.InflationCycleBlocks()*30; currHeight++ {
		if currHeight%localGovMock.InflationCycleBlocks() == 0 {
			bctx := types.TempBlockContext("mint-test-chain", currHeight, time.Now(), govMock, acctMock, nil, nil, vpowMock)
			ctrler.requestMint(bctx)
			_, xerr = ctrler.waitMint(bctx)
			require.NoError(t, xerr)
		}

		_, _, xerr = ctrler.Commit()
		require.NoError(t, xerr)
	}

	for _, wal := range valWals {
		var preRwd *Reward
		addr := wal.Address()
		for h := int64(1); h < localGovMock.InflationCycleBlocks()*30; h++ {
			bz, xerr := ctrler.queryReward(h, addr)
			require.NoError(t, xerr)

			rwd := &Reward{}
			require.NoError(t, tmjson.Unmarshal(bz, rwd))

			if preRwd == nil {
				preRwd = rwd
				continue
			}

			require.Equal(t, addr, rwd.Address())
			require.Equal(t, preRwd.Address(), rwd.Address())
			if preRwd.Height() == rwd.Height() {
				require.Equal(t, preRwd.MintedAmount().Dec(), rwd.MintedAmount().Dec())
				require.Equal(t, preRwd.WithdrawnAmount().Dec(), rwd.WithdrawnAmount().Dec())
				require.Equal(t, preRwd.SlashedAmount().Dec(), rwd.SlashedAmount().Dec())
				require.Equal(t, preRwd.CumulatedAmount().Dec(), rwd.CumulatedAmount().Dec())
			} else {
				expectedAmt := new(uint256.Int).Add(preRwd.CumulatedAmount(), rwd.MintedAmount())
				require.Equal(t, preRwd.Height()+localGovMock.InflationCycleBlocks(), rwd.Height())
				require.Equal(t, expectedAmt.Dec(), rwd.CumulatedAmount().Dec())
			}
			preRwd = rwd
		}
	}

	require.NoError(t, ctrler.Close())
	require.NoError(t, os.RemoveAll(config.RootDir))
}
