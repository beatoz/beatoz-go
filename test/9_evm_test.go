package test

import (
	bytes2 "bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/beatoz/beatoz-go/libs/jsonx"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/beatoz/beatoz-sdk-go/vm"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	tmjson "github.com/tendermint/tendermint/libs/json"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	tmtypes "github.com/tendermint/tendermint/types"
	"io/ioutil"
	"strings"
	"sync"
	"testing"
	"time"
)

var (
	evmContract *vm.EVMContract
	creator     *web3.Wallet
)

func TestERC20_DeployWithNilAddress(t *testing.T) {
	testDeployWithNilAddress(t, "./abi_erc20.json", []interface{}{"BeatozToken0", "BZT0"})
}

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

func TestERC20_NonceSeq(t *testing.T) {
	testDeploy(t, "./abi_erc20.json", []interface{}{"BeatozToken", "BZT"})
	testNonceSeq(t)
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

func TestERC20_Payer(t *testing.T) {
	testPayer_Deploy(t, "./abi_fallback_contract.json", nil)
	testPayer_Receive(t)
}

func testDeployWithNilAddress(t *testing.T, abiFile string, args []interface{}) {
	bzweb3 := randBeatozWeb3()

	creator = randCommonWallet()
	require.NoError(t, creator.Unlock(defaultRpcNode.Pass), string(defaultRpcNode.Pass))
	require.NoError(t, creator.SyncAccount(bzweb3))

	// `NewEVMContract()` in beatoz-sdk-go cannot be used to test a deployment tx with `to` as `nil`.
	// (beatoz-sdk-go always setㄴ `to` to zero address when deploying.)
	// load an abi file of erc20 contract
	bz, err := ioutil.ReadFile(abiFile)
	require.NoError(t, err)

	var erc20BuildInfo struct {
		ABI              json.RawMessage `json:"abi"`
		Bytecode         hexutil.Bytes   `json:"bytecode"`
		DeployedBytecode hexutil.Bytes   `json:"deployedBytecode"`
	}
	err = jsonx.Unmarshal(bz, &erc20BuildInfo)
	require.NoError(t, err)
	abiERC20Contract, err := abi.JSON(bytes2.NewReader(erc20BuildInfo.ABI))
	require.NoError(t, err)

	deployInput, err := abiERC20Contract.Pack("", args...)
	require.NoError(t, err)

	// creation code = contract byte code + input parameters
	deployInput = append(erc20BuildInfo.Bytecode, deployInput...)
	tx := web3.NewTrxContract(creator.Address(), nil, creator.GetNonce(), contractGas, defGasPrice, uint256.NewInt(0), deployInput)
	_, _, err = creator.SignTrxRLP(tx, bzweb3.ChainID())
	require.NoError(t, err)

	ret, err := bzweb3.SendTransactionCommit(tx)

	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, ret.CheckTx.Code, ret.CheckTx.Log)
	require.Equal(t, xerrors.ErrCodeSuccess, ret.DeliverTx.Code, ret.DeliverTx.Log)
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

	addr0 := ethcrypto.CreateAddress(creator.Address().Array20(), uint64(creator.GetNonce()))
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
	require.Equal(t, types.GasToFee(txRet.TxResult.GasUsed, defGasPrice), usedGas)
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

	// expect error
	wrongGas := estimatedGas - 10
	fmt.Println("estimatedGas", estimatedGas, "wrongGas", wrongGas)
	retTx, err := evmContract.ExecCommit("transfer", []interface{}{rAddr.Array20(), uint256.NewInt(100).ToBig()}, creator, creator.GetNonce(), wrongGas, defGasPrice, uint256.NewInt(0), bzweb3)
	require.NoError(t, err)
	// In CheckTx, the evm tx is not fully executed,
	// So, the error, out of gas, doesn't occur.
	//require.NotEqual(t, xerrors.ErrCodeSuccess, retTx.CheckTx.Code, retTx.CheckTx.Log)
	//require.Equal(t, int64(0), retTx.CheckTx.GasUsed)
	require.NotEqual(t, xerrors.ErrCodeSuccess, retTx.DeliverTx.Code, retTx.DeliverTx.Log)
	require.Equal(t, int64(0), retTx.DeliverTx.GasUsed)

	// success
	retTx, err = evmContract.ExecCommit("transfer", []interface{}{rAddr.Array20(), uint256.NewInt(100).ToBig()}, creator, creator.GetNonce(), estimatedGas, defGasPrice, uint256.NewInt(0), bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, retTx.CheckTx.Code, retTx.CheckTx.Log)
	require.Equal(t, xerrors.ErrCodeSuccess, retTx.DeliverTx.Code, retTx.DeliverTx.Log)
	require.Equal(t, estimatedGas, retTx.DeliverTx.GasUsed)
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

func testNonceSeq(t *testing.T) {
	bzweb3 := randBeatozWeb3()

	require.NoError(t, creator.Unlock(defaultRpcNode.Pass))
	require.NoError(t, creator.SyncAccount(bzweb3))

	// event subscriber
	subWg := &sync.WaitGroup{}
	sub, err := web3.NewSubscriber(defaultRpcNode.WSEnd)
	defer func() {
		sub.Stop()
	}()
	require.NoError(t, err)
	query := fmt.Sprintf("tm.event='Tx' AND tx.sender='%v'", creator.Address())
	err = sub.Start(query, func(sub *web3.Subscriber, result []byte) {

		event := &coretypes.ResultEvent{}
		err := tmjson.Unmarshal(result, event)
		require.NoError(t, err)

		eventDataTx := event.Data.(tmtypes.EventDataTx)
		require.Equal(t, xerrors.ErrCodeSuccess, eventDataTx.TxResult.Result.Code, eventDataTx.TxResult.Result.Log)

		//txHash := event.Events["tx.hash"][0]
		//fmt.Println("event - txhash:", txHash)

		subWg.Done()
	})

	for i := 0; i < 100; i++ {
		subWg.Add(1)

		retTx, err := evmContract.ExecSync("transfer", []interface{}{types.RandAddress().Array20(), uint256.NewInt(1).ToBig()}, creator, creator.GetNonce(), 300_000, defGasPrice, uint256.NewInt(0), bzweb3)

		if err != nil && strings.Contains(err.Error(), "mempool is full") {
			subWg.Done()
			fmt.Println("error", err)
			time.Sleep(time.Millisecond * 3000)

			continue
		}
		require.NoError(t, err)
		require.Equal(t, xerrors.ErrCodeSuccess, retTx.Code, retTx.Log)
		creator.AddNonce()
		//fmt.Println("transfer - txhash:", retTx.Hash)
	}
	subWg.Wait()

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

	expectedAmt := new(uint256.Int).Sub(sender.GetBalance(), types.GasToFee(ret.DeliverTx.GasUsed, defGasPrice))
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
	_ = expectedAmt.Sub(expectedAmt, types.GasToFee(ret.DeliverTx.GasUsed, defGasPrice))
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
func testPayer_Deploy(t *testing.T, abiFile string, args []interface{}) {
	bzweb3 := randBeatozWeb3()

	sender := randCommonWallet()
	payer := randCommonWallet()
	require.NotEqual(t, sender.Address(), payer.Address())
	require.NoError(t, sender.Unlock(defaultRpcNode.Pass), string(defaultRpcNode.Pass))
	require.NoError(t, sender.SyncAccount(bzweb3))
	require.NoError(t, payer.Unlock(defaultRpcNode.Pass), string(defaultRpcNode.Pass))
	require.NoError(t, payer.SyncAccount(bzweb3))

	originSenderBalance := sender.GetBalance().Clone()
	originPayerBalance := payer.GetBalance().Clone()

	contract, err := vm.NewEVMContract(abiFile)
	require.NoError(t, err)

	to := types.ZeroAddress()
	data, err := contract.Pack("", args...)
	require.NoError(t, err)
	data = append(contract.GetBytecode(), data...)

	tx := web3.NewTrxContract(
		sender.Address(),
		to,
		sender.GetNonce(),
		contractGas, defGasPrice,
		uint256.NewInt(0),
		data,
	)
	_, _, err = sender.SignTrxRLP(tx, bzweb3.ChainID())
	require.NoError(t, err)
	_, _, err = payer.SignPayerTrxRLP(tx, bzweb3.ChainID())
	require.NoError(t, err)

	ret, err := bzweb3.SendTransactionCommit(tx)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, ret.CheckTx.Code, ret.CheckTx.Log)
	require.Equal(t, xerrors.ErrCodeSuccess, ret.DeliverTx.Code, ret.DeliverTx.Log)
	require.Equal(t, 20, len(ret.DeliverTx.Data)) // contract address
	contract.SetAddress(ret.DeliverTx.Data)

	//fmt.Println("testDeploy", "usedGas", ret.DeliverTx.GasUsed)
	contAcct, err := bzweb3.QueryAccount(contract.GetAddress())
	require.NoError(t, err)
	require.Equal(t, []byte(ret.Hash), contAcct.Code)

	//txRet, err := waitTrxResult(ret.Hash, 30, bzweb3)
	//require.NoError(t, err, err)
	//require.Equal(t, xerrors.ErrCodeSuccess, txRet.TxResult.Code, txRet.TxResult.Log)

	addr0 := ethcrypto.CreateAddress(sender.Address().Array20(), uint64(sender.GetNonce()))
	require.EqualValues(t, addr0[:], ret.DeliverTx.Data)
	require.EqualValues(t, addr0[:], contract.GetAddress())
	for _, evt := range ret.DeliverTx.Events {
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

	// check balance - changed by gas
	gasAmt := new(uint256.Int).Mul(uint256.NewInt(uint64(ret.DeliverTx.GasUsed)), defGasPrice)

	//fmt.Println("--- before ----")
	//fmt.Println("sender", sender.Address(), "balance", sender.GetBalance())
	//fmt.Println("payer", payer.Address(), "balance", payer.GetBalance())
	//fmt.Println("contAcct", contAcct.Address, "balance", contAcct.GetBalance())

	expectedSenderBalance := originSenderBalance
	expectedPayerBalance := new(uint256.Int).Sub(originPayerBalance, gasAmt)

	require.NoError(t, sender.SyncAccount(bzweb3))
	require.NoError(t, payer.SyncAccount(bzweb3))

	//fmt.Println("--- after ----")
	//fmt.Println("sender", sender.Address(), "balance", sender.GetBalance())
	//fmt.Println("payer", payer.Address(), "balance", payer.GetBalance())
	//fmt.Println("contAcct", contAcct.Address, "balance", contAcct.GetBalance())

	require.Equal(t, expectedSenderBalance.Dec(), sender.GetBalance().Dec())
	require.Equal(t, expectedPayerBalance.Dec(), payer.GetBalance().Dec())
}

func testPayer_Receive(t *testing.T) {
	bzweb3 := randBeatozWeb3()

	sender := randCommonWallet()
	payer := randCommonWallet()
	require.NotEqual(t, sender.Address(), payer.Address())
	require.NoError(t, sender.Unlock(defaultRpcNode.Pass), string(defaultRpcNode.Pass))
	require.NoError(t, sender.SyncAccount(bzweb3))
	require.NoError(t, payer.Unlock(defaultRpcNode.Pass), string(defaultRpcNode.Pass))
	require.NoError(t, payer.SyncAccount(bzweb3))

	contAcct, err := bzweb3.QueryAccount(evmContract.GetAddress())
	require.NoError(t, err)
	require.Equal(t, "0", contAcct.Balance.Dec())

	originSenderBalance := sender.GetBalance().Clone()
	originPayerBalance := payer.GetBalance().Clone()
	originContractBalance := contAcct.GetBalance().Clone()

	//
	// Transfer
	//
	randAmt := bytes.RandU256IntN(sender.GetBalance())
	_ = randAmt.Sub(randAmt, baseFee)

	tx := web3.NewTrxTransfer(
		sender.Address(),
		evmContract.GetAddress(),
		sender.GetNonce(),
		bigGas, defGasPrice,
		randAmt,
	)
	_, _, err = sender.SignTrxRLP(tx, bzweb3.ChainID())
	require.NoError(t, err)
	_, _, err = payer.SignPayerTrxRLP(tx, bzweb3.ChainID())
	require.NoError(t, err)

	ret, err := bzweb3.SendTransactionCommit(tx)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, ret.CheckTx.Code, ret.CheckTx.Log)
	require.Equal(t, xerrors.ErrCodeSuccess, ret.DeliverTx.Code, ret.DeliverTx.Log)

	//fmt.Println("--- before ----")
	//fmt.Println("sender", sender.Address(), "balance", sender.GetBalance())
	//fmt.Println("payer", payer.Address(), "balance", payer.GetBalance())
	//fmt.Println("contAcct", contAcct.Address, "balance", contAcct.GetBalance())

	gasAmt := new(uint256.Int).Mul(uint256.NewInt(uint64(ret.DeliverTx.GasUsed)), defGasPrice)
	expectedSenderBalance := new(uint256.Int).Sub(originSenderBalance, randAmt)
	expectedPayerBalance := new(uint256.Int).Sub(originPayerBalance, gasAmt)
	expectedContractBalance := new(uint256.Int).Add(originContractBalance, randAmt)

	require.NoError(t, sender.SyncAccount(bzweb3))
	require.NoError(t, payer.SyncAccount(bzweb3))
	contAcct, err = bzweb3.QueryAccount(evmContract.GetAddress())
	require.NoError(t, err)

	//fmt.Println("--- after ----")
	//fmt.Println("sender", sender.Address(), "balance", sender.GetBalance())
	//fmt.Println("payer", payer.Address(), "balance", payer.GetBalance())
	//fmt.Println("contAcct", contAcct.Address, "balance", contAcct.GetBalance())

	require.Equal(t, expectedSenderBalance.Dec(), sender.GetBalance().Dec())
	require.Equal(t, expectedPayerBalance.Dec(), payer.GetBalance().Dec())
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
