package node

import (
	"github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"testing"
)

var (
	txPreparer = newTrxPreparer()
	txReqs     []*abcitypes.RequestDeliverTx
)

func init() {

	txPreparer.start()

	for i := 0; i < 10000; i++ {
		w0 := web3.NewWallet(nil)
		w1 := web3.NewWallet(nil)

		//
		// Invalid nonce
		tx := web3.NewTrxTransfer(w0.Address(), w1.Address(), 1, govParams.MinTrxGas(), govParams.GasPrice(), uint256.NewInt(1000))
		_, _, _ = w0.SignTrxRLP(tx, chainId)

		bztx, _ := tx.Encode()
		txReqs = append(txReqs, &abcitypes.RequestDeliverTx{Tx: bztx})
	}
}

func Benchmark_prepareTrxContext(b *testing.B) {
	for n := 0; n < b.N; n++ {
		for _, req := range txReqs {
			txPreparer.Add(req, func(*abcitypes.RequestDeliverTx, int) (*types.TrxContext, *abcitypes.ResponseDeliverTx) {
				txctx, xerr := newTrxCtx(req.Tx, 1)
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
			_, xerr := newTrxCtx(req.Tx, 1)
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
