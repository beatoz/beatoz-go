package ledger

import (
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/rand"
	"os"
	"path/filepath"
	"testing"
)

var (
	testLedger *FinalityLedger[*MyItem]
	testItem0  *MyItem
	testItem1  *MyItem
	testItem2  *MyItem
)

func resetLedger(t *testing.T) {
	dbDir := filepath.Join(os.TempDir(), "test")

	if testLedger != nil {
		require.NoError(t, testLedger.Close())
		os.RemoveAll(dbDir)
	}

	var err error
	testLedger, err = NewFinalityLedger[*MyItem]("treeLedger1", dbDir, 256, func() *MyItem { return &MyItem{} })
	require.NoError(t, err)

	testItem0 = NewMyItem(bytes.RandHexString(32), rand.Int32())
	testItem1 = NewMyItem(bytes.RandHexString(32), rand.Int32())
	testItem2 = NewMyItem(bytes.RandHexString(32), rand.Int32())
}

func TestFinalityLedger_Set(t *testing.T) {
	resetLedger(t)

	// do set only
	require.NoError(t, testLedger.Set(testItem0))

	// not committed finally
	item, err := testLedger.Get(testItem0.Key())
	require.NoError(t, err)
	require.NotNil(t, item)

	// testLedger is FinalityLedger.
	// So Commit() do commit FinalityLedger::finalityItems to disk not SimpleLedger::cachedItems.
	_, _, err = testLedger.Commit()
	require.NoError(t, err)

	// get item that was previously set to SimpleLedger::cachedItems
	// this is reset by Commit()
	item, err = testLedger.Get(testItem0.Key())
	require.Error(t, err)

	// not committed(finalized)
	item, err = testLedger.Read(testItem0.Key())
	require.Error(t, err)
	require.Nil(t, item)
}

func TestFinalityLedger_SetFinality(t *testing.T) {
	// set finality
	require.NoError(t, testLedger.SetFinality(testItem1))

	//  not found: testItem1 is set to FinalityLedger::finalityItems, not SimpleLedger::cachedItems
	item, err := testLedger.Get(testItem1.Key())
	require.Error(t, err)
	require.Nil(t, item)

	item, err = testLedger.GetFinality(testItem1.Key())
	require.NoError(t, err)
	require.NotNil(t, item)

	// not committed(finalized)
	item, err = testLedger.Read(testItem1.Key())
	require.Error(t, err)
	require.Nil(t, item)

	// commit finalityItems
	_, _, err = testLedger.Commit()
	require.NoError(t, err)

	item, err = testLedger.Get(testItem1.Key())
	require.NoError(t, err)
	require.NotNil(t, item)
	require.Equal(t, testItem1, item)

	item, err = testLedger.Read(testItem1.Key())
	require.NoError(t, err)
	require.NotNil(t, item)
	require.Equal(t, testItem1, item)
}

func TestFinalityLedger_Del(t *testing.T) {
	// not exist
	item, err := testLedger.Del(ToLedgerKey(rand.Bytes(32)))
	require.Error(t, err)
	require.Nil(t, item)

	item, err = testLedger.Del(testItem1.Key())
	require.NoError(t, err)
	require.NotNil(t, item)
	require.Equal(t, testItem1, item)

	// not finally deleted yet
	item, err = testLedger.Get(testItem1.Key())
	require.Error(t, err)
	require.Nil(t, item)

	// not finally deleted yet
	item, err = testLedger.Read(testItem1.Key())
	require.NoError(t, err)
	require.NotNil(t, item)
	require.Equal(t, testItem1, item)
}

func TestFinalityLedger_DelFinality(t *testing.T) {
	item, err := testLedger.DelFinality(testItem1.Key())
	require.NoError(t, err)
	require.NotNil(t, item)
	require.Equal(t, testItem1, item)

	// not finally deleted and committed
	item, err = testLedger.Get(testItem1.Key())
	require.Error(t, err)
	require.Nil(t, item)

	// commit finality items
	_, _, err = testLedger.Commit()
	require.NoError(t, err)

	// finally deleted
	item, err = testLedger.Get(testItem1.Key())
	require.Error(t, err)
	require.Nil(t, item)

	item, err = testLedger.GetFinality(testItem1.Key())
	require.Error(t, err)
	require.Nil(t, item)
}

func TestFinalityLedger_CancelSetFinality(t *testing.T) {
	resetLedger(t)

	// do set and delete
	require.NoError(t, testLedger.SetFinality(testItem0))
	err := testLedger.CancelSetFinality(testItem0.Key())
	require.NoError(t, err)

	_, _, err = testLedger.Commit()
	require.NoError(t, err)

	// not exists
	item, err := testLedger.Get(testItem0.Key())
	require.Error(t, err)
	require.Nil(t, item)

	// not exists
	item, err = testLedger.GetFinality(testItem0.Key())
	require.Error(t, err)
	require.Nil(t, item)
}

func TestFinalityLedger_SetAfterDelFinality(t *testing.T) {
	require.NoError(t, testLedger.SetFinality(testItem2))

	// commit finality items
	_, _, err := testLedger.Commit()
	require.NoError(t, err)

	item, err := testLedger.DelFinality(testItem2.Key())
	require.NoError(t, err)
	require.NotNil(t, item)
	require.Equal(t, testItem2, item)

	// again set
	require.NoError(t, testLedger.SetFinality(testItem2))

	// commit finality items
	_, _, err = testLedger.Commit()
	require.NoError(t, err)

	// finally deleted
	item, err = testLedger.Get(testItem2.Key())
	require.NoError(t, err)
	require.NotNil(t, item)

	require.NoError(t, testLedger.Close())
}
