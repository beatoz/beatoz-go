package evm

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
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
	} else if err := json.Unmarshal(bz, &buildInfoFallbackContract); err != nil {
		panic(err)
	} else if abiFallbackContract, err = abi.JSON(bytes.NewReader(buildInfoFallbackContract.ABI)); err != nil {
		panic(err)
	}
}

func Test_Fallback(t *testing.T) {
	os.RemoveAll(dbPath)
	fallbackEVM = NewEVMCtrler(dbPath, &acctHandler, tmlog.NewTMLogger(tmlog.NewSyncWriter(os.Stdout)))

	//
	// deploy
	// make transaction
	fromAcct := acctHandler.walletsArr[0].GetAccount()
	to := types.ZeroAddress()

	// BeginBlock
	bctx := ctrlertypes.NewBlockContext(abcitypes.RequestBeginBlock{Header: tmproto.Header{Height: fallbackEVM.lastBlockHeight + 1, Time: time.Now()}}, govParams, &acctHandler, nil)
	_, xerr := fallbackEVM.BeginBlock(bctx)
	require.NoError(t, xerr)

	// Execute the tx to deploy contract
	txctx := &ctrlertypes.TrxContext{
		BlockContext: bctx,
		TxHash:       bytes2.RandBytes(32),
		Tx:           web3.NewTrxContract(fromAcct.Address, to, fromAcct.GetNonce(), 3_000_000, uint256.NewInt(10_000_000_000), uint256.NewInt(0), bytes2.HexBytes(buildInfoFallbackContract.Bytecode)),
		TxIdx:        1,
		Exec:         true,
		Sender:       fromAcct,
		Receiver:     nil,
		GasUsed:      0,
		GovHandler:   govParams,
		AcctHandler:  &acctHandler,
	}
	require.NoError(t, fallbackEVM.ValidateTrx(txctx))
	require.NoError(t, fallbackEVM.ExecuteTrx(txctx))

	// When the contract deploy tx is executed,
	// `txctx.RetData` has the address of contract.
	contAddr := txctx.RetData

	// Because `ExcuteTrx` has increased `fromAcct.Nonce`,
	// Calculate an expected address with `fromAcct.Nonce-1`.
	addr0 := ethcrypto.CreateAddress(fromAcct.Address.Array20(), fromAcct.Nonce-1)
	require.Equal(t, addr0[:], contAddr)

	// EndBlock and Commit
	_, xerr = fallbackEVM.EndBlock(bctx)
	require.NoError(t, xerr)
	_, _, xerr = fallbackEVM.Commit()
	require.NoError(t, xerr)

	//
	// transfer to contract
	bctx = ctrlertypes.ExpectedNextBlockContextOf(bctx, time.Second)
	_, xerr = fallbackEVM.BeginBlock(bctx)
	require.NoError(t, xerr)

	contAcct := acctHandler.FindAccount(contAddr, true)
	require.NotNil(t, contAcct)

	originBalance0 := fromAcct.Balance.Clone()
	originBalance1 := contAcct.Balance.Clone()

	//fmt.Println("sender", originBalance0.Dec(), "address", originBalance1.Dec())

	txctx = &ctrlertypes.TrxContext{
		BlockContext: bctx,
		TxHash:       bytes2.RandBytes(32),
		Tx:           web3.NewTrxTransfer(fromAcct.Address, contAcct.Address, fromAcct.GetNonce(), govParams.MinTrxGas()*10, govParams.GasPrice(), types.ToFons(100)),
		TxIdx:        1,
		Exec:         true,
		Sender:       fromAcct,
		Receiver:     contAcct,
		GasUsed:      0,
		GovHandler:   govParams,
		AcctHandler:  &acctHandler,
	}
	require.NoError(t, fallbackEVM.ValidateTrx(txctx))
	require.NoError(t, fallbackEVM.ExecuteTrx(txctx))

	_, xerr = fallbackEVM.EndBlock(bctx)
	require.NoError(t, xerr)
	_, _, xerr = fallbackEVM.Commit()
	require.NoError(t, xerr)

	gasAmt := new(uint256.Int).Mul(uint256.NewInt(txctx.GasUsed), govParams.GasPrice())

	expectedBalance0 := new(uint256.Int).Sub(new(uint256.Int).Sub(originBalance0, gasAmt), types.ToFons(100))
	expectedBalance1 := new(uint256.Int).Add(originBalance1, types.ToFons(100))

	//fmt.Println("sender", expectedBalance0.Dec(), "contract", expectedBalance1.Dec(), "gas", gasAmt.Dec())
	require.Equal(t, expectedBalance0.Dec(), fromAcct.Balance.Dec())
	require.Equal(t, expectedBalance1.Dec(), contAcct.Balance.Dec())

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
