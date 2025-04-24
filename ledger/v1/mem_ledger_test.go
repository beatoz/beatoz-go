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
	for _, it := range oriItems {
		require.NoError(t, ledger.Set(it.Key(), it))
	}

	snap := ledger.Snapshot()
	require.Equal(t, 10000, snap)

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

	require.NoError(t, ledger.RevertToSnapshot(1))
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
	for i := 0; i < 10000; i++ {
		// key 고정
		item := newItem(staticKey, strconv.Itoa(i))
		xerr := ledger.Set(item.Key(), item)
		require.NoError(t, xerr)
	}

	require.Equal(t, 10000, ledger.Snapshot())

	for i := 10000; i >= 0; i-- {
		// partially revert
		require.NoError(t, ledger.RevertToSnapshot(i))

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
	require.Equal(t, 1, snap)

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
	require.Equal(t, 1, snap)

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
