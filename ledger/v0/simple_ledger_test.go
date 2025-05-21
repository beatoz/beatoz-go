package v0

import (
	"github.com/beatoz/beatoz-go/libs/jsonx"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

type MyItem struct {
	Name  string `json:"name"`
	Value int32  `json:"value"`
}

func NewMyItem(nm string, val int32) *MyItem {
	return &MyItem{
		Name:  nm,
		Value: val,
	}
}

func (i *MyItem) Key() LedgerKey {
	return ToLedgerKey([]byte(i.Name))
}

func (i *MyItem) Encode() ([]byte, xerrors.XError) {
	if bz, err := jsonx.Marshal(i); err != nil {
		return nil, xerrors.From(err)
	} else {
		return bz, nil
	}
}

func (i *MyItem) Decode(d []byte) xerrors.XError {
	if err := jsonx.Unmarshal(d, i); err != nil {
		return xerrors.From(err)
	}
	return nil
}

func (i *MyItem) Equal(o *MyItem) bool {
	return (i.Name == o.Name) && (i.Value == o.Value)
}

var _ ILedgerItem = (*MyItem)(nil)

func TestSimpleLedger(t *testing.T) {
	dbDir := filepath.Join(os.TempDir(), "test")
	os.RemoveAll(dbDir)

	ledger, err := NewSimpleLedger[*MyItem]("treeLedger1", dbDir, 256, func() *MyItem { return &MyItem{} })
	require.NoError(t, err)

	i0 := NewMyItem("i0", 0)
	i1 := NewMyItem("i1", 1)
	i2 := NewMyItem("i2", 2)
	//i3 := NewMyItem("i3", 3)

	require.NoError(t, ledger.Set(i0))
	require.NoError(t, ledger.Set(i1))

	item, err := ledger.Get(i0.Key())
	require.NoError(t, err)
	require.NotNil(t, item)
	require.Equal(t, i0, item)

	// not committed yet
	item, err = ledger.Get(i1.Key())
	require.NoError(t, err)
	require.NotNil(t, item)
	require.Equal(t, i1, item)

	item, err = ledger.Del(i1.Key())
	require.NoError(t, err)
	require.NotNil(t, item)
	require.Equal(t, i1, item)

	// delete not exist
	item, err = ledger.Del(i2.Key())
	require.Error(t, err)
	require.Nil(t, item)

	//
	//
	//
	// SimpleLedger's Commit() always occurs panic.

	//// only commit i0
	//_, _, err = ledger.Commit()
	//require.Error(t, err)

	//item, err = ledger.Get(i0.Key())
	//require.NoError(t, err)
	//require.NotNil(t, item)
	//require.Equal(t, i0, item)
	//
	//// deleted item
	//item, err = ledger.Get(i1.Key())
	//require.Error(t, err)
	//require.Nil(t, item)
	//
	//// not found
	//item, err = ledger.Get(i2.Key())
	//require.Error(t, err)
	//require.Nil(t, item)
	//
	//// not exist
	//item, err = ledger.Del(i3.Key())
	//require.Error(t, err)
	//require.Nil(t, item)
}
