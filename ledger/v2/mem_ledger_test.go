package v2

import (
	"testing"

	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"
)

func newMemTestLedger(t *testing.T) (*MutableLedger, *MemLedger, func()) {
	mutable, cleanup := newMutableTestLedger(t, newMutableTestItem)
	require.NoError(t, mutable.Set([]byte("a"), &mutableTestItem{value: []byte("1")}))
	require.NoError(t, mutable.Set([]byte("b"), &mutableTestItem{value: []byte("2")}))
	_, version, xerr := mutable.Commit()
	require.NoError(t, xerr)
	mem, xerr := NewMemLedgerAt(version, mutable, log.NewNopLogger())
	require.NoError(t, xerr)
	return mutable, mem, cleanup
}

func TestMemLedger_Get_Immutable(t *testing.T) {
	_, ledger, cleanup := newMemTestLedger(t)
	defer cleanup()

	item, xerr := ledger.Get([]byte("a"))

	require.NoError(t, xerr)
	require.Equal(t, []byte("1"), item.(*mutableTestItem).value)
}

func TestMemLedger_Set_Overlay(t *testing.T) {
	mutable, ledger, cleanup := newMemTestLedger(t)
	defer cleanup()

	require.NoError(t, ledger.Set([]byte("a"), &mutableTestItem{value: []byte("cache")}))
	require.NoError(t, ledger.Set([]byte("c"), &mutableTestItem{value: []byte("3")}))

	item, xerr := ledger.Get([]byte("a"))
	require.NoError(t, xerr)
	require.Equal(t, []byte("cache"), item.(*mutableTestItem).value)
	item, xerr = mutable.Get([]byte("a"))
	require.NoError(t, xerr)
	require.Equal(t, []byte("1"), item.(*mutableTestItem).value)
	item, xerr = mutable.Get([]byte("c"))
	require.Nil(t, item)
	require.ErrorIs(t, xerr, xerrors.ErrNotFoundResult)
}

func TestMemLedger_Set_InvalidItem(t *testing.T) {
	expected := xerrors.NewOrdinary("encode error")
	for _, test := range []struct {
		name     string
		item     ILedgerItem
		expected xerrors.XError
	}{
		{name: "nil item", item: nil},
		{name: "encode error", item: &mutableTestItem{encodeErr: expected}, expected: expected},
		{name: "nil encoded value", item: &mutableTestItem{}},
	} {
		_, ledger, cleanup := newMemTestLedger(t)
		defer cleanup()

		xerr := ledger.Set([]byte("key"), test.item)

		require.Error(t, xerr, "case=%s", test.name)
		if test.expected != nil {
			require.ErrorIs(t, xerr, test.expected, "case=%s", test.name)
		}
		item, getErr := ledger.Get([]byte("key"))
		require.Nil(t, item, "case=%s", test.name)
		require.ErrorIs(t, getErr, xerrors.ErrNotFoundResult, "case=%s", test.name)
	}
}

func TestMemLedger_Del_Overlay(t *testing.T) {
	mutable, ledger, cleanup := newMemTestLedger(t)
	defer cleanup()

	require.NoError(t, ledger.Del([]byte("a")))

	item, xerr := ledger.Get([]byte("a"))
	require.Nil(t, item)
	require.ErrorIs(t, xerr, xerrors.ErrNotFoundResult)
	item, xerr = mutable.Get([]byte("a"))
	require.NoError(t, xerr)
	require.Equal(t, []byte("1"), item.(*mutableTestItem).value)
}

func TestMemLedger_Seek_Overlay(t *testing.T) {
	_, ledger, cleanup := newMemTestLedger(t)
	defer cleanup()
	require.NoError(t, ledger.Set([]byte("b"), &mutableTestItem{value: []byte("cache")}))
	require.NoError(t, ledger.Set([]byte("c"), &mutableTestItem{value: []byte("3")}))
	require.NoError(t, ledger.Del([]byte("a")))

	var entries []mutableTestEntry
	xerr := ledger.Iterate(func(key LedgerKey, item ILedgerItem) xerrors.XError {
		entries = append(entries, mutableTestEntry{
			key:   string(key),
			value: string(item.(*mutableTestItem).value),
		})
		return nil
	})

	require.NoError(t, xerr)
	require.Equal(t, []mutableTestEntry{
		{key: "b", value: "cache"},
		{key: "c", value: "3"},
	}, entries)
}

func TestMemLedger_Seek_Tree(t *testing.T) {
	mutable, cleanup := newMutableTestLedger(t, newMutableTestItem)
	defer cleanup()
	require.NoError(t, mutable.Set([]byte("0"), &mutableTestItem{value: []byte("before")}))
	require.NoError(t, mutable.Set([]byte("acct/"), &mutableTestItem{value: []byte("root")}))
	require.NoError(t, mutable.Set([]byte("acct/a"), &mutableTestItem{value: []byte("1")}))
	require.NoError(t, mutable.Set([]byte("acct/b"), &mutableTestItem{value: []byte("2")}))
	require.NoError(t, mutable.Set([]byte("gov/a"), &mutableTestItem{value: []byte("after")}))
	_, version, xerr := mutable.Commit()
	require.NoError(t, xerr)
	ledger, xerr := NewMemLedgerAt(version, mutable, log.NewNopLogger())
	require.NoError(t, xerr)

	var entries []mutableTestEntry
	xerr = ledger.Seek([]byte("acct/"), true, func(key LedgerKey, item ILedgerItem) xerrors.XError {
		entries = append(entries, mutableTestEntry{
			key:   string(key),
			value: string(item.(*mutableTestItem).value),
		})
		return nil
	})
	require.NoError(t, xerr)
	require.Equal(t, []mutableTestEntry{
		{key: "acct/", value: "root"},
		{key: "acct/a", value: "1"},
		{key: "acct/b", value: "2"},
	}, entries)

	entries = nil
	xerr = ledger.Seek([]byte("acct/"), false, func(key LedgerKey, item ILedgerItem) xerrors.XError {
		entries = append(entries, mutableTestEntry{
			key:   string(key),
			value: string(item.(*mutableTestItem).value),
		})
		return nil
	})
	require.NoError(t, xerr)
	require.Equal(t, []mutableTestEntry{
		{key: "acct/b", value: "2"},
		{key: "acct/a", value: "1"},
		{key: "acct/", value: "root"},
	}, entries)
}

func TestMemLedger_Seek_Empty(t *testing.T) {
	mutable, cleanup := newMutableTestLedger(t, newMutableTestItem)
	defer cleanup()
	ledger, xerr := NewMemLedgerAt(0, mutable, log.NewNopLogger())
	require.NoError(t, xerr)
	tests := []struct {
		prefix    []byte
		ascending bool
	}{
		{prefix: nil, ascending: true},
		{prefix: nil, ascending: false},
		{prefix: []byte("acct/"), ascending: true},
		{prefix: []byte("acct/"), ascending: false},
	}

	for _, test := range tests {
		calls := 0
		xerr = ledger.Seek(test.prefix, test.ascending, func(LedgerKey, ILedgerItem) xerrors.XError {
			calls++
			return nil
		})
		require.NoError(t, xerr)
		require.Zero(t, calls)
	}
}

func TestMemLedger_Seek_EmptyCache(t *testing.T) {
	_, ledger, cleanup := newMemTestLedger(t)
	defer cleanup()
	require.NoError(t, ledger.createCache())
	var entries []mutableTestEntry

	xerr := ledger.Iterate(func(key LedgerKey, item ILedgerItem) xerrors.XError {
		entries = append(entries, mutableTestEntry{
			key:   string(key),
			value: string(item.(*mutableTestItem).value),
		})
		return nil
	})

	require.NoError(t, xerr)
	require.Equal(t, []mutableTestEntry{
		{key: "a", value: "1"},
		{key: "b", value: "2"},
	}, entries)
	require.NoError(t, ledger.clearCache())
}

func TestMemLedger_Seek_KeyCopy(t *testing.T) {
	_, ledger, cleanup := newMemTestLedger(t)
	defer cleanup()
	key := []byte("a")

	xerr := ledger.Seek(key, true, func(key LedgerKey, _ ILedgerItem) xerrors.XError {
		key[0] = 'x'
		return nil
	})
	require.NoError(t, xerr)

	item, xerr := ledger.Get(key)
	require.NoError(t, xerr)
	require.Equal(t, []byte("1"), item.(*mutableTestItem).value)
}

func TestMemLedger_Seek_TreeError(t *testing.T) {
	expected := xerrors.NewOrdinary("decode error")
	mutable, cleanup := newMutableTestLedger(t, func(LedgerKey) ILedgerItem {
		return &mutableTestItem{decodeErr: expected}
	})
	defer cleanup()
	require.NoError(t, mutable.Set([]byte("key"), &mutableTestItem{value: []byte("value")}))
	_, version, xerr := mutable.Commit()
	require.NoError(t, xerr)
	ledger, xerr := NewMemLedgerAt(version, mutable, log.NewNopLogger())
	require.NoError(t, xerr)
	calls := 0

	xerr = ledger.Seek(nil, true, func(LedgerKey, ILedgerItem) xerrors.XError {
		calls++
		return nil
	})

	require.ErrorIs(t, xerr, expected)
	require.Zero(t, calls)
}

func TestMemLedger_Seek_CallbackError(t *testing.T) {
	_, ledger, cleanup := newMemTestLedger(t)
	defer cleanup()
	expected := xerrors.NewOrdinary("callback error")
	calls := 0

	xerr := ledger.Seek(nil, true, func(LedgerKey, ILedgerItem) xerrors.XError {
		calls++
		return expected
	})

	require.ErrorIs(t, xerr, expected)
	require.Equal(t, 1, calls)
	item, xerr := ledger.Get([]byte("a"))
	require.NoError(t, xerr)
	require.Equal(t, []byte("1"), item.(*mutableTestItem).value)
}

func TestMemLedger_Set_Empty(t *testing.T) {
	mutable, cleanup := newMutableTestLedger(t, newMutableTestItem)
	defer cleanup()
	ledger, xerr := NewMemLedgerAt(0, mutable, log.NewNopLogger())
	require.NoError(t, xerr)

	require.NoError(t, ledger.Set([]byte("key"), &mutableTestItem{value: []byte("value")}))
	item, xerr := ledger.Get([]byte("key"))

	require.NoError(t, xerr)
	require.Equal(t, []byte("value"), item.(*mutableTestItem).value)
}

func TestMemLedger_Cache_Write(t *testing.T) {
	mutable, ledger, cleanup := newMemTestLedger(t)
	defer cleanup()
	require.NoError(t, ledger.Set([]byte("a"), &mutableTestItem{value: []byte("working")}))

	require.NoError(t, ledger.createCache())
	require.NoError(t, ledger.Set([]byte("a"), &mutableTestItem{value: []byte("trx")}))
	require.NoError(t, ledger.Del([]byte("b")))
	require.NoError(t, ledger.Set([]byte("c"), &mutableTestItem{value: []byte("3")}))
	require.True(t, ledger.hasActiveCache())

	require.NoError(t, ledger.writeCache())
	require.NoError(t, ledger.clearCache())

	require.False(t, ledger.hasActiveCache())
	item, xerr := ledger.Get([]byte("a"))
	require.NoError(t, xerr)
	require.Equal(t, []byte("trx"), item.(*mutableTestItem).value)
	item, xerr = ledger.Get([]byte("b"))
	require.Nil(t, item)
	require.ErrorIs(t, xerr, xerrors.ErrNotFoundResult)
	item, xerr = ledger.Get([]byte("c"))
	require.NoError(t, xerr)
	require.Equal(t, []byte("3"), item.(*mutableTestItem).value)

	item, xerr = mutable.Get([]byte("a"))
	require.NoError(t, xerr)
	require.Equal(t, []byte("1"), item.(*mutableTestItem).value)
}

func TestMemLedger_Cache_Clear(t *testing.T) {
	_, ledger, cleanup := newMemTestLedger(t)
	defer cleanup()
	require.NoError(t, ledger.Set([]byte("a"), &mutableTestItem{value: []byte("working")}))

	require.NoError(t, ledger.createCache())
	require.NoError(t, ledger.Set([]byte("a"), &mutableTestItem{value: []byte("discard")}))
	require.NoError(t, ledger.Set([]byte("c"), &mutableTestItem{value: []byte("discard")}))
	require.NoError(t, ledger.clearCache())

	require.False(t, ledger.hasActiveCache())
	item, xerr := ledger.Get([]byte("a"))
	require.NoError(t, xerr)
	require.Equal(t, []byte("working"), item.(*mutableTestItem).value)
	item, xerr = ledger.Get([]byte("c"))
	require.Nil(t, item)
	require.ErrorIs(t, xerr, xerrors.ErrNotFoundResult)
}

func TestMemLedger_Cache_Active(t *testing.T) {
	_, ledger, cleanup := newMemTestLedger(t)
	defer cleanup()

	require.NoError(t, ledger.createCache())
	require.Error(t, ledger.createCache())
	require.True(t, ledger.hasActiveCache())
	require.NoError(t, ledger.clearCache())
	require.NoError(t, ledger.clearCache())
	require.False(t, ledger.hasActiveCache())
	require.Error(t, ledger.writeCache())
}

func TestMemLedger_Cache_Seek(t *testing.T) {
	_, ledger, cleanup := newMemTestLedger(t)
	defer cleanup()
	require.NoError(t, ledger.Set([]byte("b"), &mutableTestItem{value: []byte("working")}))

	require.NoError(t, ledger.createCache())
	defer func() { _ = ledger.clearCache() }()
	require.NoError(t, ledger.Del([]byte("a")))
	require.NoError(t, ledger.Set([]byte("b"), &mutableTestItem{value: []byte("trx")}))
	require.NoError(t, ledger.Set([]byte("c"), &mutableTestItem{value: []byte("3")}))

	var entries []mutableTestEntry
	xerr := ledger.Iterate(func(key LedgerKey, item ILedgerItem) xerrors.XError {
		entries = append(entries, mutableTestEntry{
			key:   string(key),
			value: string(item.(*mutableTestItem).value),
		})
		return nil
	})

	require.NoError(t, xerr)
	require.Equal(t, []mutableTestEntry{
		{key: "b", value: "trx"},
		{key: "c", value: "3"},
	}, entries)
}
