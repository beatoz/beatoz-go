package stake

import (
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/crypto"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto/secp256k1"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"math/rand"
	"testing"
)

func TestSlash(t *testing.T) {
	for i := 0; i < 21; i++ {
		govParams := ctrlertypes.DefaultGovParams() //ctrlertypes.Test1GovParams()

		delegatee := newDelegatee()

		selfPower0 := int64(0)
		totalPower0 := int64(0)
		stakesCnt := int64(tmrand.Intn(100) + 100)

		for i := int64(0); i < stakesCnt; i++ {
			stakePower := tmrand.Int63n(100_000_000)
			totalPower0 += stakePower

			fromAddr := types.RandAddress()
			if tmrand.Int()%2 == 0 {
				// self staking
				fromAddr = delegatee.Addr
				selfPower0 += stakePower
			}

			delegatee.AddStake(
				NewStakeWithPower(
					fromAddr,
					delegatee.Addr,
					stakePower,
					i+1,                    // height
					bytes.RandHexBytes(32), //txhash
				),
			)
		}
		require.Equal(t, selfPower0, delegatee.SelfPower)
		require.Equal(t, totalPower0, delegatee.TotalPower)

		oriStakes := make([]*Stake, delegatee.StakesLen())
		for i, s0 := range delegatee.GetAllStakes() {
			oriStakes[i] = s0.Clone()
		}

		slashedPower0 := delegatee.DoSlash(govParams.SlashRate())

		expectedSlashedPower := int64(0)
		for _, s0 := range oriStakes {
			slashedPower := (s0.Power * int64(govParams.SlashRate())) / int64(100)
			if slashedPower < 1 {
				slashedPower = s0.Power
			}

			expectedPower := s0.Power - slashedPower
			require.NotEqual(t, expectedPower, s0.Power)
			require.True(t, expectedPower >= 0)

			_, s1 := delegatee.FindStake(s0.TxHash)

			if expectedPower == 0 {
				// removed
				require.Nil(t, s1)
			} else {
				require.NotNil(t, s1)
				require.Equal(t, s0.TxHash, s1.TxHash)
				require.Equal(t, expectedPower, s1.Power)
			}

			expectedSlashedPower += slashedPower
		}
		require.Equal(t, expectedSlashedPower, slashedPower0)
		require.Equal(t, totalPower0-slashedPower0, delegatee.TotalPower)
		require.Equal(t, totalPower0-slashedPower0, delegatee.SumPowerOf(nil))
	}
}

func TestAddStake(t *testing.T) {
	delegatee := newDelegatee()

	power0 := bytes.RandInt64N(ctrlertypes.MaxTotalPower())
	delegatee.AddStake(
		NewStakeWithPower(
			delegatee.Addr,
			delegatee.Addr,
			power0,
			rand.Int63n(1_000_000), // height
			bytes.RandHexBytes(32), //txhash
		),
	)

	require.Equal(t, power0, delegatee.GetSelfPower())
	require.Equal(t, power0, delegatee.GetTotalPower())

	from1 := types.RandAddress()
	power1 := bytes.RandInt64N(ctrlertypes.MaxTotalPower())
	delegatee.AddStake(
		NewStakeWithPower(
			from1,
			delegatee.Addr,
			power1,
			rand.Int63n(1_000_000), // height
			bytes.RandHexBytes(32), //txhash
		),
	)

	require.Equal(t, power0, delegatee.GetSelfPower())
	require.Equal(t, power1, delegatee.SumPowerOf(from1))
	require.Equal(t, power0+power1, delegatee.GetTotalPower())

}

//func TestDoReward_Delegatee(t *testing.T) {
//	delegatee := stake.NewDelegatee(Wallets[1].Address(), Wallets[1].GetPubKey())
//
//	// staking
//	height0 := rand.Int63n(1_000_000)
//	power0 := bytes.RandInt64N(ctrlertypes.MaxTotalPower())
//	delegatee.AddStake(
//		stake.NewStakeWithPower(
//			delegatee.Addr,
//			delegatee.Addr,
//			power0,
//			height0+1,              // height
//			bytes.RandHexBytes(32), //txhash
//		),
//	)
//	// not rewarded - height0 is not enough
//	reward0 := delegatee.DoReward(height0, govParams.RewardPerPower())
//	require.True(t, reward0 == 0)
//	require.EqualValues(t, 0, delegatee.RewardPower)
//
//	// first reward
//	reward1 := delegatee.DoReward(height0+1, govParams.RewardPerPower())
//	require.True(t, reward1 > 0, fmt.Sprintf("delegatee: %v, reward: %v\n", delegatee, reward1))
//	require.Equal(t, reward0+reward1, delegatee.RewardPower)
//
//	// second reward
//	reward2 := delegatee.DoReward(height0+2, govParams.RewardPerPower())
//	require.True(t, reward2 > 0)
//	require.Equal(t, reward1, reward2)
//	require.Equal(t, reward0+reward1+reward2, delegatee.RewardPower)
//}
//
//func BenchmarkApplyReward(b *testing.B) {
//	delegatee := stake.NewDelegatee(Wallets[1].Address(), Wallets[1].GetPubKey())
//	for i := 0; i < 10000; i++ {
//		// staking
//		power := rand.Int63n(1000000000)
//		delegatee.AddStake(
//			stake.NewStakeWithPower(
//				types.RandAddress(),
//				delegatee.Addr,
//				power,                  // power
//				rand.Int63n(1),         // height
//				bytes.RandHexBytes(32), //txhash
//			),
//		)
//	}
//
//	b.ResetTimer()
//	for i := 0; i < b.N; i++ {
//		rewarded := delegatee.DoReward(int64(i+1), govParams.RewardPerPower())
//		require.True(b, rewarded > 0)
//	}
//}

func newDelegatee() *Delegatee {
	pubBytes := secp256k1.GenPrivKey().PubKey().Bytes()
	addr, _ := crypto.PubBytes2Addr(pubBytes)
	return NewDelegatee(addr, pubBytes)
}
