package v2

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/cosmos/iavl"
	dbm "github.com/cosmos/iavl/db"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"
)

func TestMemLedgerCacheWrite(t *testing.T) {
	ledger := newMemLedgerForTest(t)

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

func TestMemLedgerCacheDiscard(t *testing.T) {
	ledger := newMemLedgerForTest(t)

	child := ledger.CacheWrap()
	require.NoError(t, child.Set(testLedgerKey(2), newTestLedgerItem(2, "child")))
	child = nil

	_, xerr := ledger.Get(testLedgerKey(2))
	require.Error(t, xerr)
	require.True(t, xerr.Contains(xerrors.ErrNotFoundResult))
	require.Nil(t, child)
}

func TestMemLedgerSetDel(t *testing.T) {
	ledger := newMemLedgerForTest(t)

	require.NoError(t, ledger.Set(testLedgerKey(2), newTestLedgerItem(2, "two")))
	require.NoError(t, ledger.Del(testLedgerKey(2)))

	_, xerr := ledger.Get(testLedgerKey(2))
	require.Error(t, xerr)
	require.True(t, xerr.Contains(xerrors.ErrNotFoundResult))
}

func TestMemLedgerParentKept(t *testing.T) {
	ledger := newMemLedgerForTest(t)

	child := ledger.CacheWrap()
	require.NoError(t, child.Set(testLedgerKey(2), newTestLedgerItem(2, "child")))
	require.NoError(t, child.Del(testLedgerKey(1)))
	child = nil

	item, xerr := ledger.Get(testLedgerKey(1))
	require.NoError(t, xerr)
	require.Equal(t, "one", item.(*testLedgerItem).data)

	_, xerr = ledger.Get(testLedgerKey(2))
	require.Error(t, xerr)
	require.True(t, xerr.Contains(xerrors.ErrNotFoundResult))
	require.Nil(t, child)
}

func TestMemLedgerGetCopy(t *testing.T) {
	ledger := newMemLedgerForTest(t)

	item, xerr := ledger.Get(testLedgerKey(1))
	require.NoError(t, xerr)
	item.(*testLedgerItem).data = "mutated"

	item, xerr = ledger.Get(testLedgerKey(1))
	require.NoError(t, xerr)
	require.Equal(t, "one", item.(*testLedgerItem).data)
}

func TestMemLedgerDeleteShadow(t *testing.T) {
	ledger := newMemLedgerForTest(t)

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

func TestMemLedgerSeekOverlay(t *testing.T) {
	ledger := newMemLedgerForTest(t)

	require.NoError(t, ledger.Set(testLedgerKey(2), newTestLedgerItem(2, "two")))
	require.NoError(t, ledger.Del(testLedgerKey(3)))

	var got []string
	xerr := ledger.Seek(nil, true, func(_ LedgerKey, item ILedgerItem) xerrors.XError {
		got = append(got, item.(*testLedgerItem).data)
		return nil
	})
	require.NoError(t, xerr)
	require.Equal(t, []string{"one", "two"}, got)
}

func TestMemLedgerSeekDesc(t *testing.T) {
	ledger := newMemLedgerForTest(t)
	require.NoError(t, ledger.Set(testLedgerKey(2), newTestLedgerItem(2, "two")))

	var got []int
	xerr := ledger.Seek(nil, false, func(key LedgerKey, _ ILedgerItem) xerrors.XError {
		got = append(got, int(binary.BigEndian.Uint32(key)))
		return nil
	})
	require.NoError(t, xerr)
	require.Equal(t, []int{3, 2, 1}, got)
}

func TestMemLedgerGuards(t *testing.T) {
	ledger := newMemLedgerForTest(t)

	require.NotPanics(t, func() {
		require.Error(t, ledger.Set(testLedgerKey(1), nil))
	})
	require.NotPanics(t, func() {
		require.Error(t, ledger.Seek(nil, true, nil))
	})

	require.NoError(t, ledger.Set(testLedgerKey(2), newTestLedgerItem(2, "two")))
	ledger.newItemFor = func(LedgerKey) ILedgerItem {
		return nil
	}
	require.NotPanics(t, func() {
		_, xerr := ledger.Get(testLedgerKey(2))
		require.Error(t, xerr)
	})

	ledger.cache = nil
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
}

func newMemLedgerForTest(t *testing.T) *MemLedger {
	t.Helper()

	tree := newImmutableTreeForTest(t,
		newTestLedgerItem(1, "one"),
		newTestLedgerItem(3, "three"),
	)
	return newMemLedger(tree, newCacheContext(&immutableTreeStore{tree: tree}), func(LedgerKey) ILedgerItem {
		return &testLedgerItem{}
	}, log.NewNopLogger())
}

func newImmutableTreeForTest(t *testing.T, items ...*testLedgerItem) *iavl.ImmutableTree {
	t.Helper()

	mtree := iavl.NewMutableTree(dbm.NewMemDB(), 100, false, iavl.NewNopLogger())
	for _, item := range items {
		bz, xerr := item.Encode()
		require.NoError(t, xerr)
		_, err := mtree.Set(item.Key(), bz)
		require.NoError(t, err)
	}
	_, version, err := mtree.SaveVersion()
	require.NoError(t, err)

	itree, err := mtree.GetImmutable(version)
	require.NoError(t, err)
	return itree
}

type testLedgerItem struct {
	key  int
	data string
}

func newTestLedgerItem(key int, data string) *testLedgerItem {
	return &testLedgerItem{
		key:  key,
		data: data,
	}
}

func (item *testLedgerItem) Key() LedgerKey {
	return testLedgerKey(item.key)
}

func testLedgerKey(key int) LedgerKey {
	bz := make([]byte, 4)
	binary.BigEndian.PutUint32(bz, uint32(key))
	return bz
}

func (item *testLedgerItem) Encode() ([]byte, xerrors.XError) {
	return []byte(fmt.Sprintf("key:%d,data:%s", item.key, item.data)), nil
}

func (item *testLedgerItem) Decode(_, value []byte) xerrors.XError {
	parts := strings.Split(string(value), ",")
	key, _ := strings.CutPrefix(parts[0], "key:")
	data, _ := strings.CutPrefix(parts[1], "data:")

	i, err := strconv.Atoi(key)
	if err != nil {
		return xerrors.From(err)
	}
	item.key = i
	item.data = data
	return nil
}

var _ ILedgerItem = (*testLedgerItem)(nil)
