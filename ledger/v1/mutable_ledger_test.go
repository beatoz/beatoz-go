package v1

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/cosmos/iavl"
	dbm "github.com/cosmos/iavl/db"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestLedger_RevertToSnapshot_Set0(t *testing.T) {
	dbDir, err := os.MkdirTemp("", "ledger_test")
	require.NoError(t, err)
	ledger, xerr := NewMutableLedger("ledger_test", dbDir, 1000000, func(key LedgerKey) ILedgerItem {
		return &Item{}
	}, log.NewNopLogger())
	require.NoError(t, xerr)

	item0 := newItem(0, "test item0 value")
	require.NoError(t, ledger.Set(item0.Key(), item0))

	snap := ledger.Snapshot()
	fmt.Println("snapshot", snap, ": item0 exists, but item1 does not exist.")

	item1 := newItem(1, "test item1 value")
	require.NoError(t, ledger.Set(item1.Key(), item1))

	// item0 exists
	_item, xerr := ledger.Get(item0.Key())
	require.NoError(t, xerr)
	require.Equal(t, item0, _item)

	// item1 exists
	_item, xerr = ledger.Get(item1.Key())
	require.NoError(t, xerr)
	require.Equal(t, item1, _item)

	// item0 should be not removed but item1 should be removed.
	require.NoError(t, ledger.RevertToSnapshot(snap))

	_item, xerr = ledger.Get(item0.Key())
	require.NoError(t, xerr)
	require.Equal(t, item0, _item)

	_item, xerr = ledger.Get(item1.Key())
	require.Error(t, xerr)
	require.Equal(t, xerrors.ErrNotFoundResult, xerr)
	require.Nil(t, _item)

	require.NoError(t, ledger.Close())
	require.NoError(t, os.RemoveAll(dbDir))
}

func TestMutableLedger_RevertToSnapshot_Set1(t *testing.T) {
	dbDir, err := os.MkdirTemp("", "ledger_test")
	require.NoError(t, err)
	ledger, xerr := NewMutableLedger("ledger_test", dbDir, 1000000, func(key LedgerKey) ILedgerItem {
		return &Item{}
	}, log.NewNopLogger())
	require.NoError(t, xerr)

	var oriItems []*Item
	for i := 0; i < 10000; i++ {
		oriItems = append(oriItems, newItem(i, fmt.Sprintf("origin:%d", i)))
	}
	var firstSnap Snapshot
	for i, it := range oriItems {
		require.NoError(t, ledger.Set(it.Key(), it))
		if i == 0 {
			firstSnap = ledger.Snapshot()
		}
	}

	snap := ledger.Snapshot()
	require.Equal(t, 10000, snap.revision)

	var newItems []*Item
	for i := 0; i < 10000; i++ {
		newItems = append(newItems, newItem(i, fmt.Sprintf("updated:%d%d", i, i)))
	}
	for _, it := range newItems {
		require.NoError(t, ledger.Set(it.Key(), it))
	}

	for i := 0; i < 10000; i++ {
		k := make([]byte, 4)
		binary.BigEndian.PutUint32(k, uint32(i))
		item, xerr := ledger.Get(k)
		require.NoError(t, xerr)
		require.Equal(t, fmt.Sprintf("updated:%d%d", i, i), item.(*Item).data)
	}

	require.NoError(t, ledger.RevertToSnapshot(snap))

	for i := 0; i < 10000; i++ {
		k := make([]byte, 4)
		binary.BigEndian.PutUint32(k, uint32(i))
		item, xerr := ledger.Get(k)
		require.NoError(t, xerr)
		require.Equal(t, fmt.Sprintf("origin:%d", i), item.(*Item).data)
	}

	require.NoError(t, ledger.RevertToSnapshot(firstSnap))
	k := make([]byte, 4)
	binary.BigEndian.PutUint32(k, uint32(0))
	item, xerr := ledger.Get(k)
	require.NoError(t, xerr)
	require.Equal(t, fmt.Sprintf("origin:%d", 0), item.(*Item).data)

	for i := 1; i < 10000; i++ {
		k := make([]byte, 4)
		binary.BigEndian.PutUint32(k, uint32(i))
		item, xerr := ledger.Get(k)
		require.Nil(t, item)
		require.Error(t, xerrors.ErrNotFoundResult, xerr)
	}

	require.NoError(t, ledger.Close())
	require.NoError(t, os.RemoveAll(dbDir))
}

func TestMutableLedger_RevertToSnapshot_Set2(t *testing.T) {
	dbDir, err := os.MkdirTemp("", "ledger_test")
	require.NoError(t, err)

	ledger, xerr := NewMutableLedger("ledger_test", dbDir, 1000000, func(key LedgerKey) ILedgerItem {
		return &Item{}
	}, log.NewNopLogger())
	require.NoError(t, xerr)

	staticKey := 123
	snapshots := make([]Snapshot, 10001)
	snapshots[0] = ledger.Snapshot()
	for i := 0; i < 10000; i++ {
		// key 고정
		item := newItem(staticKey, strconv.Itoa(i))
		xerr := ledger.Set(item.Key(), item)
		require.NoError(t, xerr)
		snapshots[i+1] = ledger.Snapshot()
	}

	require.Equal(t, 10000, ledger.Snapshot().revision)

	for i := 10000; i >= 0; i-- {
		// partially revert
		require.NoError(t, ledger.RevertToSnapshot(snapshots[i]))

		k := make([]byte, 4)
		binary.BigEndian.PutUint32(k, uint32(staticKey))

		item, xerr := ledger.Get(k)
		if i == 0 {
			// all reverted
			require.Error(t, xerrors.ErrNotFoundResult, xerr)
		} else {
			require.NoError(t, xerr, fmt.Sprintf("current index: %d", i))
			require.Equal(t, strconv.Itoa(i-1), item.(*Item).data, fmt.Sprintf("current index: %d", i))
		}
	}

	require.NoError(t, ledger.Close())
	require.NoError(t, os.RemoveAll(dbDir))
}

func TestMutableLedger_RevertToSnapshot_Set_Updated(t *testing.T) {
	dbDir, err := os.MkdirTemp("", "ledger_test")
	require.NoError(t, err)

	ledger, xerr := NewMutableLedger("ledger_test", dbDir, 1000000, func(key LedgerKey) ILedgerItem {
		return &Item{}
	}, log.NewNopLogger())
	require.NoError(t, xerr)

	key := 1234
	originData := "originData"
	updateData := "updateData"

	item := newItem(key, originData)
	xerr = ledger.Set(item.Key(), item)
	require.NoError(t, xerr)

	snap := ledger.Snapshot()
	require.Equal(t, 1, snap.revision)

	_item0, xerr := ledger.Get(item.Key())
	require.NoError(t, xerr)

	// `_item0` is identical to `item`
	_item0.(*Item).data = updateData
	xerr = ledger.Set(_item0.(*Item).Key(), _item0)
	require.NoError(t, xerr)

	_item1, xerr := ledger.Get(item.Key())
	require.NoError(t, xerr)
	require.Equal(t, updateData, _item1.(*Item).data)

	xerr = ledger.RevertToSnapshot(snap)
	require.NoError(t, xerr)

	_item4, xerr := ledger.Get(item.Key())
	require.NoError(t, xerr)
	require.Equal(t, originData, _item4.(*Item).data)

}

func TestMutableLedger_RevertToSnapshot_Del(t *testing.T) {
	dbDir, err := os.MkdirTemp("", "ledger_test")
	require.NoError(t, err)
	ledger, xerr := NewMutableLedger("ledger_test", dbDir, 1000000, func(key LedgerKey) ILedgerItem {
		return &Item{}
	}, log.NewNopLogger())
	require.NoError(t, xerr)

	item := newItem(123, "data123")

	// set new item
	require.NoError(t, ledger.Set(item.Key(), item))

	// get snapshot
	snap := ledger.Snapshot()
	require.Equal(t, 1, snap.revision)

	// delete item
	require.NoError(t, ledger.Del(item.Key()))

	// revert deletion
	require.NoError(t, ledger.RevertToSnapshot(snap))

	// expected that the deleted item was restored
	_item, xerr := ledger.Get(item.Key())
	require.NoError(t, xerr)
	require.Equal(t, item.Key(), _item.(*Item).Key())
	require.Equal(t, item.data, _item.(*Item).data)

	require.NoError(t, ledger.Close())
	require.NoError(t, os.RemoveAll(dbDir))
}

func TestMutableLedger_CommitReset(t *testing.T) {
	dbDir := t.TempDir()
	ledger, xerr := NewMutableLedger("ledger_test", dbDir, 100, func(key LedgerKey) ILedgerItem {
		return &Item{}
	}, log.NewNopLogger())
	require.NoError(t, xerr)
	t.Cleanup(func() {
		require.NoError(t, ledger.Close())
	})

	item := newItem(456, "data456")
	require.NoError(t, ledger.Set(item.Key(), item))
	beforeCommit := ledger.Snapshot()
	require.Equal(t, 1, beforeCommit.revision)

	_, _, xerr = ledger.Commit()
	require.NoError(t, xerr)
	afterCommit := ledger.Snapshot()
	require.Equal(t, 0, afterCommit.revision)
	require.Equal(t, beforeCommit.ledgerID, afterCommit.ledgerID)
	require.NotEqual(t, beforeCommit.generation, afterCommit.generation)
}

func TestMutableLedger_SnapshotIdentity(t *testing.T) {
	firstLedger := newMutableLedgerForTest(t)
	secondLedger := newMutableLedgerForTest(t)

	first := firstLedger.Snapshot()
	second := firstLedger.Snapshot()
	otherLedger := secondLedger.Snapshot()

	require.NotZero(t, first.ledgerID)
	require.NotZero(t, first.generation)
	require.Equal(t, first.ledgerID, second.ledgerID)
	require.Equal(t, first.generation, second.generation)
	require.Equal(t, first, second)
	require.NotEqual(t, first.ledgerID, otherLedger.ledgerID)
}

func TestMutableLedger_InvalidSnapshot(t *testing.T) {
	t.Run("zero", func(t *testing.T) {
		ledger := newMutableLedgerForTest(t)

		require.NotPanics(t, func() {
			xerr := ledger.RevertToSnapshot(Snapshot{})
			require.Error(t, xerr)
			require.True(t, xerr.Contains(xerrors.ErrInvalidSnapshot))
		})
	})

	t.Run("foreign", func(t *testing.T) {
		ledger := newMutableLedgerForTest(t)
		otherLedger := newMutableLedgerForTest(t)

		require.NotPanics(t, func() {
			xerr := ledger.RevertToSnapshot(otherLedger.Snapshot())
			require.Error(t, xerr)
			require.True(t, xerr.Contains(xerrors.ErrInvalidSnapshot))
		})
	})

	t.Run("stale", func(t *testing.T) {
		ledger := newMutableLedgerForTest(t)
		require.NoError(t, ledger.Set(newItem(501, "").Key(), newItem(501, "committed")))
		stale := ledger.Snapshot()
		_, _, xerr := ledger.Commit()
		require.NoError(t, xerr)

		require.NotPanics(t, func() {
			xerr = ledger.RevertToSnapshot(stale)
			require.Error(t, xerr)
			require.True(t, xerr.Contains(xerrors.ErrInvalidSnapshot))
		})
	})

	t.Run("range", func(t *testing.T) {
		ledger := newMutableLedgerForTest(t)
		snap := ledger.Snapshot()
		snap.revision = 1

		require.NotPanics(t, func() {
			xerr := ledger.RevertToSnapshot(snap)
			require.Error(t, xerr)
			require.True(t, xerr.Contains(xerrors.ErrInvalidSnapshot))
		})
	})

	t.Run("negative", func(t *testing.T) {
		ledger := newMutableLedgerForTest(t)
		snap := ledger.Snapshot()
		snap.revision = -1

		require.NotPanics(t, func() {
			xerr := ledger.RevertToSnapshot(snap)
			require.Error(t, xerr)
			require.True(t, xerr.Contains(xerrors.ErrInvalidSnapshot))
		})
	})
}

func TestMutableLedger_SnapshotLifecycle(t *testing.T) {
	t.Run("inner outer", func(t *testing.T) {
		ledger := newMutableLedgerForTest(t)
		key := newItem(601, "").Key()

		require.NoError(t, ledger.Set(key, newItem(601, "base")))
		outer := ledger.Snapshot()
		require.NoError(t, ledger.Set(key, newItem(601, "outer")))
		inner := ledger.Snapshot()
		require.NoError(t, ledger.Set(key, newItem(601, "inner")))

		require.NoError(t, ledger.RevertToSnapshot(inner))
		requireItemData(t, ledger, key, "outer")
		require.NoError(t, ledger.RevertToSnapshot(outer))
		requireItemData(t, ledger, key, "base")
	})

	t.Run("outer inner reject", func(t *testing.T) {
		ledger := newMutableLedgerForTest(t)
		key := newItem(602, "").Key()

		outer := ledger.Snapshot()
		require.NoError(t, ledger.Set(key, newItem(602, "outer")))
		inner := ledger.Snapshot()
		require.NoError(t, ledger.Set(key, newItem(602, "inner")))

		require.NoError(t, ledger.RevertToSnapshot(outer))
		xerr := ledger.RevertToSnapshot(inner)
		require.Error(t, xerr)
		require.True(t, xerr.Contains(xerrors.ErrInvalidSnapshot))
	})

	t.Run("noop reuse", func(t *testing.T) {
		ledger := newMutableLedgerForTest(t)
		key := newItem(603, "").Key()

		snap := ledger.Snapshot()
		require.NoError(t, ledger.Set(key, newItem(603, "created")))
		require.NoError(t, ledger.RevertToSnapshot(snap))
		require.NoError(t, ledger.RevertToSnapshot(snap))

		_, xerr := ledger.Get(key)
		require.Error(t, xerr)
		require.True(t, xerr.Contains(xerrors.ErrNotFoundResult))
	})

	t.Run("reuse revision", func(t *testing.T) {
		ledger := newMutableLedgerForTest(t)
		firstKey := newItem(604, "").Key()
		secondKey := newItem(605, "").Key()

		outer := ledger.Snapshot()
		require.NoError(t, ledger.Set(firstKey, newItem(604, "first")))
		oldInner := ledger.Snapshot()
		require.NoError(t, ledger.RevertToSnapshot(outer))

		require.NoError(t, ledger.Set(secondKey, newItem(605, "second")))
		require.Equal(t, oldInner.revision, ledger.Snapshot().revision)
		require.NoError(t, ledger.RevertToSnapshot(oldInner))
		requireItemData(t, ledger, secondKey, "second")
	})
}

func TestMutableLedger_RevertCache(t *testing.T) {
	ledger := newMutableLedgerForTest(t)
	key := newItem(606, "").Key()

	require.NoError(t, ledger.Set(key, newItem(606, "persisted")))
	_, _, xerr := ledger.Commit()
	require.NoError(t, xerr)

	cached, xerr := ledger.Get(key)
	require.NoError(t, xerr)
	snap := ledger.Snapshot()

	cached.(*Item).data = "mutated-without-set"
	require.Equal(t, "mutated-without-set", cached.(*Item).data)

	require.NoError(t, ledger.RevertToSnapshot(snap))

	reloaded, xerr := ledger.Get(key)
	require.NoError(t, xerr)
	require.Equal(t, "persisted", reloaded.(*Item).data)
	require.NotSame(t, cached, reloaded)
}

func TestMutableLedger_RevertFatal(t *testing.T) {
	t.Run("set", func(t *testing.T) {
		ledger, failingDB := newFailingReadLedger(t)
		key := newItem(901, "").Key()
		snap := ledger.Snapshot()
		ledger.revisions.set(key, []byte("old-value"))
		ledger.cachedObjs[string(key)] = newItem(901, "cached")
		revisionCount := len(ledger.revisions.revs)

		failingDB.failReads = true
		xerr := ledger.RevertToSnapshot(snap)

		require.Error(t, xerr)
		require.True(t, xerr.Contains(xerrors.ErrFatalLedger))
		require.Contains(t, xerr.Error(), "revert snapshot set failed")
		require.Empty(t, ledger.cachedObjs)
		require.Len(t, ledger.revisions.revs, revisionCount)
	})

	t.Run("remove", func(t *testing.T) {
		ledger, failingDB := newFailingReadLedger(t)
		key := newItem(902, "").Key()
		snap := ledger.Snapshot()
		ledger.revisions.set(key, nil)
		ledger.cachedObjs[string(key)] = newItem(902, "cached")
		revisionCount := len(ledger.revisions.revs)

		failingDB.failReads = true
		xerr := ledger.RevertToSnapshot(snap)

		require.Error(t, xerr)
		require.True(t, xerr.Contains(xerrors.ErrFatalLedger))
		require.Contains(t, xerr.Error(), "revert snapshot remove failed")
		require.Empty(t, ledger.cachedObjs)
		require.Len(t, ledger.revisions.revs, revisionCount)
	})
}

func requireItemData(t *testing.T, ledger IGettable, key LedgerKey, want string) {
	t.Helper()

	item, xerr := ledger.Get(key)
	require.NoError(t, xerr)
	require.Equal(t, want, item.(*Item).data)
}

func newMutableLedgerForTest(t *testing.T) *MutableLedger {
	t.Helper()

	ledger, xerr := NewMutableLedger("ledger_test", t.TempDir(), 100, func(key LedgerKey) ILedgerItem {
		return &Item{}
	}, log.NewNopLogger())
	require.NoError(t, xerr)
	t.Cleanup(func() {
		require.NoError(t, ledger.Close())
	})
	return ledger
}

type failingReadDB struct {
	dbm.DB
	failReads bool
}

func (db *failingReadDB) Get(key []byte) ([]byte, error) {
	if db.failReads {
		return nil, errors.New("injected IAVL read failure")
	}
	return db.DB.Get(key)
}

func newFailingReadLedger(t *testing.T) (*MutableLedger, *failingReadDB) {
	t.Helper()

	baseDB := dbm.NewMemDB()
	sourceTree := iavl.NewMutableTree(baseDB, 0, true, iavl.NewNopLogger())
	for i := 900; i < 904; i++ {
		_, err := sourceTree.Set(newItem(i, "").Key(), []byte(fmt.Sprintf("value-%d", i)))
		require.NoError(t, err)
	}
	_, version, err := sourceTree.SaveVersion()
	require.NoError(t, err)

	failingDB := &failingReadDB{DB: baseDB}
	tree := iavl.NewMutableTree(failingDB, 0, true, iavl.NewNopLogger())
	_, err = tree.LoadVersion(version)
	require.NoError(t, err)

	return &MutableLedger{
		db:         failingDB,
		tree:       tree,
		revisions:  newSnapshotList[[]byte](),
		cachedObjs: make(map[string]ILedgerItem),
		newItemFor: func(key LedgerKey) ILedgerItem { return &Item{} },
		cacheSize:  0,
		logger:     log.NewNopLogger(),
	}, failingDB
}

type Item struct {
	key  int
	data string
}

func newItem(key int, data string) *Item {
	return &Item{
		key:  key,
		data: data,
	}
}

func (i *Item) Key() []byte {
	bs := make([]byte, 4)
	binary.BigEndian.PutUint32(bs, uint32(i.key))
	return bs
}

func (i *Item) Encode() ([]byte, xerrors.XError) {
	return []byte(fmt.Sprintf("key:%v,data:%v", i.key, i.data)), nil
}

func (i *Item) Decode(k, v []byte) xerrors.XError {
	toks := strings.Split(string(v), ",")
	key, _ := strings.CutPrefix(toks[0], "key:")
	data, _ := strings.CutPrefix(toks[1], "data:")

	var err error
	if i.key, err = strconv.Atoi(key); err != nil {
		return xerrors.From(err)
	}
	i.data = data
	return nil
}

var _ ILedgerItem = (*Item)(nil)
