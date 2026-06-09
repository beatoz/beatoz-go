package node

import (
	"testing"
	"time"

	"github.com/beatoz/beatoz-go/ctrlers/mocks"
	"github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
)

var (
	txPreparer = newTrxPreparer(log.NewNopLogger())
	txReqs     []*abcitypes.RequestDeliverTx
)

func init() {

	txPreparer.start()

	for i := 0; i < 10000; i++ {
		w0 := acctMock.RandWallet() //web3.NewWallet(nil)
		w1 := web3.NewWallet(nil)

		//
		// Invalid nonce
		tx := web3.NewTrxTransfer(w0.Address(), w1.Address(), 1, govMock.MinTrxGas(), govMock.GasPrice(), uint256.NewInt(1000))
		_, _, _ = w0.SignTrxRLP(tx, chainId.Hex())

		bztx, _ := tx.Encode()
		txReqs = append(txReqs, &abcitypes.RequestDeliverTx{Tx: bztx})
	}
}

func Benchmark_prepareTrxContext(b *testing.B) {
	for n := 0; n < b.N; n++ {
		for _, req := range txReqs {
			txPreparer.Add(req, func(*abcitypes.RequestDeliverTx, int) (*types.TrxContext, *abcitypes.ResponseDeliverTx) {
				txctx, xerr := mocks.MakeTrxCtxWithBz(req.Tx, chainId.Hex(), 1, time.Now(), true, govMock, acctMock, nil, nil, nil)
				require.NoError(b, xerr)
				return txctx, nil
			})
		}
		txPreparer.Wait()
		require.Equal(b, len(txReqs), txPreparer.resultCount())
		txPreparer.reset()
	}
}

func Benchmark_sequentialTrxContext(b *testing.B) {
	for n := 0; n < b.N; n++ {
		for _, req := range txReqs {
			_, xerr := mocks.MakeTrxCtxWithBz(req.Tx, chainId.Hex(), 1, time.Now(), true, govMock, acctMock, nil, nil, nil)
			require.NoError(b, xerr)
		}
	}
}

func TestNilResult(t *testing.T) {
	for n := 0; n < 100; n++ {
		for _, req := range txReqs {
			txPreparer.Add(req, func(*abcitypes.RequestDeliverTx, int) (*types.TrxContext, *abcitypes.ResponseDeliverTx) {
				return &types.TrxContext{}, nil
			})
		}
		txPreparer.Wait()

		require.Equal(t, len(txReqs), txPreparer.resultCount())
		for idx, ret := range txPreparer.resultList() {
			require.NotNil(t, ret, "result is nil", "n", n, "idx", idx)
		}

		txPreparer.reset()
	}
}

func TestTrxPreparerPanic(t *testing.T) {
	tp := newTrxPreparer(log.NewNopLogger())
	tp.start()
	defer tp.stop()

	reqCount := len(tp.chReqParams) + 1
	reqs := make([]*abcitypes.RequestDeliverTx, reqCount)
	for i := 0; i < reqCount; i++ {
		reqs[i] = &abcitypes.RequestDeliverTx{Tx: []byte{byte(i)}}
		tp.Add(reqs[i], func(_ *abcitypes.RequestDeliverTx, idx int) (*types.TrxContext, *abcitypes.ResponseDeliverTx) {
			if idx == 0 {
				panic("prepare panic")
			}
			return &types.TrxContext{}, nil
		})
	}

	done := make(chan struct{})
	go func() {
		tp.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("TrxPreparer.Wait() hung after prepare panic")
	}

	require.Equal(t, reqCount, tp.resultCount())

	panicRet := tp.resultAt(0)
	require.NotNil(t, panicRet)
	require.Equal(t, 0, panicRet.idx)
	require.Same(t, reqs[0], panicRet.reqDeliverTx)
	require.Nil(t, panicRet.txctx)
	require.NotNil(t, panicRet.resDeliverTx)

	expectedErr := xerrors.ErrDeliverTx.Wrapf("transaction preparation failed")
	require.Equal(t, expectedErr.Code(), panicRet.resDeliverTx.Code)
	require.Equal(t, expectedErr.Error(), panicRet.resDeliverTx.Log)

	for idx, ret := range tp.resultList() {
		require.NotNil(t, ret, "result is nil at index %d", idx)
		require.Equal(t, idx, ret.idx)
		require.Same(t, reqs[idx], ret.reqDeliverTx)
	}

	// Index 0 and reqCount-1 are assigned to the same worker.
	// A non-nil result confirms that the worker continued after recovering.
	lastRet := tp.resultAt(reqCount - 1)
	require.NotNil(t, lastRet.txctx)
	require.Nil(t, lastRet.resDeliverTx)
}
