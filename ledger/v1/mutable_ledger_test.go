package v1

import (
	"encoding/binary"
	"fmt"
	"github.com/beatoz/beatoz-go/types/xerrors"
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

	ledger, xerr := NewMutableLedger("ledger_test", dbDir, 1000000, func() ILedgerItem {
		return &Item{}
	}, log.NewNopLogger())
	require.NoError(t, xerr)

	item0 := newItem(0, "test item0 value")
	require.NoError(t, ledger.Set(item0))

	snap := ledger.Snapshot()
	fmt.Println("snapshot", snap, ": item0 exists, but item1 does not exist.")

	item1 := newItem(1, "test item1 value")
	require.NoError(t, ledger.Set(item1))

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

func TestLedger_RevertToSnapshot_Set1(t *testing.T) {
	dbDir, err := os.MkdirTemp("", "ledger_test")
	require.NoError(t, err)

	ledger, xerr := NewMutableLedger("ledger_test", dbDir, 1000000, func() ILedgerItem {
		return &Item{}
	}, log.NewNopLogger())
	require.NoError(t, xerr)

	var oriItems []*Item
	for i := 0; i < 10000; i++ {
		oriItems = append(oriItems, newItem(i, fmt.Sprintf("d%d", i)))
	}
	var newItems []*Item
	for i := 0; i < 10000; i++ {
		newItems = append(newItems, newItem(i, fmt.Sprintf("d%d%d", i, i)))
	}

	for _, it := range oriItems {
		require.NoError(t, ledger.Set(it))
	}

	snap := ledger.Snapshot()
	fmt.Println("snapshot", snap)

	for _, it := range newItems {
		require.NoError(t, ledger.Set(it))
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

	require.NoError(t, ledger.Close())
	require.NoError(t, os.RemoveAll(dbDir))
}

func TestLedger_RevertToSnapshot_Set2(t *testing.T) {
	dbDir, err := os.MkdirTemp("", "ledger_test")
	require.NoError(t, err)

	ledger, xerr := NewMutableLedger("ledger_test", dbDir, 1000000, func() ILedgerItem {
		return &Item{}
	}, log.NewNopLogger())
	require.NoError(t, xerr)

	staticKey := 123
	for i := 0; i < 10000; i++ {
		// key 고정
		xerr := ledger.Set(newItem(staticKey, strconv.Itoa(i)))
		require.NoError(t, xerr)
	}

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

	require.NoError(t, ledger.Close())
	require.NoError(t, os.RemoveAll(dbDir))
}

func TestLedger_RevertToSnapshot_Del(t *testing.T) {
	dbDir, err := os.MkdirTemp("", "ledger_test")
	require.NoError(t, err)
	ledger, xerr := NewMutableLedger("ledger_test", dbDir, 1000000, func() ILedgerItem {
		return &Item{}
	}, log.NewNopLogger())
	require.NoError(t, xerr)

	item := newItem(123, "data123")

	// set new item
	require.NoError(t, ledger.Set(item))

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
	require.Equal(t, item.Key(), _item.Key())
	require.Equal(t, item.data, _item.(*Item).data)

	require.NoError(t, ledger.Close())
	require.NoError(t, os.RemoveAll(dbDir))
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

func (i *Item) Decode(bz []byte) xerrors.XError {
	toks := strings.Split(string(bz), ",")
	key, _ := strings.CutPrefix(toks[0], "key:")
	data, _ := strings.CutPrefix(toks[1], "data:")

	var err error
	if i.key, err = strconv.Atoi(key); err != nil {
		return xerrors.From(err)
	}
	i.data = data
	return nil
}
