package test

import (
	"encoding/hex"
	"fmt"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/beatoz/beatoz-sdk-go/vm"
	"github.com/beatoz/beatoz-sdk-go/web3"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	tmjson "github.com/tendermint/tendermint/libs/json"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"strings"
	"sync"
	"testing"
)

var (
	evmContract *vm.EVMContract
	creator     *web3.Wallet
)

func TestERC20_Deploy(t *testing.T) {
	// deploy
	testDeploy(t, "./abi_erc20.json", []interface{}{"BeatozToken", "BZT"})
	testQuery(t)
}

func TestERC20_EstimateGas(t *testing.T) {
	//testDeploy(t, "./abi_erc20.json", []interface{}{"BeatozToken", "BZT"})
	testEstimateGas(t)
}

func TestERC20_NonceDup(t *testing.T) {
	testDeploy(t, "./abi_erc20.json", []interface{}{"BeatozToken", "BZT"})
	testNonceDup(t)
}

func TestERC20_Payable(t *testing.T) {
	//testDeploy(t, "./abi_erc20.json", []interface{}{"BeatozToken", "BZT"})
	testPayable(t)
}

func TestERC20_Event(t *testing.T) {
	//testDeploy(t, "./abi_erc20.json", []interface{}{"BeatozToken", "BZT"})
	testEvents(t)
}

func TestERC20_Fallback(t *testing.T) {
	testDeploy(t, "./abi_fallback_contract.json", nil)
	testReceive(t)
	testFallback(t)
}

func testDeploy(t *testing.T, abiFile string, args []interface{}) {
	bzweb3 := randBeatozWeb3()

	creator = randCommonWallet()
	require.NoError(t, creator.Unlock(defaultRpcNode.Pass), string(defaultRpcNode.Pass))

	require.NoError(t, creator.SyncAccount(bzweb3))
	beforeBalance0 := creator.GetBalance().Clone()

	contract, err := vm.NewEVMContract(abiFile)
	require.NoError(t, err)

	// insufficient gas
	ret, err := contract.ExecCommit("", args,
		creator, creator.GetNonce(), defGas, defGasPrice, uint256.NewInt(0), bzweb3)
	require.NoError(t, err)
	require.NotEqual(t, xerrors.ErrCodeSuccess, ret.CheckTx.Code, ret.CheckTx.Log)
	require.Nil(t, contract.GetAddress())

	// check balance - not changed
	require.NoError(t, creator.SyncAccount(bzweb3))
	beforeBalance1 := creator.GetBalance().Clone()
	require.Equal(t, beforeBalance0.Dec(), beforeBalance1.Dec())

	// sufficient gas - deploy contract
	ret, err = contract.ExecCommit("", args,
		creator, creator.GetNonce(), contractGas, defGasPrice, uint256.NewInt(0), bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, ret.CheckTx.Code, ret.CheckTx.Log)
	require.Equal(t, xerrors.ErrCodeSuccess, ret.DeliverTx.Code, ret.DeliverTx.Log)
	require.NotNil(t, contract.GetAddress())

	//fmt.Println("testDeploy", "usedGas", ret.DeliverTx.GasUsed)
	contAcct, err := bzweb3.GetAccount(contract.GetAddress())
	require.NoError(t, err)
	require.Equal(t, []byte(ret.Hash), contAcct.Code)

	txRet, err := waitTrxResult(ret.Hash, 30, bzweb3)
	require.NoError(t, err, err)
	require.Equal(t, xerrors.ErrCodeSuccess, txRet.TxResult.Code, txRet.TxResult.Log)

	addr0 := ethcrypto.CreateAddress(creator.Address().Array20(), creator.GetNonce())
	require.EqualValues(t, addr0[:], txRet.TxResult.Data)
	require.EqualValues(t, addr0[:], contract.GetAddress())
	for _, evt := range txRet.TxResult.Events {
		if evt.Type == "evm" {
			require.GreaterOrEqual(t, len(evt.Attributes), 1)
			require.Equal(t, "contractAddress", string(evt.Attributes[0].Key), string(evt.Attributes[0].Key))
			require.Equal(t, 40, len(evt.Attributes[0].Value), string(evt.Attributes[0].Value))
			_addr, err := types.HexToAddress(string(evt.Attributes[0].Value))
			require.NoError(t, err)
			require.EqualValues(t, addr0[:], _addr)
		}
	}
	evmContract = contract

	require.NoError(t, creator.SyncAccount(bzweb3))
	afterBalance := creator.GetBalance().Clone()

	// check balance - changed by gas
	usedGas := new(uint256.Int).Sub(beforeBalance1, afterBalance)
	require.Equal(t, gasToFee(uint64(txRet.TxResult.GasUsed), defGasPrice), usedGas)
}

func testQuery(t *testing.T) {
	bzweb3 := randBeatozWeb3()

	sender := randCommonWallet()
	require.NoError(t, sender.SyncAccount(bzweb3))
	ret, err := evmContract.Call("name", nil, sender.Address(), 0, bzweb3)
	require.NoError(t, err)
	require.Equal(t, "BeatozToken", ret[0])
}

func testEstimateGas(t *testing.T) {
	bzweb3 := randBeatozWeb3()

	rAddr := types.RandAddress()
	estimatedGas, err := evmContract.EstimateGas("transfer", []interface{}{rAddr.Array20(), uint256.NewInt(100).ToBig()}, creator.Address(), 0, bzweb3)
	require.NoError(t, err)
	require.True(t, 0 < estimatedGas)

	retTx, err := evmContract.ExecCommit("transfer", []interface{}{rAddr.Array20(), uint256.NewInt(100).ToBig()}, creator, creator.GetNonce(), estimatedGas /*contractGas*/, defGasPrice, uint256.NewInt(0), bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, retTx.CheckTx.Code, retTx.CheckTx.Log)
	require.Equal(t, xerrors.ErrCodeSuccess, retTx.DeliverTx.Code, retTx.DeliverTx.Log)
	require.Equal(t, estimatedGas, uint64(retTx.DeliverTx.GasUsed))
}

func testNonceDup(t *testing.T) {
	bzweb3 := randBeatozWeb3()

	rAddr := types.RandAddress()
	estimatedGas, err := evmContract.EstimateGas("transfer", []interface{}{rAddr.Array20(), uint256.NewInt(100).ToBig()}, creator.Address(), 0, bzweb3)
	require.NoError(t, err)
	require.True(t, 0 < estimatedGas)

	nonce := creator.GetNonce()

	retTx, err := evmContract.ExecSync("transfer", []interface{}{rAddr.Array20(), uint256.NewInt(100).ToBig()}, creator, nonce, estimatedGas /*contractGas*/, defGasPrice, uint256.NewInt(0), bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, retTx.Code, retTx.Log)

	// Expected error for same nonce
	// The txs that are handled by `EVMCtrler`, the nonce validation has been not performed in `CheckTx` phase.
	// After that bug is fixed, the error should be occurred.
	retTx, err = evmContract.ExecSync("transfer", []interface{}{rAddr.Array20(), uint256.NewInt(100).ToBig()}, creator, nonce, estimatedGas /*contractGas*/, defGasPrice, uint256.NewInt(0), bzweb3)
	require.NoError(t, err)
	require.NotEqual(t, xerrors.ErrCodeSuccess, retTx.Code, retTx.Log)
}

func testPayable(t *testing.T) {
	bzweb3 := randBeatozWeb3()

	sender := randCommonWallet()
	require.NoError(t, sender.Unlock(defaultRpcNode.Pass), string(defaultRpcNode.Pass))
	require.NoError(t, sender.SyncAccount(bzweb3))

	contAcct, err := bzweb3.GetAccount(evmContract.GetAddress())
	require.NoError(t, err)
	require.Equal(t, "0", contAcct.Balance.Dec())

	//fmt.Println("initial", "sender", sender.Address(), "balance", sender.GetBalance())
	//fmt.Println("initial", "contAcct", contAcct.Address, "balance", contAcct.GetBalance())

	//
	// Transfer
	//
	randAmt := bytes.RandU256IntN(sender.GetBalance())
	_ = randAmt.Sub(randAmt, baseFee)

	ret, err := sender.TransferCommit(evmContract.GetAddress(), bigGas, defGasPrice, randAmt, bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, ret.CheckTx.Code, ret.CheckTx.Log)
	require.Equal(t, xerrors.ErrCodeSuccess, ret.DeliverTx.Code, ret.DeliverTx.Log)

	expectedAmt := new(uint256.Int).Sub(sender.GetBalance(), gasToFee(uint64(ret.DeliverTx.GasUsed), defGasPrice))
	_ = expectedAmt.Sub(expectedAmt, randAmt)
	require.NotEqual(t, sender.GetBalance(), expectedAmt)
	require.NoError(t, sender.SyncAccount(bzweb3))
	require.Equal(t, expectedAmt, sender.GetBalance())

	contAcct, err = bzweb3.GetAccount(evmContract.GetAddress())
	require.NoError(t, err)
	require.Equal(t, randAmt, contAcct.Balance)

	//fmt.Println("after transfer", "sender", sender.Address(), "balance", sender.GetBalance())
	//fmt.Println("after transfer", "contAcct", contAcct.Address, "balance", contAcct.GetBalance())

	//
	// payable function giveMeAsset
	//

	refundAmt := bytes.RandU256IntN(randAmt)
	ret, err = evmContract.ExecCommit("giveMeAsset", []interface{}{refundAmt.ToBig()}, sender, sender.GetNonce(), contractGas, defGasPrice, uint256.NewInt(0), bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, ret.CheckTx.Code, ret.CheckTx.Log)
	require.Equal(t, xerrors.ErrCodeSuccess, ret.DeliverTx.Code, ret.DeliverTx.Log)

	//txRet, err := waitTrxResult(ret.Hash, 15, bzweb3)
	//require.NoError(t, err)
	//require.Equal(t, xerrors.ErrCodeSuccess, txRet.TxResult.Code)

	fmt.Println("giveMeAsset", "usedGas", ret.DeliverTx.GasUsed)

	expectedAmt = new(uint256.Int).Add(sender.GetBalance(), refundAmt)
	_ = expectedAmt.Sub(expectedAmt, gasToFee(uint64(ret.DeliverTx.GasUsed), defGasPrice))
	require.NoError(t, sender.SyncAccount(bzweb3))
	require.Equal(t, expectedAmt, sender.GetBalance())

	expectedAmt = new(uint256.Int).Sub(contAcct.GetBalance(), refundAmt)
	contAcct, err = bzweb3.GetAccount(evmContract.GetAddress())
	require.NoError(t, err)
	require.Equal(t, expectedAmt, contAcct.GetBalance())

	fmt.Println("after refund", "sender", sender.Address(), "balance", sender.GetBalance())
	fmt.Println("after refund", "contAcct", contAcct.Address, "balance", contAcct.GetBalance())
}

func testEvents(t *testing.T) {
	bzweb3 := randBeatozWeb3()

	require.NoError(t, creator.Unlock(defaultRpcNode.Pass), string(defaultRpcNode.Pass))
	require.NoError(t, creator.SyncAccount(bzweb3))

	// subcribe events
	subWg := &sync.WaitGroup{}
	sub, err := web3.NewSubscriber(defaultRpcNode.WSEnd)
	defer func() {
		sub.Stop()
	}()
	require.NoError(t, err)
	// Transfer Event sig: ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef
	query := fmt.Sprintf("tx.type='contract' AND evm.topic.0='ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef'")
	err = sub.Start(query, func(sub *web3.Subscriber, result []byte) {
		event := &coretypes.ResultEvent{}
		err := tmjson.Unmarshal(result, event)
		require.NoError(t, err)

		subWg.Done()
	})
	require.NoError(t, err)

	// broadcast tx
	subWg.Add(1)

	rAddr := types.RandAddress()
	ret, err := evmContract.ExecSync("transfer", []interface{}{rAddr.Array20(), uint256.NewInt(100).ToBig()}, creator, creator.GetNonce(), contractGas, defGasPrice, uint256.NewInt(0), bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, ret.Code, ret.Log)

	txRet, err := waitTrxResult(ret.Hash, 30, bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, txRet.TxResult.Code)

	fmt.Println("transfer(contract)", "usedGas", txRet.TxResult.GasUsed)

	subWg.Wait()

}

func testFallback(t *testing.T) {
	bzweb3 := randBeatozWeb3()

	sender := randCommonWallet()
	require.NoError(t, sender.Unlock(defaultRpcNode.Pass), string(defaultRpcNode.Pass))
	require.NoError(t, sender.SyncAccount(bzweb3))

	ret, err := evmContract.ExecCommitWith(bytes.RandBytes(4), sender, sender.GetNonce(), bigGas, defGasPrice, uint256.NewInt(0), bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, ret.CheckTx.Code, ret.CheckTx.Log)
	require.Equal(t, xerrors.ErrCodeSuccess, ret.DeliverTx.Code, ret.DeliverTx.Log)

	found := false
	for _, evt := range ret.DeliverTx.Events {
		if evt.Type != "evm" {
			continue
		}
		for _, attr := range evt.Attributes {
			//fmt.Println(attr.String())
			if string(attr.Key) == "data" {
				val, err := hex.DecodeString(string(attr.Value))
				require.NoError(t, err)
				found = strings.HasPrefix(string(val[64:]), "fallback")
				break
			}
		}
	}

	require.True(t, found)
}

func testReceive(t *testing.T) {
	bzweb3 := randBeatozWeb3()

	sender := randCommonWallet()
	require.NoError(t, sender.Unlock(defaultRpcNode.Pass), string(defaultRpcNode.Pass))
	require.NoError(t, sender.SyncAccount(bzweb3))

	contAcct, err := bzweb3.GetAccount(evmContract.GetAddress())
	require.NoError(t, err)
	require.Equal(t, "0", contAcct.Balance.Dec())

	originSenderBalance := sender.GetBalance()
	originContractBalance := contAcct.GetBalance()

	//
	// Transfer
	//
	randAmt := bytes.RandU256IntN(sender.GetBalance())
	_ = randAmt.Sub(randAmt, baseFee)

	ret, err := sender.TransferCommit(evmContract.GetAddress(), bigGas, defGasPrice, randAmt, bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, ret.CheckTx.Code, ret.CheckTx.Log)
	require.Equal(t, xerrors.ErrCodeSuccess, ret.DeliverTx.Code, ret.DeliverTx.Log)

	fmt.Println("sender", sender.Address(), "balance", sender.GetBalance())
	fmt.Println("contAcct", contAcct.Address, "balance", contAcct.GetBalance())

	gasAmt := new(uint256.Int).Mul(uint256.NewInt(uint64(ret.DeliverTx.GasUsed)), defGasPrice)
	expectedSenderBalance := new(uint256.Int).Sub(originSenderBalance, new(uint256.Int).Add(gasAmt, randAmt))
	expectedContractBalance := new(uint256.Int).Add(originContractBalance, randAmt)

	require.NoError(t, sender.SyncAccount(bzweb3))
	contAcct, err = bzweb3.GetAccount(evmContract.GetAddress())
	require.NoError(t, err)

	require.Equal(t, expectedSenderBalance.Dec(), sender.GetBalance().Dec())
	require.Equal(t, expectedContractBalance.Dec(), contAcct.GetBalance().Dec())

	found := false
	for _, evt := range ret.DeliverTx.Events {
		if evt.Type != "evm" {
			continue
		}
		for _, attr := range evt.Attributes {
			//fmt.Println(attr.String())
			if string(attr.Key) == "data" {
				val, err := hex.DecodeString(string(attr.Value))
				require.NoError(t, err)
				found = strings.HasPrefix(string(val[64:]), "receive")
				break
			}
		}
	}

	require.True(t, found)

}
