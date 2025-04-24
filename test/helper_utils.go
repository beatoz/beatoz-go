package test

import (
	"errors"
	"fmt"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/libs"
	btztypes "github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	btzweb3types "github.com/beatoz/beatoz-sdk-go/types"
	btzweb3 "github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/holiman/uint256"
	tmjson "github.com/tendermint/tendermint/libs/json"
	"github.com/tendermint/tendermint/libs/rand"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	tmtypes "github.com/tendermint/tendermint/types"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	validatorWallets []*btzweb3.Wallet
	wallets          []*btzweb3.Wallet
	walletsMap       map[ctrlertypes.AcctKey]*btzweb3.Wallet
	W0               *btzweb3.Wallet
	W1               *btzweb3.Wallet
	amt              = bytes.RandU256IntN(uint256.NewInt(1000))
	defGovParams     = ctrlertypes.DefaultGovParams()
	defGas           = defGovParams.MinTrxGas()
	bigGas           = defGas * 10
	smallGas         = defGas - 1
	contractGas      = uint64(3_000_000)
	defGasPrice      = defGovParams.GasPrice()
	baseFee          = new(uint256.Int).Mul(uint256.NewInt(defGas), defGasPrice)
	//smallFee         = uint256.NewInt(999_999_999_999_999)
	defaultRpcNode *PeerMock
)

func prepareTest(_peers []*PeerMock) {

	walletsMap = make(map[ctrlertypes.AcctKey]*btzweb3.Wallet)

	bzweb3 := randBeatozWeb3()
	vals, err := queryValidators(1, bzweb3)
	if err != nil {
		panic(err)
	}
	if len(vals.Validators) > 1 {
		panic("More than one validator")
	}

	for _, peer := range _peers {
		// validators
		w := peer.PrivValWallet()
		for _, v := range vals.Validators {
			if bytes.Equal([]byte(v.Address), w.Address()) {
				addValidatorWallet(w)
			}
		}

		// wallets
		files, err := os.ReadDir(peer.WalletPath())
		if err == nil {
			for _, file := range files {
				if !file.IsDir() {
					if w, err := btzweb3.OpenWallet(
						libs.NewFileReader(filepath.Join(peer.WalletPath(), file.Name()))); err != nil {
						panic(err)
					} else {
						wallets = append(wallets, w)

						acctKey := ctrlertypes.ToAcctKey(w.Address())
						walletsMap[acctKey] = w

						bzweb3 := btzweb3.NewBeatozWeb3(btzweb3.NewHttpProvider(peer.RPCURL))
						if err := w.SyncAccount(bzweb3); err != nil {
							panic(err)
						}
						//fmt.Println("Init Holder", w.Address(), w.GetBalance())
					}
				}
			}
		}
	}

	fmt.Println("Init Holder count", len(wallets))

	//
	// validator's balance is 0 at now,
	// send amount to validators to use as gas.
	sender := wallets[0]
	_ = sender.Unlock(peers[0].Pass)
	_ = sender.SyncAccount(bzweb3)

	for _, peer := range _peers {
		w := peer.PrivValWallet()
		ret, _ := sender.TransferCommit(w.Address(), defGas, defGasPrice, btztypes.ToFons(10_000_000), bzweb3)
		if ret.CheckTx.Code != xerrors.ErrCodeSuccess {
			panic(ret.CheckTx.Code)
		}
		if ret.DeliverTx.Code != xerrors.ErrCodeSuccess {
			panic(ret.DeliverTx.Code)
		}
		sender.AddNonce()
	}

	W0 = wallets[0]
	W1 = wallets[1]
}

func waitTrxResult(txhash []byte, maxTimes int, bzweb3 *btzweb3.BeatozWeb3) (*btzweb3types.TrxResult, error) {
	for i := 0; i < maxTimes; i++ {
		time.Sleep(time.Second)

		txRet, err := bzweb3.GetTransaction(txhash)
		if err != nil && strings.Contains(err.Error(), ") not found") {
			continue
		} else if err != nil {
			return nil, err
		} else {
			return txRet, nil
		}
	}
	return nil, xerrors.NewOrdinary("timeout")
}

func waitTrxEvent(txhash []byte) (*btzweb3types.TrxResult, error) {
	var txResult *btzweb3types.TrxResult
	var retError error
	wg, err := waitEvent(fmt.Sprintf("tm.event='Tx' AND tx.hash='%X'", txhash),
		func(event *coretypes.ResultEvent, err error) bool {
			if err != nil {
				retError = err
				return true // stop
			}

			txEvt := event.Data.(tmtypes.EventDataTx)
			tx := &ctrlertypes.Trx{}
			_ = tx.Decode(txEvt.Tx)
			txResult = &btzweb3types.TrxResult{
				&coretypes.ResultTx{
					Hash:     tmtypes.Tx(txEvt.Tx).Hash(),
					Height:   txEvt.Height,
					Index:    txEvt.Index,
					TxResult: txEvt.Result,
					Tx:       txEvt.Tx,
				},
				tx,
			}
			return true
		})
	wg.Wait()

	if err != nil {
		return nil, err
	}
	if retError != nil {
		return nil, retError
	}
	return txResult, nil
}

func waitBlock(n int64) (int64, error) {
	var lastHeight int64
	var retError error

	subWg := sync.WaitGroup{}
	sub, err := btzweb3.NewSubscriber(defaultRpcNode.WSEnd)
	if err != nil {
		return 0, err
	}

	subWg.Add(1)
	err = sub.Start("tm.event='NewBlock'", func(sub *btzweb3.Subscriber, result []byte) {
		event := &coretypes.ResultEvent{}
		if retError = tmjson.Unmarshal(result, event); retError != nil {
			sub.Stop()
			subWg.Done()
		}
		lastHeight = event.Data.(tmtypes.EventDataNewBlock).Block.Height
		if lastHeight >= n {
			sub.Stop()
			subWg.Done()
		}
	})
	if err != nil {
		return 0, err
	}
	subWg.Wait()

	return lastHeight, nil
}

func waitEvent(query string, cb func(*coretypes.ResultEvent, error) bool) (*sync.WaitGroup, error) {
	subWg := sync.WaitGroup{}
	sub, err := btzweb3.NewSubscriber(defaultRpcNode.WSEnd)
	if err != nil {
		return nil, err
	}

	subWg.Add(1)
	err = sub.Start(query, func(sub *btzweb3.Subscriber, result []byte) {

		event := &coretypes.ResultEvent{}
		err := tmjson.Unmarshal(result, event)
		if cb(event, err) {
			sub.Stop()
			subWg.Done()
		}
	})
	if err != nil {
		return nil, err
	}

	return &subWg, nil
}

func addValidatorWallet(w *btzweb3.Wallet) {
	gmtx.Lock()
	defer gmtx.Unlock()

	validatorWallets = append(validatorWallets, w)
}

func randValidatorWallet() *btzweb3.Wallet {
	rn := rand.Intn(len(validatorWallets))
	return validatorWallets[rn]
}

func isValidatorWallet(w *btzweb3.Wallet) bool {
	return isValidator(w.Address())
}

func isValidator(addr btztypes.Address) bool {
	for _, vw := range validatorWallets {
		if bytes.Compare(vw.Address(), addr) == 0 {
			return true
		}
	}
	return false
}

func queryValidators(height int64, bzweb3 *btzweb3.BeatozWeb3) (*coretypes.ResultValidators, error) {
	return bzweb3.GetValidators(int64(height), 1, len(validatorWallets))
}

func randWallet() *btzweb3.Wallet {
	rn := rand.Intn(len(wallets))
	return wallets[rn]
}

func randCommonWallet() *btzweb3.Wallet {
	for {
		w := randWallet()
		if isValidatorWallet(w) == false {
			return w
		}
	}
}

func saveWallet(w *btzweb3.Wallet) error {
	path := filepath.Join(defaultRpcNode.WalletPath(), fmt.Sprintf("wk%X.json", w.Address()))
	return w.Save(libs.NewFileWriter(path))
}

func gasToFee(gas uint64, gasPrice *uint256.Int) *uint256.Int {
	return new(uint256.Int).Mul(gasPrice, uint256.NewInt(gas))
}

func transferFrom(sender *btzweb3.Wallet, receiver btztypes.Address, _amt *uint256.Int, bzweb3 *btzweb3.BeatozWeb3) error {
	if err := sender.SyncAccount(bzweb3); err != nil {
		return err
	}

	if sender.GetBalance().Cmp(_amt) < 0 {
		return errors.New("Not enough balance")
	}

	if txRet, err := sender.TransferCommit(receiver, defGas, defGasPrice, _amt, bzweb3); err != nil {
		return err
	} else if txRet.CheckTx.Code != xerrors.ErrCodeSuccess {
		return fmt.Errorf("CheckTx.Code:%v, CheckTx.Log: %v", txRet.CheckTx.Code, txRet.CheckTx.Log)
	} else if txRet.DeliverTx.Code != xerrors.ErrCodeSuccess {
		return fmt.Errorf("DeliverTx.Code:%v, DeliverTx.Log: %v", txRet.DeliverTx.Code, txRet.DeliverTx.Log)
	}
	return nil
}
