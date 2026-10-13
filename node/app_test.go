package node

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/beatoz/beatoz-go/cmd/config"
	"github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/genesis"
	"github.com/beatoz/beatoz-go/libs/jsonx"
	types2 "github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	tmsync "github.com/tendermint/tendermint/libs/sync"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

func makeTestWallet(privateKeyByte byte) *web3.Wallet {
	privateKey := make([]byte, 32)
	privateKey[len(privateKey)-1] = privateKeyByte
	return web3.ImportKey(privateKey, nil)
}

func newTestBeatozApp(t *testing.T, wallets []*web3.Wallet) (*BeatozApp, *beatozLocalClient, func()) {
	appState := genesis.GenesisAppState{
		AssetHolders: make([]*genesis.GenesisAssetHolder, len(wallets)),
		GovParams:    types.DefaultGovParams(),
	}
	for i, w := range wallets {
		appState.AssetHolders[i] = &genesis.GenesisAssetHolder{
			Address: w.Address(),
			Balance: types2.ToGrans(1_000),
		}
	}
	jz, err := jsonx.Marshal(appState)
	require.NoError(t, err)

	rootDir, err := os.MkdirTemp("", "beatoz-app-")
	require.NoError(t, err)
	btxcfg := config.DefaultConfig("1234")
	btxcfg.SetRoot(filepath.Join(rootDir, "beatoz-test"))

	btzApp := NewBeatozApp(btxcfg, log.NewNopLogger())
	btzClient := NewBeatozLocalClient(&tmsync.Mutex{}, btzApp)
	btzClient.SetResponseCallback(func(*abcitypes.Request, *abcitypes.Response) {})
	btzApp.SetLocalClient(btzClient)

	btzApp.Info(abcitypes.RequestInfo{})
	btzApp.InitChain(abcitypes.RequestInitChain{
		ChainId: btxcfg.ChainIdHex(),
		ConsensusParams: &abcitypes.ConsensusParams{
			Block: &abcitypes.BlockParams{
				MaxBytes: 22020096,
				MaxGas:   36_000_000,
			},
		},
		Validators: abcitypes.ValidatorUpdates{
			abcitypes.UpdateValidator(wallets[0].GetPubKey(), 1_000_000, "secp256k1"),
		},
		AppStateBytes: jz,
		InitialHeight: 1,
	})
	btzApp.Start()

	btzApp.BeginBlock(abcitypes.RequestBeginBlock{
		Header: tmproto.Header{Height: 1, ChainID: btxcfg.ChainIdHex()},
	})
	btzApp.EndBlock(abcitypes.RequestEndBlock{Height: 1})
	btzApp.Commit()

	cleanup := func() {
		btzApp.Stop()
		os.RemoveAll(rootDir)
		types.InitSigner(chainId)
	}
	return btzApp, btzClient.(*beatozLocalClient), cleanup
}

func makeTestTransferTx(
	t *testing.T,
	app *BeatozApp,
	from *web3.Wallet,
	to *web3.Wallet,
	nonce int64,
	amount uint64,
) []byte {
	tx := web3.NewTrxTransfer(
		from.Address(), to.Address(),
		nonce,
		app.govCtrler.MinTrxGas(), app.govCtrler.GasPrice(),
		uint256.NewInt(amount),
	)
	_, _, err := from.SignTrxRLP(tx, app.lastBlockCtx.ChainID())
	require.NoError(t, err)
	txbz, err := tx.Encode()
	require.NoError(t, err)
	return txbz
}

func runTestBlock(
	t *testing.T,
	app *BeatozApp,
	client *beatozLocalClient,
	txs [][]byte,
) []byte {
	var deliverTxResponses []*abcitypes.ResponseDeliverTx
	client.SetResponseCallback(func(_ *abcitypes.Request, resp *abcitypes.Response) {
		if r := resp.GetDeliverTx(); r != nil {
			deliverTxResp := *r
			deliverTxResponses = append(deliverTxResponses, &deliverTxResp)
		}
	})

	height := app.lastBlockCtx.Height() + 1
	app.BeginBlock(abcitypes.RequestBeginBlock{
		Header: tmproto.Header{
			Height:  height,
			ChainID: app.lastBlockCtx.ChainID(),
		},
	})
	for _, tx := range txs {
		app.DeliverTx(abcitypes.RequestDeliverTx{Tx: tx})
	}
	app.EndBlock(abcitypes.RequestEndBlock{Height: height})
	require.Len(t, deliverTxResponses, len(txs))
	for _, resp := range deliverTxResponses {
		require.Equal(t, abcitypes.CodeTypeOK, resp.Code, resp.Log)
	}

	commitResp := app.Commit()
	require.NotEmpty(t, commitResp.Data)
	require.Equal(t, height, app.lastBlockCtx.Height())
	return commitResp.Data
}

type testAccountSnapshot struct {
	balance string
	nonce   int64
}

func testAccountStates(
	t *testing.T,
	app *BeatozApp,
	wallets []*web3.Wallet,
) []testAccountSnapshot {
	states := make([]testAccountSnapshot, len(wallets))
	for i, wallet := range wallets {
		acct := app.acctCtrler.FindAccount(wallet.Address(), true)
		require.NotNil(t, acct)
		states[i] = testAccountSnapshot{
			balance: acct.GetBalance().Dec(),
			nonce:   acct.GetNonce(),
		}
	}
	return states
}

func Test_InitChain(t *testing.T) {
	// max total supply is less than initial total supply
	req := abcitypes.RequestInitChain{
		Validators: abcitypes.ValidatorUpdates{
			{Power: 1}, {Power: 1}, {Power: 1}, // 3000000000000000000
		},
		AppStateBytes: []byte(`{
"assetHolders": [
	{"address": "AAAAAA", "balance":"1000000000000000000"},
	{"address": "BBBBBB", "balance":"1000000000000000000"},
	{"address": "CCCCCC", "balance":"1000000000000000000"}
],
"govParams":{
	"maxTotalSupply":"5999999999999999999"
}}`)}

	_, _, err := checkRequestInitChain(req)
	require.Error(t, err)
	require.True(t, strings.HasPrefix(err.Error(), "error: initial supply"))

	// success case
	req = abcitypes.RequestInitChain{
		Validators: abcitypes.ValidatorUpdates{
			{Power: 1}, {Power: 1}, {Power: 1},
		},
		AppStateBytes: []byte(`{
"assetHolders": [
	{"address": "AAAAAA", "balance":"1000000000000000000"},
	{"address": "BBBBBB", "balance":"1000000000000000000"},
	{"address": "CCCCCC", "balance":"1000000000000000000"}
],
"govParams":{
	"maxTotalSupply":"6000000000000000000"
}}`)}

	_, genTotalSupply, err := checkRequestInitChain(req)
	require.NoError(t, err)
	require.Equal(t, "6000000000000000000", genTotalSupply.Dec())
}

func Test_AppHash(t *testing.T) {
	wallets := []*web3.Wallet{
		makeTestWallet(1),
		makeTestWallet(2),
		makeTestWallet(3),
	}
	app0, client0, cleanup0 := newTestBeatozApp(t, wallets)
	defer cleanup0()
	app1, client1, cleanup1 := newTestBeatozApp(t, wallets)
	defer cleanup1()

	txs := [][]byte{
		makeTestTransferTx(t, app0, wallets[0], wallets[1], 0, 1),
		makeTestTransferTx(t, app0, wallets[1], wallets[2], 0, 2),
		makeTestTransferTx(t, app0, wallets[0], wallets[2], 1, 3),
		makeTestTransferTx(t, app0, wallets[2], wallets[2], 0, 4),
	}

	hash0 := runTestBlock(t, app0, client0, txs)
	hash1 := runTestBlock(t, app1, client1, txs)

	require.Equal(t, hash0, hash1)
	require.Equal(t, testAccountStates(t, app0, wallets), testAccountStates(t, app1, wallets))

	emptyHash0 := runTestBlock(t, app0, client0, nil)
	emptyHash1 := runTestBlock(t, app1, client1, nil)

	require.Equal(t, emptyHash0, emptyHash1)
	require.EqualValues(t, 3, app0.lastBlockCtx.Height())
	require.EqualValues(t, 3, app1.lastBlockCtx.Height())
}

func Test_EndBlock_NoChainHalt(t *testing.T) {
	for _, test := range []struct {
		name           string
		gasDelta       int64
		gasPriceDelta  uint64
		wrongSigner    bool
		senderNotFound bool
	}{
		{
			name:          "wrong_gas_price",
			gasPriceDelta: 1,
		},
		{
			name:     "small_gas",
			gasDelta: -1,
		},
		{
			name:        "wrong_signature",
			wrongSigner: true,
		},
		{
			name:           "sender_not_found",
			senderNotFound: true,
		},
	} {
		wallets := []*web3.Wallet{makeTestWallet(1), makeTestWallet(2)}
		btzApp, btzClient, cleanup := newTestBeatozApp(t, wallets)
		defer cleanup()

		var deliverTxResp *abcitypes.ResponseDeliverTx
		btzClient.SetResponseCallback(func(_ *abcitypes.Request, resp *abcitypes.Response) {
			if r := resp.GetDeliverTx(); r != nil {
				deliverTxResp = r
			}
		})

		sender := wallets[0]
		if test.senderNotFound {
			sender = makeTestWallet(3)
		}
		signer := sender
		if test.wrongSigner {
			signer = wallets[1]
		}
		gasPrice := new(uint256.Int).Add(
			btzApp.govCtrler.GasPrice(),
			uint256.NewInt(test.gasPriceDelta),
		)
		tx := web3.NewTrxTransfer(
			sender.Address(), wallets[1].Address(),
			sender.GetNonce(),
			btzApp.govCtrler.MinTrxGas()+test.gasDelta, gasPrice,
			uint256.NewInt(1),
		)
		chainID := btzApp.lastBlockCtx.ChainID()
		_, _, err := signer.SignTrxRLP(tx, chainID)
		require.NoError(t, err, "case=%s", test.name)
		txbz, err := tx.Encode()
		require.NoError(t, err, "case=%s", test.name)

		btzApp.BeginBlock(abcitypes.RequestBeginBlock{
			Header: tmproto.Header{Height: 2, ChainID: chainID},
		})
		btzApp.DeliverTx(abcitypes.RequestDeliverTx{Tx: txbz})
		require.Equal(t, 1, btzApp.currBlockCtx.TxsCnt(), "case=%s", test.name)

		require.NotPanics(t, func() {
			btzApp.EndBlock(abcitypes.RequestEndBlock{Height: 2})
		}, "case=%s", test.name)
		require.NotNil(t, deliverTxResp, "case=%s", test.name)
		require.NotEqual(t, abcitypes.CodeTypeOK, deliverTxResp.Code, "case=%s", test.name)

		btzApp.Commit()
	}
}

func Test_AsyncExecTrxContextBTIP35_AddFee(t *testing.T) {
	wallets := []*web3.Wallet{makeTestWallet(1), makeTestWallet(2)}
	app, _, cleanup := newTestBeatozApp(t, wallets)
	defer cleanup()

	chainID := app.lastBlockCtx.ChainID()
	height := app.lastBlockCtx.Height() + 1
	require.True(t, types2.IsBTIP35(chainID, height))
	require.True(t, types2.IsBTIP45(chainID, height))

	app.BeginBlock(abcitypes.RequestBeginBlock{
		Header: tmproto.Header{Height: height, ChainID: chainID},
	})

	sender := app.acctCtrler.FindAccount(wallets[0].Address(), true)
	require.NotNil(t, sender)
	txbz := makeTestTransferTx(t, app, wallets[0], wallets[1], sender.GetNonce(), 1)
	txctx, xerr := types.NewTrxContext(txbz, app.currBlockCtx, true)
	require.NoError(t, xerr)

	failureHandler := &accountTransferFailureHandler{
		accountSetFailureHandler: &accountSetFailureHandler{
			IAccountHandler: app.acctCtrler,
			failureAddress:  wallets[1].Address(),
			setAccountErr:   xerrors.NewOrdinary("receiver account set failure"),
		},
	}
	app.currBlockCtx.AcctHandler = failureHandler

	feeBefore := app.currBlockCtx.SumFee()
	resp := app.asyncExecTrxContextBTIP35(txctx)
	feeAfter := app.currBlockCtx.SumFee()
	app.currBlockCtx.AcctHandler = app.acctCtrler

	require.NotEqual(t, abcitypes.CodeTypeOK, resp.Code)
	require.True(t, failureHandler.failed)
	require.Equal(t, txctx.Tx.Gas, txctx.GasUsed)

	addedFee := new(uint256.Int).Sub(feeAfter, feeBefore)
	expectedFee := types2.GasToFee(txctx.GasUsed, app.govCtrler.GasPrice())
	require.Equal(t, expectedFee.Dec(), addedFee.Dec())
}

func Benchmark_TxLifeCycle(b *testing.B) {
	wallets := make([]*web3.Wallet, 10000)
	appState := genesis.GenesisAppState{
		AssetHolders: make([]*genesis.GenesisAssetHolder, len(wallets)),
		GovParams:    types.DefaultGovParams(),
	}
	for i := 0; i < len(wallets); i++ {
		wallets[i] = web3.NewWallet(nil)
		appState.AssetHolders[i] = &genesis.GenesisAssetHolder{
			Address: wallets[i].Address(),
			Balance: types2.ToGrans(1_000),
		}
	}
	jz, err := jsonx.Marshal(appState)
	require.NoError(b, err)

	btxcfg := config.DefaultConfig("1234")
	btxcfg.SetRoot(filepath.Join(b.TempDir(), "bench-beatoz-app"))
	btzApp := NewBeatozApp(btxcfg, log.NewNopLogger())
	btzClient := NewBeatozLocalClient(&tmsync.Mutex{}, btzApp)
	btzClient.SetResponseCallback(func(*abcitypes.Request, *abcitypes.Response) {})
	btzApp.SetLocalClient(btzClient)
	btzApp.Info(abcitypes.RequestInfo{})
	btzApp.InitChain(abcitypes.RequestInitChain{
		ChainId: "bench-beatoz-app",
		ConsensusParams: &abcitypes.ConsensusParams{
			Block: &abcitypes.BlockParams{
				MaxBytes: 22020096,
				MaxGas:   36000000,
			},
		},
		Validators: abcitypes.ValidatorUpdates{
			abcitypes.UpdateValidator(wallets[0].GetPubKey(), 1_000_000, "secp256k1"),
		},
		AppStateBytes: jz,
		InitialHeight: 1,
	})

	btzApp.Start()
	defer func() {
		btzApp.Stop()
		os.RemoveAll(btxcfg.RootDir)
	}()

	_ = btzApp.BeginBlock(abcitypes.RequestBeginBlock{
		Header: tmproto.Header{Height: 1, ChainID: btxcfg.ChainIdHex()},
	})
	_ = btzApp.EndBlock(abcitypes.RequestEndBlock{Height: 1})
	_ = btzApp.Commit()

	_ = btzApp.BeginBlock(abcitypes.RequestBeginBlock{
		Header: tmproto.Header{Height: 2, ChainID: btxcfg.ChainIdHex()},
	})

	b.ResetTimer()

	for i := 0; i < b.N; i++ {

		b.StopTimer()

		from := wallets[i%len(wallets)]
		nonce := from.GetNonce()
		from.AddNonce()
		to := types2.RandAddress()
		tx := web3.NewTrxTransfer(from.Address(), to, nonce, btzApp.govCtrler.MinTrxGas(), btzApp.govCtrler.GasPrice(), uint256.NewInt(1))
		_, _, err := from.SignTrxRLP(tx, btxcfg.ChainIdHex())
		require.NoError(b, err)
		bztx, err := tx.Encode()
		require.NoError(b, err)

		b.StartTimer()

		checkTxResp := btzApp.CheckTx(abcitypes.RequestCheckTx{Tx: bztx})
		require.Equal(b, abcitypes.CodeTypeOK, checkTxResp.Code, checkTxResp.Log)

		_ = btzApp.DeliverTx(abcitypes.RequestDeliverTx{Tx: bztx})

	}

	_ = btzApp.EndBlock(abcitypes.RequestEndBlock{Height: 2})
	_ = btzApp.Commit()

}
