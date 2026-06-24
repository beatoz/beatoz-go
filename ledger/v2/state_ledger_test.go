package v2

import (
	"testing"

	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"
)

func TestStateLedgerExecCacheWrite(t *testing.T) {
	ledger := newStateLedgerForTest(t)

	cached, writeCache := ledger.CacheContext(true)
	require.NoError(t, cached.Set(testLedgerKey(1), newTestLedgerItem(1, "tx"), true))

	_, xerr := ledger.Get(testLedgerKey(1), true)
	require.Error(t, xerr)
	require.True(t, xerr.Contains(xerrors.ErrNotFoundResult))

	require.NoError(t, writeCache())

	item, xerr := ledger.Get(testLedgerKey(1), true)
	require.NoError(t, xerr)
	require.Equal(t, "tx", item.(*testLedgerItem).data)
}

func TestStateLedgerExecCacheDiscard(t *testing.T) {
	ledger := newStateLedgerForTest(t)

	cached, _ := ledger.CacheContext(true)
	require.NoError(t, cached.Set(testLedgerKey(1), newTestLedgerItem(1, "tx"), true))
	cached = nil

	_, xerr := ledger.Get(testLedgerKey(1), true)
	require.Error(t, xerr)
	require.True(t, xerr.Contains(xerrors.ErrNotFoundResult))
	require.Nil(t, cached)
}

func TestStateLedgerCommitRefreshesImitable(t *testing.T) {
	ledger := newStateLedgerForTest(t)

	require.NoError(t, ledger.Set(testLedgerKey(1), newTestLedgerItem(1, "one"), true))
	_, ver, xerr := ledger.Commit()
	require.NoError(t, xerr)
	require.EqualValues(t, 1, ver)

	item, xerr := ledger.Get(testLedgerKey(1), false)
	require.NoError(t, xerr)
	require.Equal(t, "one", item.(*testLedgerItem).data)

	atLedger, xerr := ledger.ImitableLedgerAt(ver)
	require.NoError(t, xerr)
	item, xerr = atLedger.Get(testLedgerKey(1))
	require.NoError(t, xerr)
	require.Equal(t, "one", item.(*testLedgerItem).data)
}

func TestStateLedgerMemCacheWrite(t *testing.T) {
	ledger := newStateLedgerForTest(t)
	require.NoError(t, ledger.Set(testLedgerKey(1), newTestLedgerItem(1, "one"), true))
	_, _, xerr := ledger.Commit()
	require.NoError(t, xerr)

	cached, writeCache := ledger.CacheContext(false)
	require.NoError(t, cached.Set(testLedgerKey(2), newTestLedgerItem(2, "check"), false))

	_, xerr = ledger.Get(testLedgerKey(2), false)
	require.Error(t, xerr)
	require.True(t, xerr.Contains(xerrors.ErrNotFoundResult))

	require.NoError(t, writeCache())

	item, xerr := ledger.Get(testLedgerKey(2), false)
	require.NoError(t, xerr)
	require.Equal(t, "check", item.(*testLedgerItem).data)

	_, xerr = ledger.Get(testLedgerKey(2), true)
	require.Error(t, xerr)
	require.True(t, xerr.Contains(xerrors.ErrNotFoundResult))
}

func TestStateLedgerGuards(t *testing.T) {
	ledger := newStateLedgerForTest(t)

	require.NotPanics(t, func() {
		require.Error(t, ledger.Set(testLedgerKey(1), nil, true))
	})
	require.NotPanics(t, func() {
		require.Error(t, ledger.Seek(nil, true, nil, true))
	})

	require.NoError(t, ledger.Close())
	require.EqualValues(t, 0, ledger.Version())
	require.NotPanics(t, func() {
		_, xerr := ledger.Get(testLedgerKey(1), true)
		require.Error(t, xerr)
	})
	require.Error(t, ledger.Iterate(func(LedgerKey, ILedgerItem) xerrors.XError {
		return nil
	}, true))
	require.Error(t, ledger.Seek(nil, true, func(LedgerKey, ILedgerItem) xerrors.XError {
		return nil
	}, true))
	require.Error(t, ledger.Set(testLedgerKey(1), newTestLedgerItem(1, "one"), true))
	require.Error(t, ledger.Del(testLedgerKey(1), true))
	cached, writeCache := ledger.CacheContext(true)
	require.Nil(t, cached)
	require.Error(t, writeCache())
	_, _, xerr := ledger.Commit()
	require.Error(t, xerr)
	_, xerr = ledger.ImitableLedgerAt(1)
	require.Error(t, xerr)
}

func newStateLedgerForTest(t *testing.T) *StateLedger {
	t.Helper()

	ledger, xerr := NewStateLedger("state_ledger_test", t.TempDir(), 100, func(LedgerKey) ILedgerItem {
		return &testLedgerItem{}
	}, log.NewNopLogger())
	require.NoError(t, xerr)
	t.Cleanup(func() {
		require.NoError(t, ledger.Close())
	})
	return ledger
}
