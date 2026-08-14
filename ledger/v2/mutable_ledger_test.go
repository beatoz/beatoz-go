package v2

import (
	"bytes"
	"os"
	"testing"
	"time"

	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"
)

type mutableTestItem struct {
	value     []byte
	encodeErr xerrors.XError
	decodeErr xerrors.XError
}

func (item *mutableTestItem) Encode() ([]byte, xerrors.XError) {
	if item.encodeErr != nil {
		return nil, item.encodeErr
	}
	return item.value, nil
}

func (item *mutableTestItem) Decode(_, value []byte) xerrors.XError {
	if item.decodeErr != nil {
		return item.decodeErr
	}
	item.value = bytes.Clone(value)
	return nil
}

func newMutableTestLedger(t *testing.T, newItem FuncNewItemFor) (*MutableLedger, func()) {
	rootDir, err := os.MkdirTemp("", "mutable-ledger-")
	require.NoError(t, err)
	ledger, xerr := NewMutableLedger(
		"ledger", rootDir, 100, newItem, log.NewNopLogger(),
	)
	if xerr != nil {
		_ = os.RemoveAll(rootDir)
	}
	require.NoError(t, xerr)
	cleanup := func() {
		_ = ledger.clearCache()
		_ = ledger.Close()
		_ = os.RemoveAll(rootDir)
	}
	return ledger, cleanup
}

func newMutableTestItem(_ LedgerKey) ILedgerItem {
	return &mutableTestItem{}
}

func newV1MutableTestItem(_ v1.LedgerKey) v1.ILedgerItem {
	return &mutableTestItem{}
}

func TestMutableLedger_Get_CacheMiss(t *testing.T) {
	ledger, cleanup := newMutableTestLedger(t, newMutableTestItem)
	defer cleanup()
	key := []byte("key")
	require.NoError(t, ledger.Set(key, &mutableTestItem{value: []byte("tree")}))
	require.NoError(t, ledger.createCache())

	item, xerr := ledger.Get(key)

	require.NoError(t, xerr)
	require.Equal(t, []byte("tree"), item.(*mutableTestItem).value)
}

func TestMutableLedger_Get_CacheSet(t *testing.T) {
	ledger, cleanup := newMutableTestLedger(t, newMutableTestItem)
	defer cleanup()
	key := []byte("key")
	require.NoError(t, ledger.Set(key, &mutableTestItem{value: []byte("tree")}))
	require.NoError(t, ledger.createCache())
	require.NoError(t, ledger.Set(key, &mutableTestItem{value: []byte("cache")}))

	item, xerr := ledger.Get(key)

	require.NoError(t, xerr)
	require.Equal(t, []byte("cache"), item.(*mutableTestItem).value)
	require.NoError(t, ledger.clearCache())
	item, xerr = ledger.Get(key)
	require.NoError(t, xerr)
	require.Equal(t, []byte("tree"), item.(*mutableTestItem).value)
}

func TestMutableLedger_Get_CacheDel(t *testing.T) {
	ledger, cleanup := newMutableTestLedger(t, newMutableTestItem)
	defer cleanup()
	key := []byte("key")
	require.NoError(t, ledger.Set(key, &mutableTestItem{value: []byte("tree")}))
	require.NoError(t, ledger.createCache())
	require.NoError(t, ledger.Del(key))

	item, xerr := ledger.Get(key)

	require.Nil(t, item)
	require.ErrorIs(t, xerr, xerrors.ErrNotFoundResult)
}

func TestMutableLedger_Set_Direct(t *testing.T) {
	ledger, cleanup := newMutableTestLedger(t, newMutableTestItem)
	defer cleanup()
	key := []byte("key")

	xerr := ledger.Set(key, &mutableTestItem{value: []byte("value")})

	require.NoError(t, xerr)
	item, xerr := ledger.Get(key)
	require.NoError(t, xerr)
	require.Equal(t, []byte("value"), item.(*mutableTestItem).value)
}

func TestMutableLedger_Del_Direct(t *testing.T) {
	ledger, cleanup := newMutableTestLedger(t, newMutableTestItem)
	defer cleanup()
	key := []byte("key")
	require.NoError(t, ledger.Set(key, &mutableTestItem{value: []byte("value")}))

	require.NoError(t, ledger.Del(key))
	require.NoError(t, ledger.Del([]byte("missing")))
	item, xerr := ledger.Get(key)
	require.Nil(t, item)
	require.ErrorIs(t, xerr, xerrors.ErrNotFoundResult)
}

func TestMutableLedger_Get_NewItem(t *testing.T) {
	ledger, cleanup := newMutableTestLedger(t, newMutableTestItem)
	defer cleanup()
	key := []byte("key")
	require.NoError(t, ledger.Set(key, &mutableTestItem{value: []byte("value")}))

	item0, xerr := ledger.Get(key)
	require.NoError(t, xerr)
	item0.(*mutableTestItem).value[0] = 'x'
	item1, xerr := ledger.Get(key)
	require.NoError(t, xerr)

	require.Equal(t, []byte("value"), item1.(*mutableTestItem).value)
}

func TestMutableLedger_Set_EncodeError(t *testing.T) {
	ledger, cleanup := newMutableTestLedger(t, newMutableTestItem)
	defer cleanup()
	expected := xerrors.NewOrdinary("encode error")

	xerr := ledger.Set([]byte("key"), &mutableTestItem{encodeErr: expected})

	require.ErrorIs(t, xerr, expected)
	item, xerr := ledger.Get([]byte("key"))
	require.Nil(t, item)
	require.ErrorIs(t, xerr, xerrors.ErrNotFoundResult)
}

func TestMutableLedger_Set_NilItem(t *testing.T) {
	ledger, cleanup := newMutableTestLedger(t, newMutableTestItem)
	defer cleanup()
	require.NoError(t, ledger.createCache())

	xerr := ledger.Set([]byte("key"), nil)

	require.Error(t, xerr)
	item, xerr := ledger.Get([]byte("key"))
	require.Nil(t, item)
	require.ErrorIs(t, xerr, xerrors.ErrNotFoundResult)
}

func TestMutableLedger_Get_DecodeError(t *testing.T) {
	expected := xerrors.NewOrdinary("decode error")
	ledger, cleanup := newMutableTestLedger(t, func(LedgerKey) ILedgerItem {
		return &mutableTestItem{decodeErr: expected}
	})
	defer cleanup()
	key := []byte("key")
	// Insert encoded storage directly because Set rejects items that cannot encode.
	_, err := ledger.tree.Set(key, []byte("value"))
	require.NoError(t, err)

	item, xerr := ledger.Get(key)

	require.Nil(t, item)
	require.ErrorIs(t, xerr, expected)
}

func TestMutableLedger_Set_NilEncodedValue(t *testing.T) {
	ledger, cleanup := newMutableTestLedger(t, newMutableTestItem)
	defer cleanup()

	xerr := ledger.Set([]byte("key"), &mutableTestItem{})

	require.Error(t, xerr)
	item, xerr := ledger.Get([]byte("key"))
	require.Nil(t, item)
	require.ErrorIs(t, xerr, xerrors.ErrNotFoundResult)
}

type mutableTestEntry struct {
	key   string
	value string
}

func mutableTestSeek(
	ledger *MutableLedger,
	prefix []byte,
	ascending bool,
) ([]mutableTestEntry, xerrors.XError) {
	var entries []mutableTestEntry
	xerr := ledger.Seek(prefix, ascending, func(key LedgerKey, item ILedgerItem) xerrors.XError {
		entries = append(entries, mutableTestEntry{
			key:   string(key),
			value: string(item.(*mutableTestItem).value),
		})
		return nil
	})
	return entries, xerr
}

func TestMutableLedger_Iterate_Tree(t *testing.T) {
	ledger, cleanup := newMutableTestLedger(t, newMutableTestItem)
	defer cleanup()
	require.NoError(t, ledger.Set([]byte("c"), &mutableTestItem{value: []byte("3")}))
	require.NoError(t, ledger.Set([]byte("a"), &mutableTestItem{value: []byte("1")}))
	require.NoError(t, ledger.Set([]byte("b"), &mutableTestItem{value: []byte("2")}))

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
		{key: "c", value: "3"},
	}, entries)
}

func TestMutableLedger_Seek_Desc(t *testing.T) {
	ledger, cleanup := newMutableTestLedger(t, newMutableTestItem)
	defer cleanup()
	require.NoError(t, ledger.Set([]byte("c"), &mutableTestItem{value: []byte("3")}))
	require.NoError(t, ledger.Set([]byte("a"), &mutableTestItem{value: []byte("1")}))
	require.NoError(t, ledger.Set([]byte("b"), &mutableTestItem{value: []byte("2")}))

	entries, xerr := mutableTestSeek(ledger, nil, false)

	require.NoError(t, xerr)
	require.Equal(t, []mutableTestEntry{
		{key: "c", value: "3"},
		{key: "b", value: "2"},
		{key: "a", value: "1"},
	}, entries)
}

func TestMutableLedger_Seek_TreePrefix(t *testing.T) {
	ledger, cleanup := newMutableTestLedger(t, newMutableTestItem)
	defer cleanup()
	require.NoError(t, ledger.Set([]byte("0"), &mutableTestItem{value: []byte("before")}))
	require.NoError(t, ledger.Set([]byte("acct/"), &mutableTestItem{value: []byte("root")}))
	require.NoError(t, ledger.Set([]byte("acct/a"), &mutableTestItem{value: []byte("1")}))
	require.NoError(t, ledger.Set([]byte("acct/b"), &mutableTestItem{value: []byte("2")}))
	require.NoError(t, ledger.Set([]byte("gov/a"), &mutableTestItem{value: []byte("after")}))

	entries, xerr := mutableTestSeek(ledger, []byte("acct/"), true)
	require.NoError(t, xerr)
	require.Equal(t, []mutableTestEntry{
		{key: "acct/", value: "root"},
		{key: "acct/a", value: "1"},
		{key: "acct/b", value: "2"},
	}, entries)

	entries, xerr = mutableTestSeek(ledger, []byte("acct/"), false)
	require.NoError(t, xerr)
	require.Equal(t, []mutableTestEntry{
		{key: "acct/b", value: "2"},
		{key: "acct/a", value: "1"},
		{key: "acct/", value: "root"},
	}, entries)
}

func TestMutableLedger_Seek_EmptyTree(t *testing.T) {
	ledger, cleanup := newMutableTestLedger(t, newMutableTestItem)
	defer cleanup()
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
		xerr := ledger.Seek(test.prefix, test.ascending, func(LedgerKey, ILedgerItem) xerrors.XError {
			calls++
			return nil
		})
		require.NoError(t, xerr)
		require.Zero(t, calls)
	}
}

func TestMutableLedger_Seek_TreeError(t *testing.T) {
	expected := xerrors.NewOrdinary("decode error")
	ledger, cleanup := newMutableTestLedger(t, func(LedgerKey) ILedgerItem {
		return &mutableTestItem{decodeErr: expected}
	})
	defer cleanup()
	// Insert encoded storage directly to exercise the tree decode failure path.
	_, err := ledger.tree.Set([]byte("key"), []byte("value"))
	require.NoError(t, err)
	calls := 0

	xerr := ledger.Seek(nil, true, func(LedgerKey, ILedgerItem) xerrors.XError {
		calls++
		return nil
	})

	require.ErrorIs(t, xerr, expected)
	require.Zero(t, calls)
}

func TestMutableLedger_Seek_EmptyCache(t *testing.T) {
	ledger, cleanup := newMutableTestLedger(t, newMutableTestItem)
	defer cleanup()
	require.NoError(t, ledger.Set([]byte("a"), &mutableTestItem{value: []byte("1")}))
	require.NoError(t, ledger.Set([]byte("b"), &mutableTestItem{value: []byte("2")}))
	require.NoError(t, ledger.createCache())

	entries, xerr := mutableTestSeek(ledger, nil, true)

	require.NoError(t, xerr)
	require.Equal(t, []mutableTestEntry{
		{key: "a", value: "1"},
		{key: "b", value: "2"},
	}, entries)
	require.NoError(t, ledger.clearCache())
}

func TestMutableLedger_Seek_KeyCopy(t *testing.T) {
	ledger, cleanup := newMutableTestLedger(t, newMutableTestItem)
	defer cleanup()
	key := []byte("key")
	require.NoError(t, ledger.Set(key, &mutableTestItem{value: []byte("value")}))

	xerr := ledger.Seek(nil, true, func(key LedgerKey, _ ILedgerItem) xerrors.XError {
		key[0] = 'x'
		return nil
	})
	require.NoError(t, xerr)

	item, xerr := ledger.Get(key)
	require.NoError(t, xerr)
	require.Equal(t, []byte("value"), item.(*mutableTestItem).value)
}

func TestMutableLedger_Seek_CacheSet(t *testing.T) {
	ledger, cleanup := newMutableTestLedger(t, newMutableTestItem)
	defer cleanup()
	require.NoError(t, ledger.Set([]byte("b"), &mutableTestItem{value: []byte("tree")}))
	require.NoError(t, ledger.createCache())
	require.NoError(t, ledger.Set([]byte("a"), &mutableTestItem{value: []byte("new")}))
	require.NoError(t, ledger.Set([]byte("b"), &mutableTestItem{value: []byte("cache")}))

	entries, xerr := mutableTestSeek(ledger, nil, true)

	require.NoError(t, xerr)
	require.Equal(t, []mutableTestEntry{
		{key: "a", value: "new"},
		{key: "b", value: "cache"},
	}, entries)
}

func TestMutableLedger_Seek_CacheDel(t *testing.T) {
	ledger, cleanup := newMutableTestLedger(t, newMutableTestItem)
	defer cleanup()
	require.NoError(t, ledger.Set([]byte("a"), &mutableTestItem{value: []byte("1")}))
	require.NoError(t, ledger.Set([]byte("b"), &mutableTestItem{value: []byte("2")}))
	require.NoError(t, ledger.createCache())
	require.NoError(t, ledger.Del([]byte("a")))

	entries, xerr := mutableTestSeek(ledger, nil, true)

	require.NoError(t, xerr)
	require.Equal(t, []mutableTestEntry{
		{key: "b", value: "2"},
	}, entries)
}

func TestMutableLedger_Seek_Prefix(t *testing.T) {
	ledger, cleanup := newMutableTestLedger(t, newMutableTestItem)
	defer cleanup()
	require.NoError(t, ledger.Set([]byte("acct/b"), &mutableTestItem{value: []byte("2")}))
	require.NoError(t, ledger.Set([]byte("gov/a"), &mutableTestItem{value: []byte("3")}))
	require.NoError(t, ledger.createCache())
	require.NoError(t, ledger.Set([]byte("acct/a"), &mutableTestItem{value: []byte("1")}))

	entries, xerr := mutableTestSeek(ledger, []byte("acct/"), true)

	require.NoError(t, xerr)
	require.Equal(t, []mutableTestEntry{
		{key: "acct/a", value: "1"},
		{key: "acct/b", value: "2"},
	}, entries)

	entries, xerr = mutableTestSeek(ledger, []byte("acct/"), false)

	require.NoError(t, xerr)
	require.Equal(t, []mutableTestEntry{
		{key: "acct/b", value: "2"},
		{key: "acct/a", value: "1"},
	}, entries)
}

func TestMutableLedger_Seek_Duplicate(t *testing.T) {
	ledger, cleanup := newMutableTestLedger(t, newMutableTestItem)
	defer cleanup()
	require.NoError(t, ledger.Set([]byte("key"), &mutableTestItem{value: []byte("tree")}))
	require.NoError(t, ledger.createCache())
	require.NoError(t, ledger.Set([]byte("key"), &mutableTestItem{value: []byte("cache")}))

	entries, xerr := mutableTestSeek(ledger, nil, true)

	require.NoError(t, xerr)
	require.Equal(t, []mutableTestEntry{
		{key: "key", value: "cache"},
	}, entries)
}

func TestMutableLedger_Seek_CallbackError(t *testing.T) {
	ledger, cleanup := newMutableTestLedger(t, newMutableTestItem)
	defer cleanup()
	require.NoError(t, ledger.Set([]byte("a"), &mutableTestItem{value: []byte("1")}))
	require.NoError(t, ledger.Set([]byte("b"), &mutableTestItem{value: []byte("2")}))
	expected := xerrors.NewOrdinary("callback error")
	calls := 0

	xerr := ledger.Seek(nil, true, func(LedgerKey, ILedgerItem) xerrors.XError {
		calls++
		return expected
	})

	require.ErrorIs(t, xerr, expected)
	require.Equal(t, 1, calls)
}

func TestMutableLedger_Seek_NilCallback(t *testing.T) {
	ledger, cleanup := newMutableTestLedger(t, newMutableTestItem)
	defer cleanup()

	require.Error(t, ledger.Seek(nil, true, nil))
	require.Error(t, ledger.Iterate(nil))
}

func TestMutableLedger_Seek_Deterministic(t *testing.T) {
	ledger0, cleanup0 := newMutableTestLedger(t, newMutableTestItem)
	defer cleanup0()
	require.NoError(t, ledger0.createCache())
	require.NoError(t, ledger0.Set([]byte("c"), &mutableTestItem{value: []byte("3")}))
	require.NoError(t, ledger0.Set([]byte("a"), &mutableTestItem{value: []byte("1")}))
	require.NoError(t, ledger0.Set([]byte("b"), &mutableTestItem{value: []byte("2")}))

	ledger1, cleanup1 := newMutableTestLedger(t, newMutableTestItem)
	defer cleanup1()
	require.NoError(t, ledger1.createCache())
	require.NoError(t, ledger1.Set([]byte("b"), &mutableTestItem{value: []byte("2")}))
	require.NoError(t, ledger1.Set([]byte("c"), &mutableTestItem{value: []byte("3")}))
	require.NoError(t, ledger1.Set([]byte("a"), &mutableTestItem{value: []byte("1")}))

	entries0, xerr := mutableTestSeek(ledger0, nil, true)
	require.NoError(t, xerr)
	entries1, xerr := mutableTestSeek(ledger1, nil, true)
	require.NoError(t, xerr)
	require.Equal(t, entries0, entries1)
}

func TestMutableLedger_Cache_Write(t *testing.T) {
	ledger, cleanup := newMutableTestLedger(t, newMutableTestItem)
	defer cleanup()
	require.NoError(t, ledger.Set([]byte("del"), &mutableTestItem{value: []byte("old")}))

	require.NoError(t, ledger.createCache())
	require.NoError(t, ledger.Set([]byte("set"), &mutableTestItem{value: []byte("new")}))
	require.NoError(t, ledger.Del([]byte("del")))
	item, xerr := ledger.Get([]byte("set"))
	require.NoError(t, xerr)
	require.Equal(t, []byte("new"), item.(*mutableTestItem).value)

	require.NoError(t, ledger.writeCache())
	require.NoError(t, ledger.clearCache())

	require.False(t, ledger.hasActiveCache())
	item, xerr = ledger.Get([]byte("set"))
	require.NoError(t, xerr)
	require.Equal(t, []byte("new"), item.(*mutableTestItem).value)
	item, xerr = ledger.Get([]byte("del"))
	require.Nil(t, item)
	require.ErrorIs(t, xerr, xerrors.ErrNotFoundResult)
}

func TestMutableLedger_Cache_Clear(t *testing.T) {
	ledger, cleanup := newMutableTestLedger(t, newMutableTestItem)
	defer cleanup()

	require.NoError(t, ledger.createCache())
	require.NoError(t, ledger.Set([]byte("key"), &mutableTestItem{value: []byte("value")}))

	require.NoError(t, ledger.clearCache())

	require.False(t, ledger.hasActiveCache())
	item, xerr := ledger.Get([]byte("key"))
	require.Nil(t, item)
	require.ErrorIs(t, xerr, xerrors.ErrNotFoundResult)
}

func TestMutableLedger_Cache_Lifecycle(t *testing.T) {
	ledger, cleanup := newMutableTestLedger(t, newMutableTestItem)
	defer cleanup()

	require.Error(t, ledger.writeCache(), "case=write_inactive")
	require.NoError(t, ledger.createCache(), "case=create")
	require.Error(t, ledger.createCache(), "case=create_active")
	require.True(t, ledger.hasActiveCache(), "case=create_active")
	require.NoError(t, ledger.clearCache(), "case=clear")
	require.NoError(t, ledger.clearCache(), "case=clear_inactive")
	require.False(t, ledger.hasActiveCache(), "case=clear_inactive")

	require.NoError(t, ledger.createCache(), "case=create_after_clear")
	require.NoError(t, ledger.Set([]byte("key"), &mutableTestItem{value: []byte("value")}))
	require.NoError(t, ledger.writeCache(), "case=write")
	require.NoError(t, ledger.clearCache(), "case=clear_after_write")
	require.Error(t, ledger.writeCache(), "case=write_inactive")
	require.False(t, ledger.hasActiveCache(), "case=write")

	item, xerr := ledger.Get([]byte("key"))
	require.NoError(t, xerr)
	require.Equal(t, []byte("value"), item.(*mutableTestItem).value)
}

func TestMutableLedger_Cache_NoDeadlock(t *testing.T) {
	ledger, cleanup := newMutableTestLedger(t, newMutableTestItem)
	defer cleanup()
	result := make(chan struct {
		cacheErr xerrors.XError
		runErr   xerrors.XError
	}, 1)

	go func() {
		cacheErr := ledger.createCache()
		if cacheErr != nil {
			result <- struct {
				cacheErr xerrors.XError
				runErr   xerrors.XError
			}{cacheErr: cacheErr}
			return
		}
		defer func() { _ = ledger.clearCache() }()

		runErr := ledger.Set([]byte("key"), &mutableTestItem{value: []byte("value")})
		if runErr == nil {
			_, runErr = ledger.Get([]byte("key"))
		}
		if runErr == nil {
			runErr = ledger.Del([]byte("key"))
		}
		if runErr == nil {
			runErr = ledger.writeCache()
		}
		result <- struct {
			cacheErr xerrors.XError
			runErr   xerrors.XError
		}{cacheErr: cacheErr, runErr: runErr}
	}()

	select {
	case result := <-result:
		require.NoError(t, result.cacheErr)
		require.NoError(t, result.runErr)
	case <-time.After(2 * time.Second):
		t.Fatal("cache lifecycle deadlocked")
	}
}

func TestMutableLedger_Commit_Active(t *testing.T) {
	ledger, cleanup := newMutableTestLedger(t, newMutableTestItem)
	defer cleanup()
	require.NoError(t, ledger.createCache())

	hash, version, xerr := ledger.Commit()

	require.Nil(t, hash)
	require.Zero(t, version)
	require.Error(t, xerr)
	require.NoError(t, ledger.Set([]byte("key"), &mutableTestItem{value: []byte("pending")}))
	item, xerr := ledger.Get([]byte("key"))
	require.NoError(t, xerr)
	require.Equal(t, []byte("pending"), item.(*mutableTestItem).value)
	require.NoError(t, ledger.clearCache())
	require.False(t, ledger.hasActiveCache())
}

func TestMutableLedger_Close_Active(t *testing.T) {
	ledger, cleanup := newMutableTestLedger(t, newMutableTestItem)
	defer cleanup()
	require.NoError(t, ledger.createCache())

	xerr := ledger.Close()

	require.Error(t, xerr)
	require.NoError(t, ledger.Set([]byte("key"), &mutableTestItem{value: []byte("pending")}))
	item, xerr := ledger.Get([]byte("key"))
	require.NoError(t, xerr)
	require.Equal(t, []byte("pending"), item.(*mutableTestItem).value)
	require.NoError(t, ledger.clearCache())
	require.False(t, ledger.hasActiveCache())
	require.NoError(t, ledger.Close())
}

func TestMutableLedger_Commit_CacheSuccess(t *testing.T) {
	ledger, cleanup := newMutableTestLedger(t, newMutableTestItem)
	defer cleanup()
	require.NoError(t, ledger.createCache())
	defer func() { _ = ledger.clearCache() }()
	require.NoError(t, ledger.Set([]byte("key"), &mutableTestItem{value: []byte("value")}))
	require.NoError(t, ledger.writeCache())
	require.NoError(t, ledger.clearCache())

	hash, version, xerr := ledger.Commit()

	require.NoError(t, xerr)
	require.NotEmpty(t, hash)
	require.EqualValues(t, 1, version)
	tree, xerr := ledger.GetReadOnlyTree(version)
	require.NoError(t, xerr)
	value, err := tree.Get([]byte("key"))
	require.NoError(t, err)
	require.Equal(t, []byte("value"), value)
}

func TestMutableLedger_Commit_CacheFailure(t *testing.T) {
	ledger, cleanup := newMutableTestLedger(t, newMutableTestItem)
	defer cleanup()
	require.NoError(t, ledger.createCache())
	require.NoError(t, ledger.Set([]byte("key"), &mutableTestItem{value: []byte("value")}))
	require.NoError(t, ledger.clearCache())

	_, version, xerr := ledger.Commit()

	require.NoError(t, xerr)
	require.EqualValues(t, 1, version)
	tree, xerr := ledger.GetReadOnlyTree(version)
	require.NoError(t, xerr)
	value, err := tree.Get([]byte("key"))
	require.NoError(t, err)
	require.Nil(t, value)
}

func TestMutableLedger_Commit_Empty(t *testing.T) {
	ledger, cleanup := newMutableTestLedger(t, newMutableTestItem)
	defer cleanup()

	hash0, version0, xerr := ledger.Commit()
	require.NoError(t, xerr)
	hash1, version1, xerr := ledger.Commit()

	require.NoError(t, xerr)
	require.EqualValues(t, 1, version0)
	require.EqualValues(t, 2, version1)
	require.Equal(t, hash0, hash1)
}

func TestMutableLedger_DBCompatibility(t *testing.T) {
	const (
		dbName    = "compat"
		cacheSize = 100
	)

	dbDir, err := os.MkdirTemp("", "mutable-ledger-compat-")
	require.NoError(t, err)
	defer os.RemoveAll(dbDir)
	logger := log.NewNopLogger()

	v1Ledger, xerr := v1.NewMutableLedger(
		dbName, dbDir, cacheSize, newV1MutableTestItem, logger,
	)
	require.NoError(t, xerr)
	defer func() {
		_ = v1Ledger.Close()
	}()

	require.NoError(t, v1Ledger.Set(
		[]byte("acct/a"), &mutableTestItem{value: []byte("a1")},
	))
	require.NoError(t, v1Ledger.Set(
		[]byte("acct/b"), &mutableTestItem{value: []byte("b1")},
	))
	require.NoError(t, v1Ledger.Set(
		[]byte("gov/a"), &mutableTestItem{value: []byte("g1")},
	))
	hash1, version1, xerr := v1Ledger.Commit()
	require.NoError(t, xerr)
	hash1 = bytes.Clone(hash1)
	require.EqualValues(t, 1, version1)

	require.NoError(t, v1Ledger.Set(
		[]byte("acct/a"), &mutableTestItem{value: []byte("a2")},
	))
	require.NoError(t, v1Ledger.Del([]byte("acct/b")))
	require.NoError(t, v1Ledger.Set(
		[]byte("acct/c"), &mutableTestItem{value: []byte("c2")},
	))
	hash2, version2, xerr := v1Ledger.Commit()
	require.NoError(t, xerr)
	hash2 = bytes.Clone(hash2)
	require.EqualValues(t, 2, version2)
	require.NoError(t, v1Ledger.Close())

	v2Ledger, xerr := NewMutableLedger(
		dbName, dbDir, cacheSize, newMutableTestItem, logger,
	)
	require.NoError(t, xerr)
	defer func() {
		_ = v2Ledger.Close()
	}()

	require.EqualValues(t, version2, v2Ledger.Version())
	tree1, xerr := v2Ledger.GetReadOnlyTree(version1)
	require.NoError(t, xerr)
	require.Equal(t, hash1, tree1.Hash())
	value, err := tree1.Get([]byte("acct/a"))
	require.NoError(t, err)
	require.Equal(t, []byte("a1"), value)

	tree2, xerr := v2Ledger.GetReadOnlyTree(version2)
	require.NoError(t, xerr)
	require.Equal(t, hash2, tree2.Hash())

	item, xerr := v2Ledger.Get([]byte("acct/a"))
	require.NoError(t, xerr)
	require.Equal(t, []byte("a2"), item.(*mutableTestItem).value)
	item, xerr = v2Ledger.Get([]byte("acct/b"))
	require.Nil(t, item)
	require.ErrorIs(t, xerr, xerrors.ErrNotFoundResult)

	entries, xerr := mutableTestSeek(v2Ledger, []byte("acct/"), true)
	require.NoError(t, xerr)
	require.Equal(t, []mutableTestEntry{
		{key: "acct/a", value: "a2"},
		{key: "acct/c", value: "c2"},
	}, entries)

	require.NoError(t, v2Ledger.Set(
		[]byte("acct/a"), &mutableTestItem{value: []byte("a3")},
	))
	require.NoError(t, v2Ledger.Del([]byte("acct/c")))
	require.NoError(t, v2Ledger.Set(
		[]byte("acct/d"), &mutableTestItem{value: []byte("d3")},
	))
	hash3, version3, xerr := v2Ledger.Commit()
	require.NoError(t, xerr)
	hash3 = bytes.Clone(hash3)
	require.EqualValues(t, 3, version3)
	require.NoError(t, v2Ledger.Close())

	reopened, xerr := NewMutableLedger(
		dbName, dbDir, cacheSize, newMutableTestItem, logger,
	)
	require.NoError(t, xerr)
	defer func() {
		_ = reopened.Close()
	}()

	require.EqualValues(t, version3, reopened.Version())
	tree1, xerr = reopened.GetReadOnlyTree(version1)
	require.NoError(t, xerr)
	require.Equal(t, hash1, tree1.Hash())
	tree2, xerr = reopened.GetReadOnlyTree(version2)
	require.NoError(t, xerr)
	require.Equal(t, hash2, tree2.Hash())
	tree3, xerr := reopened.GetReadOnlyTree(version3)
	require.NoError(t, xerr)
	require.Equal(t, hash3, tree3.Hash())

	item, xerr = reopened.Get([]byte("acct/a"))
	require.NoError(t, xerr)
	require.Equal(t, []byte("a3"), item.(*mutableTestItem).value)
	item, xerr = reopened.Get([]byte("acct/c"))
	require.Nil(t, item)
	require.ErrorIs(t, xerr, xerrors.ErrNotFoundResult)
	item, xerr = reopened.Get([]byte("acct/d"))
	require.NoError(t, xerr)
	require.Equal(t, []byte("d3"), item.(*mutableTestItem).value)
}

func TestMutableLedger_ReadOnly_Unsaved(t *testing.T) {
	ledger, cleanup := newMutableTestLedger(t, newMutableTestItem)
	defer cleanup()
	require.NoError(t, ledger.Set([]byte("old"), &mutableTestItem{value: []byte("saved")}))
	_, version, xerr := ledger.Commit()
	require.NoError(t, xerr)

	require.NoError(t, ledger.createCache())
	defer func() { _ = ledger.clearCache() }()
	require.NoError(t, ledger.Set([]byte("old"), &mutableTestItem{value: []byte("cached")}))
	require.NoError(t, ledger.Set([]byte("new"), &mutableTestItem{value: []byte("cached")}))
	tree, xerr := ledger.GetReadOnlyTree(version)
	require.NoError(t, xerr)
	value, err := tree.Get([]byte("old"))
	require.NoError(t, err)
	require.Equal(t, []byte("saved"), value)
	value, err = tree.Get([]byte("new"))
	require.NoError(t, err)
	require.Nil(t, value)
	require.NoError(t, ledger.clearCache())

	require.NoError(t, ledger.Set([]byte("old"), &mutableTestItem{value: []byte("unsaved")}))
	require.NoError(t, ledger.Set([]byte("new"), &mutableTestItem{value: []byte("unsaved")}))
	tree, xerr = ledger.GetReadOnlyTree(version)
	require.NoError(t, xerr)
	value, err = tree.Get([]byte("old"))
	require.NoError(t, err)
	require.Equal(t, []byte("saved"), value)
	value, err = tree.Get([]byte("new"))
	require.NoError(t, err)
	require.Nil(t, value)
}

func TestMutableLedger_Close_Operation(t *testing.T) {
	ledger, cleanup := newMutableTestLedger(t, newMutableTestItem)
	defer cleanup()
	require.NoError(t, ledger.Close())

	item, xerr := ledger.Get([]byte("key"))
	require.Nil(t, item)
	require.Error(t, xerr)
	require.Error(t, ledger.Set([]byte("key"), &mutableTestItem{value: []byte("value")}))
	require.Error(t, ledger.Del([]byte("key")))
	require.Error(t, ledger.Seek(nil, true, func(LedgerKey, ILedgerItem) xerrors.XError {
		return nil
	}))
	require.Error(t, ledger.Iterate(func(LedgerKey, ILedgerItem) xerrors.XError {
		return nil
	}))
	tree, xerr := ledger.GetReadOnlyTree(1)
	require.Nil(t, tree)
	require.Error(t, xerr)
	hash, version, xerr := ledger.Commit()
	require.Nil(t, hash)
	require.Zero(t, version)
	require.Error(t, xerr)
	require.Error(t, ledger.createCache())
	require.Error(t, ledger.writeCache())
	require.NoError(t, ledger.clearCache())
	require.Zero(t, ledger.Version())
	require.NoError(t, ledger.Close())
}
