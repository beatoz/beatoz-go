package web3

import (
	"github.com/beatoz/beatoz-go/libs/web3/types"
	"sync"
)

type BeatozWeb3 struct {
	chainId  string
	provider types.Provider
	callId   int64
	mtx      sync.RWMutex
}

func NewBeatozWeb3(provider types.Provider) *BeatozWeb3 {
	types.NewRequest(0, "genesis")

	bzweb3 := &BeatozWeb3{
		provider: provider,
	}
	gen, err := bzweb3.Genesis()
	if err != nil {
		panic(err)
		return nil
	}
	bzweb3.chainId = gen.Genesis.ChainID
	return bzweb3
}

func (bzweb3 *BeatozWeb3) ChainID() string {
	bzweb3.mtx.RLock()
	defer bzweb3.mtx.RUnlock()

	return bzweb3.chainId
}

func (bzweb3 *BeatozWeb3) SetChainID(cid string) {
	bzweb3.mtx.RLock()
	defer bzweb3.mtx.RUnlock()

	bzweb3.chainId = cid
}

func (bzweb3 *BeatozWeb3) NewRequest(method string, args ...interface{}) (*types.JSONRpcReq, error) {
	bzweb3.mtx.Lock()
	defer bzweb3.mtx.Unlock()

	bzweb3.callId++

	return types.NewRequest(bzweb3.callId, method, args...)
}
