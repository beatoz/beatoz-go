package test

import (
	"fmt"
	"github.com/beatoz/beatoz-go/libs/web3/vm"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	"math/big"
	"testing"
)

func TestTransferFrom(t *testing.T) {
	contract, err := vm.NewEVMContract("./abi_lfit_contract.json")
	require.NoError(t, err)

	bzweb3 := randBeatozWeb3()
	deployer := randCommonWallet()
	require.NoError(t, deployer.Unlock(defaultRpcNode.Pass), string(defaultRpcNode.Pass))
	require.NoError(t, deployer.SyncAccount(bzweb3))
	fmt.Println("deployer address", deployer.Address(), "balance", deployer.GetBalance().Dec(), "nonce", deployer.GetNonce())

	// deploy

	retTx, err := contract.ExecCommit("", []interface{}{},
		deployer, deployer.GetNonce(), contractGas*2, defGasPrice, uint256.NewInt(0), bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, retTx.CheckTx.Code, retTx.CheckTx.Log)
	require.Equal(t, xerrors.ErrCodeSuccess, retTx.DeliverTx.Code, retTx.DeliverTx.Log)
	require.NotNil(t, contract.GetAddress())

	contAddr := contract.GetAddress()
	fmt.Println("contract address", contAddr)
	fmt.Println("gas used        ", retTx.DeliverTx.GasUsed)

	owner := deployer
	spender := randCommonWallet()
	require.NotEqual(t, owner.Address(), spender.Address())
	require.NoError(t, owner.SyncAccount(bzweb3))

	retTx, err = contract.ExecCommit("approve", []interface{}{spender.Address().Array20(), big.NewInt(1e+18)}, owner, owner.GetNonce(), contractGas, defGasPrice, uint256.NewInt(0), bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, retTx.CheckTx.Code, retTx.CheckTx.Log)
	require.Equal(t, xerrors.ErrCodeSuccess, retTx.DeliverTx.Code, retTx.DeliverTx.Log)

	respCall, err := contract.Call("allowance", []interface{}{owner.Address().Array20(), spender.Address().Array20()}, owner.Address(), 0, bzweb3)
	require.NoError(t, err)
	fmt.Println(respCall[0])

	receiptAddr := types.RandAddress()
	require.NoError(t, spender.Unlock(defaultRpcNode.Pass), string(defaultRpcNode.Pass))
	require.NoError(t, spender.SyncAccount(bzweb3))
	retTx, err = contract.ExecCommit("transferFrom",
		[]interface{}{owner.Address().Array20(), receiptAddr.Array20(), big.NewInt(5e+17)},
		spender, spender.GetNonce(), contractGas, defGasPrice, uint256.NewInt(0),
		bzweb3,
	)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, retTx.CheckTx.Code, retTx.CheckTx.Log)
	require.Equal(t, xerrors.ErrCodeSuccess, retTx.DeliverTx.Code, retTx.DeliverTx.Log)

	respCall, err = contract.Call("balanceOf", []interface{}{owner.Address().Array20()}, owner.Address(), 0, bzweb3)
	require.NoError(t, err)
	bal0 := respCall[0]
	respCall, err = contract.Call("balanceOf", []interface{}{spender.Address().Array20()}, owner.Address(), 0, bzweb3)
	require.NoError(t, err)
	bal1 := respCall[0]
	respCall, err = contract.Call("balanceOf", []interface{}{receiptAddr.Array20()}, owner.Address(), 0, bzweb3)
	require.NoError(t, err)
	bal2 := respCall[0]

	require.Equal(t, bal0.(*big.Int).Text(10), "2999999999500000000000000000")
	require.Equal(t, bal1.(*big.Int).Text(10), "0")
	require.Equal(t, bal2, big.NewInt(5e+17))

}
