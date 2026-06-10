package v1

import (
	"encoding/binary"
	"fmt"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"
	"math"
	"os"
	"strconv"
	"testing"
)

var sourceLedger *MutableLedger
var preExistedItem *Item

func init() {
	dbDir, err := os.MkdirTemp("", "ledger_test")
	if err != nil {
		panic(err)
	}
	_ = os.RemoveAll(dbDir)

	_ledger, xerr := NewMutableLedger("ledger_test", dbDir, 1000000, func(key LedgerKey) ILedgerItem {
		return &Item{}
	}, log.NewNopLogger())
	if xerr != nil {
		panic(xerr)
	}

	preExistedItem = newItem(math.MaxInt32, "data001")
	if xerr := _ledger.Set(preExistedItem.Key(), preExistedItem); xerr != nil {
		panic(xerr)
	}
	if _, _, xerr = _ledger.Commit(); xerr != nil {
		panic(xerr)
	}

	sourceLedger = _ledger
}

func TestMemLedger_Del_MemItem(t *testing.T) {
	ledger, xerr := NewMemLedgerAt(1, sourceLedger, log.NewNopLogger())
	require.NoError(t, xerr)

	// set new item
	item := newItem(10, "data123")
	require.NoError(t, ledger.Set(item.Key(), item))

	_item, xerr := ledger.Get(item.Key())
	require.NoError(t, xerr)
	require.Equal(t, item.Key(), _item.(*Item).Key())
	require.Equal(t, item.data, _item.(*Item).data)

	// delete item
	require.NoError(t, ledger.Del(item.Key()))

	_item, xerr = ledger.Get(item.Key())
	require.Error(t, xerr)
}

func TestMemLedger_Del_PreItem(t *testing.T) {
	ledger, xerr := NewMemLedgerAt(1, sourceLedger, log.NewNopLogger())
	require.NoError(t, xerr)

	_item, xerr := ledger.Get(preExistedItem.Key())
	require.NoError(t, xerr)
	require.Equal(t, preExistedItem.Key(), _item.(*Item).Key())
	require.Equal(t, preExistedItem.data, _item.(*Item).data)

	// delete item
	require.NoError(t, ledger.Del(preExistedItem.Key()))

	_item, xerr = ledger.Get(preExistedItem.Key())
	require.Error(t, xerr)

	// the other MemLedger('otherLedger') must have the item which was deleted on the first MemLedger('ledger').
	otherLedger, xerr := NewMemLedgerAt(1, sourceLedger, log.NewNopLogger())
	require.NoError(t, xerr)
	_item, xerr = otherLedger.Get(preExistedItem.Key())
	require.NoError(t, xerr)
	require.Equal(t, preExistedItem.Key(), _item.(*Item).Key())
	require.Equal(t, preExistedItem.data, _item.(*Item).data)

}

// todo: Recheck for testing RevertToSnapshot

func TestMemLedger_RevertToSnapshot_Set0(t *testing.T) {
	ledger, xerr := NewMemLedgerAt(1, sourceLedger, log.NewNopLogger())
	require.NoError(t, xerr)

	item0 := newItem(101, "this item will be remained")
	require.NoError(t, ledger.Set(item0.Key(), item0))

	snap := ledger.Snapshot()
	fmt.Println("snapshot", snap, ": item0 exists, but item1 does not exist.")

	item1 := newItem(102, "this item will be removed after reverting.")
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
}

func TestMemLedger_RevertToSnapshot_Set1(t *testing.T) {
	ledger, xerr := NewMemLedgerAt(1, sourceLedger, log.NewNopLogger())
	require.NoError(t, xerr)

	var oriItems []*Item
	for i := 0; i < 10000; i++ {
		oriItems = append(oriItems, newItem(i, fmt.Sprintf("d%d", i)))
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
		newItems = append(newItems, newItem(i, fmt.Sprintf("d%d%d", i, i)))
	}
	for _, it := range newItems {
		require.NoError(t, ledger.Set(it.Key(), it))
	}

	for i := 0; i < 10000; i++ {
		k := make([]byte, 4)
		binary.BigEndian.PutUint32(k, uint32(i))
		item, xerr := ledger.Get(k)
		require.NoError(t, xerr)
		require.Equal(t, fmt.Sprintf("d%d%d", i, i), item.(*Item).data)
	}

	require.NoError(t, ledger.RevertToSnapshot(snap))

	for i := 0; i < 10000; i++ {
		k := make([]byte, 4)
		binary.BigEndian.PutUint32(k, uint32(i))
		item, xerr := ledger.Get(k)
		require.NoError(t, xerr)
		require.Equal(t, fmt.Sprintf("d%d", i), item.(*Item).data)
	}

	require.NoError(t, ledger.RevertToSnapshot(firstSnap))
	k := make([]byte, 4)
	binary.BigEndian.PutUint32(k, uint32(0))
	item, xerr := ledger.Get(k)
	require.NoError(t, xerr)
	require.Equal(t, fmt.Sprintf("d%d", 0), item.(*Item).data)

	for i := 1; i < 10000; i++ {
		k := make([]byte, 4)
		binary.BigEndian.PutUint32(k, uint32(i))
		item, xerr := ledger.Get(k)
		require.Nil(t, item)
		require.Error(t, xerrors.ErrNotFoundResult, xerr)
	}
}

func TestMemLedger_RevertToSnapshot_Set2(t *testing.T) {
	ledger, xerr := NewMemLedgerAt(1, sourceLedger, log.NewNopLogger())
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
}

func TestMemLedger_RevertToSnapshot_Set_Updated(t *testing.T) {
	ledger, xerr := NewMemLedgerAt(1, sourceLedger, log.NewNopLogger())
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

func TestMemLedger_RevertToSnapshot_Del(t *testing.T) {
	ledger, xerr := NewMemLedgerAt(1, sourceLedger, log.NewNopLogger())
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
}

func TestMemLedger_RevertToSnapshot_DelCommittedItem(t *testing.T) {
	ledger, xerr := NewMemLedgerAt(1, sourceLedger, log.NewNopLogger())
	require.NoError(t, xerr)

	snap := ledger.Snapshot()
	require.NoError(t, ledger.Del(preExistedItem.Key()))
	require.Equal(t, snap.revision+1, ledger.Snapshot().revision)

	_, xerr = ledger.Get(preExistedItem.Key())
	require.Error(t, xerr)
	require.True(t, xerr.Contains(xerrors.ErrNotFoundResult))

	require.NoError(t, ledger.RevertToSnapshot(snap))
	restored, xerr := ledger.Get(preExistedItem.Key())
	require.NoError(t, xerr)
	require.Equal(t, preExistedItem.data, restored.(*Item).data)
}

func TestMemLedger_DelDoesNotRecordDuplicateRevision(t *testing.T) {
	ledger, xerr := NewMemLedgerAt(1, sourceLedger, log.NewNopLogger())
	require.NoError(t, xerr)

	require.NoError(t, ledger.Del(preExistedItem.Key()))
	afterFirstDelete := ledger.Snapshot()
	require.NoError(t, ledger.Del(preExistedItem.Key()))
	require.Equal(t, afterFirstDelete, ledger.Snapshot())

	missingKey := newItem(987654, "").Key()
	beforeMissingDelete := ledger.Snapshot()
	require.NoError(t, ledger.Del(missingKey))
	require.Equal(t, beforeMissingDelete, ledger.Snapshot())
}

func TestMemLedger_RevertToSnapshotRejectsInvalidSnapshot(t *testing.T) {
	ledger, xerr := NewMemLedgerAt(1, sourceLedger, log.NewNopLogger())
	require.NoError(t, xerr)

	t.Run("zero value", func(t *testing.T) {
		require.NotPanics(t, func() {
			xerr := ledger.RevertToSnapshot(Snapshot{})
			require.Error(t, xerr)
			require.True(t, xerr.Contains(xerrors.ErrInvalidSnapshot))
		})
	})

	t.Run("foreign ledger", func(t *testing.T) {
		otherLedger, xerr := NewMemLedgerAt(1, sourceLedger, log.NewNopLogger())
		require.NoError(t, xerr)

		require.NotPanics(t, func() {
			xerr = ledger.RevertToSnapshot(otherLedger.Snapshot())
			require.Error(t, xerr)
			require.True(t, xerr.Contains(xerrors.ErrInvalidSnapshot))
		})
	})

	t.Run("generation mismatch", func(t *testing.T) {
		snap := ledger.Snapshot()
		snap.generation++

		require.NotPanics(t, func() {
			xerr := ledger.RevertToSnapshot(snap)
			require.Error(t, xerr)
			require.True(t, xerr.Contains(xerrors.ErrInvalidSnapshot))
		})
	})

	t.Run("revision out of range", func(t *testing.T) {
		snap := ledger.Snapshot()
		snap.revision++

		require.NotPanics(t, func() {
			xerr := ledger.RevertToSnapshot(snap)
			require.Error(t, xerr)
			require.True(t, xerr.Contains(xerrors.ErrInvalidSnapshot))
		})
	})

	t.Run("negative revision", func(t *testing.T) {
		snap := ledger.Snapshot()
		snap.revision = -1

		require.NotPanics(t, func() {
			xerr := ledger.RevertToSnapshot(snap)
			require.Error(t, xerr)
			require.True(t, xerr.Contains(xerrors.ErrInvalidSnapshot))
		})
	})
}

func TestMemLedger_SnapshotLifecycle(t *testing.T) {
	newLedger := func(t *testing.T) *MemLedger {
		t.Helper()

		ledger, xerr := NewMemLedgerAt(1, sourceLedger, log.NewNopLogger())
		require.NoError(t, xerr)
		return ledger
	}

	t.Run("inner then outer revert", func(t *testing.T) {
		ledger := newLedger(t)
		key := newItem(701, "").Key()

		require.NoError(t, ledger.Set(key, newItem(701, "base")))
		outer := ledger.Snapshot()
		require.NoError(t, ledger.Set(key, newItem(701, "outer")))
		inner := ledger.Snapshot()
		require.NoError(t, ledger.Set(key, newItem(701, "inner")))

		require.NoError(t, ledger.RevertToSnapshot(inner))
		requireSnapshotItemData(t, ledger, key, "outer")
		require.NoError(t, ledger.RevertToSnapshot(outer))
		requireSnapshotItemData(t, ledger, key, "base")
	})

	t.Run("outer then inner revert is rejected while out of range", func(t *testing.T) {
		ledger := newLedger(t)
		key := newItem(702, "").Key()

		outer := ledger.Snapshot()
		require.NoError(t, ledger.Set(key, newItem(702, "outer")))
		inner := ledger.Snapshot()
		require.NoError(t, ledger.Set(key, newItem(702, "inner")))

		require.NoError(t, ledger.RevertToSnapshot(outer))
		xerr := ledger.RevertToSnapshot(inner)
		require.Error(t, xerr)
		require.True(t, xerr.Contains(xerrors.ErrInvalidSnapshot))
	})

	t.Run("same snapshot can be reused as no-op", func(t *testing.T) {
		ledger := newLedger(t)
		key := newItem(703, "").Key()

		snap := ledger.Snapshot()
		require.NoError(t, ledger.Set(key, newItem(703, "created")))
		require.NoError(t, ledger.RevertToSnapshot(snap))
		require.NoError(t, ledger.RevertToSnapshot(snap))

		_, xerr := ledger.Get(key)
		require.Error(t, xerr)
		require.True(t, xerr.Contains(xerrors.ErrNotFoundResult))
	})

	t.Run("old marker is valid when revision position is reused", func(t *testing.T) {
		ledger := newLedger(t)
		firstKey := newItem(704, "").Key()
		secondKey := newItem(705, "").Key()

		outer := ledger.Snapshot()
		require.NoError(t, ledger.Set(firstKey, newItem(704, "first")))
		oldInner := ledger.Snapshot()
		require.NoError(t, ledger.RevertToSnapshot(outer))

		require.NoError(t, ledger.Set(secondKey, newItem(705, "second")))
		require.Equal(t, oldInner.revision, ledger.Snapshot().revision)
		require.NoError(t, ledger.RevertToSnapshot(oldInner))
		requireSnapshotItemData(t, ledger, secondKey, "second")
	})
}
