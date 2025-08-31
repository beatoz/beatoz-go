package test

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	rtypes0 "github.com/beatoz/beatoz-go/types"
	bytes2 "github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestQueryValidators(t *testing.T) {
	require.NoError(t, checkValidatorSet(1))
}
func TestStaking(t *testing.T) {
	bzweb3 := randBeatozWeb3()

	newValWal := peers[1].PrivValWallet() // not validator yet
	require.NoError(t, newValWal.Unlock(peers[1].Pass))
	require.NoError(t, newValWal.SyncAccount(bzweb3))
	_, err := bzweb3.QueryDelegatee(newValWal.Address())
	require.Error(t, err)

	govParams, err := bzweb3.GetGovParams()
	require.NoError(t, err)

	//
	// too small amount
	power := govParams.MinValidatorPower() - 1
	powAmt := rtypes0.PowerToAmount(power)

	ret, err := newValWal.StakingCommit(newValWal.Address(), defGas, defGasPrice, powAmt, bzweb3)
	require.NoError(t, err)
	require.NotEqual(t, xerrors.ErrCodeSuccess, ret.CheckTx.Code, ret.CheckTx.Log, "power", power, "balance", newValWal.GetBalance().Dec())
	require.Contains(t, ret.CheckTx.Log, "too small power to become validator", ret.CheckTx.Log)

	//
	// sufficient amount
	power = govParams.MinValidatorPower()
	powAmt = rtypes0.PowerToAmount(power)

	ret, err = newValWal.StakingCommit(newValWal.Address(), defGas, defGasPrice, powAmt, bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, ret.CheckTx.Code, ret.CheckTx.Log)
	require.Equal(t, xerrors.ErrCodeSuccess, ret.DeliverTx.Code, ret.DeliverTx.Log)
	txHash := ret.Hash

	// check stakes
	require.NoError(t, checkStake(newValWal.Address(), power, txHash))
	require.NoError(t, checkDelegatee(newValWal.Address(), power, power))

	valStakes1, err := bzweb3.QueryDelegatee(newValWal.Address())
	require.NoError(t, err)
	require.Equal(t, power, valStakes1.SelfPower)
	require.Equal(t, power, valStakes1.TotalPower)

	addValidatorWallet(newValWal)

	lastHeight, err := waitBlock(ret.Height + 4)
	require.NoError(t, err)
	require.NoError(t, checkValidatorSet(lastHeight))
}

func TestInvalidStakeAmount(t *testing.T) {
	bzweb3 := randBeatozWeb3()

	newValWal := peers[1].PrivValWallet()
	require.NoError(t, newValWal.SyncAccount(bzweb3))
	require.NoError(t, newValWal.Unlock(defaultRpcNode.Pass))

	// not multiple
	stakeAmt := uint256.MustFromDecimal("1000000000000000001")

	ret, err := newValWal.StakingSync(newValWal.Address(), defGas, defGasPrice, stakeAmt, bzweb3)
	require.NoError(t, err)
	require.NotEqual(t, xerrors.ErrCodeSuccess, ret.Code, ret.Log)
}

func TestMinValidatorPower(t *testing.T) {
	bzweb3 := randBeatozWeb3()

	// self-staking must be allowed always
	// when already power + new power >= govParams.MinValidatorPower.
	// The `peers[0]` already has the power equal to MinValidatorPower.
	// So, any power quantity, even less than MinValidatorPower, must be allowed.
	valWal := peers[0].PrivValWallet()
	require.NoError(t, valWal.SyncAccount(bzweb3))
	require.NoError(t, valWal.Unlock(peers[0].Pass))
	power := int64(1)
	powAmt := rtypes0.PowerToAmount(power)
	ret, err := valWal.StakingSync(valWal.Address(), defGas, defGasPrice, powAmt, bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, ret.Code, ret.Log, "power", power, "balance", valWal.GetBalance().Dec())

	txRet, err := waitTrxResult(ret.Hash, 30, bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, txRet.TxResult.Code, txRet.TxResult.Log, "balance", valWal.GetBalance().Dec())
}

func TestDelegating(t *testing.T) {
	bzweb3 := randBeatozWeb3()

	govParams, err := bzweb3.GetGovParams()
	require.NoError(t, err)

	valWal := peers[0].PrivValWallet()
	valStakes0, err := bzweb3.QueryDelegatee(valWal.Address())
	require.NoError(t, err)

	stakePower := govParams.MinDelegatorPower()
	stakeAmt := rtypes0.PowerToAmount(stakePower)

	delegator := randCommonWallet()
	require.NoError(t, delegator.SyncAccount(bzweb3))
	require.NoError(t, delegator.Unlock(defaultRpcNode.Pass))

	ret, err := delegator.StakingCommit(valWal.Address(), defGas, defGasPrice, stakeAmt, bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, ret.CheckTx.Code, ret.CheckTx.Log)
	require.Equal(t, xerrors.ErrCodeSuccess, ret.DeliverTx.Code, ret.DeliverTx.Log)
	txHash := ret.Hash

	require.Equal(t, defGas, ret.DeliverTx.GasUsed)

	// check stakes
	require.NoError(t, checkStake(delegator.Address(), stakePower, txHash))
	require.NoError(t, checkDelegatee(valWal.Address(), valStakes0.TotalPower+stakePower, valStakes0.SelfPower))

	_, err = waitBlock(ret.Height + 4)
	require.NoError(t, err)
	require.NoError(t, checkValidator(valWal.Address(), valStakes0.TotalPower+stakePower, 0))
}

func TestMinDelegatorPower(t *testing.T) {
	bzweb3 := randBeatozWeb3()

	govParams, err := bzweb3.GetGovParams()
	require.NoError(t, err)

	delegator := randCommonWallet()
	require.NoError(t, delegator.Unlock(defaultRpcNode.Pass))
	require.NoError(t, delegator.SyncAccount(bzweb3))

	valWal := peers[0].PrivValWallet()

	// not allowed
	powAmt := rtypes0.PowerToAmount(bytes2.RandInt64N(govParams.MinDelegatorPower())) // < MinDelegatorPower
	ret, err := delegator.StakingSync(valWal.Address(), defGas, defGasPrice, powAmt, bzweb3)
	require.NoError(t, err)
	require.NotEqual(t, xerrors.ErrCodeSuccess, ret.Code, ret.Log)
	require.True(t, strings.Contains(ret.Log, "invalid delegation: must be >="), ret.Log)
}

func TestDelegating_OverMinSelfStakeRatio(t *testing.T) {
	bzweb3 := randBeatozWeb3()

	govParams, err := bzweb3.QueryGovParams()
	require.NoError(t, err)

	valWal := peers[0].PrivValWallet()
	valStakes, err := bzweb3.QueryDelegatee(valWal.Address())
	require.NoError(t, err)

	//json, _ := jsonx.MarshalIndent(valStakes, "", "  ")
	//fmt.Println(string(json))

	delegator := randCommonWallet()
	require.NoError(t, delegator.Unlock(defaultRpcNode.Pass))
	require.NoError(t, delegator.SyncAccount(bzweb3))

	//
	// max...
	maxAllowedPower := valStakes.SelfPower * int64(100) / int64(govParams.MinSelfPowerRate())
	maxAllowedPower = maxAllowedPower - valStakes.TotalPower
	maxAllowedAmt := rtypes0.PowerToAmount(maxAllowedPower)

	fmt.Println("TotalPower", valStakes.TotalPower)
	fmt.Println("MinSelfPowerRate", govParams.MinSelfPowerRate())
	fmt.Println("maxAllowedPower", maxAllowedPower)
	fmt.Println("maxAllowedPower", maxAllowedPower)
	fmt.Println("maxAllowedAmt", maxAllowedAmt.Dec())

	// expected error triggered by govParams.MaxUpdatablePowerRate()
	ret, err := delegator.StakingCommit(valWal.Address(), defGas, defGasPrice, maxAllowedAmt, bzweb3)
	require.NoError(t, err)
	require.NotEqual(t, xerrors.ErrCodeSuccess, ret.CheckTx.Code, ret.CheckTx.Log)
	require.True(t, strings.Contains(ret.CheckTx.Log, "allowedRate"), ret.CheckTx.Log)

	//
	// Delegate in parts to avoid the error triggered by govParams.MaxUpdatablePowerRate().
	n, remain := int64(0), int64(0)
	updatablePow := (valStakes.TotalPower * int64(govParams.MaxUpdatablePowerRate()) / int64(100)) - 1
	for {
		remain = maxAllowedPower % updatablePow
		if remain >= govParams.MinDelegatorPower() {
			// to avoid the error triggered by govParams.MinDelegatorPower().
			n = maxAllowedPower / updatablePow
			break
		}
		updatablePow -= 1
	}

	for i := 0; i < int(n); i++ {
		ret, err = delegator.StakingCommit(valWal.Address(), defGas, defGasPrice, rtypes0.PowerToAmount(updatablePow), bzweb3)
		require.NoError(t, err)
		require.Equal(t, xerrors.ErrCodeSuccess, ret.CheckTx.Code, ret.CheckTx.Log)
		require.Equal(t, xerrors.ErrCodeSuccess, ret.DeliverTx.Code, ret.DeliverTx.Log)
		delegator.AddNonce()
	}
	// process for remain
	ret, err = delegator.StakingCommit(valWal.Address(), defGas, defGasPrice, rtypes0.PowerToAmount(remain), bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, ret.CheckTx.Code, ret.CheckTx.Log)
	require.Equal(t, xerrors.ErrCodeSuccess, ret.DeliverTx.Code, ret.DeliverTx.Log)
	delegator.AddNonce()

	// At now, the peers[0] has maximum power delegating from others.
	// Any more delegation should not be allowed, because the peers[0] is already delegated by maximum power.
	ret, err = delegator.StakingCommit(valWal.Address(), defGas, defGasPrice, rtypes0.ToGrans(4000), bzweb3)
	require.NoError(t, err)
	require.NotEqual(t, xerrors.ErrCodeSuccess, ret.CheckTx.Code, ret.CheckTx.Log)
	require.True(t, strings.Contains(ret.CheckTx.Log, "not enough self power"), ret.CheckTx.Log)

}

func checkDelegatee(localValAddr rtypes0.Address, expectedTotalPower, expectedSelfPower int64) error {
	bzweb3 := randBeatozWeb3()

	val, err := bzweb3.QueryDelegatee(localValAddr)
	if err != nil {
		return err
	}
	if expectedTotalPower != val.TotalPower {
		return errors.New("total power is mismatch")
	}
	if expectedSelfPower != val.SelfPower {
		return errors.New("self power is mismatch")
	}
	return nil
}

func checkStake(addr rtypes0.Address, expectedPower int64, txhash []byte) error {
	bzweb3 := randBeatozWeb3()
	stakes, err := bzweb3.QueryStakes(addr)
	if err != nil {
		return err
	}

	found := false
	for _, s0 := range stakes {
		if bytes.Compare(s0.TxHash, txhash) == 0 {
			if found {
				return errors.New("already found stake in stakes")
			}
			if expectedPower != s0.Power {
				return errors.New("power is mismatch")
			}
			found = true
		}
	}
	if !found {
		return errors.New("stake not found in stakes")
	}
	return nil
}

func checkValidator(valAddr rtypes0.Address, expectedPower, height int64) error {
	bzweb3 := randBeatozWeb3()
	ret, err := queryValidators(height, bzweb3)
	if err != nil {
		return err
	}

	found := false
	for _, val := range ret.Validators {
		if bytes.Equal(val.Address, valAddr) {
			if found {
				return errors.New("already found validator")
			}
			if expectedPower >= 0 && expectedPower != val.VotingPower {
				return errors.New("power is mismatch")
			}
			found = true
		}
	}
	if !found {
		return errors.New("validator not found")
	}
	return nil
}

func checkValidatorSet(height int64) error {
	bzweb3 := randBeatozWeb3()
	ret, err := queryValidators(height, bzweb3)
	if err != nil {
		return err
	}

	if len(validatorWallets) != len(ret.Validators) {
		return fmt.Errorf("validators length mismatch - wallet:%v, validators:%v", len(validatorWallets), len(ret.Validators))
	}
	for _, localVal := range validatorWallets {
		found := false
		for _, val := range ret.Validators {
			if bytes.Equal(val.Address, localVal.Address()) {
				if found {
					return errors.New("validator is duplicated")
				}
				found = true

				fmt.Println("checkValidators", "Validator", val.Address, "power", val.VotingPower)
			}
		}
		if !found {
			return errors.New("validator not found")
		}
	}
	return nil
}
