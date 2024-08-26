package test

import (
	"fmt"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestWithdraw(t *testing.T) {

	bzweb3 := randBeatozWeb3()
	val0 := randValidatorWallet()
	require.NoError(t, val0.SyncAccount(bzweb3))
	require.NoError(t, val0.Unlock(defaultRpcNode.Pass))

	at := int64(0)
	for {
		status, err := bzweb3.Status()
		require.NoError(t, err)

		if status.SyncInfo.LatestBlockHeight > 4 {
			at = status.SyncInfo.LatestBlockHeight
			break
		}
		time.Sleep(time.Second)
	}

	rwd0, err := bzweb3.QueryReward(val0.Address(), at)
	require.NoError(t, err)
	require.Equal(t, 1, rwd0.GetIssued().Sign())
	require.Equal(t, uint256.NewInt(0), rwd0.GetWithdrawn())
	require.Equal(t, uint256.NewInt(0), rwd0.GetSlashed())
	require.Equal(t, 1, rwd0.GetCumulated().Cmp(rwd0.GetIssued()))
	fmt.Println("QueryReward at", at, "reward", rwd0)

	// try to withdraw amount more than current reward
	reqAmt := new(uint256.Int).AddUint64(rwd0.GetCumulated(), uint64(1))
	fmt.Println("try to withdraw amount", reqAmt.Dec(), "more than the cumulated reward", rwd0.GetCumulated().Dec())
	retTxCommit, err := val0.WithdrawCommit(defGas, defGasPrice, reqAmt, bzweb3)
	require.NoError(t, err)
	require.NotEqual(t, xerrors.ErrCodeSuccess, retTxCommit.CheckTx.Code, retTxCommit.CheckTx.Log)

	// try to withdraw amount less than current reward
	reqAmt = bytes.RandU256IntN(rwd0.GetCumulated())
	fmt.Println("try to withdraw amount", reqAmt.Dec(), "less than the cumulated reward", rwd0.GetCumulated().Dec())
	retTxCommit, err = val0.WithdrawCommit(defGas, defGasPrice, reqAmt, bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, retTxCommit.CheckTx.Code, retTxCommit.CheckTx.Log)
	require.Equal(t, xerrors.ErrCodeSuccess, retTxCommit.DeliverTx.Code, retTxCommit.DeliverTx.Log)

	// check reward status
	rwd1, err := bzweb3.QueryReward(val0.Address(), retTxCommit.Height)
	require.NoError(t, err)
	require.Equal(t, reqAmt, rwd1.GetWithdrawn())
	fmt.Println("QueryReward at", retTxCommit.Height, "reward", rwd1)

	blocks := rwd1.Height() - rwd0.Height()

	sumIssued := new(uint256.Int).Mul(rwd1.GetIssued(), uint256.NewInt(uint64(blocks)))
	expected := new(uint256.Int).Sub(rwd0.GetCumulated(), rwd1.GetWithdrawn())
	_ = expected.Add(expected, sumIssued)

	require.Equal(t, expected, rwd1.GetCumulated())

	// check balance of val0
	oriBal := val0.GetBalance()

	time.Sleep(3 * time.Second)

	require.NoError(t, val0.SyncAccount(bzweb3))

	curBal := val0.GetBalance()
	expectedBal := new(uint256.Int).Add(oriBal, reqAmt)
	require.Equal(t, expectedBal, curBal)

	fmt.Println(oriBal.Dec(), reqAmt.Dec(), curBal.Dec())
}
