package ctrlers

import (
	"github.com/beatoz/beatoz-go/cmd/config"
	"github.com/beatoz/beatoz-go/ctrlers/account"
	"github.com/beatoz/beatoz-go/ctrlers/types"
	types2 "github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	tmlog "github.com/tendermint/tendermint/libs/log"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

var (
	mempoolSize = 17_000
)

func prepare() (*account.AcctCtrler, *config.Config, [][]byte) {
	dbPath, _ := os.MkdirTemp("", "ledger_performance_test_commit_v1_*")
	root := filepath.Dir(dbPath)
	base := filepath.Base(dbPath)

	bcfg := config.DefaultConfig()
	bcfg.SetRoot(root)
	bcfg.DBPath = base
	bcfg.ChainID = "test_chain_id"

	_ctrler, xerr := account.NewAcctCtrler(bcfg, tmlog.NewNopLogger())
	if xerr != nil {
		panic(xerr)
	}

	var _txs [][]byte
	for i := 0; i < mempoolSize*2; i++ {
		// randomly create accounts
		w := web3.NewWallet(nil)
		w.GetAccount().SetBalance(uint256.MustFromDecimal("1000000000000000000000000000"))
		if xerr := _ctrler.SetAccount(w.GetAccount(), true); xerr != nil {
			panic(xerr)
		}

		// randomly create transfer transactions
		tx := web3.NewTrxTransfer(w.Address(), types2.RandAddress(), w.GetNonce(), govHandler.MinTrxGas(), govHandler.GasPrice(), uint256.NewInt(rand.Uint64()))
		if bz, _, err := w.SignTrxRLP(tx, bcfg.ChainID); err != nil {
			panic(err)
		} else if tx.Sig = bz; tx.Sig == nil {
			panic("not reachable")
		} else if txbz, xerr := tx.Encode(); xerr != nil {
			panic(xerr)
		} else {
			_txs = append(_txs, txbz)
		}
	}
	if _, _, xerr := _ctrler.Commit(); xerr != nil {
		panic(xerr)
	}
	//fmt.Println("AcctCtrler is created on", bcfg.DBPath)

	return _ctrler, bcfg, _txs
}

func Benchmark_AcctCtrler_ASync_ByChannel(b *testing.B) {
	acctCtrler, bcfg, txs := prepare()

	var txctxs []*types.TrxContext
	var mtx sync.Mutex
	var wg sync.WaitGroup
	var chIns []chan int

	for i := 0; i < runtime.NumCPU(); i++ {
		chIn := make(chan int, mempoolSize/runtime.NumCPU()+1)
		chIns = append(chIns, chIn)
		go makeTrxCtxRoutineEx(chIn, txs,
			bcfg.ChainID,
			govHandler,
			acctCtrler,
			func(_txctx *types.TrxContext) xerrors.XError {
				mtx.Lock()
				txctxs = append(txctxs, _txctx)
				mtx.Unlock()
				wg.Done()
				return nil
			})

	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		txctxs = nil

		for j := 0; j < mempoolSize; j++ {
			wg.Add(1)
			//chIns[j%len(chIns)] <- txs[j%len(txs)]
			chIns[j%len(chIns)] <- rand.Intn(len(txs))
		}

		// wait until all trxctxs are made
		wg.Wait()

		for j := 0; j < len(txctxs); j++ {
			require.NoError(b, acctCtrler.ValidateTrx(txctxs[j]))
			require.NoError(b, acctCtrler.ExecuteTrx(txctxs[j]))
		}
		_, _, xerr := acctCtrler.Commit()
		require.NoError(b, xerr)
	}
	b.StopTimer()

	for i := 0; i < len(chIns); i++ {
		close(chIns[i])
	}
	require.NoError(b, acctCtrler.Close())

	// TPS
	tps := int(float64(mempoolSize*b.N) / b.Elapsed().Seconds())
	b.Log("Benchmark_AcctCtrler_ASync_ByChannel", tps, "tps")
}

func Benchmark_AcctCtrler_ASync(b *testing.B) {
	acctCtrler, bcfg, txs := prepare()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var txctxs []*types.TrxContext
		wg := &sync.WaitGroup{}
		mtx := &sync.Mutex{}

		for j := 0; j < mempoolSize; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				txbz := txs[rand.Intn(len(txs))]
				txctx := _makeTrxCtx(txbz, bcfg.ChainID, govHandler, acctCtrler)

				mtx.Lock()
				txctxs = append(txctxs, txctx)
				mtx.Unlock()
			}()
		}
		wg.Wait()

		for j := 0; j < len(txctxs); j++ {
			require.NoError(b, acctCtrler.ValidateTrx(txctxs[j]))
			require.NoError(b, acctCtrler.ExecuteTrx(txctxs[j]))
		}
		_, _, xerr := acctCtrler.Commit()
		require.NoError(b, xerr)
	}
	b.StopTimer()

	require.NoError(b, acctCtrler.Close())

	// TPS
	tps := int(float64(mempoolSize*b.N) / b.Elapsed().Seconds())
	b.Log("Benchmark_AcctCtrler_ASync", tps, "tps")
}

func Benchmark_AcctCtrler_Sync(b *testing.B) {
	acctCtrler, bcfg, txs := prepare()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < mempoolSize; j++ {
			bctx := types.TempBlockContext(bcfg.ChainID, rand.Int63(), time.Now(), govHandler, acctCtrler, nil, nil, nil)
			txbz := txs[rand.Intn(len(txs))]
			txctx, xerr := types.NewTrxContext(txbz,
				bctx, // issue #39: set block time expected to be executed.
				true)
			require.NoError(b, xerr)
			require.NoError(b, acctCtrler.ValidateTrx(txctx))
			require.NoError(b, acctCtrler.ExecuteTrx(txctx))
		}
		_, _, xerr := acctCtrler.Commit()
		require.NoError(b, xerr)
	}

	b.StopTimer()
	require.NoError(b, acctCtrler.Close())

	// TPS
	tps := int(float64(mempoolSize*b.N) / b.Elapsed().Seconds())
	b.Log("Benchmark_AcctCtrler_Sync", tps, "tps")
}

//
//func makeTrxCtxRoutine(chIn chan []byte, chDone chan bool, cb0, cb1 func(ctx *types.TrxContext) xerrors.XError) {
//EXIT:
//	for {
//		select {
//		case txbz := <-chIn:
//			if txbz != nil {
//				txctx := _makeTrxCtx(txbz, cb0)
//				cb1(txctx)
//			}
//		case <-chDone:
//			break EXIT
//		}
//	}
//}

func makeTrxCtxRoutineEx(chIn chan int, txs [][]byte, chainId string, g types.IGovHandler, a types.IAccountHandler, cb1 func(ctx *types.TrxContext) xerrors.XError) {
	for i := range chIn {
		txbz := txs[i%len(txs)]
		txctx := _makeTrxCtx(txbz, chainId, g, a)
		cb1(txctx)
	}
}

func _makeTrxCtx(txbz []byte, chainId string, g types.IGovHandler, a types.IAccountHandler) *types.TrxContext {
	_bctx := types.TempBlockContext(chainId, rand.Int63(), time.Now(), g, a, nil, nil, nil)
	txctx, xerr := types.NewTrxContext(txbz,
		_bctx, // issue #39: set block time expected to be executed.
		true,
	)
	if xerr != nil {
		panic(xerr)
	}
	return txctx
}
