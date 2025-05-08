package evm

import (
	"bytes"
	"encoding/json"
	"fmt"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	bytes2 "github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmlog "github.com/tendermint/tendermint/libs/log"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var (
	govParams   = ctrlertypes.DefaultGovParams()
	acctHandler acctHandlerMock
	dbPath      = filepath.Join(os.TempDir(), "beatoz-evm-test")
)

var (
	erc20EVM         *EVMCtrler
	erc20BuildInfo   TruffleBuild
	abiERC20Contract abi.ABI
	erc20ContAddr    types.Address
)

type TruffleBuild struct {
	ABI              json.RawMessage `json:"abi"`
	Bytecode         hexutil.Bytes   `json:"bytecode"`
	DeployedBytecode hexutil.Bytes   `json:"deployedBytecode"`
}

func init() {
	// initialize acctHandler
	acctHandler.origin = true
	acctHandler.walletsMap = make(map[string]*web3.Wallet)
	for i := 0; i < 10; i++ {
		w := web3.NewWallet(nil)
		w.GetAccount().AddBalance(uint256.MustFromDecimal("1000000000000000000000000000"))
		acctHandler.walletsMap[w.Address().String()] = w
		acctHandler.walletsArr = append(acctHandler.walletsArr, w)
	}

	// load an abi file of erc20 contract
	abiFile := "../../../test/abi_erc20.json"
	if bz, err := ioutil.ReadFile(abiFile); err != nil {
		panic(err)
	} else if err := json.Unmarshal(bz, &erc20BuildInfo); err != nil {
		panic(err)
	} else if abiERC20Contract, err = abi.JSON(bytes.NewReader(erc20BuildInfo.ABI)); err != nil {
		panic(err)
	} else {
		//for _, method := range abiERC20Contract.Methods {
		//	fmt.Printf("%x: %s\n", method.ID, method.Sig)
		//}
		//for _, evt := range abiERC20Contract.Events {
		//	fmt.Printf("%x: %s\n", evt.ID, evt.Sig)
		//}
	}

}

func Test_callEVM_Deploy(t *testing.T) {
	os.RemoveAll(dbPath)
	erc20EVM = NewEVMCtrler(dbPath, &acctHandler, tmlog.NewTMLogger(tmlog.NewSyncWriter(os.Stdout)))

	deployInput, err := abiERC20Contract.Pack("", "TokenOnBeatoz", "TOR")
	require.NoError(t, err)

	// creation code = contract byte code + input parameters
	deployInput = append(erc20BuildInfo.Bytecode, deployInput...)

	contAddr, txctx := testDeployContract(t, deployInput)
	fromAcct := txctx.Sender
	height := txctx.Height
	erc20ContAddr = contAddr

	fmt.Println("TestDeploy", "contract address", erc20ContAddr)
	fmt.Println("TestDeploy", "used gas", txctx.GasUsed)
	fmt.Println("TestDeploy", "Commit block", height)

	bzCode, xerr := erc20EVM.QueryCode(erc20ContAddr, height)
	require.NoError(t, xerr)
	require.Equal(t, []byte(erc20BuildInfo.DeployedBytecode), []byte(bzCode))

	queryAcct := web3.NewWallet(nil)
	retUnpack, xerr := callMethod(abiERC20Contract, queryAcct.Address(), erc20ContAddr, erc20EVM.lastBlockHeight, time.Now().Unix(), "name")
	require.NoError(t, xerr)
	require.Equal(t, "TokenOnBeatoz", retUnpack[0])
	fmt.Println("TestDeploy", "name", retUnpack[0])

	retUnpack, xerr = callMethod(abiERC20Contract, fromAcct.Address, erc20ContAddr, erc20EVM.lastBlockHeight, time.Now().Unix(), "symbol")
	require.NoError(t, xerr)
	require.Equal(t, "TOR", retUnpack[0])
	fmt.Println("TestDeploy", "symbol", retUnpack[0])

	_, height, xerr = erc20EVM.Commit()
	require.NoError(t, xerr)
	fmt.Println("TestDeploy", "Commit block", height)
}

func Test_callEVM_Transfer(t *testing.T) {
	state, xerr := erc20EVM.MemStateAt(erc20EVM.lastBlockHeight)
	require.NoError(t, xerr)

	fromAcct := acctHandler.walletsArr[0].GetAccount()
	toAcct := acctHandler.walletsArr[1].GetAccount()
	queryAcct := web3.NewWallet(nil)

	ret, xerr := callMethod(abiERC20Contract, queryAcct.Address(), erc20ContAddr, erc20EVM.lastBlockHeight, time.Now().Unix(),
		"balanceOf", toAddrArr(fromAcct.Address))
	require.NoError(t, xerr)
	fmt.Println("(BEFORE) balanceOf", fromAcct.Address, ret[0], "nonce", state.GetNonce(fromAcct.Address.Array20()))

	ret, xerr = callMethod(abiERC20Contract, queryAcct.Address(), erc20ContAddr, erc20EVM.lastBlockHeight, time.Now().Unix(),
		"balanceOf", toAddrArr(toAcct.Address))
	require.NoError(t, xerr)
	fmt.Println("(BEFORE) balanceOf", toAcct.Address, ret[0], "nonce", state.GetNonce(fromAcct.Address.Array20()))

	bctx := ctrlertypes.NewBlockContext(abcitypes.RequestBeginBlock{Header: tmproto.Header{Height: erc20EVM.lastBlockHeight + 1}}, govParams, &acctHandler, nil, nil)
	_, xerr = erc20EVM.BeginBlock(bctx)
	require.NoError(t, xerr)

	ret, xerr = execMethod(abiERC20Contract, fromAcct.Address, erc20ContAddr, fromAcct.GetNonce(), 3_000_000, uint256.NewInt(10_000_000_000), uint256.NewInt(0), bctx.Height(), time.Now().Unix(),
		"transfer", toAddrArr(toAcct.Address), toWei(100000000))
	require.NoError(t, xerr)
	fmt.Println("<transferred>")

	_, height, xerr := erc20EVM.Commit()
	require.NoError(t, xerr)
	fmt.Println("Commit block", height)

	state, xerr = erc20EVM.MemStateAt(erc20EVM.lastBlockHeight)
	require.NoError(t, xerr)

	ret, xerr = callMethod(abiERC20Contract, queryAcct.Address(), erc20ContAddr, erc20EVM.lastBlockHeight, time.Now().Unix(),
		"balanceOf", toAddrArr(fromAcct.Address))
	require.NoError(t, xerr)
	fmt.Println(" (AFTER) balanceOf", fromAcct.Address, ret[0], "nonce", state.GetNonce(fromAcct.Address.Array20()))

	ret, xerr = callMethod(abiERC20Contract, queryAcct.Address(), erc20ContAddr, erc20EVM.lastBlockHeight, time.Now().Unix(),
		"balanceOf", toAddrArr(toAcct.Address))
	require.NoError(t, xerr)
	fmt.Println(" (AFTER) balanceOf", toAcct.Address, ret[0], "nonce", state.GetNonce(fromAcct.Address.Array20()))

	bctx = ctrlertypes.NewBlockContext(abcitypes.RequestBeginBlock{Header: tmproto.Header{Height: erc20EVM.lastBlockHeight + 1}}, govParams, &acctHandler, nil, nil)
	_, xerr = erc20EVM.BeginBlock(bctx)
	require.NoError(t, xerr)

	_, height, xerr = erc20EVM.Commit()
	require.NoError(t, xerr)
	fmt.Println("Commit block", height)
	xerr = erc20EVM.Close()
	require.NoError(t, xerr)

	erc20EVM = NewEVMCtrler(dbPath, &acctHandler, tmlog.NewNopLogger())

	state, xerr = erc20EVM.MemStateAt(erc20EVM.lastBlockHeight)
	require.NoError(t, xerr)

	ret, xerr = callMethod(abiERC20Contract, queryAcct.Address(), erc20ContAddr, erc20EVM.lastBlockHeight, time.Now().Unix(),
		"balanceOf", toAddrArr(fromAcct.Address))
	require.NoError(t, xerr)
	fmt.Println("(REOPEN) balanceOf", fromAcct.Address, ret[0], "nonce", state.GetNonce(fromAcct.Address.Array20()))

	ret, xerr = callMethod(abiERC20Contract, queryAcct.Address(), erc20ContAddr, erc20EVM.lastBlockHeight, time.Now().Unix(),
		"balanceOf", toAddrArr(toAcct.Address))
	require.NoError(t, xerr)
	fmt.Println("(REOPEN) balanceOf", toAcct.Address, ret[0], "nonce", state.GetNonce(fromAcct.Address.Array20()))

	bctx = ctrlertypes.NewBlockContext(abcitypes.RequestBeginBlock{Header: tmproto.Header{Height: erc20EVM.lastBlockHeight + 1}}, govParams, &acctHandler, nil, nil)
	_, xerr = erc20EVM.BeginBlock(bctx)
	require.NoError(t, xerr)

	_, height, xerr = erc20EVM.Commit()
	require.NoError(t, xerr)
	fmt.Println("Commit block", height)
	xerr = erc20EVM.Close()
	require.NoError(t, xerr)
}

func testDeployContract(t *testing.T, input []byte) (types.Address, *ctrlertypes.TrxContext) {
	// make transaction
	fromAcct := acctHandler.walletsArr[0].GetAccount()
	to := types.ZeroAddress()

	bctx := ctrlertypes.NewBlockContext(abcitypes.RequestBeginBlock{Header: tmproto.Header{Height: erc20EVM.lastBlockHeight + 1}}, govParams, &acctHandler, nil, nil)
	_, xerr := erc20EVM.BeginBlock(bctx)
	require.NoError(t, xerr)

	txctx := &ctrlertypes.TrxContext{
		Height:      bctx.Height(),
		BlockTime:   time.Now().Unix(),
		TxHash:      bytes2.RandBytes(32),
		Tx:          web3.NewTrxContract(fromAcct.Address, to, fromAcct.GetNonce(), 3_000_000, uint256.NewInt(10_000_000_000), uint256.NewInt(0), input),
		TxIdx:       1,
		Exec:        true,
		Sender:      fromAcct,
		Receiver:    nil,
		GasUsed:     0,
		GovParams:   govParams,
		AcctHandler: &acctHandler,
	}

	xerr = erc20EVM.ExecuteTrx(txctx)
	require.NoError(t, xerr)

	var contAddr types.Address
	for _, evt := range txctx.Events {
		if evt.Type == "evm" {
			require.GreaterOrEqual(t, len(evt.Attributes), 1)
			require.Equal(t, "contractAddress", string(evt.Attributes[0].Key), string(evt.Attributes[0].Key))
			require.Equal(t, 40, len(evt.Attributes[0].Value), string(evt.Attributes[0].Value))
			_addr, err := types.HexToAddress(string(evt.Attributes[0].Value))
			require.NoError(t, err)

			// Because `ExcuteTrx` has increased `fromAcct.Nonce`,
			// Calculate an expected address with `fromAcct.Nonce-1`.
			addr0 := ethcrypto.CreateAddress(fromAcct.Address.Array20(), fromAcct.Nonce-1)
			require.EqualValues(t, addr0[:], _addr)
			require.EqualValues(t, addr0[:], txctx.RetData)

			contAddr = _addr
		}
	}

	_, height, xerr := erc20EVM.Commit()
	require.NoError(t, xerr)
	require.Equal(t, txctx.Height, height)

	return contAddr, txctx
}

func execMethod(abiObj abi.ABI, from, to types.Address, nonce, gas uint64, gasPrice, amt *uint256.Int, bn, bt int64, methodName string, args ...interface{}) ([]interface{}, xerrors.XError) {
	input, err := abiObj.Pack(methodName, args...)
	if err != nil {
		return nil, xerrors.From(err)
	}

	fromAcct := acctHandler.FindAccount(from, true)
	toAcct := acctHandler.FindAccount(to, true)
	txctx := &ctrlertypes.TrxContext{
		Height:      1,
		BlockTime:   time.Now().Unix(),
		TxHash:      bytes2.RandBytes(32),
		Tx:          web3.NewTrxContract(from, to, nonce, gas, gasPrice, amt, input),
		TxIdx:       1,
		Exec:        true,
		Sender:      fromAcct,
		Receiver:    toAcct,
		GasUsed:     0,
		GovParams:   govParams,
		AcctHandler: &acctHandler,
	}
	xerr := erc20EVM.ExecuteTrx(txctx)
	if xerr != nil {
		return nil, xerr
	}

	fmt.Println("execMethod", methodName, "used_gas", txctx.GasUsed)

	retUnpack, err := abiObj.Unpack(methodName, txctx.RetData)
	if err != nil {
		return nil, xerrors.From(err)
	}
	return retUnpack, nil
}

func callMethod(abiObj abi.ABI, from, to types.Address, bn, bt int64, methodName string, args ...interface{}) ([]interface{}, xerrors.XError) {
	input, err := abiObj.Pack(methodName, args...)
	if err != nil {
		return nil, xerrors.From(err)
	}

	ret, xerr := erc20EVM.callVM(from, to, input, bn, bt)
	if xerr != nil {
		return nil, xerr
	}
	if ret.Err != nil {
		return nil, xerrors.From(ret.Err)
	}

	retUnpack, err := abiObj.Unpack(methodName, ret.ReturnData)
	if err != nil {
		return nil, xerrors.From(err)
	}
	return retUnpack, nil
}

func toWei(c int64) *big.Int {
	return new(big.Int).Mul(big.NewInt(c), big.NewInt(1000000000000000000))
}

func toAddrArr(addr []byte) common.Address {
	var ret common.Address
	copy(ret[:], addr)
	return ret
}

type acctHandlerMock struct {
	walletsMap map[string]*web3.Wallet
	walletsArr []*web3.Wallet
	contAccts  []*ctrlertypes.Account
	origin     bool
}

func (handler *acctHandlerMock) FindOrNewAccount(addr types.Address, exec bool) *ctrlertypes.Account {
	ret := handler.FindAccount(addr, exec)
	if ret != nil {
		return ret
	}
	ret = ctrlertypes.NewAccount(addr)
	handler.contAccts = append(handler.contAccts, ret)
	return ret
}

func (handler *acctHandlerMock) FindAccount(addr types.Address, exec bool) *ctrlertypes.Account {
	if w, ok := handler.walletsMap[addr.String()]; ok {
		return w.GetAccount()
	}
	for _, a := range handler.contAccts {
		if bytes.Compare(addr, a.Address) == 0 {
			return a
		}
	}
	return nil
}
func (a *acctHandlerMock) Transfer(from, to types.Address, amt *uint256.Int, exec bool) xerrors.XError {
	if sender := a.FindAccount(from, exec); sender == nil {
		return xerrors.ErrNotFoundAccount
	} else if receiver := a.FindAccount(to, exec); receiver == nil {
		return xerrors.ErrNotFoundAccount
	} else if xerr := sender.SubBalance(amt); xerr != nil {
		return xerr
	} else if xerr := receiver.AddBalance(amt); xerr != nil {
		return xerr
	}
	return nil
}
func (a *acctHandlerMock) Reward(to types.Address, amt *uint256.Int, exec bool) xerrors.XError {
	if receiver := a.FindAccount(to, exec); receiver == nil {
		return xerrors.ErrNotFoundAccount
	} else if xerr := receiver.AddBalance(amt); xerr != nil {
		return xerr
	}
	return nil
}

func (handler *acctHandlerMock) ImmutableAcctCtrlerAt(i int64) (ctrlertypes.IAccountHandler, xerrors.XError) {
	return nil, nil
}
func (handler *acctHandlerMock) SimuAcctCtrlerAt(i int64) (ctrlertypes.IAccountHandler, xerrors.XError) {
	walletsMap := make(map[string]*web3.Wallet)
	walletsArr := make([]*web3.Wallet, len(handler.walletsArr))
	for i, w := range handler.walletsArr {
		w0 := w.Clone()
		walletsMap[w.Address().String()] = w0
		walletsArr[i] = w0
	}
	contAccts := make([]*ctrlertypes.Account, len(handler.contAccts))
	for i, a := range handler.contAccts {
		contAccts[i] = a.Clone()
	}

	return &acctHandlerMock{
		walletsMap: walletsMap,
		walletsArr: walletsArr,
		contAccts:  contAccts,
		origin:     false,
	}, nil
}
func (handler *acctHandlerMock) SetAccount(acct *ctrlertypes.Account, exec bool) xerrors.XError {
	return nil
}

var _ ctrlertypes.IAccountHandler = (*acctHandlerMock)(nil)
