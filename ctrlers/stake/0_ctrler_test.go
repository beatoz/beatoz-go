package stake_test

import (
	"bytes"
	"fmt"
	beatozcfg "github.com/beatoz/beatoz-go/cmd/config"
	"github.com/beatoz/beatoz-go/ctrlers/mocks"
	"github.com/beatoz/beatoz-go/ctrlers/stake"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/libs/web3"
	beatoztypes "github.com/beatoz/beatoz-go/types"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmlog "github.com/tendermint/tendermint/libs/log"
	tmtypes "github.com/tendermint/tendermint/proto/tendermint/types"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

var (
	config      = beatozcfg.DefaultConfig()
	DBDIR       = filepath.Join(os.TempDir(), "stake-ctrler-test")
	govParams00 = ctrlertypes.Test6GovParams_NoStakeLimiter() // `maxUpdatableStakeRatio = 100, maxIndividualStakeRatio = 10000000`
	acctMock00  *mocks.AcctHandlerMock
	stakeCtrler *stake.StakeCtrler

	DelegateeWallets     []*web3.Wallet
	stakingToSelfTrxCtxs []*ctrlertypes.TrxContext
	stakingTrxCtxs       []*ctrlertypes.TrxContext
	unstakingTrxCtxs     []*ctrlertypes.TrxContext

	dummyGas      = uint64(0)
	dummyGasPrice = uint256.NewInt(0)
	dummyNonce    = uint64(0)

	lastHeight = int64(1)
)

func TestMain(m *testing.M) {
	os.RemoveAll(DBDIR)

	config.DBPath = DBDIR
	stakeCtrler, _ = stake.NewStakeCtrler(config, govParams00, tmlog.NewNopLogger())

	acctMock00 = mocks.NewAccountHandlerMock(100 + int(govParams00.MaxValidatorCnt()))
	acctMock00.Iterate(func(idx int, w *web3.Wallet) bool {
		w.GetAccount().SetBalance(beatoztypes.ToFons(1_000_000_000))
		return true
	})
	for i := 0; i < 5; i++ {
		if txctx, err := randMakeStakingToSelfTrxContext(); err != nil {
			panic(err)
		} else {
			stakingToSelfTrxCtxs = append(stakingToSelfTrxCtxs, txctx)
		}
		if rand.Int()%3 == 0 {
			lastHeight++
		}
	}

	lastHeight++
	lastHeight++

	for i := 0; i < 1000; i++ {
		if txctx, err := randMakeStakingTrxContext(); err != nil {
			panic(err)
		} else {
			stakingTrxCtxs = append(stakingTrxCtxs, txctx)
		}
		if rand.Int()%3 == 0 {
			lastHeight++
		}
	}

	lastHeight += 10

	for i := 0; i < 100; i++ {
		if txctx, err := randMakeUnstakingTrxContext(); err != nil {
			panic(err)
		} else {
			already := false
			for _, _ctx := range unstakingTrxCtxs {
				if bytes.Compare(_ctx.Tx.Payload.(*ctrlertypes.TrxPayloadUnstaking).TxHash, txctx.Tx.Payload.(*ctrlertypes.TrxPayloadUnstaking).TxHash) == 0 {
					already = true
				}
			}
			if !already {
				unstakingTrxCtxs = append(unstakingTrxCtxs, txctx)
			}

		}
		if rand.Int()%3 == 0 {
			lastHeight++
		}
	}

	exitCode := m.Run()

	os.RemoveAll(DBDIR)

	os.Exit(exitCode)
}

func TestTrxStakingToSelf(t *testing.T) {
	sumAmt := uint256.NewInt(0)
	sumPower := int64(0)

	_ = mocks.InitBlockCtxWith(0, acctMock00, govParams00, nil)
	require.NoError(t, doBeginBlock())

	for _, txctx := range stakingToSelfTrxCtxs {
		if mocks.LastBlockHeight() < txctx.Height {
			for mocks.LastBlockHeight() < txctx.Height {
				require.NoError(t, doEndBlockCommit())
				require.NoError(t, doBeginBlock())
			}
		}

		err := stakeCtrler.ValidateTrx(txctx)
		require.NoError(t, err)
		err = stakeCtrler.ExecuteTrx(txctx)
		require.NoError(t, err)

		_ = sumAmt.Add(sumAmt, txctx.Tx.Amount)
		sumPower += ctrlertypes.AmountToPower(txctx.Tx.Amount)
	}

	require.NoError(t, doEndBlockCommit())

	require.Equal(t, sumAmt.String(), stakeCtrler.ReadTotalAmount().String())
	require.Equal(t, sumPower, stakeCtrler.ReadTotalPower())
}

func TestTrxStakingByTx(t *testing.T) {
	sumAmt := stakeCtrler.ReadTotalAmount()
	sumPower := stakeCtrler.ReadTotalPower()

	require.NoError(t, doBeginBlock())

	for i, txctx := range stakingTrxCtxs {
		if mocks.LastBlockHeight() < txctx.Height {
			for mocks.LastBlockHeight() < txctx.Height {
				require.NoError(t, doEndBlockCommit())
				require.NoError(t, doBeginBlock())
			}
		}

		power0 := stakeCtrler.SelfPowerOf(txctx.Tx.To)
		power1 := stakeCtrler.DelegatedPowerOf(txctx.Tx.To)
		maxAmt := ctrlertypes.PowerToAmount(power0 - power1)

		err := stakeCtrler.ValidateTrx(txctx)
		if txctx.Tx.Amount.Cmp(maxAmt) > 0 && txctx.Tx.From.Compare(txctx.Tx.To) != 0 {
			// it's error to try delegating to validator over self_stake_ratio
			require.Error(t, err)
			// this tx should be removed from `unstakingTrxCtxs` for test successfully.
			for i, ctx := range unstakingTrxCtxs {
				if bytes.Compare(ctx.Tx.Payload.(*ctrlertypes.TrxPayloadUnstaking).TxHash, txctx.TxHash) == 0 {
					unstakingTrxCtxs = append(unstakingTrxCtxs[:i], unstakingTrxCtxs[i+1:]...)
					break
				}
			}
			continue
		} else {
			require.NoError(t, err, fmt.Sprintf("index:%v", i), txctx.Tx)
		}

		err = stakeCtrler.ExecuteTrx(txctx)
		require.NoError(t, err)
		_ = sumAmt.Add(sumAmt, txctx.Tx.Amount)
		sumPower += ctrlertypes.AmountToPower(txctx.Tx.Amount)
	}

	require.NoError(t, doEndBlockCommit())

	require.Equal(t, sumAmt.String(), stakeCtrler.ReadTotalAmount().String())
	require.Equal(t, sumPower, stakeCtrler.ReadTotalPower())
}

func TestDoReward(t *testing.T) {
	validators, _ := stakeCtrler.Validators()
	require.Greater(t, len(validators), 0)

	expectedReward := uint256.NewInt(0)
	var votes []abcitypes.VoteInfo
	for _, val := range validators {

		votes = append(votes, abcitypes.VoteInfo{
			Validator: abcitypes.Validator{
				Address: val.Address,
				Power:   val.Power,
			},
			SignedLastBlock: true,
		})

		expectedReward.Add(expectedReward,
			new(uint256.Int).Mul(uint256.NewInt(uint64(val.Power)), govParams00.RewardPerPower()))
	}
	lastHeight++
	issued, err := stakeCtrler.DoReward(mocks.LastBlockHeight(), votes)
	require.NoError(t, err)
	require.Equal(t, expectedReward, issued)
}

func TestPunish(t *testing.T) {
	allTotalPower0 := stakeCtrler.ReadTotalPower()
	for _, byzanWallet := range DelegateeWallets {
		totalPower0 := stakeCtrler.TotalPowerOf(byzanWallet.Address())
		selfPower0 := stakeCtrler.SelfPowerOf(byzanWallet.Address())
		require.Greater(t, selfPower0, int64(0))

		_slashed := uint256.NewInt(uint64(selfPower0))
		_ = _slashed.Mul(_slashed, uint256.NewInt(uint64(govParams00.SlashRatio())))
		_ = _slashed.Div(_slashed, uint256.NewInt(uint64(100)))
		expectedSlashed := int64(_slashed.Uint64())
		require.Greater(t, expectedSlashed, int64(0))
		require.Greater(t, selfPower0, expectedSlashed)

		// original stake state
		delegatee := stakeCtrler.Delegatee(byzanWallet.Address())
		oriStakes := make([]*stake.Stake, delegatee.StakesLen())
		for i, s0 := range delegatee.GetAllStakes() {
			oriStakes[i] = s0.Clone()
		}

		sumSlashedPower0, xerr := stakeCtrler.DoPunish(&abcitypes.Evidence{
			Validator: abcitypes.Validator{
				Address: byzanWallet.Address(),
				Power:   totalPower0,
			},
		}, govParams00.SlashRatio())
		require.NoError(t, xerr)

		delegatee = stakeCtrler.Delegatee(byzanWallet.Address())

		expectedSumSlashedPower := int64(0)
		for _, s0 := range oriStakes {
			slashedPower := (s0.Power * govParams00.SlashRatio()) / int64(100)
			if slashedPower < 1 {
				slashedPower = s0.Power
			}
			expectedPower := s0.Power - slashedPower
			require.True(t, expectedPower >= 0)
			require.NotEqual(t, expectedPower, s0.Power)

			_, s1 := delegatee.FindStake(s0.TxHash)
			if expectedPower == 0 {
				// it is expected that the s0 is removed
				require.Nil(t, s1)
			} else {
				require.NotNil(t, s1)
				require.Equal(t, s0.TxHash, s1.TxHash)
				require.Equal(t, expectedPower, s1.Power)
			}

			expectedSumSlashedPower += slashedPower
		}
		require.Equal(t, expectedSumSlashedPower, sumSlashedPower0)
		require.Equal(t, totalPower0-sumSlashedPower0, delegatee.TotalPower)
		require.Equal(t, totalPower0-sumSlashedPower0, delegatee.SumPowerOf(nil))

		_, _, xerr = stakeCtrler.Commit()
		require.NoError(t, xerr)

		require.Equal(t, totalPower0-expectedSumSlashedPower, stakeCtrler.TotalPowerOf(delegatee.Addr))
		require.Equal(t, allTotalPower0-expectedSumSlashedPower, stakeCtrler.ReadTotalPower())

		allTotalPower0 -= expectedSumSlashedPower
	}
}

// test for issue #43
func TestUnstakingByNotOwner(t *testing.T) {
	require.NoError(t, doBeginBlock())

	for _, txctx := range unstakingTrxCtxs {

		if mocks.LastBlockHeight() < txctx.Height {
			for mocks.LastBlockHeight() < txctx.Height {
				require.NoError(t, doEndBlockCommit())
				require.NoError(t, doBeginBlock())
			}
		}

		ori := txctx.Tx.From
		txctx.Tx.From = beatoztypes.RandAddress()
		err := stakeCtrler.ValidateTrx(txctx)
		require.Error(t, err)
		//err = stakeCtrler.ExecuteTrx(txctx)
		//require.Error(t, err)

		txctx.Tx.From = ori
	}

	require.NoError(t, doEndBlockCommit())
}

func TestUnstakingByTx(t *testing.T) {
	sumPower0 := stakeCtrler.ReadTotalPower()
	sumUnstakingPower := int64(0)

	require.NoError(t, doBeginBlock())

	for _, txctx := range unstakingTrxCtxs {

		if mocks.LastBlockHeight() < txctx.Height {
			for mocks.LastBlockHeight() < txctx.Height {
				require.NoError(t, doEndBlockCommit())
				require.NoError(t, doBeginBlock())
			}
		}

		stakingTxHash := txctx.Tx.Payload.(*ctrlertypes.TrxPayloadUnstaking).TxHash
		delegatee := stakeCtrler.Delegatee(txctx.Tx.To)
		require.NotNil(t, delegatee)

		_, s0 := delegatee.FindStake(stakingTxHash)
		require.NotNil(t, s0)

		err := stakeCtrler.ValidateTrx(txctx)
		require.NoError(t, err)

		err = stakeCtrler.ExecuteTrx(txctx)
		require.NoError(t, err)

		sumUnstakingPower += s0.Power
	}

	require.NoError(t, doEndBlockCommit())

	require.Equal(t, sumPower0-sumUnstakingPower, stakeCtrler.ReadTotalPower())

	// test freezing reward
	frozenStakes := stakeCtrler.ReadFrozenStakes()
	require.Equal(t, len(unstakingTrxCtxs), len(frozenStakes))

	sumFrozenPower := int64(0)
	for _, s := range frozenStakes {
		sumFrozenPower += s.Power
	}
	require.Equal(t, sumFrozenPower, sumUnstakingPower)
}

func TestUnfreezing(t *testing.T) {
	type refundInfo struct {
		addr            beatoztypes.Address
		originalBalance *uint256.Int
		refundedAmount  *uint256.Int
	}

	expectedRefunds := make(map[string]*refundInfo)
	frozenStakes := stakeCtrler.ReadFrozenStakes()
	require.Greater(t, len(frozenStakes), 0)

	for _, s0 := range frozenStakes {
		//require.NotEqual(t, "0", s0.ReceivedReward.Dec(), "your test may not reward")

		acct := acctMock00.FindAccount(s0.From, true)
		require.NotNil(t, acct)

		er, ok := expectedRefunds[acct.Address.String()]
		if !ok {
			er = &refundInfo{
				addr:            acct.Address,
				originalBalance: acct.Balance.Clone(),
				refundedAmount:  uint256.NewInt(0),
			}
			expectedRefunds[acct.Address.String()] = er
		}

		_ = er.refundedAmount.Add(
			er.refundedAmount,
			ctrlertypes.PowerToAmount(s0.Power))
		//new(uint256.Int).Add(s0.Amount, s0.ReceivedReward))
	}

	toBlockHeight := mocks.LastBlockHeight() + govParams00.LazyRewardBlocks()
	for mocks.LastBlockHeight() < toBlockHeight {
		require.NoError(t, doEndBlockCommit())
		require.NoError(t, doBeginBlock())
	}

	// execute block at lastHeight
	req := abcitypes.RequestBeginBlock{
		Header: tmtypes.Header{
			Height: lastHeight,
		},
	}
	bctx := ctrlertypes.NewBlockContext(req, govParams00, acctMock00, nil)
	bctx.AddFee(uint256.NewInt(10))

	mocks.InitBlockCtx(bctx)

	require.NoError(t, doEndBlockCommit())

	frozenStakes = stakeCtrler.ReadFrozenStakes()
	require.Equal(t, 0, len(frozenStakes))

	for _, er := range expectedRefunds {
		acct1 := acctMock00.FindAccount(er.addr, true)
		require.NotNil(t, acct1)
		require.NotEqual(t, acct1.Balance.Dec(), er.originalBalance.Dec())

		expectedBalance := new(uint256.Int).Add(er.originalBalance, er.refundedAmount)
		require.Equal(t, expectedBalance.Dec(), acct1.Balance.Dec())

	}
}

func doBeginBlock() error {
	bctx := mocks.NextBlockCtx()
	_, err := stakeCtrler.BeginBlock(bctx)
	return err
}
func doEndBlock() error {
	bctx := mocks.LastBlockCtx()
	if _, err := stakeCtrler.EndBlock(bctx); err != nil {
		return err
	}
	return nil
}
func doCommitBlock() error {
	if _, _, err := stakeCtrler.Commit(); err != nil {
		return err
	}
	return nil
}

func doEndBlockCommit() error {
	if err := doEndBlock(); err != nil {
		return err
	}
	if err := doCommitBlock(); err != nil {
		return err
	}
	return nil
}
