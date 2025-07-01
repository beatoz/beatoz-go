package evm

import (
	"bytes"
	"encoding/hex"
	"fmt"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/libs/jsonx"
	"github.com/beatoz/beatoz-go/types"
	bytes2 "github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/ethereum/go-ethereum/accounts/abi"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmlog "github.com/tendermint/tendermint/libs/log"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"
)

var (
	fallbackEVM               *EVMCtrler
	buildInfoFallbackContract TruffleBuild
	abiFallbackContract       abi.ABI
)

func init() {
	// load an abi file of contract
	if bz, err := ioutil.ReadFile("../../../test/abi_fallback_contract.json"); err != nil {
		panic(err)
	} else if err := jsonx.Unmarshal(bz, &buildInfoFallbackContract); err != nil {
		panic(err)
	} else if abiFallbackContract, err = abi.JSON(bytes.NewReader(buildInfoFallbackContract.ABI)); err != nil {
		panic(err)
	}
}

func Test_Fallback(t *testing.T) {
	os.RemoveAll(dbPath)
	fallbackEVM = NewEVMCtrler(dbPath, &acctHandler, tmlog.NewNopLogger() /*tmlog.NewTMLogger(tmlog.NewSyncWriter(os.Stdout))*/)

	//
	// deploy
	// make transaction
	fromAcct := acctHandler.walletsArr[0].GetAccount()
	toAcct := ctrlertypes.NewAccount(types.ZeroAddress())

	// BeginBlock
	bctx := ctrlertypes.NewBlockContext(
		abcitypes.RequestBeginBlock{Header: tmproto.Header{Height: fallbackEVM.lastBlockHeight + 1, Time: time.Now()}},
		govMock, &acctHandler, nil, nil, nil)
	_, xerr := fallbackEVM.BeginBlock(bctx)
	require.NoError(t, xerr)

	// Execute the tx to deploy contract
	txctx := &ctrlertypes.TrxContext{
		BlockContext: bctx,
		TxHash:       bytes2.RandBytes(32),
		Tx:           web3.NewTrxContract(fromAcct.Address, toAcct.Address, fromAcct.GetNonce(), 3_000_000, uint256.NewInt(10_000_000_000), uint256.NewInt(0), bytes2.HexBytes(buildInfoFallbackContract.Bytecode)),
		TxIdx:        1,
		Exec:         true,
		Sender:       fromAcct,
		Receiver:     toAcct,
		GasUsed:      0,
	}
	require.NoError(t, fallbackEVM.ValidateTrx(txctx))
	require.NoError(t, fallbackEVM.ExecuteTrx(txctx))

	// When the contract deploy tx is executed,
	// `txctx.RetData` has the address of contract.
	contAddr := txctx.RetData

	addr0 := ethcrypto.CreateAddress(fromAcct.Address.Array20(), uint64(fromAcct.Nonce))
	require.Equal(t, addr0[:], contAddr)

	// EndBlock and Commit
	_, xerr = fallbackEVM.EndBlock(bctx)
	require.NoError(t, xerr)
	_, _, xerr = fallbackEVM.Commit()
	require.NoError(t, xerr)

	//
	// transfer to contract
	bctx.SetHeight(bctx.Height() + 1)
	_, xerr = fallbackEVM.BeginBlock(bctx)
	require.NoError(t, xerr)

	contAcct := acctHandler.FindAccount(contAddr, true)
	require.NotNil(t, contAcct)

	originBalance0 := fromAcct.Balance.Clone()
	originBalance1 := contAcct.Balance.Clone()
	originNonce := fromAcct.GetNonce()

	//fmt.Println("sender", originBalance0.Dec(), "address", originBalance1.Dec())

	txctx = &ctrlertypes.TrxContext{
		BlockContext: bctx,
		Tx:           web3.NewTrxTransfer(fromAcct.Address, contAcct.Address, fromAcct.GetNonce(), govMock.MinTrxGas()*10, govMock.GasPrice(), types.ToGrans(100)),
		TxIdx:        1,
		TxHash:       bytes2.RandBytes(32),
		Exec:         true,
		Sender:       fromAcct,
		Receiver:     contAcct,
		GasUsed:      0,
	}
	require.NoError(t, fallbackEVM.ValidateTrx(txctx))
	require.NoError(t, fallbackEVM.ExecuteTrx(txctx))

	_, xerr = fallbackEVM.EndBlock(bctx)
	require.NoError(t, xerr)
	_, _, xerr = fallbackEVM.Commit()
	require.NoError(t, xerr)

	// EVMCtrler doesn't handle the gas anymore
	//gasAmt := types.GasToFee(txctx.GasUsed, govMock.GasPrice())
	expectedBalance0 := new(uint256.Int).Sub(originBalance0, types.ToGrans(100)) //new(uint256.Int).Sub(new(uint256.Int).Sub(originBalance0, gasAmt), types.ToGrans(100))
	expectedBalance1 := new(uint256.Int).Add(originBalance1, types.ToGrans(100))

	// EVMCtrler doesn't handle the nonce anymore
	expectedNonce := originNonce // + 1

	//fmt.Println("sender", expectedBalance0.Dec(), "contract", expectedBalance1.Dec(), "gas", gasAmt.Dec())
	require.Equal(t, expectedNonce, fromAcct.GetNonce(), "from's nonce wrong")
	require.Equal(t, expectedBalance0.Dec(), fromAcct.Balance.Dec(), "from's balanace wrong")
	require.Equal(t, expectedBalance1.Dec(), contAcct.Balance.Dec(), "to's balance wrong")

	found := false
	for _, attr := range txctx.Events[0].Attributes {
		//fmt.Println(attr.String())
		if string(attr.Key) == "data" {
			val, err := hex.DecodeString(string(attr.Value))
			require.NoError(t, err)
			found = strings.HasPrefix(string(val[64:]), "receive")
			break
		}
	}
	require.True(t, found)

	_, height, xerr := fallbackEVM.Commit()
	require.NoError(t, xerr)
	fmt.Println("TestDeploy", "Commit block", height)

	require.NoError(t, fallbackEVM.Close())
}
