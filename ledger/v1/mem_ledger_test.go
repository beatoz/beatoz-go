package v1

import (
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"
	"math"
	"os"
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
