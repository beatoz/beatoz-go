package test

import (
	"fmt"
	types3 "github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	"testing"
)

// when tx fee should be given to the validator(proposer) but the validator does not exist in ledger,
// a panic is generated in previous version.
func TestTransfer0(t *testing.T) {

	bzweb3 := randBeatozWeb3()

	sender := randCommonWallet()
	require.NoError(t, sender.Unlock([]byte("1111")))
	require.NoError(t, sender.SyncAccount(bzweb3))
	senderBalance := sender.GetBalance()

	receiver := randCommonWallet()
	require.NoError(t, receiver.Unlock([]byte("1111")))
	require.NoError(t, receiver.SyncAccount(bzweb3))
	receiverBalance := sender.GetBalance()

	fmt.Println("sender", sender.Address(), "receiver", receiver.Address())
	fmt.Println("before: sender balance", senderBalance.Dec(), "receiver balance", receiverBalance.Dec())
	fmt.Println("transfer 1 BTOZ")

	txRet, err := sender.TransferCommit(receiver.Address(), defGas, defGasPrice, types3.ToGrans(1), bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, txRet.CheckTx.Code, txRet.CheckTx.Log)
	require.Equal(t, xerrors.ErrCodeSuccess, txRet.DeliverTx.Code, txRet.DeliverTx.Log)

	senderBalance.Sub(senderBalance, types3.GasToFee(defGas, defGasPrice))
	senderBalance.Sub(senderBalance, types3.ToGrans(1))
	receiverBalance.Add(receiverBalance, types3.ToGrans(1))

	require.NoError(t, sender.SyncAccount(bzweb3))
	require.NoError(t, receiver.SyncAccount(bzweb3))
	fmt.Println("after: sender balance", senderBalance.Dec(), "receiver balance", receiverBalance.Dec())

	require.Equal(t, senderBalance.Dec(), sender.GetBalance().Dec())
	require.Equal(t, receiverBalance.Dec(), receiver.GetBalance().Dec())

	// For next test
	// transfer asset to validator
	_amt := new(uint256.Int).Div(sender.GetBalance(), uint256.NewInt(2))
	val0 := validatorWallets[0]
	require.NoError(t, val0.SyncAccount(bzweb3))
	validatorBalance := val0.GetBalance()
	require.NoError(t, sender.SyncAccount(bzweb3))
	senderBalance = sender.GetBalance()

	fmt.Println("--------------------------------------------------------")
	fmt.Println("sender", sender.Address(), "validator0", val0.Address())
	fmt.Println("before: sender balance", senderBalance.Dec(), "validator0 balance", validatorBalance.Dec())
	fmt.Println("transfer", _amt.Dec())

	txRet, err = sender.TransferCommit(val0.Address(), defGas, defGasPrice, _amt, bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, txRet.CheckTx.Code, txRet.CheckTx.Log)
	require.Equal(t, xerrors.ErrCodeSuccess, txRet.DeliverTx.Code, txRet.DeliverTx.Log)

	senderBalance.Sub(senderBalance, types3.GasToFee(defGas, defGasPrice))
	senderBalance.Sub(senderBalance, _amt)
	validatorBalance.Add(validatorBalance, _amt)
	_rwd := types3.GasToFee(defGas, defGasPrice)
	_rwd = _rwd.Mul(_rwd, uint256.NewInt(uint64(defGovParams.TxFeeRewardRate())))
	_rwd = _rwd.Div(_rwd, uint256.NewInt(uint64(100)))
	validatorBalance.Add(validatorBalance, _rwd)

	require.NoError(t, sender.SyncAccount(bzweb3))
	require.NoError(t, val0.SyncAccount(bzweb3))
	fmt.Println("after: sender balance", sender.GetBalance().Dec(), "validator0 balance", val0.GetBalance().Dec())

	require.Equal(t, senderBalance.Dec(), sender.GetBalance().Dec())
	require.Equal(t, validatorBalance.Dec(), val0.GetBalance().Dec())

}

/*
99999998999000000000000000 10000000000000000000000000
49999999499500000000000000 49999999499500000000000000
49999999498500000000000000 59999999499500000000000000
                           59999999500400000000000000 (expected)
*/

// Disable test case
// the validator has already over minPower.

//func TestStaking2GenesisValidator(t *testing.T) {
//	bzweb3 := randBeatozWeb3()
//	govParams, err := bzweb3.GetGovParams()
//	require.NoError(t, err)
//
//	valWal := validatorWallets[0]
//	require.NoError(t, valWal.SyncAccount(bzweb3))
//	require.NoError(t, valWal.Unlock(defaultRpcNode.Pass))
//
//	valStakes0, err := bzweb3.GetDelegatee(valWal.Address())
//	require.NoError(t, err)
//
//	fmt.Println("valStake0.SelfAmount", valStakes0.SelfPower)
//
//	amtStake := uint256.NewInt(1000000000000000000)
//	ret, err := valWal.StakingCommit(valWal.Address(), defGas, defGasPrice, amtStake, bzweb3)
//	require.NoError(t, err)
//	require.NotEqual(t, xerrors.ErrCodeSuccess, ret.CheckTx.Code)
//	require.Contains(t, ret.CheckTx.Log, "too small stake to become validator", ret.CheckTx.Log)
//
//	amtStake = new(uint256.Int).Sub(govParams.MinValidatorStake(), types.PowerToAmount(valStakes0.SelfPower))
//	ret, err = valWal.StakingCommit(valWal.Address(), defGas, defGasPrice, amtStake, bzweb3)
//	require.NoError(t, err)
//	require.Equal(t, xerrors.ErrCodeSuccess, ret.DeliverTx.Code, ret.DeliverTx.Log)
//
//	stakes, err := bzweb3.GetStakes(valWal.Address())
//	require.NoError(t, err)
//	require.Equal(t, 2, len(stakes), stakes)
//
//	found := false
//	for _, s := range stakes {
//		if bytes.Compare(ret.Hash, s.TxHash) == 0 {
//			found = true
//			break
//		}
//	}
//	require.True(t, found)
//
//	valStakes1, err := bzweb3.GetDelegatee(valWal.Address())
//	require.NoError(t, err)
//	require.Equal(t,
//		new(uint256.Int).Add(types.PowerToAmount(valStakes0.TotalPower), amtStake),
//		types.PowerToAmount(valStakes1.TotalPower))
//	require.Equal(t, valStakes1.TotalPower,
//		valStakes1.SumPower())
//
//	//fmt.Println("valStakes1.SelfAmount", valStakes1.SelfAmount.Dec())
//
//}
