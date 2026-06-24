package v2

import (
	"encoding/binary"
	"testing"

	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"
)

func TestMutableLedgerCacheWrite(t *testing.T) {
	ledger := newMutableLedgerForTest(t)
	require.NoError(t, ledger.Set(testLedgerKey(1), newTestLedgerItem(1, "parent")))

	child := ledger.CacheWrap()
	require.NoError(t, child.Set(testLedgerKey(2), newTestLedgerItem(2, "child")))

	_, xerr := ledger.Get(testLedgerKey(2))
	require.Error(t, xerr)
	require.True(t, xerr.Contains(xerrors.ErrNotFoundResult))

	require.NoError(t, child.Write())
	item, xerr := ledger.Get(testLedgerKey(2))
	require.NoError(t, xerr)
	require.Equal(t, "child", item.(*testLedgerItem).data)
}

func TestMutableLedgerCacheDiscard(t *testing.T) {
	ledger := newMutableLedgerForTest(t)

	child := ledger.CacheWrap()
	require.NoError(t, child.Set(testLedgerKey(1), newTestLedgerItem(1, "child")))
	child = nil

	_, xerr := ledger.Get(testLedgerKey(1))
	require.Error(t, xerr)
	require.True(t, xerr.Contains(xerrors.ErrNotFoundResult))
	require.Nil(t, child)
}

func TestMutableLedgerCachePrecedence(t *testing.T) {
	ledger := newMutableLedgerForTest(t)
	require.NoError(t, ledger.Set(testLedgerKey(1), newTestLedgerItem(1, "tree")))
	_, _, xerr := ledger.Commit()
	require.NoError(t, xerr)

	require.NoError(t, ledger.Set(testLedgerKey(1), newTestLedgerItem(1, "parent")))
	child := ledger.CacheWrap()
	require.NoError(t, child.Set(testLedgerKey(1), newTestLedgerItem(1, "child")))

	item, xerr := child.Get(testLedgerKey(1))
	require.NoError(t, xerr)
	require.Equal(t, "child", item.(*testLedgerItem).data)

	item, xerr = ledger.Get(testLedgerKey(1))
	require.NoError(t, xerr)
	require.Equal(t, "parent", item.(*testLedgerItem).data)

	require.NoError(t, child.Write())
	item, xerr = ledger.Get(testLedgerKey(1))
	require.NoError(t, xerr)
	require.Equal(t, "child", item.(*testLedgerItem).data)
}

func TestMutableLedgerLastWriteWins(t *testing.T) {
	ledger := newMutableLedgerForTest(t)

	require.NoError(t, ledger.Set(testLedgerKey(1), newTestLedgerItem(1, "first")))
	require.NoError(t, ledger.Del(testLedgerKey(1)))
	require.NoError(t, ledger.Set(testLedgerKey(1), newTestLedgerItem(1, "final")))

	item, xerr := ledger.Get(testLedgerKey(1))
	require.NoError(t, xerr)
	require.Equal(t, "final", item.(*testLedgerItem).data)

	_, _, xerr = ledger.Commit()
	require.NoError(t, xerr)

	item, xerr = ledger.Get(testLedgerKey(1))
	require.NoError(t, xerr)
	require.Equal(t, "final", item.(*testLedgerItem).data)
}

func TestMutableLedgerUnwrittenChild(t *testing.T) {
	ledger := newMutableLedgerForTest(t)

	child := ledger.CacheWrap()
	require.NoError(t, child.Set(testLedgerKey(1), newTestLedgerItem(1, "child")))

	_, ver, xerr := ledger.Commit()
	require.NoError(t, xerr)
	require.EqualValues(t, 1, ver)

	_, xerr = ledger.Get(testLedgerKey(1))
	require.Error(t, xerr)
	require.True(t, xerr.Contains(xerrors.ErrNotFoundResult))

	readonly, xerr := ledger.GetReadOnlyTree(ver)
	require.NoError(t, xerr)
	raw, err := readonly.Get(testLedgerKey(1))
	require.NoError(t, err)
	require.Nil(t, raw)
}

func TestMutableLedgerSetDel(t *testing.T) {
	ledger := newMutableLedgerForTest(t)

	require.NoError(t, ledger.Set(testLedgerKey(1), newTestLedgerItem(1, "one")))
	require.NoError(t, ledger.Del(testLedgerKey(1)))

	_, xerr := ledger.Get(testLedgerKey(1))
	require.Error(t, xerr)
	require.True(t, xerr.Contains(xerrors.ErrNotFoundResult))

	_, ver, xerr := ledger.Commit()
	require.NoError(t, xerr)

	readonly, xerr := ledger.GetReadOnlyTree(ver)
	require.NoError(t, xerr)
	raw, err := readonly.Get(testLedgerKey(1))
	require.NoError(t, err)
	require.Nil(t, raw)
}

func TestMutableLedgerParentKept(t *testing.T) {
	ledger := newMutableLedgerForTest(t)
	require.NoError(t, ledger.Set(testLedgerKey(1), newTestLedgerItem(1, "parent")))

	child := ledger.CacheWrap()
	require.NoError(t, child.Set(testLedgerKey(1), newTestLedgerItem(1, "child")))
	require.NoError(t, child.Set(testLedgerKey(2), newTestLedgerItem(2, "child")))
	child = nil

	item, xerr := ledger.Get(testLedgerKey(1))
	require.NoError(t, xerr)
	require.Equal(t, "parent", item.(*testLedgerItem).data)

	_, xerr = ledger.Get(testLedgerKey(2))
	require.Error(t, xerr)
	require.True(t, xerr.Contains(xerrors.ErrNotFoundResult))
	require.Nil(t, child)
}

func TestMutableLedgerGetCopy(t *testing.T) {
	ledger := newMutableLedgerForTest(t)
	require.NoError(t, ledger.Set(testLedgerKey(1), newTestLedgerItem(1, "one")))

	item, xerr := ledger.Get(testLedgerKey(1))
	require.NoError(t, xerr)
	item.(*testLedgerItem).data = "mutated"

	item, xerr = ledger.Get(testLedgerKey(1))
	require.NoError(t, xerr)
	require.Equal(t, "one", item.(*testLedgerItem).data)
}

func TestMutableLedgerCommitFlushesCache(t *testing.T) {
	ledger := newMutableLedgerForTest(t)

	require.NoError(t, ledger.Set(testLedgerKey(1), newTestLedgerItem(1, "one")))

	raw, err := ledger.tree.Get(testLedgerKey(1))
	require.NoError(t, err)
	require.Nil(t, raw)

	_, ver, xerr := ledger.Commit()
	require.NoError(t, xerr)
	require.EqualValues(t, 1, ver)

	raw, err = ledger.tree.Get(testLedgerKey(1))
	require.NoError(t, err)
	require.NotNil(t, raw)

	readonly, xerr := ledger.GetReadOnlyTree(ver)
	require.NoError(t, xerr)
	raw, err = readonly.Get(testLedgerKey(1))
	require.NoError(t, err)
	require.NotNil(t, raw)
}

func TestMutableLedgerDeleteShadow(t *testing.T) {
	ledger := newMutableLedgerForTest(t)
	require.NoError(t, ledger.Set(testLedgerKey(1), newTestLedgerItem(1, "one")))
	_, _, xerr := ledger.Commit()
	require.NoError(t, xerr)

	child := ledger.CacheWrap()
	require.NoError(t, child.Del(testLedgerKey(1)))

	item, xerr := ledger.Get(testLedgerKey(1))
	require.NoError(t, xerr)
	require.Equal(t, "one", item.(*testLedgerItem).data)

	_, xerr = child.Get(testLedgerKey(1))
	require.Error(t, xerr)
	require.True(t, xerr.Contains(xerrors.ErrNotFoundResult))

	require.NoError(t, child.Write())
	_, xerr = ledger.Get(testLedgerKey(1))
	require.Error(t, xerr)
	require.True(t, xerr.Contains(xerrors.ErrNotFoundResult))
}

func TestMutableLedgerSeekOverlay(t *testing.T) {
	ledger := newMutableLedgerForTest(t)
	require.NoError(t, ledger.Set(testLedgerKey(1), newTestLedgerItem(1, "one")))
	require.NoError(t, ledger.Set(testLedgerKey(3), newTestLedgerItem(3, "three")))
	_, _, xerr := ledger.Commit()
	require.NoError(t, xerr)

	require.NoError(t, ledger.Set(testLedgerKey(2), newTestLedgerItem(2, "two")))
	require.NoError(t, ledger.Del(testLedgerKey(3)))

	var got []string
	xerr = ledger.Seek(nil, true, func(_ LedgerKey, item ILedgerItem) xerrors.XError {
		got = append(got, item.(*testLedgerItem).data)
		return nil
	})
	require.NoError(t, xerr)
	require.Equal(t, []string{"one", "two"}, got)
}

func TestMutableLedgerSeekUpdated(t *testing.T) {
	ledger := newMutableLedgerForTest(t)
	require.NoError(t, ledger.Set(testLedgerKey(1), newTestLedgerItem(1, "one")))
	require.NoError(t, ledger.Set(testLedgerKey(2), newTestLedgerItem(2, "two")))
	_, _, xerr := ledger.Commit()
	require.NoError(t, xerr)

	require.NoError(t, ledger.Set(testLedgerKey(1), newTestLedgerItem(1, "uno")))

	var got []string
	xerr = ledger.Seek(nil, true, func(_ LedgerKey, item ILedgerItem) xerrors.XError {
		got = append(got, item.(*testLedgerItem).data)
		return nil
	})
	require.NoError(t, xerr)
	require.Equal(t, []string{"uno", "two"}, got)
}

func TestMutableLedgerSeekDesc(t *testing.T) {
	ledger := newMutableLedgerForTest(t)
	require.NoError(t, ledger.Set(testLedgerKey(1), newTestLedgerItem(1, "one")))
	require.NoError(t, ledger.Set(testLedgerKey(3), newTestLedgerItem(3, "three")))
	_, _, xerr := ledger.Commit()
	require.NoError(t, xerr)
	require.NoError(t, ledger.Set(testLedgerKey(2), newTestLedgerItem(2, "two")))

	var got []int
	xerr = ledger.Seek(nil, false, func(key LedgerKey, _ ILedgerItem) xerrors.XError {
		got = append(got, int(binary.BigEndian.Uint32(key)))
		return nil
	})
	require.NoError(t, xerr)
	require.Equal(t, []int{3, 2, 1}, got)
}

func TestMutableLedgerMissingVersion(t *testing.T) {
	ledger := newMutableLedgerForTest(t)

	_, xerr := ledger.GetReadOnlyTree(1)
	require.Error(t, xerr)
}

func TestMutableLedgerGuards(t *testing.T) {
	ledger := newMutableLedgerForTest(t)

	require.NotPanics(t, func() {
		require.Error(t, ledger.Set(testLedgerKey(1), nil))
	})
	require.NotPanics(t, func() {
		require.Error(t, ledger.Seek(nil, true, nil))
	})

	require.NoError(t, ledger.Set(testLedgerKey(1), newTestLedgerItem(1, "one")))
	ledger.newItemFor = func(LedgerKey) ILedgerItem {
		return nil
	}
	require.NotPanics(t, func() {
		_, xerr := ledger.Get(testLedgerKey(1))
		require.Error(t, xerr)
	})

	require.NoError(t, ledger.Close())
	require.EqualValues(t, 0, ledger.Version())
	require.Nil(t, ledger.CacheWrap())
	require.NotPanics(t, func() {
		_, xerr := ledger.Get(testLedgerKey(1))
		require.Error(t, xerr)
	})
	require.Error(t, ledger.Write())
	require.Error(t, ledger.Set(testLedgerKey(1), newTestLedgerItem(1, "one")))
	require.Error(t, ledger.Del(testLedgerKey(1)))
	require.Error(t, ledger.Seek(nil, true, func(LedgerKey, ILedgerItem) xerrors.XError {
		return nil
	}))
	_, _, xerr := ledger.Commit()
	require.Error(t, xerr)
	_, xerr = ledger.GetReadOnlyTree(1)
	require.Error(t, xerr)
}

func newMutableLedgerForTest(t *testing.T) *MutableLedger {
	t.Helper()

	ledger, xerr := NewMutableLedger("mutable_ledger_test", t.TempDir(), 100, func(LedgerKey) ILedgerItem {
		return &testLedgerItem{}
	}, log.NewNopLogger())
	require.NoError(t, xerr)
	t.Cleanup(func() {
		require.NoError(t, ledger.Close())
	})
	return ledger
}
