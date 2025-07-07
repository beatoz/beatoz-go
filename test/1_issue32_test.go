package test

import (
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	tmjson "github.com/tendermint/tendermint/libs/json"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	tmtypes "github.com/tendermint/tendermint/types"
	"sync"
	"testing"
)

// https://github.com/orgs/beatoz/projects/3?pane=issue&itemId=26408019
func TestIssue32(t *testing.T) {
	bzweb3 := randBeatozWeb3()

	recv := types.RandAddress()
	var senders []*web3.Wallet
	for _, w := range wallets {
		if isValidatorWallet(w) {
			continue
		}
		senders = append(senders, w)
		if len(senders) == 400 {
			break
		}
	}
	require.NotEqualValues(t, senders[0].Address(), senders[1].Address())
	require.NotEqualValues(t, senders[1].Address(), senders[2].Address())
	require.NotEqualValues(t, senders[2].Address(), senders[0].Address())

	wg := sync.WaitGroup{}
	sub, xerr := web3.NewSubscriber(defaultRpcNode.WSEnd)
	require.NoError(t, xerr)
	defer func() {
		sub.Stop()
	}()
	xerr = sub.Start("tm.event='Tx'", func(sub *web3.Subscriber, result []byte) {
		event := &coretypes.ResultEvent{}
		require.NoError(t, tmjson.Unmarshal(result, event))

		eventData := event.Data.(tmtypes.EventDataTx)
		//txHash := event.Events["tx.hash"][0]
		//fromAddr := event.Events["tx.sender"][0]
		//toAddr := event.Events["tx.receiver"][0]
		//fmt.Println("event", "from", fromAddr, "to", toAddr, "height", eventData.Height, "txhash", txHash, "code", eventData.TxResult.Result.Code)

		require.Equal(t, xerrors.ErrCodeSuccess, eventData.TxResult.Result.Code, eventData.TxResult.Result.Log)

		wg.Done()
	})

	oriBalances := make([]*uint256.Int, len(senders))
	for i, sender := range senders {
		require.NoError(t, sender.Unlock(defaultRpcNode.Pass))
		require.NoError(t, sender.SyncAccount(bzweb3))

		oriBalances[i] = sender.GetBalance()
	}

	sendAmt := uint256.NewInt(100)
	intSumSendAmt := uint64(0)
	txCntPerSender := 10 // 400 sender * 10 txs = 4000 txs
	for i := 0; i < txCntPerSender; i++ {
		for _, sender := range senders {
			wg.Add(1)
			ret, err := sender.TransferAsync(recv, defGas, defGasPrice, sendAmt, bzweb3)
			require.NoError(t, err)
			require.Equal(t, xerrors.ErrCodeSuccess, ret.Code)
			//fmt.Println("request", "txhash", bytes.HexBytes(ret.Hash))
			intSumSendAmt += sendAmt.Uint64()

			sender.AddNonce()
		}
	}
	wg.Wait()

	sumSendAmt := uint256.NewInt(intSumSendAmt)
	recvAcct, err := bzweb3.QueryAccount(recv)
	require.NoError(t, err)
	require.Equal(t, sumSendAmt.Dec(), recvAcct.Balance.Dec())

	for i, sender := range senders {
		require.NoError(t, sender.SyncAccount(bzweb3))
		expectedBalance := new(uint256.Int).Sub(oriBalances[i], new(uint256.Int).Mul(sendAmt, uint256.NewInt(uint64(txCntPerSender))))
		_ = expectedBalance.Sub(expectedBalance, types.GasToFee(defGas*int64(txCntPerSender), defGasPrice))
		require.Equal(t, expectedBalance.Dec(), sender.GetBalance().Dec())
	}
}
