package test

import (
	"bytes"
	"fmt"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/libs/web3"
	rtypes0 "github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
	"time"
)

//
// DO NOT RUN this test code yet.

func TestQueryValidators(t *testing.T) {
	bzweb3 := randBeatozWeb3()

	ret, err := queryValidators(1, bzweb3)

	require.NoError(t, err)
	for _, val := range ret.Validators {
		require.True(t, isValidator(rtypes0.Address(val.Address)))
	}
}

func TestMinSelfStakeRatio(t *testing.T) {
	bzweb3 := randBeatozWeb3()

	govParams, err := bzweb3.GetGovParams()
	require.NoError(t, err)

	valWal := validatorWallets[0]
	valStakes, err := bzweb3.GetDelegatee(valWal.Address())
	require.NoError(t, err)

	sender := randCommonWallet()
	require.NoError(t, sender.Unlock(defaultRpcNode.Pass))
	require.NoError(t, sender.SyncAccount(bzweb3))

	// get allowed delegating
	maxAllowedAmt := ctrlertypes.PowerToAmount(valStakes.TotalPower)
	ret, err := sender.StakingSync(valWal.Address(), defGas, defGasPrice, maxAllowedAmt, bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, ret.Code, ret.Log)

	sender.AddNonce()

	// not allowed delegating, because `maxAllowedAmt` is already delegated.
	ret, err = sender.StakingSync(valWal.Address(), defGas, defGasPrice, rtypes0.ToFons(1), bzweb3)
	require.NoError(t, err)
	require.NotEqual(t, xerrors.ErrCodeSuccess, ret.Code, ret.Log)
	require.True(t, strings.Contains(ret.Log, "not enough self power"), ret.Log)

	// self-staking must be allowed.
	// already stake + new stake >= govParams.MinValidatorStake
	allowedMinStake := new(uint256.Int).Sub(govParams.MinValidatorStake(), ctrlertypes.PowerToAmount(valStakes.SelfPower))
	if allowedMinStake.Sign() <= 0 {
		allowedMinStake = rtypes0.ToFons(10)
	}

	require.NoError(t, valWal.SyncAccount(bzweb3))
	require.NoError(t, valWal.Unlock(defaultRpcNode.Pass))

	ret, err = valWal.StakingSync(valWal.Address(), defGas, defGasPrice, allowedMinStake, bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, ret.Code, ret.Log, allowedMinStake.Dec())

	txRet, err := waitTrxResult(ret.Hash, 30, bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, txRet.TxResult.Code, txRet.TxResult.Log)
}

func TestInvalidStakeAmount(t *testing.T) {
	bzweb3 := randBeatozWeb3()

	w := randCommonWallet()
	require.NoError(t, w.SyncAccount(bzweb3))
	require.NoError(t, w.Unlock(defaultRpcNode.Pass))

	// too small
	stakeAmt := uint256.MustFromDecimal("1111")

	ret, err := w.StakingSync(w.Address(), defGas, defGasPrice, stakeAmt, bzweb3)
	require.NoError(t, err)
	require.NotEqual(t, xerrors.ErrCodeSuccess, ret.Code, ret.Log)

	// not multiple
	stakeAmt = uint256.MustFromDecimal("1000000000000000001")

	ret, err = w.StakingSync(w.Address(), defGas, defGasPrice, stakeAmt, bzweb3)
	require.NoError(t, err)
	require.NotEqual(t, xerrors.ErrCodeSuccess, ret.Code, ret.Log)
}

func TestDelegating(t *testing.T) {
	bzweb3 := randBeatozWeb3()

	var delegatorWallet *web3.Wallet
	for {
		delegatorWallet = randCommonWallet()
		require.NoError(t, delegatorWallet.SyncAccount(bzweb3))
		if delegatorWallet.GetBalance().Sign() > 0 {
			break
		}
	}

	vals, err := queryValidators(0, bzweb3)
	require.NoError(t, err)
	//fmt.Println("query validator power", vals.Validators[0].VotingPower)
	valAddr := rtypes0.Address(vals.Validators[0].Address)

	valStakes0, err := bzweb3.GetDelegatee(valAddr)
	require.NoError(t, err)

	stakeAmt := rtypes0.ToFons(1) // 10^18
	stakePower := int64(1)

	require.NoError(t, delegatorWallet.Unlock(defaultRpcNode.Pass))

	// self staking
	ret, err := delegatorWallet.StakingCommit(valAddr, defGas, defGasPrice, stakeAmt, bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, ret.CheckTx.Code, ret.CheckTx.Log)
	require.Equal(t, xerrors.ErrCodeSuccess, ret.DeliverTx.Code, ret.DeliverTx.Log)
	txHash := ret.Hash

	require.Equal(t, defGas, uint64(ret.DeliverTx.GasUsed))

	// check stakes
	found := false
	stakes, err := bzweb3.GetStakes(delegatorWallet.Address())
	require.NoError(t, err)
	require.True(t, len(stakes) > 0)
	for _, s0 := range stakes {
		if bytes.Compare(s0.TxHash, txHash) == 0 {
			require.Equal(t, stakeAmt, ctrlertypes.PowerToAmount(s0.Power))
			found = true
		}
	}
	require.True(t, found)

	valStakes1, err := bzweb3.GetDelegatee(valAddr)
	require.NoError(t, err)

	require.Equal(t, valStakes0.SelfPower, valStakes1.SelfPower)
	require.Equal(t, valStakes0.TotalPower+stakePower, valStakes1.TotalPower)
	require.Equal(t, new(uint256.Int).Add(ctrlertypes.PowerToAmount(valStakes0.TotalPower), stakeAmt), ctrlertypes.PowerToAmount(valStakes1.TotalPower))

	fmt.Println("Wait 5 seconds...")
	time.Sleep(5 * time.Second)

	//waitBlock(ret.Height + 4)

	vals, err = queryValidators(0, bzweb3)
	require.NoError(t, err)

	fmt.Println("query validator power", vals.Validators[0].VotingPower)
	fmt.Println("query valStakes0", valStakes0.TotalPower)
	fmt.Println("query valStakes1", valStakes1.TotalPower)
	require.Equal(t, valStakes1.TotalPower, vals.Validators[0].VotingPower)
}

func TestMinValidatorStake(t *testing.T) {
	bzweb3 := randBeatozWeb3()

	govParams, err := bzweb3.GetGovParams()
	require.NoError(t, err)

	sender := randCommonWallet()
	require.NoError(t, sender.Unlock(defaultRpcNode.Pass))
	require.NoError(t, sender.SyncAccount(bzweb3))

	minValidatorStake := govParams.MinValidatorStake()
	_amt := new(uint256.Int).Sub(minValidatorStake, ctrlertypes.PowerToAmount(1))
	ret, err := sender.StakingSync(sender.Address(), defGas, defGasPrice, _amt, bzweb3)
	require.NoError(t, err)
	require.NotEqual(t, xerrors.ErrCodeSuccess, ret.Code)
	require.Contains(t, ret.Log, "too small stake to become validator", ret.Log)
}
