package web3

import (
	"strings"
	"sync"

	"github.com/holiman/uint256"
)

type BeatozWeb3 struct {
	chainId  string
	provider Provider
	callId   int64
	mtx      sync.RWMutex
}

func NewBeatozWeb3(provider Provider) *BeatozWeb3 {
	NewRequest(0, "genesis")

	bzweb3 := &BeatozWeb3{
		provider: provider,
	}
	gen, err := bzweb3.Genesis()
	if err != nil {
		panic(err)
		return nil
	}
	bzweb3.chainId = gen.ChainID
	return bzweb3
}

func (bzweb3 *BeatozWeb3) ChainID() string {
	bzweb3.mtx.RLock()
	defer bzweb3.mtx.RUnlock()

	return bzweb3.chainId
}

func (bzweb3 *BeatozWeb3) ChainIDInt() *uint256.Int {
	bzweb3.mtx.RLock()
	defer bzweb3.mtx.RUnlock()

	if strings.HasPrefix(bzweb3.chainId, "0x") {
		return uint256.MustFromHex(bzweb3.chainId)
	} else {
		return uint256.MustFromDecimal(bzweb3.chainId)
	}
}

func (bzweb3 *BeatozWeb3) SetChainID(cid string) {
	bzweb3.mtx.RLock()
	defer bzweb3.mtx.RUnlock()

	bzweb3.chainId = cid
}

func (bzweb3 *BeatozWeb3) NewRequest(method string, args ...interface{}) (*JSONRpcReq, error) {
	bzweb3.mtx.Lock()
	defer bzweb3.mtx.Unlock()

	bzweb3.callId++

	return NewRequest(bzweb3.callId, method, args...)
}
