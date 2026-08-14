package node

import (
	"fmt"
	"os"
	"testing"
	"time"

	btzcfg "github.com/beatoz/beatoz-go/cmd/config"
	"github.com/beatoz/beatoz-go/ctrlers/account"
	"github.com/beatoz/beatoz-go/ctrlers/mocks"
	supplymock "github.com/beatoz/beatoz-go/ctrlers/mocks/supply"
	vpowermock "github.com/beatoz/beatoz-go/ctrlers/mocks/vpower"
	"github.com/beatoz/beatoz-go/ctrlers/supply"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/ledger/common"
	"github.com/beatoz/beatoz-go/libs/jsonx"
	"github.com/beatoz/beatoz-go/types"
	btzbytes "github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

type testStateLedger struct {
	name string

	events *[]string
	active bool

	createErr xerrors.XError
	writeErr  xerrors.XError
	clearErr  xerrors.XError
}

func (stateLedger *testStateLedger) CreateCache(exec bool) xerrors.XError {
	*stateLedger.events = append(*stateLedger.events, fmt.Sprintf("%s:create:%t", stateLedger.name, exec))
	if stateLedger.createErr != nil {
		return stateLedger.createErr
	}
	stateLedger.active = true
	return nil
}

func (stateLedger *testStateLedger) WriteCache(exec bool) xerrors.XError {
	*stateLedger.events = append(*stateLedger.events, fmt.Sprintf("%s:write:%t", stateLedger.name, exec))
	if stateLedger.writeErr != nil {
		return stateLedger.writeErr
	}
	stateLedger.active = false
	return nil
}

func (stateLedger *testStateLedger) ClearCache(exec bool) xerrors.XError {
	if !stateLedger.active {
		return nil
	}
	*stateLedger.events = append(*stateLedger.events, fmt.Sprintf("%s:clear:%t", stateLedger.name, exec))
	if stateLedger.clearErr != nil {
		return stateLedger.clearErr
	}
	stateLedger.active = false
	return nil
}

type trxCacheControllerHandler interface {
	ctrlertypes.IAccountHandler
	ctrlertypes.IGovHandler
	ctrlertypes.ISupplyHandler
	ctrlertypes.IVPowerHandler
}

type testController struct {
	trxCacheControllerHandler
	stateLedger *testStateLedger
}

func (controller *testController) CreateCache(exec bool) xerrors.XError {
	return controller.stateLedger.CreateCache(exec)
}

func (controller *testController) WriteCache(exec bool) xerrors.XError {
	return controller.stateLedger.WriteCache(exec)
}

func (controller *testController) ClearCache(exec bool) xerrors.XError {
	return controller.stateLedger.ClearCache(exec)
}

func newTrxCacheBlockContext() (
	*ctrlertypes.BlockContext,
	map[string]*testStateLedger,
	*[]string,
) {
	events := new([]string)
	stateLedgers := map[string]*testStateLedger{
		"account": {name: "acct", events: events},
		"gov":     {name: "gov", events: events},
		"supply":  {name: "supply", events: events},
		"vpower":  {name: "vpower", events: events},
	}
	bctx := &ctrlertypes.BlockContext{
		AcctHandler:   &testController{stateLedger: stateLedgers["account"]},
		GovHandler:    &testController{stateLedger: stateLedgers["gov"]},
		SupplyHandler: &testController{stateLedger: stateLedgers["supply"]},
		VPowerHandler: &testController{stateLedger: stateLedgers["vpower"]},
	}
	return bctx, stateLedgers, events
}

type accountSetFailureHandler struct {
	ctrlertypes.IAccountHandler
	failureAddress types.Address
	setAccountErr  xerrors.XError
	failed         bool
	failedAddress  types.Address
	failedAccount  testAccountSnapshot
	failedExec     bool
}

func (handler *accountSetFailureHandler) SetAccount(
	acct *ctrlertypes.Account,
	exec bool,
) xerrors.XError {
	if !handler.failed && btzbytes.Equal(acct.Address, handler.failureAddress) {
		handler.failed = true
		handler.failedAddress = append(types.Address(nil), acct.Address...)
		handler.failedAccount = testAccountSnapshot{
			balance: acct.GetBalance().Dec(),
			nonce:   acct.GetNonce(),
		}
		handler.failedExec = exec
		return handler.setAccountErr
	}
	return handler.IAccountHandler.SetAccount(acct, exec)
}

type accountRewardFailureHandler struct {
	ctrlertypes.IAccountHandler
	failureAddress types.Address
	rewardErr      xerrors.XError
	failed         bool
	failedAddress  types.Address
	failedAmount   string
	failedExec     bool
}

func (handler *accountRewardFailureHandler) Reward(
	to types.Address,
	amount *uint256.Int,
	exec bool,
) xerrors.XError {
	if !handler.failed && btzbytes.Equal(to, handler.failureAddress) {
		handler.failed = true
		handler.failedAddress = append(types.Address(nil), to...)
		handler.failedAmount = amount.Dec()
		handler.failedExec = exec
		return handler.rewardErr
	}
	return handler.IAccountHandler.Reward(to, amount, exec)
}

type accountTransferFailureHandler struct {
	*accountSetFailureHandler
}

func (handler *accountTransferFailureHandler) ExecuteTrx(
	ctx *ctrlertypes.TrxContext,
) xerrors.XError {
	sender := handler.FindAccount(ctx.Tx.From, ctx.Exec)
	if sender == nil {
		return xerrors.ErrNotFoundAccount.Wrapf(
			"Transfer - address: %v", ctx.Tx.From,
		)
	}
	if xerr := sender.SubBalance(ctx.Tx.Amount); xerr != nil {
		return xerr
	}
	if xerr := handler.SetAccount(sender, ctx.Exec); xerr != nil {
		return xerr
	}

	receiver := handler.FindAccount(ctx.Tx.To, ctx.Exec)
	if receiver == nil {
		receiver = ctrlertypes.NewAccountWithName(ctx.Tx.To, "")
	}
	if xerr := receiver.AddBalance(ctx.Tx.Amount); xerr != nil {
		return xerr
	}
	return handler.SetAccount(receiver, ctx.Exec)
}

type createCacheErrorAcctHandler struct {
	ctrlertypes.IAccountHandler
	createErr xerrors.XError
}

func (handler *createCacheErrorAcctHandler) CreateCache(bool) xerrors.XError {
	return handler.createErr
}

func newAcctExecutorFixture(
	t *testing.T,
	wallets ...*web3.Wallet,
) (*account.AcctCtrler, *ctrlertypes.BlockContext, *TrxExecutor, string, func()) {
	rootDir, err := os.MkdirTemp("", "beatoz-trx-cache-")
	require.NoError(t, err)

	config := btzcfg.DefaultConfig(chainId.Hex())
	config.SetRoot(rootDir)
	acctCtrler, err := account.NewAcctCtrler(config, log.NewNopLogger())
	if err != nil {
		_ = os.RemoveAll(rootDir)
	}
	require.NoError(t, err)
	require.NoError(t, acctCtrler.UpgradeLedgerVersion(common.LedgerV2))

	for _, wallet := range wallets {
		require.NoError(t, acctCtrler.SetAccount(wallet.GetAccount(), true))
	}
	_, _, xerr := acctCtrler.Commit()
	require.NoError(t, xerr)

	bctx := ctrlertypes.TempBlockContext(
		config.ChainIdHex(), 2, time.Unix(1, 0),
		govMock, acctCtrler, nil,
		supplymock.NewSupplyHandlerMock(),
		vpowermock.NewVPowerHandlerMock(nil, 0),
	)
	cleanup := func() {
		closeErr := acctCtrler.Close()
		removeErr := os.RemoveAll(rootDir)
		require.NoError(t, closeErr)
		require.NoError(t, removeErr)
	}
	return acctCtrler, bctx, NewTrxExecutor(log.NewNopLogger()), config.ChainIdHex(), cleanup
}

type trxRollbackFixture struct {
	app      *BeatozApp
	blockCtx *ctrlertypes.BlockContext

	validator *web3.Wallet
	delegator *web3.Wallet
	receiver  *web3.Wallet
}

func newTrxRollbackFixture(t *testing.T) (*trxRollbackFixture, func()) {
	validator := makeTestWallet(1)
	delegator := makeTestWallet(2)
	receiver := makeTestWallet(3)
	app, _, cleanup := newTestBeatozApp(
		t,
		[]*web3.Wallet{validator, delegator, receiver},
	)

	fixture := &trxRollbackFixture{
		app:       app,
		validator: validator,
		delegator: delegator,
		receiver:  receiver,
	}
	height := fixture.app.lastBlockCtx.Height() + 1
	chainID := fixture.app.lastBlockCtx.ChainID()
	fixture.app.BeginBlock(abcitypes.RequestBeginBlock{
		Header: tmproto.Header{
			Height:  height,
			ChainID: chainID,
		},
	})
	require.NotNil(t, fixture.app.currBlockCtx)
	require.True(t, types.IsBTIP45(chainID, height))
	fixture.blockCtx = fixture.app.currBlockCtx
	return fixture, cleanup
}

func (fixture *trxRollbackFixture) commit(t *testing.T) {
	height := fixture.blockCtx.Height()
	fixture.app.EndBlock(abcitypes.RequestEndBlock{Height: height})
	commitResp := fixture.app.Commit()
	require.NotEmpty(t, commitResp.Data)
	require.Equal(t, height, fixture.app.lastBlockCtx.Height())
}

type testSupplyRewardSnapshot struct {
	address   string
	issued    string
	withdrawn string
	slashed   string
	cumulated string
	height    int64
}

func testSupplyRewardState(
	t *testing.T,
	app *BeatozApp,
	height int64,
	address types.Address,
) testSupplyRewardSnapshot {
	resp := app.Query(abcitypes.RequestQuery{
		Path:   "reward",
		Height: height,
		Data:   address,
	})
	require.Equal(t, abcitypes.CodeTypeOK, resp.Code, resp.Log)

	reward := &supply.Reward{}
	require.NoError(t, jsonx.Unmarshal(resp.Value, reward))
	return testSupplyRewardSnapshot{
		address:   reward.Address().String(),
		issued:    reward.MintedAmount().Dec(),
		withdrawn: reward.WithdrawnAmount().Dec(),
		slashed:   reward.SlashedAmount().Dec(),
		cumulated: reward.CumulatedAmount().Dec(),
		height:    reward.Height(),
	}
}

type testVPowerStakeSnapshot struct {
	Owner       types.Address     `json:"owner"`
	Delegatee   types.Address     `json:"to"`
	TxHash      btzbytes.HexBytes `json:"txhash"`
	StartHeight int64             `json:"startHeight,string"`
	Power       int64             `json:"power,string"`
}

func testVPowerStakeState(
	t *testing.T,
	app *BeatozApp,
	height int64,
	owner types.Address,
) []testVPowerStakeSnapshot {
	resp := app.Query(abcitypes.RequestQuery{
		Path:   "stakes",
		Height: height,
		Data:   owner,
	})
	require.Equal(t, abcitypes.CodeTypeOK, resp.Code, resp.Log)

	var stakes []testVPowerStakeSnapshot
	require.NoError(t, jsonx.Unmarshal(resp.Value, &stakes))
	return stakes
}

type testVPowerDelegateeSnapshot struct {
	Address             types.Address     `json:"address"`
	PubKey              btzbytes.HexBytes `json:"pubKey"`
	SelfPower           int64             `json:"selfPower,string"`
	TotalPower          int64             `json:"totalPower,string"`
	SlashedPower        int64             `json:"slashedPower,string"`
	Delegators          []types.Address   `json:"delegators"`
	NotSignedBlockCount int64             `json:"notSingedBlockCount,string"`
}

func testVPowerDelegateeState(
	t *testing.T,
	app *BeatozApp,
	height int64,
	address types.Address,
) testVPowerDelegateeSnapshot {
	resp := app.Query(abcitypes.RequestQuery{
		Path:   "delegatee",
		Height: height,
		Data:   address,
	})
	require.Equal(t, abcitypes.CodeTypeOK, resp.Code, resp.Log)

	delegatee := testVPowerDelegateeSnapshot{}
	require.NoError(t, jsonx.Unmarshal(resp.Value, &delegatee))
	return delegatee
}

type trxRollbackSnapshot struct {
	fixture *trxRollbackFixture
	sender  *web3.Wallet
	exec    bool

	account      testAccountSnapshot
	balance      *uint256.Int
	blockGasUsed int64
	retryNonce   int64
}

func testAccountState(
	t *testing.T,
	app *BeatozApp,
	wallet *web3.Wallet,
	exec bool,
) testAccountSnapshot {
	acct := app.acctCtrler.FindAccount(wallet.Address(), exec)
	require.NotNil(t, acct)
	return testAccountSnapshot{
		balance: acct.GetBalance().Dec(),
		nonce:   acct.GetNonce(),
	}
}

func newTrxRollbackSnapshot(
	t *testing.T,
	fixture *trxRollbackFixture,
	sender *web3.Wallet,
	exec bool,
) *trxRollbackSnapshot {
	account := testAccountState(t, fixture.app, sender, exec)
	retryNonce := account.nonce
	if exec {
		retryNonce++
	}
	return &trxRollbackSnapshot{
		fixture:      fixture,
		sender:       sender,
		exec:         exec,
		account:      account,
		balance:      uint256.MustFromDecimal(account.balance),
		blockGasUsed: fixture.blockCtx.GetBlockGasUsed(),
		retryNonce:   retryNonce,
	}
}

func (snapshot *trxRollbackSnapshot) assertFailure(
	t *testing.T,
	ctx *ctrlertypes.TrxContext,
) {
	expectedAccount := snapshot.account
	expectedBlockGas := snapshot.blockGasUsed
	if snapshot.exec {
		expectedBalance := snapshot.balance.Clone()
		expectedBalance.Sub(
			expectedBalance, types.GasToFee(ctx.Tx.Gas, ctx.Tx.GasPrice),
		)
		expectedAccount.balance = expectedBalance.Dec()
		expectedAccount.nonce++
		expectedBlockGas += ctx.Tx.Gas
		require.Equal(t, ctx.Tx.Gas, ctx.GasUsed)
	} else {
		require.Zero(t, ctx.GasUsed)
	}
	require.Equal(
		t, expectedAccount,
		testAccountState(t, snapshot.fixture.app, snapshot.sender, snapshot.exec),
	)
	require.Equal(t, expectedBlockGas, snapshot.fixture.blockCtx.GetBlockGasUsed())
}

func (snapshot *trxRollbackSnapshot) assertRetry(
	t *testing.T,
	failedCtx *ctrlertypes.TrxContext,
	retryCtx *ctrlertypes.TrxContext,
	debit *uint256.Int,
	credit *uint256.Int,
) testAccountSnapshot {
	require.Equal(t, retryCtx.Tx.Gas, retryCtx.GasUsed)

	expectedBalance := snapshot.balance.Clone()
	expectedBalance.Sub(
		expectedBalance, types.GasToFee(retryCtx.Tx.Gas, retryCtx.Tx.GasPrice),
	)
	expectedNonce := snapshot.account.nonce + 1
	expectedBlockGas := snapshot.blockGasUsed + retryCtx.Tx.Gas
	if snapshot.exec {
		expectedBalance.Sub(
			expectedBalance, types.GasToFee(failedCtx.Tx.Gas, failedCtx.Tx.GasPrice),
		)
		expectedNonce++
		expectedBlockGas += failedCtx.Tx.Gas
	}
	if debit != nil {
		expectedBalance.Sub(expectedBalance, debit)
	}
	if credit != nil {
		expectedBalance.Add(expectedBalance, credit)
	}
	expectedAccount := testAccountSnapshot{
		balance: expectedBalance.Dec(),
		nonce:   expectedNonce,
	}
	require.Equal(
		t, expectedAccount,
		testAccountState(t, snapshot.fixture.app, snapshot.sender, snapshot.exec),
	)
	require.Equal(t, expectedBlockGas, snapshot.fixture.blockCtx.GetBlockGasUsed())
	return expectedAccount
}

func (snapshot *trxRollbackSnapshot) assertCommittedAccount(
	t *testing.T,
	afterRetry testAccountSnapshot,
) {
	expected := snapshot.account
	if snapshot.exec {
		expected = afterRetry
	}
	for _, readExec := range []bool{false, true} {
		require.Equal(
			t, expected,
			testAccountState(t, snapshot.fixture.app, snapshot.sender, readExec),
			"exec=%t", readExec,
		)
	}
}

func makeTransferCtx(
	t *testing.T,
	bctx *ctrlertypes.BlockContext,
	chainID string,
	sender *web3.Wallet,
	receiver types.Address,
	payer *web3.Wallet,
	nonce int64,
	amount *uint256.Int,
	exec bool,
) *ctrlertypes.TrxContext {
	tx := web3.NewTrxTransfer(
		sender.Address(), receiver, nonce,
		govMock.MinTrxGas(), govMock.GasPrice(), amount,
	)
	_, _, err := sender.SignTrxRLP(tx, chainID)
	require.NoError(t, err)
	if payer != nil {
		_, _, err = payer.SignPayerTrxRLP(tx, chainID)
		require.NoError(t, err)
	}

	ctx, xerr := mocks.MakeTrxCtxWithTrxBctx(tx, bctx, exec)
	require.NoError(t, xerr)
	return ctx
}
