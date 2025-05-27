package test

import (
	"github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_Withdraw(t *testing.T) {
	bweb3 := randBeatozWeb3()
	w := validatorWallets[0] // active validator
	require.NoError(t, w.Unlock(defaultRpcNode.Pass))
	require.NoError(t, w.SyncAccount(bweb3))
	oriBalance := w.GetBalance()
	//fmt.Println("validator", w.Address(), "balance", oriBalance.Dec())

	govParams, err := bweb3.QueryGovParams()
	require.NoError(t, err)

	_, err = waitBlock(govParams.InflationCycleBlocks() * 2)
	require.NoError(t, err)

	rwd, err := bweb3.QueryReward(w.Address(), 0)
	require.NoError(t, err)
	//jz, err := jsonx.MarshalIndent(rwd, "", "  ")
	//require.NoError(t, err)
	//fmt.Println(string(jz))

	cumulatedRwd := uint256.MustFromDecimal(rwd.Cumulated)
	withdrawReqAmt := new(uint256.Int).Div(cumulatedRwd, uint256.NewInt(2))

	retTx, err := w.WithdrawCommit(defGas, defGasPrice, withdrawReqAmt, bweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, retTx.CheckTx.Code, retTx.CheckTx.Log)
	require.Equal(t, xerrors.ErrCodeSuccess, retTx.DeliverTx.Code, retTx.DeliverTx.Log)

	sumFee := types.GasToFee(retTx.DeliverTx.GasUsed, defGasPrice)
	rwdAmt := new(uint256.Int).Mul(sumFee, uint256.NewInt(uint64(govParams.TxFeeRewardRate())))
	rwdAmt = new(uint256.Int).Div(rwdAmt, uint256.NewInt(100))
	expectedBalance := new(uint256.Int).Add(oriBalance, withdrawReqAmt)
	_ = expectedBalance.Add(expectedBalance, rwdAmt)
	_ = expectedBalance.Sub(expectedBalance, sumFee)

	require.NoError(t, w.SyncAccount(bweb3))
	//fmt.Println("validator", w.Address(), "balance", w.GetBalance().Dec())
	require.Equal(t, expectedBalance.Dec(), w.GetBalance().Dec())

	rwd, err = bweb3.QueryReward(w.Address(), 0)
	require.NoError(t, err)
	//jz, err = jsonx.MarshalIndent(rwd, "", "  ")
	//require.NoError(t, err)
	require.Equal(t, withdrawReqAmt.Dec(), rwd.Withdrawn)
	//fmt.Println(string(jz))

	n, err := waitBlock(rwd.Height + govParams.InflationCycleBlocks() + 1)
	require.NoError(t, err)
	//fmt.Println("last.height", n)

	for h := rwd.Height; h < n; h++ {
		rwd, err = bweb3.QueryReward(w.Address(), h)
		require.NoError(t, err, "try.height", h)
		//jz, err = jsonx.MarshalIndent(rwd, "", "  ")
		//require.NoError(t, err)
		//fmt.Println("current height", h, "reward", string(jz))
	}
}

// the used GovParams's InflationBlocksCycle is too big.
// Before the block height becomes the InflationBlocksCycle or higher,
// any reward information can not be found.
// Because waiting until the block height reaches InflationBlocksCycle is too long,
// skip TestWithdraw at here
//func TestWithdraw(t *testing.T) {
//bzweb3 := randBeatozWeb3()
//val0 := validatorWallets[0] // active validator
//require.NoError(t, val0.Unlock(defaultRpcNode.Pass))
//require.NoError(t, val0.SyncAccount(bzweb3))
//
//fmt.Println("validator", val0.Address(), "balance", val0.GetBalance().Dec())
//
//at := int64(0)
//for {
//	status, err := bzweb3.Status()
//	require.NoError(t, err)
//
//	if status.SyncInfo.LatestBlockHeight > 4 {
//		at = status.SyncInfo.LatestBlockHeight
//		break
//	}
//	time.Sleep(time.Second)
//}
//
//rwd0, err := bzweb3.QueryReward(val0.Address(), at)
//require.NoError(t, err)
//require.Equal(t, 1, rwd0.GetIssued().Sign())
//require.Equal(t, uint256.NewInt(0), rwd0.GetWithdrawn())
//require.Equal(t, uint256.NewInt(0), rwd0.GetSlashed())
//require.Equal(t, 1, rwd0.GetCumulated().Cmp(rwd0.GetIssued()))
//fmt.Println("QueryReward at", at, "reward", rwd0)
//
//// try to withdraw amount more than current reward
//reqAmt := new(uint256.Int).AddUint64(rwd0.GetCumulated(), uint64(1))
//fmt.Println("try to withdraw amount", reqAmt.Dec(), "more than the cumulated reward", rwd0.GetCumulated().Dec())
//retTxCommit, err := val0.WithdrawCommit(defGas, defGasPrice, reqAmt, bzweb3)
//require.NoError(t, err)
//require.NotEqual(t, xerrors.ErrCodeSuccess, retTxCommit.CheckTx.Code, retTxCommit.CheckTx.Log)
//
//// try to withdraw amount less than current reward
//reqAmt = bytes.RandU256IntN(rwd0.GetCumulated())
//fmt.Println("try to withdraw amount", reqAmt.Dec(), "less than the cumulated reward", rwd0.GetCumulated().Dec())
//retTxCommit, err = val0.WithdrawCommit(defGas, defGasPrice, reqAmt, bzweb3)
//require.NoError(t, err)
//require.Equal(t, xerrors.ErrCodeSuccess, retTxCommit.CheckTx.Code, retTxCommit.CheckTx.Log)
//require.Equal(t, xerrors.ErrCodeSuccess, retTxCommit.DeliverTx.Code, retTxCommit.DeliverTx.Log)
//fmt.Println("Gas", retTxCommit.DeliverTx.GasWanted, retTxCommit.DeliverTx.GasUsed, "at", retTxCommit.Height)
//
//// check reward status
//rwd1, err := bzweb3.QueryReward(val0.Address(), retTxCommit.Height)
//require.NoError(t, err)
//require.Equal(t, reqAmt, rwd1.GetWithdrawn())
//fmt.Println("QueryReward at", retTxCommit.Height, "reward", rwd1)
//
//blocks := rwd1.Height() - rwd0.Height()
//
//sumIssued := new(uint256.Int).Mul(rwd1.GetIssued(), uint256.NewInt(uint64(blocks)))
//expected := new(uint256.Int).Sub(rwd0.GetCumulated(), rwd1.GetWithdrawn())
//_ = expected.Add(expected, sumIssued)
//
//require.Equal(t, expected, rwd1.GetCumulated())
//
//// check balance of val0
//oriBal := val0.GetBalance()
//usedFee := types.GasToFee(defGas, defGasPrice)
//rwdFee := new(uint256.Int).Mul(usedFee, uint256.NewInt(uint64(defGovParams.TxFeeRewardRate())))
//rwdFee = rwdFee.Div(rwdFee, uint256.NewInt(uint64(100)))
//expectedBal := new(uint256.Int).Add(oriBal, reqAmt)
//expectedBal = expectedBal.Sub(expectedBal, usedFee)
//expectedBal = expectedBal.Add(expectedBal, rwdFee)
//
//require.NoError(t, val0.SyncAccount(bzweb3))
//curBal := val0.GetBalance()
//
//require.Equal(t, expectedBal, curBal)
//}
