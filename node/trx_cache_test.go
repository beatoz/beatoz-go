package node

import (
	"fmt"
	"testing"

	"github.com/beatoz/beatoz-go/ctrlers/gov/proposal"
	"github.com/beatoz/beatoz-go/ctrlers/mocks"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

func Test_createTrxCaches(t *testing.T) {
	//
	// Success
	for _, exec := range []bool{false, true} {
		bctx, stateLedgers, events := newTrxCacheBlockContext()

		require.NoError(t, createTrxCaches(bctx, exec), "case=success exec=%t", exec)
		require.Equal(t, []string{
			fmt.Sprintf("vpower:create:%t", exec),
			fmt.Sprintf("supply:create:%t", exec),
			fmt.Sprintf("gov:create:%t", exec),
			fmt.Sprintf("acct:create:%t", exec),
		}, *events, "case=success exec=%t", exec)
		for _, stateLedger := range stateLedgers {
			require.True(
				t, stateLedger.active,
				"case=success controller=%s exec=%t", stateLedger.name, exec,
			)
		}
	}

	//
	// Create error
	tests := []struct {
		name              string
		failAt            string
		wantEventPrefixes []string
	}{
		{
			name:              "vpower",
			failAt:            "vpower",
			wantEventPrefixes: []string{"vpower:create"},
		},
		{
			name:   "supply",
			failAt: "supply",
			wantEventPrefixes: []string{
				"vpower:create",
				"supply:create",
				"vpower:clear",
			},
		},
		{
			name:   "gov",
			failAt: "gov",
			wantEventPrefixes: []string{
				"vpower:create",
				"supply:create",
				"gov:create",
				"supply:clear",
				"vpower:clear",
			},
		},
		{
			name:   "account",
			failAt: "account",
			wantEventPrefixes: []string{
				"vpower:create",
				"supply:create",
				"gov:create",
				"acct:create",
				"gov:clear",
				"supply:clear",
				"vpower:clear",
			},
		},
	}

	for _, tt := range tests {
		for _, exec := range []bool{false, true} {
			bctx, stateLedgers, events := newTrxCacheBlockContext()
			wantErr := xerrors.NewOrdinary(tt.name + " create error")
			stateLedgers[tt.failAt].createErr = wantErr

			xerr := createTrxCaches(bctx, exec)

			require.Equal(
				t, wantErr, xerr,
				"case=create_error controller=%s exec=%t", tt.name, exec,
			)
			wantEvents := make([]string, 0, len(tt.wantEventPrefixes))
			for _, prefix := range tt.wantEventPrefixes {
				wantEvents = append(wantEvents, fmt.Sprintf("%s:%t", prefix, exec))
			}
			require.Equal(
				t, wantEvents, *events,
				"case=create_error controller=%s exec=%t", tt.name, exec,
			)
			for _, stateLedger := range stateLedgers {
				require.False(
					t, stateLedger.active,
					"case=create_error failed_at=%s controller=%s exec=%t",
					tt.name, stateLedger.name, exec,
				)
			}
		}
	}

	//
	// Unowned cache
	bctx, stateLedgers, events := newTrxCacheBlockContext()
	wantErr := xerrors.NewOrdinary("governance create error")
	stateLedgers["gov"].active = true
	stateLedgers["gov"].createErr = wantErr
	stateLedgers["account"].active = true

	xerr := createTrxCaches(bctx, false)

	require.Equal(t, wantErr, xerr, "case=unowned_cache")
	require.True(t, stateLedgers["gov"].active, "case=unowned_cache controller=gov")
	require.True(t, stateLedgers["account"].active, "case=unowned_cache controller=account")
	require.False(t, stateLedgers["supply"].active, "case=unowned_cache controller=supply")
	require.False(t, stateLedgers["vpower"].active, "case=unowned_cache controller=vpower")
	require.Equal(t, []string{
		"vpower:create:false",
		"supply:create:false",
		"gov:create:false",
		"supply:clear:false",
		"vpower:clear:false",
	}, *events, "case=unowned_cache")

	//
	// Invalid block
	require.Error(t, createTrxCaches(nil, true), "case=nil_block")

	//
	// Invalid handler
	nilHandlerBctx, _, nilHandlerEvents := newTrxCacheBlockContext()
	nilHandlerBctx.AcctHandler = nil

	require.Error(t, createTrxCaches(nilHandlerBctx, true), "case=nil_handler")
	require.Empty(t, *nilHandlerEvents, "case=nil_handler")
}

func Test_writeTrxCaches(t *testing.T) {
	//
	// Success
	for _, exec := range []bool{false, true} {
		bctx, stateLedgers, events := newTrxCacheBlockContext()
		for _, stateLedger := range stateLedgers {
			stateLedger.active = true
		}

		require.NoError(t, writeTrxCaches(bctx, exec), "case=success exec=%t", exec)
		require.Equal(t, []string{
			fmt.Sprintf("acct:write:%t", exec),
			fmt.Sprintf("gov:write:%t", exec),
			fmt.Sprintf("supply:write:%t", exec),
			fmt.Sprintf("vpower:write:%t", exec),
		}, *events, "case=success exec=%t", exec)
		for _, stateLedger := range stateLedgers {
			require.False(
				t, stateLedger.active,
				"case=success controller=%s exec=%t", stateLedger.name, exec,
			)
		}
	}

	//
	// Write error
	tests := []struct {
		name              string
		failAt            string
		wantEventPrefixes []string
		wantActive        []string
	}{
		{
			name:              "account",
			failAt:            "account",
			wantEventPrefixes: []string{"acct:write"},
			wantActive:        []string{"account", "gov", "supply", "vpower"},
		},
		{
			name:   "gov",
			failAt: "gov",
			wantEventPrefixes: []string{
				"acct:write",
				"gov:write",
			},
			wantActive: []string{"gov", "supply", "vpower"},
		},
		{
			name:   "supply",
			failAt: "supply",
			wantEventPrefixes: []string{
				"acct:write",
				"gov:write",
				"supply:write",
			},
			wantActive: []string{"supply", "vpower"},
		},
		{
			name:   "vpower",
			failAt: "vpower",
			wantEventPrefixes: []string{
				"acct:write",
				"gov:write",
				"supply:write",
				"vpower:write",
			},
			wantActive: []string{"vpower"},
		},
	}

	for _, tt := range tests {
		for _, exec := range []bool{false, true} {
			bctx, stateLedgers, events := newTrxCacheBlockContext()
			for _, stateLedger := range stateLedgers {
				stateLedger.active = true
			}
			wantErr := xerrors.NewOrdinary(tt.name + " write error")
			stateLedgers[tt.failAt].writeErr = wantErr

			xerr := writeTrxCaches(bctx, exec)

			require.Equal(
				t, wantErr, xerr,
				"case=write_error controller=%s exec=%t", tt.name, exec,
			)
			wantEvents := make([]string, 0, len(tt.wantEventPrefixes))
			for _, prefix := range tt.wantEventPrefixes {
				wantEvents = append(wantEvents, fmt.Sprintf("%s:%t", prefix, exec))
			}
			require.Equal(
				t, wantEvents, *events,
				"case=write_error controller=%s exec=%t", tt.name, exec,
			)
			wantActive := make(map[string]bool, len(tt.wantActive))
			for _, name := range tt.wantActive {
				wantActive[name] = true
			}
			for name, stateLedger := range stateLedgers {
				require.Equal(
					t, wantActive[name], stateLedger.active,
					"case=write_error failed_at=%s controller=%s exec=%t",
					tt.name, stateLedger.name, exec,
				)
			}
		}
	}
}

func Test_clearTrxCaches(t *testing.T) {
	//
	// Success
	for _, exec := range []bool{false, true} {
		bctx, stateLedgers, events := newTrxCacheBlockContext()
		for _, stateLedger := range stateLedgers {
			stateLedger.active = true
		}

		clearTrxCaches(bctx, exec)

		require.Equal(t, []string{
			fmt.Sprintf("acct:clear:%t", exec),
			fmt.Sprintf("gov:clear:%t", exec),
			fmt.Sprintf("supply:clear:%t", exec),
			fmt.Sprintf("vpower:clear:%t", exec),
		}, *events, "case=success exec=%t", exec)
		for _, stateLedger := range stateLedgers {
			require.False(
				t, stateLedger.active,
				"case=success controller=%s exec=%t", stateLedger.name, exec,
			)
		}
	}

	//
	// Continue after error
	for _, exec := range []bool{false, true} {
		bctx, stateLedgers, events := newTrxCacheBlockContext()
		for _, stateLedger := range stateLedgers {
			stateLedger.active = true
		}
		stateLedgers["account"].clearErr = xerrors.NewOrdinary("account clear error")

		clearTrxCaches(bctx, exec)

		require.Equal(t, []string{
			fmt.Sprintf("acct:clear:%t", exec),
			fmt.Sprintf("gov:clear:%t", exec),
			fmt.Sprintf("supply:clear:%t", exec),
			fmt.Sprintf("vpower:clear:%t", exec),
		}, *events, "case=continue_after_error exec=%t", exec)
		require.True(
			t, stateLedgers["account"].active,
			"case=continue_after_error controller=account exec=%t", exec,
		)
		require.False(
			t, stateLedgers["gov"].active,
			"case=continue_after_error controller=gov exec=%t", exec,
		)
		require.False(
			t, stateLedgers["supply"].active,
			"case=continue_after_error controller=supply exec=%t", exec,
		)
		require.False(
			t, stateLedgers["vpower"].active,
			"case=continue_after_error controller=vpower exec=%t", exec,
		)
	}
}

func Test_Rollback_AccountTransfer(t *testing.T) {
	for _, exec := range []bool{false, true} {
		fixture, cleanup := newTrxRollbackFixture(t)
		acctCtrler := fixture.app.acctCtrler
		rollback := newTrxRollbackSnapshot(t, fixture, fixture.delegator, exec)
		receiverBefore := testAccountState(t, fixture.app, fixture.receiver, exec)
		amount := uint256.NewInt(1_000)
		expectedReceiverBalance := uint256.MustFromDecimal(receiverBefore.balance)
		expectedReceiverBalance.Add(expectedReceiverBalance, amount)
		expectedReceiver := testAccountSnapshot{
			balance: expectedReceiverBalance.Dec(),
			nonce:   receiverBefore.nonce,
		}

		ctx := makeTransferCtx(
			t, fixture.blockCtx, fixture.blockCtx.ChainID(),
			fixture.delegator, fixture.receiver.Address(), nil,
			rollback.account.nonce, amount, exec,
		)
		wantErr := xerrors.NewOrdinary("receiver account set failure")
		failureHandler := &accountTransferFailureHandler{
			accountSetFailureHandler: &accountSetFailureHandler{
				IAccountHandler: acctCtrler,
				failureAddress:  fixture.receiver.Address(),
				setAccountErr:   wantErr,
			},
		}
		fixture.blockCtx.AcctHandler = failureHandler
		xerr := fixture.app.txExecutor.ExecuteSync(ctx)
		require.Equal(t, wantErr, xerr, "exec=%t", exec)
		rollback.assertFailure(t, ctx)
		require.True(t, failureHandler.failed, "exec=%t", exec)
		require.Equal(
			t, fixture.receiver.Address(), failureHandler.failedAddress,
			"exec=%t", exec,
		)
		require.Equal(
			t, expectedReceiver, failureHandler.failedAccount,
			"exec=%t", exec,
		)
		require.Equal(t, exec, failureHandler.failedExec, "exec=%t", exec)
		fixture.blockCtx.AcctHandler = acctCtrler

		require.Equal(
			t, receiverBefore,
			testAccountState(t, fixture.app, fixture.receiver, exec),
			"exec=%t", exec,
		)

		retryCtx := makeTransferCtx(
			t, fixture.blockCtx, fixture.blockCtx.ChainID(),
			fixture.delegator, fixture.receiver.Address(), nil,
			rollback.retryNonce, amount, exec,
		)
		require.NoError(
			t, fixture.app.txExecutor.ExecuteSync(retryCtx), "exec=%t", exec,
		)
		expectedSender := rollback.assertRetry(
			t, ctx, retryCtx, amount, nil,
		)
		require.Equal(
			t, expectedReceiver,
			testAccountState(t, fixture.app, fixture.receiver, exec),
			"exec=%t", exec,
		)

		fixture.commit(t)
		expectedCommittedReceiver := receiverBefore
		if exec {
			expectedCommittedReceiver = expectedReceiver
		}
		rollback.assertCommittedAccount(t, expectedSender)
		for _, readExec := range []bool{false, true} {
			require.Equal(
				t, expectedCommittedReceiver,
				testAccountState(t, fixture.app, fixture.receiver, readExec),
				"exec=%t read_exec=%t", exec, readExec,
			)
		}

		cleanup()
	}
}

func Test_Rollback_SupplyWithdraw(t *testing.T) {
	for _, exec := range []bool{false, true} {
		fixture, cleanup := newTrxRollbackFixture(t)
		acctCtrler := fixture.app.acctCtrler
		govCtrler := fixture.app.govCtrler
		govCtrler.GovParams.SetValue(func(params *ctrlertypes.GovParamsProto) {
			params.InflationCycleBlocks = 3
			params.RipeningBlocks = 2
		})

		fixture.commit(t)
		mintHeight := fixture.app.lastBlockCtx.Height() + 1
		chainID := fixture.app.lastBlockCtx.ChainID()
		fixture.app.BeginBlock(abcitypes.RequestBeginBlock{
			Header: tmproto.Header{
				Height:  mintHeight,
				ChainID: chainID,
			},
		})
		require.NotNil(t, fixture.app.currBlockCtx, "exec=%t", exec)
		fixture.blockCtx = fixture.app.currBlockCtx
		fixture.commit(t)

		mintedReward := testSupplyRewardState(
			t, fixture.app, mintHeight, fixture.validator.Address(),
		)
		require.Equal(
			t, fixture.validator.Address().String(), mintedReward.address,
			"exec=%t", exec,
		)
		require.Equal(t, mintHeight, mintedReward.height, "exec=%t", exec)
		require.NotEqual(t, "0", mintedReward.cumulated, "exec=%t", exec)

		for _, withdrawCase := range []string{"partial", "full"} {
			committedHeight := fixture.app.lastBlockCtx.Height()
			rewardBefore := testSupplyRewardState(
				t, fixture.app, committedHeight, fixture.validator.Address(),
			)
			require.Equal(
				t, fixture.validator.Address().String(), rewardBefore.address,
				"case=%s exec=%t", withdrawCase, exec,
			)
			require.NotEqual(
				t, "0", rewardBefore.cumulated,
				"case=%s exec=%t", withdrawCase, exec,
			)
			cumulatedAmount := uint256.MustFromDecimal(rewardBefore.cumulated)
			withdrawAmount := cumulatedAmount.Clone()
			if withdrawCase == "partial" {
				withdrawAmount.Div(withdrawAmount, uint256.NewInt(2))
				require.Positive(
					t, withdrawAmount.Sign(),
					"case=%s exec=%t", withdrawCase, exec,
				)
				require.Negative(
					t, withdrawAmount.Cmp(cumulatedAmount),
					"case=%s exec=%t", withdrawCase, exec,
				)
			}

			withdrawHeight := committedHeight + 1
			fixture.app.BeginBlock(abcitypes.RequestBeginBlock{
				Header: tmproto.Header{
					Height:  withdrawHeight,
					ChainID: chainID,
				},
			})
			require.NotNil(
				t, fixture.app.currBlockCtx,
				"case=%s exec=%t", withdrawCase, exec,
			)
			require.True(
				t, types.IsBTIP45(chainID, withdrawHeight),
				"case=%s exec=%t", withdrawCase, exec,
			)
			fixture.blockCtx = fixture.app.currBlockCtx
			rollback := newTrxRollbackSnapshot(
				t, fixture, fixture.validator, exec,
			)

			tx := web3.NewTrxWithdraw(
				fixture.validator.Address(), fixture.validator.Address(),
				rollback.account.nonce,
				govCtrler.MinTrxGas(), govCtrler.GasPrice(), withdrawAmount,
			)
			_, _, err := fixture.validator.SignTrxRLP(
				tx, fixture.blockCtx.ChainID(),
			)
			require.NoError(
				t, err, "case=%s exec=%t", withdrawCase, exec,
			)
			ctx, xerr := mocks.MakeTrxCtxWithTrxBctx(
				tx, fixture.blockCtx, exec,
			)
			require.NoError(
				t, xerr, "case=%s exec=%t", withdrawCase, exec,
			)
			wantErr := xerrors.NewOrdinary(
				"supply withdraw account reward failure",
			)
			failureHandler := &accountRewardFailureHandler{
				IAccountHandler: acctCtrler,
				failureAddress:  fixture.validator.Address(),
				rewardErr:       wantErr,
			}
			fixture.blockCtx.AcctHandler = failureHandler
			xerr = fixture.app.txExecutor.ExecuteSync(ctx)
			require.Equal(
				t, wantErr, xerr, "case=%s exec=%t", withdrawCase, exec,
			)
			rollback.assertFailure(t, ctx)
			require.True(
				t, failureHandler.failed,
				"case=%s exec=%t", withdrawCase, exec,
			)
			require.Equal(
				t, fixture.validator.Address(), failureHandler.failedAddress,
				"case=%s exec=%t", withdrawCase, exec,
			)
			require.Equal(
				t, withdrawAmount.Dec(), failureHandler.failedAmount,
				"case=%s exec=%t", withdrawCase, exec,
			)
			require.Equal(
				t, exec, failureHandler.failedExec,
				"case=%s exec=%t", withdrawCase, exec,
			)
			require.Equal(
				t, rewardBefore,
				testSupplyRewardState(
					t, fixture.app, committedHeight,
					fixture.validator.Address(),
				),
				"case=%s exec=%t", withdrawCase, exec,
			)
			fixture.blockCtx.AcctHandler = acctCtrler

			retryTx := web3.NewTrxWithdraw(
				fixture.validator.Address(), fixture.validator.Address(),
				rollback.retryNonce,
				govCtrler.MinTrxGas(), govCtrler.GasPrice(), withdrawAmount,
			)
			_, _, err = fixture.validator.SignTrxRLP(
				retryTx, fixture.blockCtx.ChainID(),
			)
			require.NoError(
				t, err, "case=%s exec=%t", withdrawCase, exec,
			)
			retryCtx, xerr := mocks.MakeTrxCtxWithTrxBctx(
				retryTx, fixture.blockCtx, exec,
			)
			require.NoError(
				t, xerr, "case=%s exec=%t", withdrawCase, exec,
			)
			require.NoError(
				t, fixture.app.txExecutor.ExecuteSync(retryCtx),
				"case=%s exec=%t", withdrawCase, exec,
			)
			expectedSender := rollback.assertRetry(
				t, ctx, retryCtx, nil, withdrawAmount,
			)

			fixture.commit(t)
			expectedCommittedReward := rewardBefore
			if exec {
				expectedCommittedReward = testSupplyRewardSnapshot{
					address:   fixture.validator.Address().String(),
					issued:    "0",
					withdrawn: "0",
					slashed:   "0",
					cumulated: "0",
					height:    0,
				}
				if withdrawCase == "partial" {
					remainingAmount := cumulatedAmount.Clone()
					remainingAmount.Sub(remainingAmount, withdrawAmount)
					expectedCommittedReward.withdrawn = withdrawAmount.Dec()
					expectedCommittedReward.cumulated = remainingAmount.Dec()
					expectedCommittedReward.height = withdrawHeight
				}
			}

			committedReward := testSupplyRewardState(
				t, fixture.app, withdrawHeight, fixture.validator.Address(),
			)
			require.Equal(
				t, expectedCommittedReward, committedReward,
				"case=%s exec=%t", withdrawCase, exec,
			)
			require.Equal(
				t, mintedReward,
				testSupplyRewardState(
					t, fixture.app, mintHeight, fixture.validator.Address(),
				),
				"case=%s exec=%t", withdrawCase, exec,
			)
			rollback.assertCommittedAccount(t, expectedSender)
		}

		cleanup()
	}
}

func Test_Rollback_VPowerStaking(t *testing.T) {
	for _, exec := range []bool{false, true} {
		fixture, cleanup := newTrxRollbackFixture(t)
		acctCtrler := fixture.app.acctCtrler
		vpowerCtrler := fixture.app.vpowCtrler
		govCtrler := fixture.app.govCtrler
		require.True(
			t, vpowerCtrler.IsValidator(fixture.validator.Address()),
			"exec=%t", exec,
		)

		committedHeight := fixture.app.lastBlockCtx.Height()
		stakesBefore := testVPowerStakeState(
			t, fixture.app, committedHeight, fixture.delegator.Address(),
		)
		require.Len(t, stakesBefore, 0, "exec=%t", exec)
		delegateeBefore := testVPowerDelegateeState(
			t, fixture.app, committedHeight, fixture.validator.Address(),
		)
		rollback := newTrxRollbackSnapshot(
			t, fixture, fixture.delegator, exec,
		)
		stakePower := govCtrler.MinDelegatorPower()
		require.Greater(t, stakePower, int64(0), "exec=%t", exec)
		stakeAmount := types.PowerToAmount(stakePower)

		tx := web3.NewTrxStaking(
			fixture.delegator.Address(), fixture.validator.Address(),
			rollback.account.nonce,
			govCtrler.MinTrxGas(), govCtrler.GasPrice(), stakeAmount,
		)
		_, _, err := fixture.delegator.SignTrxRLP(
			tx, fixture.blockCtx.ChainID(),
		)
		require.NoError(t, err, "exec=%t", exec)
		ctx, xerr := mocks.MakeTrxCtxWithTrxBctx(
			tx, fixture.blockCtx, exec,
		)
		require.NoError(t, xerr, "exec=%t", exec)
		wantErr := xerrors.NewOrdinary("vpower staking account set failure")
		failureHandler := &accountSetFailureHandler{
			IAccountHandler: acctCtrler,
			failureAddress:  fixture.delegator.Address(),
			setAccountErr:   wantErr,
		}
		fixture.blockCtx.AcctHandler = failureHandler
		xerr = fixture.app.txExecutor.ExecuteSync(ctx)
		require.Equal(t, wantErr, xerr, "exec=%t", exec)
		rollback.assertFailure(t, ctx)

		expectedFailedAccount := rollback.account
		expectedFailedBalance := rollback.balance.Clone()
		expectedFailedBalance.Sub(expectedFailedBalance, stakeAmount)
		expectedFailedAccount.balance = expectedFailedBalance.Dec()
		require.True(t, failureHandler.failed, "exec=%t", exec)
		require.Equal(
			t, fixture.delegator.Address(), failureHandler.failedAddress,
			"exec=%t", exec,
		)
		require.Equal(
			t, expectedFailedAccount, failureHandler.failedAccount,
			"exec=%t", exec,
		)
		require.Equal(t, exec, failureHandler.failedExec, "exec=%t", exec)
		fixture.blockCtx.AcctHandler = acctCtrler

		failedStakeTx := web3.NewTrxUnstaking(
			fixture.delegator.Address(), fixture.validator.Address(),
			rollback.retryNonce,
			govCtrler.MinTrxGas(), govCtrler.GasPrice(), ctx.TxHash,
		)
		_, _, err = fixture.delegator.SignTrxRLP(
			failedStakeTx, fixture.blockCtx.ChainID(),
		)
		require.NoError(t, err, "exec=%t", exec)
		failedStakeCtx, xerr := mocks.MakeTrxCtxWithTrxBctx(
			failedStakeTx, fixture.blockCtx, exec,
		)
		require.NoError(t, xerr, "exec=%t", exec)
		xerr = fixture.app.txExecutor.ExecuteSync(failedStakeCtx)
		require.Error(t, xerr, "exec=%t", exec)
		require.True(
			t, xerr.Contains(xerrors.ErrNotFoundStake),
			"exec=%t error=%v", exec, xerr,
		)

		retryTx := web3.NewTrxStaking(
			fixture.delegator.Address(), fixture.validator.Address(),
			rollback.retryNonce,
			govCtrler.MinTrxGas(), govCtrler.GasPrice(), stakeAmount,
		)
		_, _, err = fixture.delegator.SignTrxRLP(
			retryTx, fixture.blockCtx.ChainID(),
		)
		require.NoError(t, err, "exec=%t", exec)
		retryCtx, xerr := mocks.MakeTrxCtxWithTrxBctx(
			retryTx, fixture.blockCtx, exec,
		)
		require.NoError(t, xerr, "exec=%t", exec)
		require.NoError(
			t, fixture.app.txExecutor.ExecuteSync(retryCtx), "exec=%t", exec,
		)
		expectedSender := rollback.assertRetry(
			t, ctx, retryCtx, stakeAmount, nil,
		)

		fixture.commit(t)
		rollback.assertCommittedAccount(t, expectedSender)

		expectedStakes := stakesBefore
		expectedDelegatee := delegateeBefore
		if exec {
			expectedStakes = append(
				expectedStakes,
				testVPowerStakeSnapshot{
					Owner:       fixture.delegator.Address(),
					Delegatee:   fixture.validator.Address(),
					TxHash:      retryCtx.TxHash,
					StartHeight: fixture.blockCtx.Height(),
					Power:       stakePower,
				},
			)
			expectedDelegatee.TotalPower += stakePower
			expectedDelegatee.Delegators = append(
				append(
					[]types.Address(nil),
					delegateeBefore.Delegators...,
				),
				fixture.delegator.Address(),
			)
		}

		committedHeight = fixture.app.lastBlockCtx.Height()
		require.Equal(
			t, expectedStakes,
			testVPowerStakeState(
				t, fixture.app, committedHeight, fixture.delegator.Address(),
			),
			"exec=%t", exec,
		)
		require.Equal(
			t, expectedDelegatee,
			testVPowerDelegateeState(
				t, fixture.app, committedHeight, fixture.validator.Address(),
			),
			"exec=%t", exec,
		)

		cleanup()
	}
}

func Test_Rollback_Gov(t *testing.T) {
	for _, exec := range []bool{false, true} {
		fixture, cleanup := newTrxRollbackFixture(t)
		govCtrler := fixture.app.govCtrler
		chainID := fixture.blockCtx.ChainID()
		proposalAccount := testAccountState(
			t, fixture.app, fixture.validator, exec,
		)
		startVotingHeight := fixture.blockCtx.Height() + 1
		votingPeriodBlocks := govCtrler.MinVotingPeriodBlocks()
		applyingHeight := startVotingHeight +
			votingPeriodBlocks + govCtrler.LazyApplyingBlocks()

		proposalTx := web3.NewTrxProposal(
			fixture.validator.Address(), types.ZeroAddress(),
			proposalAccount.nonce,
			govCtrler.MinTrxGas(), govCtrler.GasPrice(),
			"state unchanged on failed voting",
			startVotingHeight, votingPeriodBlocks, applyingHeight,
			proposal.PROPOSAL_COMMON, []byte("option"),
		)
		_, _, err := fixture.validator.SignTrxRLP(proposalTx, chainID)
		require.NoError(t, err, "exec=%t", exec)
		proposalCtx, xerr := mocks.MakeTrxCtxWithTrxBctx(
			proposalTx, fixture.blockCtx, exec,
		)
		require.NoError(t, xerr, "exec=%t", exec)
		require.NoError(
			t, fixture.app.txExecutor.ExecuteSync(proposalCtx),
			"exec=%t", exec,
		)

		proposalBefore, xerr := govCtrler.ReadProposal(
			proposalCtx.TxHash, exec,
		)
		require.NoError(t, xerr, "exec=%t", exec)
		proposalStateBefore, xerr := proposalBefore.Encode()
		require.NoError(t, xerr, "exec=%t", exec)
		accountBefore := testAccountState(
			t, fixture.app, fixture.validator, exec,
		)
		blockGasBefore := fixture.blockCtx.GetBlockGasUsed()

		votingTx := web3.NewTrxVoting(
			fixture.validator.Address(), types.ZeroAddress(),
			accountBefore.nonce,
			govCtrler.MinTrxGas(), govCtrler.GasPrice(),
			proposalCtx.TxHash, 1,
		)
		_, _, err = fixture.validator.SignTrxRLP(votingTx, chainID)
		require.NoError(t, err, "exec=%t", exec)
		votingCtx, xerr := mocks.MakeTrxCtxWithTrxBctx(
			votingTx, fixture.blockCtx, exec,
		)
		require.NoError(t, xerr, "exec=%t", exec)
		xerr = fixture.app.txExecutor.ExecuteSync(votingCtx)
		require.Equal(
			t, xerrors.ErrInvalidTrxPayloadParams, xerr,
			"exec=%t", exec,
		)
		require.Zero(t, votingCtx.GasUsed, "exec=%t", exec)

		proposalAfter, xerr := govCtrler.ReadProposal(
			proposalCtx.TxHash, exec,
		)
		require.NoError(t, xerr, "exec=%t", exec)
		proposalStateAfter, xerr := proposalAfter.Encode()
		require.NoError(t, xerr, "exec=%t", exec)
		require.Equal(
			t, proposalStateBefore, proposalStateAfter,
			"exec=%t", exec,
		)
		require.Equal(
			t, accountBefore,
			testAccountState(t, fixture.app, fixture.validator, exec),
			"exec=%t", exec,
		)
		require.Equal(
			t, blockGasBefore, fixture.blockCtx.GetBlockGasUsed(),
			"exec=%t", exec,
		)

		cleanup()
	}
}

func Test_CacheSequence(t *testing.T) {
	sender := makeTestWallet(1)
	receiver := makeTestWallet(2)
	initialBalance := uint256.NewInt(balance)
	sender.GetAccount().SetBalance(initialBalance)
	receiver.GetAccount().SetBalance(uint256.NewInt(0))

	acctCtrler, bctx, txe, chainID, cleanup := newAcctExecutorFixture(t, sender, receiver)
	defer cleanup()

	fee := types.GasToFee(govMock.MinTrxGas(), govMock.GasPrice())
	amount0 := uint256.NewInt(1000)
	discardedAmount := uint256.NewInt(2000)
	amount2 := uint256.NewInt(3000)

	ctx0 := makeTransferCtx(
		t, bctx, chainID,
		sender, receiver.Address(), nil,
		0, amount0, true,
	)
	ctx1 := makeTransferCtx(
		t, bctx, chainID,
		sender, receiver.Address(), nil,
		1, discardedAmount, true,
	)
	ctx2 := makeTransferCtx(
		t, bctx, chainID,
		sender, receiver.Address(), nil,
		2, amount2, true,
	)

	require.NoError(t, txe.ExecuteSync(ctx0))

	wantErr := xerrors.NewOrdinary("receiver account set failure")
	failureHandler := &accountTransferFailureHandler{
		accountSetFailureHandler: &accountSetFailureHandler{
			IAccountHandler: acctCtrler,
			failureAddress:  receiver.Address(),
			setAccountErr:   wantErr,
		},
	}
	bctx.AcctHandler = failureHandler
	xerr := txe.ExecuteSync(ctx1)
	require.Equal(t, wantErr, xerr)
	require.True(t, failureHandler.failed)
	require.Equal(t, receiver.Address(), failureHandler.failedAddress)

	bctx.AcctHandler = acctCtrler
	require.NoError(t, txe.ExecuteSync(ctx2))

	actualSender := acctCtrler.FindAccount(sender.Address(), true)
	require.NotNil(t, actualSender)
	require.Equal(t, int64(3), actualSender.GetNonce())
	expectedSenderBalance := initialBalance.Clone()
	expectedSenderBalance.Sub(
		expectedSenderBalance,
		new(uint256.Int).Add(
			new(uint256.Int).Add(amount0, amount2),
			new(uint256.Int).Mul(fee, uint256.NewInt(3)),
		),
	)
	require.Equal(t, expectedSenderBalance.Dec(), actualSender.GetBalance().Dec())

	actualReceiver := acctCtrler.FindAccount(receiver.Address(), true)
	require.NotNil(t, actualReceiver)
	expectedReceiverBalance := new(uint256.Int).Add(amount0, amount2)
	require.Equal(t, expectedReceiverBalance.Dec(), actualReceiver.GetBalance().Dec())
}

func Test_CheckTxCacheSequence(t *testing.T) {
	sender := makeTestWallet(1)
	receiver := makeTestWallet(2)
	initialBalance := uint256.NewInt(balance)
	sender.GetAccount().SetBalance(initialBalance)
	receiver.GetAccount().SetBalance(uint256.NewInt(0))

	acctCtrler, bctx, txe, chainID, cleanup := newAcctExecutorFixture(t, sender, receiver)
	defer cleanup()
	fee := types.GasToFee(govMock.MinTrxGas(), govMock.GasPrice())
	amount0 := uint256.NewInt(1_000)
	amount1 := uint256.NewInt(2_000)

	ctx0 := makeTransferCtx(
		t, bctx, chainID,
		sender, receiver.Address(), nil,
		0, amount0, false,
	)
	require.NoError(t, txe.ExecuteSync(ctx0))

	ctx1 := makeTransferCtx(
		t, bctx, chainID,
		sender, receiver.Address(), nil,
		1, amount1, false,
	)
	require.NoError(t, txe.ExecuteSync(ctx1))

	checkSender := acctCtrler.FindAccount(sender.Address(), false)
	require.NotNil(t, checkSender)
	require.Equal(t, int64(2), checkSender.GetNonce())
	expectedCheckBalance := initialBalance.Clone()
	expectedCheckBalance.Sub(
		expectedCheckBalance,
		new(uint256.Int).Add(
			new(uint256.Int).Add(amount0, amount1),
			new(uint256.Int).Mul(fee, uint256.NewInt(2)),
		),
	)
	require.Equal(t, expectedCheckBalance.Dec(), checkSender.GetBalance().Dec())

	checkReceiver := acctCtrler.FindAccount(receiver.Address(), false)
	require.NotNil(t, checkReceiver)
	require.Equal(t, new(uint256.Int).Add(amount0, amount1).Dec(), checkReceiver.GetBalance().Dec())

	deliverSender := acctCtrler.FindAccount(sender.Address(), true)
	require.NotNil(t, deliverSender)
	require.Zero(t, deliverSender.GetNonce())
	require.Equal(t, initialBalance.Dec(), deliverSender.GetBalance().Dec())
	deliverReceiver := acctCtrler.FindAccount(receiver.Address(), true)
	require.NotNil(t, deliverReceiver)
	require.Zero(t, deliverReceiver.GetBalance().Sign())
}

func Test_CheckTxFailureCleanup(t *testing.T) {
	sender := makeTestWallet(1)
	receiver := makeTestWallet(2)
	initialBalance := uint256.NewInt(balance)
	sender.GetAccount().SetBalance(initialBalance)
	receiver.GetAccount().SetBalance(uint256.NewInt(0))

	acctCtrler, bctx, txe, chainID, cleanup := newAcctExecutorFixture(t, sender, receiver)
	defer cleanup()
	fee := types.GasToFee(govMock.MinTrxGas(), govMock.GasPrice())
	amount0 := uint256.NewInt(1_000)
	discardedAmount := uint256.NewInt(2_000)
	amount2 := uint256.NewInt(3_000)

	ctx0 := makeTransferCtx(
		t, bctx, chainID,
		sender, receiver.Address(), nil,
		0, amount0, false,
	)
	require.NoError(t, txe.ExecuteSync(ctx0))
	usedGas := bctx.GetBlockGasUsed()

	wantErr := xerrors.NewOrdinary("receiver account set failure")
	failureHandler := &accountTransferFailureHandler{
		accountSetFailureHandler: &accountSetFailureHandler{
			IAccountHandler: acctCtrler,
			failureAddress:  receiver.Address(),
			setAccountErr:   wantErr,
		},
	}
	bctx.AcctHandler = failureHandler
	failedCtx := makeTransferCtx(
		t, bctx, chainID,
		sender, receiver.Address(), nil,
		1, discardedAmount, false,
	)
	xerr := txe.ExecuteSync(failedCtx)
	require.Equal(t, wantErr, xerr)
	require.True(t, failureHandler.failed)
	require.Equal(t, receiver.Address(), failureHandler.failedAddress)
	require.Zero(t, failedCtx.GasUsed)
	require.Equal(t, usedGas, bctx.GetBlockGasUsed())

	bctx.AcctHandler = acctCtrler
	checkSender := acctCtrler.FindAccount(sender.Address(), false)
	require.NotNil(t, checkSender)
	require.Equal(t, int64(1), checkSender.GetNonce())
	expectedAfterFailure := initialBalance.Clone()
	expectedAfterFailure.Sub(
		expectedAfterFailure,
		new(uint256.Int).Add(amount0, fee),
	)
	require.Equal(t, expectedAfterFailure.Dec(), checkSender.GetBalance().Dec())
	checkReceiver := acctCtrler.FindAccount(receiver.Address(), false)
	require.NotNil(t, checkReceiver)
	require.Equal(t, amount0.Dec(), checkReceiver.GetBalance().Dec())

	ctx2 := makeTransferCtx(
		t, bctx, chainID,
		sender, receiver.Address(), nil,
		1, amount2, false,
	)
	require.NoError(t, txe.ExecuteSync(ctx2))

	checkSender = acctCtrler.FindAccount(sender.Address(), false)
	require.NotNil(t, checkSender)
	require.Equal(t, int64(2), checkSender.GetNonce())
	expectedAfterSuccess := initialBalance.Clone()
	expectedAfterSuccess.Sub(
		expectedAfterSuccess,
		new(uint256.Int).Add(
			new(uint256.Int).Add(amount0, amount2),
			new(uint256.Int).Mul(fee, uint256.NewInt(2)),
		),
	)
	require.Equal(t, expectedAfterSuccess.Dec(), checkSender.GetBalance().Dec())
	checkReceiver = acctCtrler.FindAccount(receiver.Address(), false)
	require.NotNil(t, checkReceiver)
	require.Equal(t, new(uint256.Int).Add(amount0, amount2).Dec(), checkReceiver.GetBalance().Dec())
}

func Test_CachePayer(t *testing.T) {
	//
	// Separate receiver
	sender := makeTestWallet(1)
	payer := makeTestWallet(2)
	receiver := makeTestWallet(3)
	initialSenderBalance := uint256.NewInt(balance)
	initialPayerBalance := uint256.NewInt(balance)
	sender.GetAccount().SetBalance(initialSenderBalance)
	payer.GetAccount().SetBalance(initialPayerBalance)
	receiver.GetAccount().SetBalance(uint256.NewInt(0))

	acctCtrler, bctx, txe, chainID, cleanup := newAcctExecutorFixture(t, sender, payer, receiver)
	fee := types.GasToFee(govMock.MinTrxGas(), govMock.GasPrice())

	amount0 := uint256.NewInt(1000)
	ctx0 := makeTransferCtx(
		t, bctx, chainID,
		sender, receiver.Address(), payer,
		0, amount0, true,
	)
	require.NoError(t, txe.ExecuteSync(ctx0), "case=separate_receiver")
	require.Equal(
		t, govMock.MinTrxGas(), ctx0.GasUsed, "case=separate_receiver",
	)

	actualSender := acctCtrler.FindAccount(sender.Address(), true)
	require.NotNil(t, actualSender, "case=separate_receiver account=sender")
	require.Equal(
		t, int64(1), actualSender.GetNonce(), "case=separate_receiver account=sender",
	)
	expectedSenderBalance := new(uint256.Int).Sub(initialSenderBalance, amount0)
	require.Equal(
		t, expectedSenderBalance.Dec(), actualSender.GetBalance().Dec(),
		"case=separate_receiver account=sender",
	)

	actualPayer := acctCtrler.FindAccount(payer.Address(), true)
	require.NotNil(t, actualPayer, "case=separate_receiver account=payer")
	require.Zero(t, actualPayer.GetNonce(), "case=separate_receiver account=payer")
	expectedPayerBalance := new(uint256.Int).Sub(initialPayerBalance, fee)
	require.Equal(
		t, expectedPayerBalance.Dec(), actualPayer.GetBalance().Dec(),
		"case=separate_receiver account=payer",
	)

	actualReceiver := acctCtrler.FindAccount(receiver.Address(), true)
	require.NotNil(t, actualReceiver, "case=separate_receiver account=receiver")
	require.Equal(
		t, amount0.Dec(), actualReceiver.GetBalance().Dec(),
		"case=separate_receiver account=receiver",
	)

	wantErr := xerrors.NewOrdinary("receiver account set failure")
	failureHandler := &accountTransferFailureHandler{
		accountSetFailureHandler: &accountSetFailureHandler{
			IAccountHandler: acctCtrler,
			failureAddress:  receiver.Address(),
			setAccountErr:   wantErr,
		},
	}
	bctx.AcctHandler = failureHandler
	amount1 := uint256.NewInt(2000)
	ctx1 := makeTransferCtx(
		t, bctx, chainID,
		sender, receiver.Address(), payer,
		1, amount1, true,
	)
	xerr := txe.ExecuteSync(ctx1)
	require.Equal(t, wantErr, xerr, "case=separate_receiver")
	require.True(t, failureHandler.failed, "case=separate_receiver")
	require.Equal(
		t, receiver.Address(), failureHandler.failedAddress,
		"case=separate_receiver",
	)
	require.Equal(
		t, govMock.MinTrxGas(), ctx1.GasUsed, "case=separate_receiver",
	)

	actualSender = acctCtrler.FindAccount(sender.Address(), true)
	require.NotNil(t, actualSender, "case=separate_receiver account=sender")
	require.Equal(
		t, int64(2), actualSender.GetNonce(), "case=separate_receiver account=sender",
	)
	require.Equal(
		t, expectedSenderBalance.Dec(), actualSender.GetBalance().Dec(),
		"case=separate_receiver account=sender",
	)

	actualPayer = acctCtrler.FindAccount(payer.Address(), true)
	require.NotNil(t, actualPayer, "case=separate_receiver account=payer")
	expectedPayerBalance.Sub(expectedPayerBalance, fee)
	require.Equal(
		t, expectedPayerBalance.Dec(), actualPayer.GetBalance().Dec(),
		"case=separate_receiver account=payer",
	)

	actualReceiver = acctCtrler.FindAccount(receiver.Address(), true)
	require.NotNil(t, actualReceiver, "case=separate_receiver account=receiver")
	require.Equal(
		t, amount0.Dec(), actualReceiver.GetBalance().Dec(),
		"case=separate_receiver account=receiver",
	)

	cleanup()

	//
	// Receiver as payer
	sender = makeTestWallet(1)
	payer = makeTestWallet(2)
	initialSenderBalance = uint256.NewInt(balance)
	initialPayerBalance = uint256.NewInt(balance)
	sender.GetAccount().SetBalance(initialSenderBalance)
	payer.GetAccount().SetBalance(initialPayerBalance)

	acctCtrler, bctx, txe, chainID, cleanup = newAcctExecutorFixture(t, sender, payer)
	fee = types.GasToFee(govMock.MinTrxGas(), govMock.GasPrice())
	amount := uint256.NewInt(1_000)
	ctx := makeTransferCtx(
		t, bctx, chainID,
		sender, payer.Address(), payer,
		0, amount, true,
	)

	require.NoError(t, txe.ExecuteSync(ctx), "case=receiver_as_payer")

	actualSender = acctCtrler.FindAccount(sender.Address(), true)
	require.NotNil(t, actualSender, "case=receiver_as_payer account=sender")
	require.Equal(
		t, int64(1), actualSender.GetNonce(), "case=receiver_as_payer account=sender",
	)
	expectedSenderBalance = new(uint256.Int).Sub(initialSenderBalance.Clone(), amount)
	require.Equal(
		t, expectedSenderBalance.Dec(), actualSender.GetBalance().Dec(),
		"case=receiver_as_payer account=sender",
	)

	actualPayer = acctCtrler.FindAccount(payer.Address(), true)
	require.NotNil(t, actualPayer, "case=receiver_as_payer account=payer")
	require.Zero(t, actualPayer.GetNonce(), "case=receiver_as_payer account=payer")
	expectedPayerBalance = new(uint256.Int).Add(initialPayerBalance.Clone(), amount)
	expectedPayerBalance.Sub(expectedPayerBalance, fee)
	require.Equal(
		t, expectedPayerBalance.Dec(), actualPayer.GetBalance().Dec(),
		"case=receiver_as_payer account=payer",
	)

	cleanup()
}

func Test_SelfTransfer(t *testing.T) {
	//
	// Without payer
	sender := makeTestWallet(1)
	receiver := makeTestWallet(2)
	initialBalance := uint256.NewInt(balance)
	sender.GetAccount().SetBalance(initialBalance)
	receiver.GetAccount().SetBalance(uint256.NewInt(0))

	acctCtrler, bctx, txe, chainID, cleanup := newAcctExecutorFixture(t, sender, receiver)
	fee := types.GasToFee(govMock.MinTrxGas(), govMock.GasPrice())

	selfAmount := uint256.NewInt(1_000)
	selfCtx := makeTransferCtx(
		t, bctx, chainID,
		sender, sender.Address(), nil,
		0, selfAmount, true,
	)
	require.NoError(t, txe.ExecuteSync(selfCtx), "case=without_payer")
	require.Equal(
		t, govMock.MinTrxGas(), selfCtx.GasUsed, "case=without_payer",
	)

	actualSender := acctCtrler.FindAccount(sender.Address(), true)
	require.NotNil(t, actualSender, "case=without_payer account=sender")
	require.Equal(
		t, int64(1), actualSender.GetNonce(), "case=without_payer account=sender",
	)
	expectedSenderBalance := new(uint256.Int).Sub(initialBalance.Clone(), fee)
	require.Equal(
		t, expectedSenderBalance.Dec(), actualSender.GetBalance().Dec(),
		"case=without_payer account=sender",
	)

	transferAmount := uint256.NewInt(2_000)
	transferCtx := makeTransferCtx(
		t, bctx, chainID,
		sender, receiver.Address(), nil,
		1, transferAmount, true,
	)
	require.NoError(t, txe.ExecuteSync(transferCtx), "case=without_payer")
	require.Equal(
		t, govMock.MinTrxGas(), transferCtx.GasUsed, "case=without_payer",
	)

	actualSender = acctCtrler.FindAccount(sender.Address(), true)
	require.NotNil(t, actualSender, "case=without_payer account=sender")
	require.Equal(
		t, int64(2), actualSender.GetNonce(), "case=without_payer account=sender",
	)
	expectedSenderBalance.Sub(
		expectedSenderBalance,
		new(uint256.Int).Add(transferAmount, fee),
	)
	require.Equal(
		t, expectedSenderBalance.Dec(), actualSender.GetBalance().Dec(),
		"case=without_payer account=sender",
	)

	actualReceiver := acctCtrler.FindAccount(receiver.Address(), true)
	require.NotNil(t, actualReceiver, "case=without_payer account=receiver")
	require.Equal(
		t, transferAmount.Dec(), actualReceiver.GetBalance().Dec(),
		"case=without_payer account=receiver",
	)

	cleanup()

	//
	// With payer
	sender = makeTestWallet(1)
	payer := makeTestWallet(2)
	initialSenderBalance := uint256.NewInt(balance)
	initialPayerBalance := uint256.NewInt(balance)
	sender.GetAccount().SetBalance(initialSenderBalance)
	payer.GetAccount().SetBalance(initialPayerBalance)

	acctCtrler, bctx, txe, chainID, cleanup = newAcctExecutorFixture(t, sender, payer)
	fee = types.GasToFee(govMock.MinTrxGas(), govMock.GasPrice())
	ctx := makeTransferCtx(
		t, bctx, chainID,
		sender, sender.Address(), payer,
		0, uint256.NewInt(1_000), true,
	)

	require.NoError(t, txe.ExecuteSync(ctx), "case=with_payer")
	require.Equal(t, govMock.MinTrxGas(), ctx.GasUsed, "case=with_payer")

	actualSender = acctCtrler.FindAccount(sender.Address(), true)
	require.NotNil(t, actualSender, "case=with_payer account=sender")
	require.Equal(
		t, int64(1), actualSender.GetNonce(), "case=with_payer account=sender",
	)
	require.Equal(
		t, initialSenderBalance.Dec(), actualSender.GetBalance().Dec(),
		"case=with_payer account=sender",
	)

	actualPayer := acctCtrler.FindAccount(payer.Address(), true)
	require.NotNil(t, actualPayer, "case=with_payer account=payer")
	require.Zero(t, actualPayer.GetNonce(), "case=with_payer account=payer")
	expectedPayerBalance := new(uint256.Int).Sub(initialPayerBalance.Clone(), fee)
	require.Equal(
		t, expectedPayerBalance.Dec(), actualPayer.GetBalance().Dec(),
		"case=with_payer account=payer",
	)

	cleanup()
}

func Test_SetDocSameAddress(t *testing.T) {
	sender := makeTestWallet(1)
	initialBalance := uint256.NewInt(balance)
	sender.GetAccount().SetBalance(initialBalance)

	acctCtrler, bctx, txe, chainID, cleanup := newAcctExecutorFixture(t, sender)
	defer cleanup()
	name, docURL := "self-doc", "https://example.com/self-doc"
	tx := web3.NewTrxSetDoc(
		sender.Address(), 0,
		govMock.MinTrxGas(), govMock.GasPrice(),
		name, docURL,
	)
	tx.To = sender.Address()
	_, _, err := sender.SignTrxRLP(tx, chainID)
	require.NoError(t, err)
	ctx, xerr := mocks.MakeTrxCtxWithTrxBctx(tx, bctx, true)
	require.NoError(t, xerr)

	require.NoError(t, txe.ExecuteSync(ctx))

	actual := acctCtrler.FindAccount(sender.Address(), true)
	require.NotNil(t, actual)
	require.Equal(t, name, actual.GetName())
	require.Equal(t, docURL, actual.GetDocURL())
	require.Equal(t, int64(1), actual.GetNonce())
	expectedBalance := new(uint256.Int).Sub(
		initialBalance.Clone(),
		types.GasToFee(govMock.MinTrxGas(), govMock.GasPrice()),
	)
	require.Equal(t, expectedBalance.Dec(), actual.GetBalance().Dec())
}

func Test_CacheReceiverSequence(t *testing.T) {
	sender0 := makeTestWallet(1)
	sender1 := makeTestWallet(2)
	receiver := makeTestWallet(3)
	initialBalance := uint256.NewInt(balance)
	sender0.GetAccount().SetBalance(initialBalance)
	sender1.GetAccount().SetBalance(initialBalance)
	receiver.GetAccount().SetBalance(uint256.NewInt(0))

	acctCtrler, bctx, txe, chainID, cleanup := newAcctExecutorFixture(t, sender0, sender1, receiver)
	defer cleanup()
	fee := types.GasToFee(govMock.MinTrxGas(), govMock.GasPrice())
	amount0 := uint256.NewInt(1000)
	amount1 := uint256.NewInt(2000)

	ctx0 := makeTransferCtx(
		t, bctx, chainID,
		sender0, receiver.Address(), nil,
		0, amount0, true,
	)
	require.NoError(t, txe.ExecuteSync(ctx0))
	require.Equal(t, govMock.MinTrxGas(), ctx0.GasUsed)
	ctx1 := makeTransferCtx(
		t, bctx, chainID,
		sender1, receiver.Address(), nil,
		0, amount1, true,
	)
	require.NoError(t, txe.ExecuteSync(ctx1))
	require.Equal(t, govMock.MinTrxGas(), ctx1.GasUsed)

	actualSender0 := acctCtrler.FindAccount(sender0.Address(), true)
	require.NotNil(t, actualSender0)
	expectedSender0Balance := initialBalance.Clone()
	expectedSender0Balance.Sub(
		expectedSender0Balance,
		new(uint256.Int).Add(amount0, fee),
	)
	require.Equal(t, int64(1), actualSender0.GetNonce())
	require.Equal(t, expectedSender0Balance.Dec(), actualSender0.GetBalance().Dec())

	actualSender1 := acctCtrler.FindAccount(sender1.Address(), true)
	require.NotNil(t, actualSender1)
	expectedSender1Balance := initialBalance.Clone()
	expectedSender1Balance.Sub(
		expectedSender1Balance,
		new(uint256.Int).Add(amount1, fee),
	)
	require.Equal(t, int64(1), actualSender1.GetNonce())
	require.Equal(t, expectedSender1Balance.Dec(), actualSender1.GetBalance().Dec())

	actualReceiver := acctCtrler.FindAccount(receiver.Address(), true)
	require.NotNil(t, actualReceiver)
	expectedReceiverBalance := new(uint256.Int).Add(amount0, amount1)
	require.Equal(t, expectedReceiverBalance.Dec(), actualReceiver.GetBalance().Dec())
}

func Test_ExecutionHalt(t *testing.T) {
	sender := makeTestWallet(1)
	receiver := makeTestWallet(2)
	sender.GetAccount().SetBalance(uint256.NewInt(balance))

	acctCtrler, bctx, txe, chainID, cleanup := newAcctExecutorFixture(t, sender, receiver)
	defer cleanup()
	expected := xerrors.NewOrdinary("cache error")
	bctx.AcctHandler = &createCacheErrorAcctHandler{
		IAccountHandler: acctCtrler,
		createErr:       expected,
	}
	ctx := makeTransferCtx(
		t, bctx, chainID,
		sender, receiver.Address(), nil,
		sender.GetNonce(), uint256.NewInt(1), true,
	)
	actualSender := acctCtrler.FindAccount(sender.Address(), true)
	require.NotNil(t, actualSender)
	nonce := actualSender.GetNonce()
	balance := actualSender.GetBalance()

	xerr := txe.ExecuteSync(ctx)

	require.Equal(t, expected, xerr)
	actualSender = acctCtrler.FindAccount(sender.Address(), true)
	require.NotNil(t, actualSender)
	require.Equal(t, nonce, actualSender.GetNonce())
	require.Equal(t, balance.Dec(), actualSender.GetBalance().Dec())
	require.Zero(t, ctx.GasUsed)
}
