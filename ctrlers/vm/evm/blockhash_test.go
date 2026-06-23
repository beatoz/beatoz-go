package evm

import (
	"testing"
	"time"

	cfg "github.com/beatoz/beatoz-go/cmd/config"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	bytes2 "github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmlog "github.com/tendermint/tendermint/libs/log"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

func Test_Blockhash(t *testing.T) {
	config := cfg.DefaultConfig()
	config.SetChainId("0xDEA8D3")
	config.SetRoot(t.TempDir())

	acctHandler := newBlockhashTestAcctHandler()
	ctrler := NewEVMCtrler(config, acctHandler, tmlog.NewNopLogger())
	t.Cleanup(func() {
		require.NoError(t, ctrler.Close())
	})

	block1Hash := common.HexToHash("0x1111111111111111111111111111111111111111111111111111111111111111")
	block1Ctx := ctrlertypes.NewBlockContext(
		abcitypes.RequestBeginBlock{
			Hash: block1Hash.Bytes(),
			Header: tmproto.Header{
				ChainID: config.ChainIdHex(),
				Height:  1,
				Time:    time.Now(),
			},
		},
		govMock, acctHandler, ctrler, nil, nil,
	)

	_, xerr := ctrler.BeginBlock(block1Ctx)
	require.NoError(t, xerr)
	_, _, xerr = ctrler.Commit()
	require.NoError(t, xerr)

	block2Ctx := ctrlertypes.NewBlockContext(
		abcitypes.RequestBeginBlock{
			Hash: common.HexToHash("0x2222222222222222222222222222222222222222222222222222222222222222").Bytes(),
			Header: tmproto.Header{
				ChainID: config.ChainIdHex(),
				Height:  2,
				Time:    time.Now(),
			},
		},
		govMock, acctHandler, ctrler, nil, nil,
	)

	_, xerr = ctrler.BeginBlock(block2Ctx)
	require.NoError(t, xerr)

	fromAcct := acctHandler.walletsArr[0].GetAccount()
	contractAcct := ctrlertypes.NewAccount(types.RandAddress())
	contractAcct.SetCode([]byte{1})
	acctHandler.contAccts = append(acctHandler.contAccts, contractAcct)

	// PUSH1 1; BLOCKHASH; PUSH1 0; MSTORE; PUSH1 32; PUSH1 0; RETURN.
	blockhashRuntime := []byte{
		0x60, 0x01,
		0x40,
		0x60, 0x00,
		0x52,
		0x60, 0x20,
		0x60, 0x00,
		0xf3,
	}
	ctrler.stateDBWrapper.SetCode(contractAcct.Address.Array20(), blockhashRuntime)

	txctx := &ctrlertypes.TrxContext{
		BlockContext: block2Ctx,
		TxHash:       bytes2.RandBytes(32),
		Tx: web3.NewTrxContract(
			fromAcct.Address,
			contractAcct.Address,
			fromAcct.GetNonce(),
			100_000,
			govMock.GasPrice(),
			uint256.NewInt(0),
			nil,
		),
		TxIdx:    1,
		Exec:     true,
		Sender:   fromAcct,
		Receiver: contractAcct,
	}

	require.NoError(t, ctrler.ValidateTrx(txctx))
	require.NotPanics(t, func() {
		xerr = ctrler.ExecuteTrx(txctx)
	})
	require.NoError(t, xerr)
	require.Equal(t, block1Hash.Bytes(), txctx.RetData)
}

func Test_BlockhashProvider(t *testing.T) {
	hash := common.HexToHash("0x3333333333333333333333333333333333333333333333333333333333333333")
	provider := newBlockHashProvider(300, func(height int64) (common.Hash, bool) {
		if height == 299 {
			return hash, true
		}
		return common.Hash{}, false
	})

	require.Equal(t, hash, provider(299))
	require.Equal(t, common.Hash{}, provider(300))
	require.Equal(t, common.Hash{}, provider(301))
	require.Equal(t, common.Hash{}, provider(298))
}

func newBlockhashTestAcctHandler() *acctHandlerMock {
	handler := &acctHandlerMock{
		origin:     true,
		walletsMap: make(map[string]*web3.Wallet),
	}
	for i := 0; i < 2; i++ {
		wallet := web3.NewWallet(nil)
		wallet.GetAccount().AddBalance(uint256.MustFromDecimal("1000000000000000000000000000"))
		handler.walletsMap[wallet.Address().String()] = wallet
		handler.walletsArr = append(handler.walletsArr, wallet)
	}
	return handler
}
