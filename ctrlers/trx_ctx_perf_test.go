package ctrlers

import (
	"fmt"
	"github.com/beatoz/beatoz-go/ctrlers/mocks/account"
	types2 "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	"math/rand"
	"sync"
	"testing"
	"time"
)

var (
	txbzs       [][]byte
	acctHandler *account.AcctHandlerMock
	govParams   types2.IGovParams
)

func init() {
	govParams = types2.DefaultGovParams()
	acctHandler = account.NewAccountHandlerMock(20000)
	wals := acctHandler.GetAllWallets()
	for _, w0 := range wals {
		tx := web3.NewTrxTransfer(w0.Address(), types.RandAddress(), rand.Uint64(), govParams.MinTrxGas(), govParams.GasPrice(), bytes.RandU256IntN(uint256.NewInt(1_000_000_000_000_000)))
		if bz, _, err := w0.SignTrxRLP(tx, "test_chain_id"); err != nil {
			panic(err)
		} else if tx.Sig = bz; tx.Sig == nil {
			panic("not reachable")
		} else if txbz, xerr := tx.Encode(); xerr != nil {
			panic(xerr)
		} else {
			txbzs = append(txbzs, txbz)
		}
	}
}

func BenchmarkNewTrxContext_Sync(b *testing.B) {
	for i := 0; i < b.N; i++ {
		xerr := newTrxCtx(txbzs[i%len(txbzs)])
		require.NoError(b, xerr, fmt.Sprintf("index: %v", i))
	}
}

func BenchmarkNewTrxContext_ASync(b *testing.B) {
	wg := &sync.WaitGroup{}
	for i := 0; i < b.N; i++ {
		wg.Add(1)
		go func() {
			xerr := newTrxCtx(txbzs[i%len(txbzs)])
			require.NoError(b, xerr, fmt.Sprintf("index: %v", i))
			wg.Done()
		}()
	}
	wg.Wait()
}

func newTrxCtx(txbz []byte) xerrors.XError {
	_, xerr := types2.NewTrxContext(txbz,
		rand.Int63(),
		time.Now().Unix(), // issue #39: set block time expected to be executed.
		true,
		func(_txctx *types2.TrxContext) xerrors.XError {
			_txctx.ChainID = "test_chain_id"
			_txctx.GovParams = govParams
			_txctx.AcctHandler = acctHandler
			return nil
		})
	return xerr
}
