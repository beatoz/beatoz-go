package ctrlers

import (
	"fmt"
	"github.com/beatoz/beatoz-go/ctrlers/mocks/acct"
	"github.com/beatoz/beatoz-go/ctrlers/mocks/gov"
	types2 "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
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
	acctHandler *acct.AcctHandlerMock
	govHandler  *gov.GovHandlerMock
)

func init() {
	govHandler = gov.NewGovHandlerMock(types2.DefaultGovParams())
	acctHandler = acct.NewAccountHandlerMock(20000)
	wals := acctHandler.GetAllWallets()
	for _, w0 := range wals {
		tx := web3.NewTrxTransfer(w0.Address(), types.RandAddress(), rand.Int63(), govHandler.MinTrxGas(), govHandler.GasPrice(), bytes.RandU256IntN(uint256.NewInt(1_000_000_000_000_000)))
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
		_, xerr := types2.NewTrxContext(txbzs[i%len(txbzs)],
			types2.TempBlockContext(
				"test_chain_id", rand.Int63(), time.Now(), govHandler, acctHandler, nil, nil, nil),
			true,
		)
		require.NoError(b, xerr, fmt.Sprintf("index: %v", i))
	}
}

func BenchmarkNewTrxContext_ASync(b *testing.B) {
	wg := &sync.WaitGroup{}
	for i := 0; i < b.N; i++ {
		wg.Add(1)
		go func() {
			_, xerr := types2.NewTrxContext(txbzs[i%len(txbzs)],
				types2.TempBlockContext(
					"test_chain_id", rand.Int63(), time.Now(), govHandler, acctHandler, nil, nil, nil),
				true,
			)
			require.NoError(b, xerr, fmt.Sprintf("index: %v", i))
			wg.Done()
		}()
	}
	wg.Wait()
}
