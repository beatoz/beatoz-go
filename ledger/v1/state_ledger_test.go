package v1

import (
	"testing"

	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"
)

func TestStateLedger_SnapshotDelegatesByExecMode(t *testing.T) {
	ledger, xerr := NewStateLedger("state_ledger_test", t.TempDir(), 100, func(key LedgerKey) ILedgerItem {
		return &Item{}
	}, log.NewNopLogger())
	require.NoError(t, xerr)
	t.Cleanup(func() {
		require.NoError(t, ledger.Close())
	})

	require.NoError(t, ledger.Set(newItem(800, "").Key(), newItem(800, "base"), true))
	_, _, xerr = ledger.Commit()
	require.NoError(t, xerr)

	execKey := newItem(801, "").Key()
	simulationKey := newItem(802, "").Key()
	execSnap := ledger.Snapshot(true)
	simulationSnap := ledger.Snapshot(false)

	require.NoError(t, ledger.Set(execKey, newItem(801, "exec"), true))
	require.NoError(t, ledger.Set(simulationKey, newItem(802, "simulation"), false))

	require.NoError(t, ledger.RevertToSnapshot(execSnap, true))
	_, xerr = ledger.Get(execKey, true)
	require.Error(t, xerr)
	require.True(t, xerr.Contains(xerrors.ErrNotFoundResult))
	requireStateItemData(t, ledger, simulationKey, "simulation", false)

	require.NoError(t, ledger.RevertToSnapshot(simulationSnap, false))
	_, xerr = ledger.Get(simulationKey, false)
	require.Error(t, xerr)
	require.True(t, xerr.Contains(xerrors.ErrNotFoundResult))
}

func TestStateLedger_RejectsSnapshotFromOtherExecMode(t *testing.T) {
	ledger, xerr := NewStateLedger("state_ledger_test", t.TempDir(), 100, func(key LedgerKey) ILedgerItem {
		return &Item{}
	}, log.NewNopLogger())
	require.NoError(t, xerr)
	t.Cleanup(func() {
		require.NoError(t, ledger.Close())
	})

	require.NoError(t, ledger.Set(newItem(810, "").Key(), newItem(810, "base"), true))
	_, _, xerr = ledger.Commit()
	require.NoError(t, xerr)

	execSnap := ledger.Snapshot(true)
	simulationSnap := ledger.Snapshot(false)

	xerr = ledger.RevertToSnapshot(execSnap, false)
	require.Error(t, xerr)
	require.True(t, xerr.Contains(xerrors.ErrInvalidSnapshot))

	xerr = ledger.RevertToSnapshot(simulationSnap, true)
	require.Error(t, xerr)
	require.True(t, xerr.Contains(xerrors.ErrInvalidSnapshot))
}

func requireStateItemData(
	t *testing.T,
	ledger *StateLedger,
	key LedgerKey,
	want string,
	exec bool,
) {
	t.Helper()

	item, xerr := ledger.Get(key, exec)
	require.NoError(t, xerr)
	require.Equal(t, want, item.(*Item).data)
}
