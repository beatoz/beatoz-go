package test

import (
	"encoding/hex"
	"fmt"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	tmjson "github.com/tendermint/tendermint/libs/json"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	tmtypes "github.com/tendermint/tendermint/types"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestTransfer_WrongAppHash(t *testing.T) {
	bzweb3 := randBeatozWeb3()

	w := randCommonWallet()
	require.NoError(t, w.Unlock(defaultRpcNode.Pass))
	require.NoError(t, w.SyncAccount(bzweb3))

	// event subscriber
	subWg := &sync.WaitGroup{}
	sub, err := web3.NewSubscriber(defaultRpcNode.WSEnd)
	defer func() {
		sub.Stop()
	}()
	require.NoError(t, err)
	query := fmt.Sprintf("tm.event='Tx' AND tx.sender='%v'", w.Address())
	err = sub.Start(query, func(sub *web3.Subscriber, result []byte) {

		event := &coretypes.ResultEvent{}
		err := tmjson.Unmarshal(result, event)
		require.NoError(t, err)

		eventDataTx := event.Data.(tmtypes.EventDataTx)
		require.Equal(t, xerrors.ErrCodeSuccess, eventDataTx.TxResult.Result.Code, eventDataTx.TxResult.Result.Log)

		//txHash := event.Events["tx.hash"][0]
		//fmt.Println("event - txhash:", txHash)

		subWg.Done()
	})

	for i := 0; i < 3000; i++ {
		subWg.Add(1)

		ret, err := w.TransferSync(types.RandAddress(), defGas, defGasPrice, uint256.NewInt(1), bzweb3)
		if err != nil && strings.Contains(err.Error(), "mempool is full") {
			subWg.Done()
			fmt.Println("error", err)
			time.Sleep(time.Millisecond * 3000)

			continue
		}
		require.NoError(t, err)
		require.Equal(t, xerrors.ErrCodeSuccess, ret.Code, ret.Log)
		w.AddNonce()
		//fmt.Println("transfer - txhash:", ret.Hash)
	}
	subWg.Wait()
}

func TestTransfer_GasUsed(t *testing.T) {
	bzweb3 := randBeatozWeb3()

	w := randCommonWallet()
	require.NoError(t, w.Unlock(defaultRpcNode.Pass))
	require.NoError(t, w.SyncAccount(bzweb3))

	oriBalance := w.GetBalance().Clone()

	raddr := types.RandAddress()
	trAmt := uint256.MustFromDecimal("1000")
	ret, err := w.TransferSync(raddr, defGas, defGasPrice, trAmt, bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, ret.Code, ret.Log)

	txRet, xerr := waitTrxResult(ret.Hash, 30, bzweb3)
	require.NoError(t, xerr)
	require.Equal(t, defGas, txRet.TrxObj.Gas)

	expectedBalance := new(uint256.Int).Sub(oriBalance, new(uint256.Int).Add(trAmt, baseFee))
	require.NoError(t, w.SyncAccount(bzweb3))
	require.Equal(t, expectedBalance.Dec(), w.GetBalance().Dec())

}

func TestTransfer_GasPayer(t *testing.T) {
	bzweb3 := randBeatozWeb3()

	sender := randCommonWallet()
	payer := randCommonWallet()
	require.NotEqual(t, sender.Address(), payer.Address())
	require.NoError(t, sender.Unlock(defaultRpcNode.Pass))
	require.NoError(t, sender.SyncAccount(bzweb3))
	require.NoError(t, payer.Unlock(defaultRpcNode.Pass))
	require.NoError(t, payer.SyncAccount(bzweb3))

	oriBalance := sender.GetBalance().Clone()
	oriPayerBalance := payer.GetBalance().Clone()

	raddr := types.RandAddress()
	trAmt := uint256.MustFromDecimal("1000")

	tx := web3.NewTrxTransfer(
		sender.Address(),
		raddr,
		sender.GetNonce(),
		defGas, defGasPrice,
		trAmt,
	)
	_, _, err := sender.SignTrxRLP(tx, bzweb3.ChainID())
	require.NoError(t, err)
	_, _, err = payer.SignPayerTrxRLP(tx, bzweb3.ChainID())
	require.NoError(t, err)

	ret, err := bzweb3.SendTransactionSync(tx)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, ret.Code, ret.Log)

	txRet, xerr := waitTrxResult(ret.Hash, 30, bzweb3)
	require.NoError(t, xerr)
	require.Equal(t, defGas, txRet.TrxObj.Gas)

	fee := types.GasToFee(txRet.TrxObj.Gas, defGasPrice)

	expectedPayerBalance := new(uint256.Int).Sub(oriPayerBalance, fee)
	require.NoError(t, payer.SyncAccount(bzweb3))
	require.Equal(t, expectedPayerBalance.Dec(), payer.GetBalance().Dec())

	expectedBalance := new(uint256.Int).Sub(oriBalance, trAmt)
	require.NoError(t, sender.SyncAccount(bzweb3))
	require.Equal(t, expectedBalance.Dec(), sender.GetBalance().Dec())

}

func TestTransfer_BulkCommit(t *testing.T) {
	bzweb3 := randBeatozWeb3()

	var allAcctObjs []*acctObj
	var senderAcctObjs []*acctObj
	for _, w := range wallets {
		if isValidatorWallet(w) {
			continue
		}

		require.NoError(t, w.SyncAccount(bzweb3))

		acctTestObj := newAcctObj(w)
		allAcctObjs = append(allAcctObjs, acctTestObj)
		//fmt.Println("TestBulkTransfer - used accounts:", w.Address(), w.GetNonce(), w.GetBalance())

		if len(senderAcctObjs) < 1000 && w.GetBalance().Cmp(uint256.NewInt(1000000)) >= 0 {
			senderAcctObjs = append(senderAcctObjs, acctTestObj)
		}
	}
	require.Greater(t, len(senderAcctObjs), 0)

	// add more account to be used as receive account.
	for i := len(allAcctObjs); i < 100; i++ {
		newAcctTestObj := newAcctObj(web3.NewWallet(defaultRpcNode.Pass))
		require.NoError(t, saveWallet(newAcctTestObj.w))
		allAcctObjs = append(allAcctObjs, newAcctTestObj)
	}

	wg := sync.WaitGroup{}

	fmt.Printf("TestTransfer_BulkCommit - sender accounts: %d, receiver accounts: %d\n", len(senderAcctObjs), len(allAcctObjs))
	for _, v := range senderAcctObjs {
		wg.Add(1)
		go bulkTransferCommit(t, &wg, v, allAcctObjs, 10) // 100 txs per sender
	}

	wg.Wait()

	fmt.Printf("TestTransfer_BulkCommit - Check %v accounts ...\n", len(allAcctObjs))

	sentTxsCnt, retTxsCnt := 0, 0
	for _, acctObj := range allAcctObjs {
		//fmt.Println("\tCheck account", acctObj.w.Address())

		//for k, _ := range acctObj.txHashes {
		//	hash, err := hex.DecodeString(k)
		//	require.NoError(t, err)
		//
		//	ret, err := waitTrxResult(hash, 50)
		//	require.NoError(t, err, k)
		//	require.Equal(t, xerrors.ErrCodeSuccess, ret.TxResult.Code,
		//		fmt.Sprintf("error: %v, address: %v, txhash: %v", ret.TxResult.Log, acctObj.w.Address(), ret.Hash))
		//}

		require.NoError(t, acctObj.w.SyncAccount(bzweb3))
		require.Equal(t, acctObj.expectedBalance, acctObj.w.GetBalance(), acctObj.w.Address().String())
		require.Equal(t, acctObj.expectedNonce, acctObj.w.GetNonce(), acctObj.w.Address().String())

		//fmt.Println("TestBulkTransfer", "account", acctObj.w.Address(), "nonce", acctObj.w.GetNonce(), "balance", acctObj.w.GetBalance().Dec())

		sentTxsCnt += acctObj.sentTxsCnt
		retTxsCnt += acctObj.retTxsCnt
	}

	fmt.Printf("TestTransfer_BulkCommit - senders: %d, sent txs: %d, result txs: %d\n", len(senderAcctObjs), sentTxsCnt, retTxsCnt)
}

func TestTransfer_BulkSync(t *testing.T) {
	bzweb3 := randBeatozWeb3()

	var allAcctObjs []*acctObj
	var senderAcctObjs []*acctObj

	for _, w := range wallets {
		if isValidatorWallet(w) {
			continue
		}

		require.NoError(t, w.SyncAccount(bzweb3))

		acctTestObj := newAcctObj(w)
		allAcctObjs = append(allAcctObjs, acctTestObj)

		if len(senderAcctObjs) < 1000 && w.GetBalance().Cmp(uint256.NewInt(1000000)) >= 0 {
			senderAcctObjs = append(senderAcctObjs, acctTestObj)
		}
	}
	require.Greater(t, len(senderAcctObjs), 0)

	wg := sync.WaitGroup{}

	//
	// Subscriber listens Txs.
	//
	sub, err := web3.NewSubscriber(defaultRpcNode.WSEnd)
	defer func() {
		sub.Stop()
	}()
	require.NoError(t, err)
	err = sub.Start("tm.event='Tx'", func(sub *web3.Subscriber, result []byte) {
		event := &coretypes.ResultEvent{}
		err := tmjson.Unmarshal(result, event)
		require.NoError(t, err)

		txHash, err := hex.DecodeString(event.Events["tx.hash"][0])
		require.NoError(t, err)

		eventDataTx := event.Data.(tmtypes.EventDataTx)
		tx := &ctrlertypes.Trx{}
		require.NoError(t, tx.Decode(eventDataTx.TxResult.Tx))

		// find sender account object
		var senderObj *acctObj
		for i := 0; i < len(senderAcctObjs); i++ {
			if senderAcctObjs[i].w.Address().Compare(tx.From) == 0 {
				senderObj = senderAcctObjs[i]
				break
			}
		}
		require.NotNil(t, senderObj)
		require.Equal(t, senderObj.getAddrOfTxHash(txHash), senderObj.w.Address())
		senderObj.retTxsCnt++

		wg.Done()
	})
	require.NoError(t, err)

	fmt.Printf("TestTransfer_BulkSync - sender accounts: %d, receiver accounts: %d\n", len(senderAcctObjs), len(allAcctObjs))
	for _, sender := range senderAcctObjs {
		wg.Add(1)
		go bulkTransferSync(t, &wg, sender, allAcctObjs, defaultRpcNode.Config.Mempool.Size/len(senderAcctObjs) /*txs count*/)
	}

	chMonitor := make(chan interface{})
	go func() {
		run := true
		for run {
			select {
			case _ = <-chMonitor:
				run = false
			case <-time.After(time.Second * 3):
				sumSentTxs := 0
				sumRetTxs := 0
				var txHashes []bytes.HexBytes
				for _, sender := range senderAcctObjs {
					sumSentTxs += sender.sentTxsCnt
					sumRetTxs += sender.retTxsCnt
					txHashes = append(txHashes, sender.GetTxHashes()...)
				}
				fmt.Println("monitoring: num_txs", len(txHashes), "sent", sumSentTxs, "event", sumRetTxs)

				for _, hash := range txHashes {
					resp, err := bzweb3.QueryTransaction(hash)
					if err != nil {
						//fmt.Println("query tx error", "txhash", hash, "error", err)
						continue
					}
					require.Equal(t, xerrors.ErrCodeSuccess, resp.TxResult.Code, resp.TxResult.Log)
				}
			}
		}
	}()

	wg.Wait()

	close(chMonitor)
	sumSentTxs := 0
	sumRetTxs := 0
	var txHashes []bytes.HexBytes
	for _, sender := range senderAcctObjs {
		sumSentTxs += sender.sentTxsCnt
		sumRetTxs += sender.retTxsCnt
		txHashes = append(txHashes, sender.GetTxHashes()...)
	}
	fmt.Println("TestTransfer_BulkSync - monitored: num_txs", len(txHashes), "sent", sumSentTxs, "event", sumRetTxs)

	fmt.Printf("TestTransfer_BulkSync - Check %v accounts...\n", len(allAcctObjs))

	for _, acctObj := range allAcctObjs {
		require.NoError(t, acctObj.w.SyncAccount(bzweb3))
		require.Equal(t, acctObj.expectedBalance, acctObj.w.GetBalance(), acctObj.w.Address().String())
		require.Equal(t, acctObj.expectedNonce, acctObj.w.GetNonce(), acctObj.w.Address().String())

		//fmt.Println("\tCheck account", acctObj.w.Address(), acctObj.expectedNonce, acctObj.expectedBalance, acctObj.w.GetBalance())
	}
}

var mtx sync.Mutex
var peerConns = make(map[int]int)

func TestTransfer_OverBalance(t *testing.T) {
	bzweb3 := randBeatozWeb3()

	require.NoError(t, W0.SyncBalance(bzweb3))
	require.NoError(t, W1.SyncBalance(bzweb3))
	require.NoError(t, W0.Unlock(defaultRpcNode.Pass))

	testObj0 := newAcctObj(W0)
	testObj1 := newAcctObj(W1)

	overAmt := W0.GetBalance() // baseFee is not included

	ret, err := W0.TransferSync(W1.Address(), defGas, defGasPrice, overAmt, bzweb3)
	require.NoError(t, err)
	require.NotEqual(t, xerrors.ErrCodeSuccess, ret.Code)
	//require.Equal(t, xerrors.ErrCheckTx.Wrap(xerrors.ErrInsufficientFund).Error(), ret.Log)

	require.NoError(t, W0.SyncBalance(bzweb3))
	require.NoError(t, W1.SyncBalance(bzweb3))

	require.Equal(t, testObj0.originBalance, W0.GetBalance())
	require.Equal(t, testObj1.originBalance, W1.GetBalance())
	require.Equal(t, testObj0.originNonce, W0.GetNonce())

	overAmt = new(uint256.Int).Add(new(uint256.Int).Sub(W0.GetBalance(), baseFee), uint256.NewInt(1)) // amt - baseFee + 1
	ret, err = W0.TransferSync(W1.Address(), defGas, defGasPrice, overAmt, bzweb3)
	require.NoError(t, err)
	require.NotEqual(t, xerrors.ErrCodeSuccess, ret.Code)
	//require.Equal(t, xerrors.ErrCheckTx.Wrap(xerrors.ErrInsufficientFund).Error(), ret.Log)

	require.NoError(t, W0.SyncBalance(bzweb3))
	require.NoError(t, W1.SyncBalance(bzweb3))

	require.Equal(t, testObj0.originBalance, W0.GetBalance())
	require.Equal(t, testObj1.originBalance, W1.GetBalance())
	require.Equal(t, testObj0.originNonce, W0.GetNonce())

}

func TestTransfer_WrongAddr(t *testing.T) {
	bzweb3 := randBeatozWeb3()

	require.NoError(t, W0.SyncBalance(bzweb3))
	require.NoError(t, W0.Unlock(defaultRpcNode.Pass))
	require.NotEqual(t, uint256.NewInt(0).String(), W0.GetBalance().String())

	tmpAmt := new(uint256.Int).Div(W0.GetBalance(), uint256.NewInt(2))
	ret, err := W0.TransferSync(nil, defGas, defGasPrice, tmpAmt, bzweb3)
	require.NoError(t, err)
	require.NotEqual(t, xerrors.ErrCodeSuccess, ret.Code, ret.Code)

	ret, err = W0.TransferSync([]byte{0x00}, defGas, defGasPrice, tmpAmt, bzweb3)
	require.NoError(t, err)
	require.NotEqual(t, xerrors.ErrCodeSuccess, ret.Code, ret.Code)
}

func bulkTransferSync(t *testing.T, wg *sync.WaitGroup, senderAcctObj *acctObj, receivers []*acctObj, cnt int) {
	mtx.Lock()
	peer := randPeer()
	_bzweb3 := web3.NewBeatozWeb3(web3.NewHttpProvider(peer.RPCURL))
	n, ok := peerConns[peer.PeerIdx]
	if ok {
		peerConns[peer.PeerIdx] = n + 1
	} else {
		peerConns[peer.PeerIdx] = 1
	}
	mtx.Unlock()
	peerInfo := fmt.Sprintf("peerIdx: %v, conns: %v", peer.PeerIdx, peerConns[peer.PeerIdx])

	w := senderAcctObj.w
	require.NoError(t, w.Unlock(defaultRpcNode.Pass))
	require.NoError(t, w.SyncAccount(_bzweb3))

	//
	// Broadcast txs
	//
	maxAmt := new(uint256.Int).Div(senderAcctObj.originBalance, uint256.NewInt(uint64(cnt)))
	maxAmt = new(uint256.Int).Sub(maxAmt, baseFee)

	for i := 0; i < cnt; i++ {
		rn := rand.Intn(len(receivers))
		if bytes.Compare(receivers[rn].w.Address(), w.Address()) == 0 {
			rn = (rn + 1) % len(receivers)
		}

		racctObj := receivers[rn]
		raddr := racctObj.w.Address()

		randAmt := bytes.RandU256IntN(maxAmt)
		if randAmt.Sign() == 0 {
			randAmt = uint256.NewInt(1)
		}
		needAmt := new(uint256.Int).Add(randAmt, baseFee)

		wg.Add(1)

		ret, err := w.TransferSync(raddr, defGas, defGasPrice, randAmt, _bzweb3)

		if err != nil && strings.Contains(err.Error(), "mempool is full") {
			wg.Done()
			fmt.Println("error", err.Error())
			time.Sleep(time.Millisecond * 3000)
			require.NoError(t, w.SyncAccount(_bzweb3))
			continue
		}
		require.NoError(t, err, "peerInfo", peerInfo)
		require.Equal(t, xerrors.ErrCodeSuccess, ret.Code, ret.Log, "peerInfo", peerInfo)

		racctObj.addExpectedBalance(randAmt)
		senderAcctObj.addTxHashOfAddr(bytes.HexBytes(ret.Hash), w.Address())
		senderAcctObj.addSpentGas(baseFee)
		senderAcctObj.subExpectedBalance(needAmt)
		senderAcctObj.addExpectedNonce()
		senderAcctObj.sentTxsCnt++

		w.AddNonce()
	}

	wg.Done()
}

func bulkTransferCommit(t *testing.T, wg *sync.WaitGroup, senderAcctObj *acctObj, receivers []*acctObj, cnt int) {
	w := senderAcctObj.w
	require.NoError(t, w.Unlock(defaultRpcNode.Pass))

	mtx.Lock()
	peer := randPeer()
	_bzweb3 := web3.NewBeatozWeb3(web3.NewHttpProvider(peer.RPCURL))
	n, ok := peerConns[peer.PeerIdx]
	if ok {
		peerConns[peer.PeerIdx] = n + 1
	} else {
		peerConns[peer.PeerIdx] = 1
	}
	mtx.Unlock()
	peerInfo := fmt.Sprintf("peerIdx: %v, conns: %v", peer.PeerIdx, peerConns[peer.PeerIdx])

	maxAmt := new(uint256.Int).Div(senderAcctObj.originBalance, uint256.NewInt(uint64(cnt)))
	maxAmt = new(uint256.Int).Sub(maxAmt, baseFee)

	for i := 0; i < cnt; i++ {
		rn := rand.Intn(len(receivers))
		if bytes.Compare(receivers[rn].w.Address(), w.Address()) == 0 {
			rn = (rn + 1) % len(receivers)
		}

		racctObj := receivers[rn]
		raddr := racctObj.w.Address()
		randAmt := bytes.RandU256IntN(maxAmt)
		if randAmt.Sign() == 0 {
			randAmt = uint256.NewInt(1)
		}
		needAmt := new(uint256.Int).Add(randAmt, baseFee)

		ret, err := w.TransferCommit(raddr, defGas, defGasPrice, randAmt, _bzweb3)
		require.NoError(t, err, "peerInfo", peerInfo)
		require.Equal(t, xerrors.ErrCodeSuccess, ret.CheckTx.Code, "peerInfo", peerInfo)
		require.Equal(t, xerrors.ErrCodeSuccess, ret.DeliverTx.Code, "peerInfo", peerInfo)

		racctObj.addExpectedBalance(randAmt)
		senderAcctObj.addTxHashOfAddr(bytes.HexBytes(ret.Hash), w.Address())
		senderAcctObj.addSpentGas(baseFee)
		senderAcctObj.subExpectedBalance(needAmt)
		senderAcctObj.addExpectedNonce()
		senderAcctObj.sentTxsCnt++

		w.AddNonce()
	}
	//fmt.Println(senderAcctObj.w.Address(), "sent", senderAcctObj.sentTxsCnt, "ret", senderAcctObj.retTxsCnt)

	wg.Done()
}
