package test

import (
	"fmt"
	rtypes1 "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/libs"
	beatozweb3 "github.com/beatoz/beatoz-go/libs/web3"
	bzweb3types "github.com/beatoz/beatoz-go/libs/web3/types"
	rtypes0 "github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	tmjson "github.com/tendermint/tendermint/libs/json"
	"github.com/tendermint/tendermint/libs/rand"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	validatorWallets []*beatozweb3.Wallet
	wallets          []*beatozweb3.Wallet
	walletsMap       map[rtypes1.AcctKey]*beatozweb3.Wallet
	W0               *beatozweb3.Wallet
	W1               *beatozweb3.Wallet
	amt              = bytes.RandU256IntN(uint256.NewInt(1000))
	defGovParams     = rtypes1.DefaultGovParams()
	defGas           = defGovParams.MinTrxGas()
	bigGas           = defGas * 10
	smallGas         = defGas - 1
	contractGas      = uint64(3_000_000)
	defGasPrice      = defGovParams.GasPrice()
	baseFee          = new(uint256.Int).Mul(uint256.NewInt(defGas), defGasPrice)
	//smallFee         = uint256.NewInt(999_999_999_999_999)
	defaultRpcNode *PeerMock
)

func prepareTest(peers []*PeerMock) {
	for _, peer := range peers {
		// validators
		if w, err := beatozweb3.OpenWallet(libs.NewFileReader(peer.PrivValKeyPath())); err != nil {
			panic(err)
		} else {
			addValidatorWallet(w)
			fmt.Println("Validator", w.Address(), w.GetBalance())
		}

		// wallets
		files, err := os.ReadDir(peer.WalletPath())
		if err != nil {
			panic(err)
		}

		walletsMap = make(map[rtypes1.AcctKey]*beatozweb3.Wallet)

		for _, file := range files {
			if !file.IsDir() {
				if w, err := beatozweb3.OpenWallet(
					libs.NewFileReader(filepath.Join(peer.WalletPath(), file.Name()))); err != nil {
					panic(err)
				} else {
					wallets = append(wallets, w)

					acctKey := rtypes1.ToAcctKey(w.Address())
					walletsMap[acctKey] = w

					bzweb3 := beatozweb3.NewBeatozWeb3(beatozweb3.NewHttpProvider(peer.RPCURL))
					if err := w.SyncAccount(bzweb3); err != nil {
						panic(err)
					}
					//fmt.Println("Init Holder", w.Address(), w.GetBalance())
				}
			}
		}
		fmt.Println("Init Holder count", len(wallets))
	}

	W0 = wallets[0]
	W1 = wallets[1]
}

func waitTrxResult(txhash []byte, maxTimes int, bzweb3 *beatozweb3.BeatozWeb3) (*bzweb3types.TrxResult, error) {
	for i := 0; i < maxTimes; i++ {
		time.Sleep(100 * time.Millisecond)

		// todo: check why it takes more than 10 secs to fetch a transaction

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

func waitBlock(n int64) {
	subWg, err := waitEvent(fmt.Sprintf("tm.event='NewBlock' AND block.height = %v", n), func(event *coretypes.ResultEvent, err error) bool {
		return true
	})
	if err != nil {
		panic(err)
	}
	subWg.Wait()
}

func waitEvent(query string, cb func(*coretypes.ResultEvent, error) bool) (*sync.WaitGroup, error) {
	subWg := sync.WaitGroup{}
	sub, err := beatozweb3.NewSubscriber(defaultRpcNode.WSEnd)
	if err != nil {
		return nil, err
	}

	subWg.Add(1)
	err = sub.Start(query, func(sub *beatozweb3.Subscriber, result []byte) {

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

func addValidatorWallet(w *beatozweb3.Wallet) {
	gmtx.Lock()
	defer gmtx.Unlock()

	validatorWallets = append(validatorWallets, w)
}

func isValidatorWallet(w *beatozweb3.Wallet) bool {
	return isValidator(w.Address())
}

func isValidator(addr rtypes0.Address) bool {
	for _, vw := range validatorWallets {
		if bytes.Compare(vw.Address(), addr) == 0 {
			return true
		}
	}
	return false
}

func queryValidators(height int, bzweb3 *beatozweb3.BeatozWeb3) (*coretypes.ResultValidators, error) {
	return bzweb3.GetValidators(int64(height), 1, len(validatorWallets))
}

func randWallet() *beatozweb3.Wallet {
	rn := rand.Intn(len(wallets))
	return wallets[rn]
}

func randValidatorWallet() *beatozweb3.Wallet {
	rn := rand.Intn(len(validatorWallets))
	return validatorWallets[rn]
}

func randCommonWallet() *beatozweb3.Wallet {
	for {
		w := randWallet()
		if isValidatorWallet(w) == false {
			return w
		}
	}
}

func saveWallet(w *beatozweb3.Wallet) error {
	path := filepath.Join(defaultRpcNode.WalletPath(), fmt.Sprintf("wk%X.json", w.Address()))
	return w.Save(libs.NewFileWriter(path))
}

func gasToFee(gas uint64, gasPrice *uint256.Int) *uint256.Int {
	return new(uint256.Int).Mul(gasPrice, uint256.NewInt(gas))
}
