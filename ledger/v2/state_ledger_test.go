package v2

import (
	"os"
	"testing"
	"time"

	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"
)

func newStateTestLedger(t *testing.T) (*StateLedger, func()) {
	rootDir, err := os.MkdirTemp("", "state-ledger-")
	require.NoError(t, err)
	ledger, xerr := NewStateLedger(
		"state", rootDir, 100, newMutableTestItem, log.NewNopLogger(),
	)
	if xerr != nil {
		_ = os.RemoveAll(rootDir)
	}
	require.NoError(t, xerr)
	cleanup := func() {
		_ = ledger.Close()
		_ = os.RemoveAll(rootDir)
	}
	return ledger, cleanup
}

func TestStateLedger_Cache_Write(t *testing.T) {
	for _, exec := range []bool{false, true} {
		ledger, cleanup := newStateTestLedger(t)
		defer cleanup()

		require.NoError(t, ledger.CreateCache(exec), "exec=%t", exec)
		require.NoError(t, ledger.Set(
			[]byte("key"), &mutableTestItem{value: []byte("value")}, exec,
		), "exec=%t", exec)
		require.NoError(t, ledger.WriteCache(exec), "exec=%t", exec)
		require.NoError(t, ledger.ClearCache(exec), "exec=%t", exec)

		item, xerr := ledger.Get([]byte("key"), exec)
		require.NoError(t, xerr, "exec=%t", exec)
		require.Equal(t, []byte("value"), item.(*mutableTestItem).value, "exec=%t", exec)
		item, xerr = ledger.Get([]byte("key"), !exec)
		require.Nil(t, item, "exec=%t", exec)
		require.ErrorIs(t, xerr, xerrors.ErrNotFoundResult, "exec=%t", exec)
	}
}

func TestStateLedger_Cache_Clear(t *testing.T) {
	for _, exec := range []bool{false, true} {
		ledger, cleanup := newStateTestLedger(t)
		defer cleanup()

		require.NoError(t, ledger.CreateCache(exec), "exec=%t", exec)
		require.NoError(t, ledger.Set(
			[]byte("key"), &mutableTestItem{value: []byte("value")}, exec,
		), "exec=%t", exec)
		require.NoError(t, ledger.ClearCache(exec), "exec=%t", exec)

		item, xerr := ledger.Get([]byte("key"), exec)
		require.Nil(t, item, "exec=%t", exec)
		require.ErrorIs(t, xerr, xerrors.ErrNotFoundResult, "exec=%t", exec)
	}
}

func TestStateLedger_Cache_NoDeadlock(t *testing.T) {
	for _, exec := range []bool{false, true} {
		ledger, cleanup := newStateTestLedger(t)
		defer cleanup()
		done := make(chan struct {
			cacheErr xerrors.XError
			runErr   xerrors.XError
		}, 1)

		go func() {
			cacheErr := ledger.CreateCache(exec)
			if cacheErr != nil {
				done <- struct {
					cacheErr xerrors.XError
					runErr   xerrors.XError
				}{cacheErr: cacheErr}
				return
			}
			defer func() { _ = ledger.ClearCache(exec) }()

			runErr := ledger.Set(
				[]byte("key"), &mutableTestItem{value: []byte("value")}, exec,
			)
			if runErr == nil {
				_, runErr = ledger.Get([]byte("key"), exec)
			}
			if runErr == nil {
				runErr = ledger.Del([]byte("key"), exec)
			}
			if runErr == nil {
				runErr = ledger.WriteCache(exec)
			}
			done <- struct {
				cacheErr xerrors.XError
				runErr   xerrors.XError
			}{cacheErr: cacheErr, runErr: runErr}
		}()

		select {
		case result := <-done:
			require.NoError(t, result.cacheErr, "exec=%t", exec)
			require.NoError(t, result.runErr, "exec=%t", exec)
		case <-time.After(2 * time.Second):
			t.Fatalf("state ledger cache lifecycle deadlocked: exec=%t", exec)
		}
	}
}

func TestStateLedger_Cache_Routing(t *testing.T) {
	ledger, cleanup := newStateTestLedger(t)
	defer cleanup()

	require.NoError(t, ledger.CreateCache(false))
	require.NoError(t, ledger.Set(
		[]byte("key"), &mutableTestItem{value: []byte("check")}, false,
	))

	require.Error(t, ledger.WriteCache(true))
	require.NoError(t, ledger.ClearCache(true))
	item, xerr := ledger.Get([]byte("key"), false)
	require.NoError(t, xerr)
	require.Equal(t, []byte("check"), item.(*mutableTestItem).value)

	require.NoError(t, ledger.ClearCache(false))
	item, xerr = ledger.Get([]byte("key"), false)
	require.Nil(t, item)
	require.ErrorIs(t, xerr, xerrors.ErrNotFoundResult)
}

func TestStateLedger_Commit_Refresh(t *testing.T) {
	ledger, cleanup := newStateTestLedger(t)
	defer cleanup()
	require.NoError(t, ledger.CreateCache(true))
	defer func() { _ = ledger.ClearCache(true) }()
	require.NoError(t, ledger.Set(
		[]byte("key"), &mutableTestItem{value: []byte("commit")}, true,
	))
	require.NoError(t, ledger.WriteCache(true))
	require.NoError(t, ledger.ClearCache(true))

	_, version, xerr := ledger.Commit()

	require.NoError(t, xerr)
	require.EqualValues(t, 1, version)
	require.Equal(t, version, ledger.Version())
	item, xerr := ledger.Get([]byte("key"), false)
	require.NoError(t, xerr)
	require.Equal(t, []byte("commit"), item.(*mutableTestItem).value)
}

func TestStateLedger_Commit_ActiveCache(t *testing.T) {
	for _, exec := range []bool{false, true} {
		ledger, cleanup := newStateTestLedger(t)
		defer cleanup()
		require.NoError(t, ledger.CreateCache(exec), "exec=%t", exec)

		hash, version, xerr := ledger.Commit()

		require.Nil(t, hash, "exec=%t", exec)
		require.Zero(t, version, "exec=%t", exec)
		require.Error(t, xerr, "exec=%t", exec)
		require.NoError(t, ledger.Set(
			[]byte("key"), &mutableTestItem{value: []byte("pending")}, exec,
		), "exec=%t", exec)
		item, xerr := ledger.Get([]byte("key"), exec)
		require.NoError(t, xerr, "exec=%t", exec)
		require.Equal(t, []byte("pending"), item.(*mutableTestItem).value, "exec=%t", exec)
		require.NoError(t, ledger.ClearCache(exec), "exec=%t", exec)
	}
}

func TestStateLedger_Close_ActiveCache(t *testing.T) {
	for _, exec := range []bool{false, true} {
		ledger, cleanup := newStateTestLedger(t)
		defer cleanup()
		require.NoError(t, ledger.CreateCache(exec), "exec=%t", exec)

		xerr := ledger.Close()

		require.Error(t, xerr, "exec=%t", exec)
		require.NoError(t, ledger.Set(
			[]byte("key"), &mutableTestItem{value: []byte("pending")}, exec,
		), "exec=%t", exec)
		item, xerr := ledger.Get([]byte("key"), exec)
		require.NoError(t, xerr, "exec=%t", exec)
		require.Equal(t, []byte("pending"), item.(*mutableTestItem).value, "exec=%t", exec)
		require.NoError(t, ledger.ClearCache(exec), "exec=%t", exec)
		require.NoError(t, ledger.Close(), "exec=%t", exec)
	}
}

func TestStateLedger_Routing(t *testing.T) {
	ledger, cleanup := newStateTestLedger(t)
	defer cleanup()
	require.NoError(t, ledger.Set(
		[]byte("key"), &mutableTestItem{value: []byte("check")}, false,
	))

	item, xerr := ledger.Get([]byte("key"), false)
	require.NoError(t, xerr)
	require.Equal(t, []byte("check"), item.(*mutableTestItem).value)
	item, xerr = ledger.Get([]byte("key"), true)
	require.Nil(t, item)
	require.ErrorIs(t, xerr, xerrors.ErrNotFoundResult)
}

func TestStateLedger_Closed(t *testing.T) {
	ledger, cleanup := newStateTestLedger(t)
	defer cleanup()
	require.NoError(t, ledger.Close())

	item, xerr := ledger.Get([]byte("key"), true)
	require.Nil(t, item)
	require.Error(t, xerr)
	require.Error(t, ledger.Set(
		[]byte("key"), &mutableTestItem{value: []byte("value")}, true,
	))
	require.Error(t, ledger.CreateCache(true))
	require.Error(t, ledger.CreateCache(false))
	require.Error(t, ledger.WriteCache(true))
	require.Error(t, ledger.WriteCache(false))
	require.NoError(t, ledger.ClearCache(true))
	require.NoError(t, ledger.ClearCache(false))
	_, _, xerr = ledger.Commit()
	require.Error(t, xerr)
	require.Zero(t, ledger.Version())
}
