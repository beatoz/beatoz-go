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
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	tmsync "github.com/tendermint/tendermint/libs/sync"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

func newTestBeatozApp(t *testing.T, wallets []*web3.Wallet) (*BeatozApp, *beatozLocalClient, func()) {
	t.Helper()

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

	btxcfg := config.DefaultConfig("1234")
	btxcfg.SetRoot(filepath.Join(t.TempDir(), "beatoz-test"))

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
		os.RemoveAll(btxcfg.RootDir)
		types.InitSigner(chainId)
	}
	return btzApp, btzClient.(*beatozLocalClient), cleanup
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

func Test_EndBlock_NoChainHalt(t *testing.T) {
	testCases := []struct {
		name  string
		build func(t *testing.T, app *BeatozApp, chainID string, wallets []*web3.Wallet) []byte
	}{
		{
			name: "wrong_gas_price",
			build: func(t *testing.T, app *BeatozApp, chainID string, wallets []*web3.Wallet) []byte {
				gasPrice := new(uint256.Int).Add(app.govCtrler.GasPrice(), uint256.NewInt(1))
				tx := web3.NewTrxTransfer(
					wallets[0].Address(), types2.RandAddress(),
					wallets[0].GetNonce(),
					app.govCtrler.MinTrxGas(), gasPrice,
					uint256.NewInt(1),
				)
				_, _, err := wallets[0].SignTrxRLP(tx, chainID)
				require.NoError(t, err)
				txbz, err := tx.Encode()
				require.NoError(t, err)
				return txbz
			},
		},
		{
			name: "small_gas",
			build: func(t *testing.T, app *BeatozApp, chainID string, wallets []*web3.Wallet) []byte {
				tx := web3.NewTrxTransfer(
					wallets[0].Address(), types2.RandAddress(),
					wallets[0].GetNonce(),
					app.govCtrler.MinTrxGas()-1, app.govCtrler.GasPrice(),
					uint256.NewInt(1),
				)
				_, _, err := wallets[0].SignTrxRLP(tx, chainID)
				require.NoError(t, err)
				txbz, err := tx.Encode()
				require.NoError(t, err)
				return txbz
			},
		},
		{
			name: "wrong_signature",
			build: func(t *testing.T, app *BeatozApp, chainID string, wallets []*web3.Wallet) []byte {
				tx := web3.NewTrxTransfer(
					wallets[0].Address(), types2.RandAddress(),
					wallets[0].GetNonce(),
					app.govCtrler.MinTrxGas(), app.govCtrler.GasPrice(),
					uint256.NewInt(1),
				)
				_, _, err := wallets[1].SignTrxRLP(tx, chainID)
				require.NoError(t, err)
				txbz, err := tx.Encode()
				require.NoError(t, err)
				return txbz
			},
		},
		{
			name: "sender_not_found",
			build: func(t *testing.T, app *BeatozApp, chainID string, wallets []*web3.Wallet) []byte {
				unknownSender := web3.NewWallet(nil)
				tx := web3.NewTrxTransfer(
					unknownSender.Address(), types2.RandAddress(),
					unknownSender.GetNonce(),
					app.govCtrler.MinTrxGas(), app.govCtrler.GasPrice(),
					uint256.NewInt(1),
				)
				_, _, err := unknownSender.SignTrxRLP(tx, chainID)
				require.NoError(t, err)
				txbz, err := tx.Encode()
				require.NoError(t, err)
				return txbz
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			wallets := []*web3.Wallet{web3.NewWallet(nil), web3.NewWallet(nil)}
			btzApp, btzClient, cleanup := newTestBeatozApp(t, wallets)
			defer cleanup()

			var deliverTxResp *abcitypes.ResponseDeliverTx
			btzClient.SetResponseCallback(func(req *abcitypes.Request, resp *abcitypes.Response) {
				if r := resp.GetDeliverTx(); r != nil {
					deliverTxResp = r
				}
			})

			chainID := btzApp.lastBlockCtx.ChainID()
			txbz := tc.build(t, btzApp, chainID, wallets)

			btzApp.BeginBlock(abcitypes.RequestBeginBlock{
				Header: tmproto.Header{Height: 2, ChainID: chainID},
			})
			btzApp.DeliverTx(abcitypes.RequestDeliverTx{Tx: txbz})
			require.Equal(t, 1, btzApp.currBlockCtx.TxsCnt())

			require.NotPanics(t, func() {
				btzApp.EndBlock(abcitypes.RequestEndBlock{Height: 2})
			})
			require.NotNil(t, deliverTxResp)
			require.NotEqual(t, abcitypes.CodeTypeOK, deliverTxResp.Code)

			btzApp.Commit()
		})
	}
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
