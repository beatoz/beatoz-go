package vpower

import (
	"testing"
	"time"

	beatozcfg "github.com/beatoz/beatoz-go/cmd/config"
	"github.com/beatoz/beatoz-go/ctrlers/mocks"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/stretchr/testify/require"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

func Test_limiter(t *testing.T) {
	lastTotal := int64(100)
	allowRate := int32(10)
	limiter := NewVPowerLimiter()
	limiter.Reset(lastTotal, allowRate)

	expectedAdded := int64(0)
	expectedSubed := int64(0)
	expectedNewTotal := lastTotal

	// applied: added=11, subed=0, remain.total=100, new.total=111
	// remain.total / new.total = 0.90
	// expected: no error
	diffPower := int64(11)
	expectedAdded += diffPower
	expectedNewTotal += diffPower
	require.NoError(t, limiter.CheckLimit(diffPower, ADD_POWER))
	require.EqualValues(t, lastTotal, limiter.lastTotalPower)
	require.EqualValues(t, expectedNewTotal, limiter.estimatedTotalPower)
	require.EqualValues(t, expectedAdded, limiter.addingPower)
	require.EqualValues(t, expectedSubed, limiter.subingPower)

	// applied: added=12, subed=0, remain.total=100, new.total=112
	// remain.total / new.total = 0.89
	// expected: error
	diffPower = int64(1)
	require.Error(t, limiter.CheckLimit(diffPower, ADD_POWER))
	require.EqualValues(t, lastTotal, limiter.lastTotalPower)
	require.EqualValues(t, expectedNewTotal, limiter.estimatedTotalPower)
	require.EqualValues(t, expectedAdded, limiter.addingPower)
	require.EqualValues(t, expectedSubed, limiter.subingPower)

	limiter.Reset(lastTotal, allowRate)
	expectedAdded = int64(0)
	expectedSubed = int64(0)
	expectedNewTotal = lastTotal

	// applied: added=0, subed=10, remain.total=90, new.total=90
	// expected: no error
	diffPower = int64(10)
	expectedSubed += diffPower
	expectedNewTotal -= diffPower
	require.NoError(t, limiter.CheckLimit(diffPower, SUB_POWER))
	require.EqualValues(t, lastTotal, limiter.lastTotalPower)
	require.EqualValues(t, expectedNewTotal, limiter.estimatedTotalPower)
	require.EqualValues(t, expectedAdded, limiter.addingPower)
	require.EqualValues(t, expectedSubed, limiter.subingPower)

	// applied: added=10, subed=10, remain.total=90, new.total=100
	// expected: no error
	// remain.total/new.total = 0.9
	diffPower = int64(10)
	expectedAdded += diffPower
	expectedNewTotal += diffPower
	require.NoError(t, limiter.CheckLimit(diffPower, ADD_POWER))
	require.EqualValues(t, lastTotal, limiter.lastTotalPower)
	require.EqualValues(t, expectedNewTotal, limiter.estimatedTotalPower)
	require.EqualValues(t, expectedAdded, limiter.addingPower)
	require.EqualValues(t, expectedSubed, limiter.subingPower)

	// applied: added=11, subed=10, remain.total=90, new.total=101
	// expected: no error
	// remain.total/new.total = 0.8910
	diffPower = int64(1)
	require.Error(t, limiter.CheckLimit(diffPower, ADD_POWER))
	require.EqualValues(t, lastTotal, limiter.lastTotalPower)
	require.EqualValues(t, expectedNewTotal, limiter.estimatedTotalPower)
	require.EqualValues(t, expectedAdded, limiter.addingPower)
	require.EqualValues(t, expectedSubed, limiter.subingPower)
}

// CheckTx after BeginBlock must update only the CheckTx limiter.
func Test_Limiter_CheckTxAfterBeginBlock(t *testing.T) {
	cfg := beatozcfg.DefaultConfig(config.ChainIdHex())
	cfg.SetRoot(t.TempDir())
	ctrler, _, _, xerr := initLedger(cfg)
	require.NoError(t, xerr)
	t.Cleanup(func() { require.NoError(t, ctrler.Close()) })

	_, lastHeight, xerr := ctrler.Commit()
	require.NoError(t, xerr)
	mocks.InitBlockCtxWith(cfg.ChainIdHex(), lastHeight+1, govMock, acctMock, nil, nil, ctrler)

	// EndBlock initializes the CheckTx limiter for the next block.
	doBeginBlockWithVotes(t, ctrler)
	require.NoError(t, mocks.DoEndBlock(ctrler))
	require.NoError(t, mocks.DoCommit(ctrler))

	// BeginBlock resets the DeliverTx limiter: 21 validators x 10M power,
	// no accumulated changes, and the governance-defined limit (default 33%).
	doBeginBlockWithVotes(t, ctrler)
	const totalPower = int64(210_000_000)
	const txPower = int64(60_000_000)
	require.Equal(t, totalPower, ctrler.sumPowerOfValidators())
	expectedInitial := VPowerLimiter{
		lastTotalPower:      totalPower,
		estimatedTotalPower: totalPower,
		addingPower:         0,
		subingPower:         0,
		allowRate:           govMock.MaxUpdatablePowerRate(),
	}
	require.Equal(t, expectedInitial, *ctrler.vpowLimiter)
	require.Equal(t, expectedInitial, *ctrler.checkVPowLimiter)
	before := *ctrler.vpowLimiter

	staker := acctMock.GetWallet(990)
	checkCtx, xerr := makeBondingTrxCtxExec(staker, staker.Address(), txPower, mocks.CurrBlockHeight(), false)
	require.NoError(t, xerr)
	require.NoError(t, ctrler.ValidateTrx(checkCtx))

	// DeliverTx budget is unchanged; only CheckTx accumulates the 60M power.
	require.Equal(t, before, *ctrler.vpowLimiter)
	require.Equal(t, VPowerLimiter{
		lastTotalPower:      totalPower,
		estimatedTotalPower: totalPower + txPower,
		addingPower:         txPower,
		subingPower:         0,
		allowRate:           govMock.MaxUpdatablePowerRate(),
	}, *ctrler.checkVPowLimiter)
}

// doBeginBlockWithVotes runs BeginBlock with the votes signed by all current validators.
// VPowerCtrler.BeginBlock returns early without resetting vpowLimiter
// if LastCommitInfo has no votes, which does not happen after the initial height in a real chain.
func doBeginBlockWithVotes(t *testing.T, ctrler *VPowerCtrler) {
	bctx := mocks.CurrBlockCtx()
	bi := bctx.BlockInfo()
	bi.LastCommitInfo.Votes = nil
	for _, val := range ctrler.lastValidators {
		bi.LastCommitInfo.Votes = append(bi.LastCommitInfo.Votes, abcitypes.VoteInfo{
			Validator:       abcitypes.Validator{Address: val.addr, Power: val.SumPower},
			SignedLastBlock: true,
		})
	}
	bctx.SetBlockInfo(bi)
	require.NoError(t, mocks.DoBeginBlock(ctrler))
}

// makeBondingTrxCtxExec is the same as makeBondingTrxCtx, but `exec` can be specified.
func makeBondingTrxCtxExec(fromAcct *web3.Wallet, to types.Address, power, height int64, exec bool) (*ctrlertypes.TrxContext, xerrors.XError) {
	tx := web3.NewTrxStaking(
		fromAcct.Address(),
		to,
		fromAcct.GetNonce(),
		govMock.MinTrxGas(), govMock.GasPrice(),
		types.PowerToAmount(power),
	)
	if _, _, err := fromAcct.SignTrxRLP(tx, config.ChainIdHex()); err != nil {
		return nil, xerrors.From(err)
	}
	return mocks.MakeTrxCtxWithTrx(tx, config.ChainIdHex(), height, time.Now(), exec, govMock, acctMock, nil, nil, nil)
}
