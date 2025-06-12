package node

import (
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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

	btxcfg := config.DefaultConfig()
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
		Header: tmproto.Header{Height: 1, ChainID: btxcfg.ChainID},
	})
	_ = btzApp.EndBlock(abcitypes.RequestEndBlock{Height: 1})
	_ = btzApp.Commit()

	_ = btzApp.BeginBlock(abcitypes.RequestBeginBlock{
		Header: tmproto.Header{Height: 2, ChainID: btxcfg.ChainID},
	})

	b.ResetTimer()

	for i := 0; i < b.N; i++ {

		b.StopTimer()

		from := wallets[i%len(wallets)]
		nonce := from.GetNonce()
		from.AddNonce()
		to := types2.RandAddress()
		tx := web3.NewTrxTransfer(from.Address(), to, nonce, btzApp.govCtrler.MinTrxGas(), btzApp.govCtrler.GasPrice(), uint256.NewInt(1))
		_, _, err := from.SignTrxRLP(tx, btxcfg.ChainID)
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
