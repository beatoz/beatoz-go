package supply

import (
	"fmt"
	btzcfg "github.com/beatoz/beatoz-go/cmd/config"
	"github.com/beatoz/beatoz-go/ctrlers"
	"github.com/beatoz/beatoz-go/ctrlers/account"
	"github.com/beatoz/beatoz-go/ctrlers/mocks"
	vpowmock "github.com/beatoz/beatoz-go/ctrlers/mocks/vpower"
	"github.com/beatoz/beatoz-go/ctrlers/types"
	v2 "github.com/beatoz/beatoz-go/ledger/v2"
	types2 "github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"
	"os"
	"testing"
	"time"
)

type rewardFailingAcctHandler struct {
	types.IAccountHandler
}

func (handler *rewardFailingAcctHandler) Reward(_ types2.Address, _ *uint256.Int, _ bool) xerrors.XError {
	return xerrors.ErrInvalidAmount.Wrapf("forced account reward failure")
}

func (handler *rewardFailingAcctHandler) CacheHandlerContext(exec bool) (types.IAccountHandler, func() xerrors.XError) {
	cached, writeCache := handler.IAccountHandler.(types.ICacheableAccountHandler).CacheHandlerContext(exec)
	return &rewardFailingAcctHandler{IAccountHandler: cached}, writeCache
}

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

				item, xerr := ctrler.supplyState.Get(v2.LedgerKeyReward(mintRwd.addr), true)
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

func TestWithdrawRollback(t *testing.T) {
	localConfig := btzcfg.DefaultConfig("1234")
	localConfig.SetRoot(t.TempDir())
	types.InitSigner(localConfig.ChainId())

	acctCtrler, err := account.NewAcctCtrler(localConfig, log.NewNopLogger())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, acctCtrler.Close())
	}()

	supplyCtrler, xerr := NewSupplyCtrler(localConfig, log.NewNopLogger())
	require.NoError(t, xerr)
	defer func() {
		require.NoError(t, supplyCtrler.Close())
	}()

	wal := web3.NewWallet(nil)
	acct := wal.GetAccount()
	acct.SetBalance(uint256.NewInt(1_000_000_000))
	require.NoError(t, acctCtrler.SetAccount(acct, true))
	_, _, xerr = acctCtrler.Commit()
	require.NoError(t, xerr)

	reward := NewReward(wal.Address())
	require.NoError(t, reward.Issue(uint256.NewInt(1000), 1))
	require.NoError(t, supplyCtrler.supplyState.Set(v2.LedgerKeyReward(wal.Address()), reward, true))
	_, _, xerr = supplyCtrler.Commit()
	require.NoError(t, xerr)

	before, xerr := supplyCtrler.readReward(wal.Address())
	require.NoError(t, xerr)
	beforeCumulated := before.CumulatedAmount()
	beforeWithdrawn := before.WithdrawnAmount()
	beforeBalance := acct.GetBalance()

	reqAmt := uint256.NewInt(100)
	tx := web3.NewTrxWithdraw(wal.Address(), wal.Address(), wal.GetNonce(), govMock.MinTrxGas(), govMock.GasPrice(), reqAmt)
	_, _, err = wal.SignTrxRLP(tx, localConfig.ChainIdHex())
	require.NoError(t, err)

	failingAcct := &rewardFailingAcctHandler{IAccountHandler: acctCtrler}
	bctx := types.TempBlockContext(localConfig.ChainIdHex(), 2, time.Now(), govMock, failingAcct, nil, supplyCtrler, nil)
	txctx, xerr := mocks.MakeTrxCtxWithTrxBctx(tx, bctx, true)
	require.NoError(t, xerr)
	require.NoError(t, supplyCtrler.ValidateTrx(txctx))

	scope := ctrlers.NewBlockCacheContext(bctx, true)
	require.NotNil(t, scope)
	defer scope.Restore(bctx)

	xerr = bctx.SupplyHandler.ExecuteTrx(txctx)
	require.Error(t, xerr)
	require.True(t, xerr.Contains(xerrors.ErrInvalidAmount))
	scope.Restore(bctx)

	after, xerr := supplyCtrler.readReward(wal.Address())
	require.NoError(t, xerr)
	require.Equal(t, beforeCumulated.Dec(), after.CumulatedAmount().Dec())
	require.Equal(t, beforeWithdrawn.Dec(), after.WithdrawnAmount().Dec())

	afterAcct := acctCtrler.FindAccount(wal.Address(), true)
	require.NotNil(t, afterAcct)
	require.Equal(t, beforeBalance.Dec(), afterAcct.GetBalance().Dec())

	_, _, xerr = supplyCtrler.Commit()
	require.NoError(t, xerr)
	committed, xerr := supplyCtrler.readReward(wal.Address())
	require.NoError(t, xerr)
	require.Equal(t, beforeCumulated.Dec(), committed.CumulatedAmount().Dec())
	require.Equal(t, beforeWithdrawn.Dec(), committed.WithdrawnAmount().Dec())
}
