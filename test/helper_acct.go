package test

import (
	"encoding/hex"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/holiman/uint256"
	"sync"
)

type acctObj struct {
	w *web3.Wallet

	originBalance *uint256.Int
	originNonce   int64

	sentTxsCnt int
	retTxsCnt  int

	txHashes        map[string]types.Address
	spentGas        *uint256.Int
	expectedBalance *uint256.Int
	expectedNonce   int64

	mtx sync.RWMutex
}

func newAcctObj(w *web3.Wallet) *acctObj {
	return &acctObj{
		w:               w,
		originBalance:   w.GetBalance(),
		originNonce:     w.GetNonce(),
		txHashes:        make(map[string]types.Address),
		spentGas:        uint256.NewInt(0),
		expectedBalance: w.GetBalance(),
		expectedNonce:   w.GetNonce(),
	}
}

func (obj *acctObj) GetTxHashes() []bytes.HexBytes {
	obj.mtx.RLock()
	defer obj.mtx.RUnlock()

	var ret []bytes.HexBytes
	for k, _ := range obj.txHashes {
		hash, err := hex.DecodeString(k)
		if err != nil {
			panic(err)
		}
		ret = append(ret, hash)
	}
	return ret

}

func (obj *acctObj) addTxHashOfAddr(txhash bytes.HexBytes, addr types.Address) {
	obj.mtx.Lock()
	defer obj.mtx.Unlock()

	obj.txHashes[txhash.String()] = addr
}
func (obj *acctObj) delTxHashOfAddr(txhash bytes.HexBytes) {
	obj.mtx.Lock()
	defer obj.mtx.Unlock()

	delete(obj.txHashes, txhash.String())
}

func (obj *acctObj) getAddrOfTxHash(txHash bytes.HexBytes) types.Address {
	obj.mtx.RLock()
	defer obj.mtx.RUnlock()

	return obj.txHashes[txHash.String()]
}

func (obj *acctObj) addSpentGas(d *uint256.Int) {
	obj.mtx.Lock()
	defer obj.mtx.Unlock()

	_ = obj.spentGas.Add(obj.spentGas, d)
}

func (obj *acctObj) addExpectedBalance(d *uint256.Int) {
	obj.mtx.Lock()
	defer obj.mtx.Unlock()

	_ = obj.expectedBalance.Add(obj.expectedBalance, d)
}

func (obj *acctObj) subExpectedBalance(d *uint256.Int) {
	obj.mtx.Lock()
	defer obj.mtx.Unlock()

	_ = obj.expectedBalance.Sub(obj.expectedBalance, d)
}

func (obj *acctObj) addExpectedNonce() {
	obj.mtx.Lock()
	defer obj.mtx.Unlock()

	obj.expectedNonce++
}
