package v0

import (
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/cosmos/iavl"
	dbm "github.com/cosmos/iavl/db"
	"sort"
	"sync"
)

type FinalityLedger[T ILedgerItem] struct {
	SimpleLedger[T]
	finalityItems *memItems[T]

	mtx sync.RWMutex
}

func NewFinalityLedger[T ILedgerItem](name, dbDir string, cacheSize int, cb func() T) (*FinalityLedger[T], xerrors.XError) {
	db, err := dbm.NewGoLevelDB(name, dbDir)
	if err != nil {
		return nil, xerrors.From(err)
	}

	tree := iavl.NewMutableTree(db, cacheSize, false, iavl.NewNopLogger())
	if _, err := tree.Load(); err != nil {
		_ = db.Close()
		return nil, xerrors.From(err)
	} else {
		return &FinalityLedger[T]{
			SimpleLedger: SimpleLedger[T]{
				db:          db,
				tree:        tree,
				cachedItems: newMemItems[T](),
				getNewItem:  cb,
			},
			finalityItems: newMemItems[T](),
		}, nil
	}
}

func (ledger *FinalityLedger[T]) ImmutableLedgerAt(n int64, cacheSize int) (ILedger[T], xerrors.XError) {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()
	simledger, xerr := ledger.SimpleLedger.ImmutableLedgerAt(n, cacheSize)
	if xerr != nil {
		return nil, xerr
	}

	return simledger, nil

	//return &FinalityLedger[T]{
	//	SimpleLedger:  *simledger,
	//	finalityItems: newMemItems[T](),
	//}, nil
}

func (ledger *FinalityLedger[T]) SetFinality(item T) xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	ledger.finalityItems.setUpdatedItem(item)
	ledger.finalityItems.setGotItem(item)
	return nil
}

func (ledger *FinalityLedger[T]) CancelSetFinality(key LedgerKey) xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	ledger.finalityItems.delUpdatedItem(key)
	ledger.finalityItems.delGotItem(key)
	return nil
}

func (ledger *FinalityLedger[T]) GetFinality(key LedgerKey) (T, xerrors.XError) {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	return ledger.getFinality(key)
}

func (ledger *FinalityLedger[T]) getFinality(key LedgerKey) (T, xerrors.XError) {
	var emptyNil T

	// if the item is already removed, return xerrors.ErrNotFoundResult
	if ledger.finalityItems.isRemovedKey(key) {
		return emptyNil, xerrors.ErrNotFoundResult
	}

	if item, ok := ledger.finalityItems.getGotItem(key); ok {
		return item, nil
	}

	if item, xerr := ledger.read(key); xerr != nil {
		return emptyNil, xerr
	} else {
		ledger.finalityItems.setGotItem(item)
		return item, nil
	}
}

func (ledger *FinalityLedger[T]) DelFinality(key LedgerKey) (T, xerrors.XError) {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	// in case of deleting,
	// it should be removed from SimpleLedger too.
	_, _ = ledger.SimpleLedger.del(key)

	var emptyNil T

	if item, err := ledger.getFinality(key); err != nil {
		return emptyNil, err
	} else {
		ledger.finalityItems.delGotItem(key)       // delete(ledger.gotItems, key)
		ledger.finalityItems.delUpdatedItem(key)   // delete(ledger.updatedItems, key)
		ledger.finalityItems.appendRemovedKey(key) // ledger.removedKeys = append(ledger.removedKeys, key)

		return item, nil
	}
}

func (ledger *FinalityLedger[T]) CancelDelFinality(key LedgerKey) xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	ledger.finalityItems.delRemovedKey(key)
	return nil
}

func (ledger *FinalityLedger[T]) IterateReadAllFinalityItems(cb func(T) xerrors.XError) xerrors.XError {
	return ledger.SimpleLedger.IterateReadAllItems(cb)
}

func (ledger *FinalityLedger[T]) IterateFinalityGotItems(cb func(T) xerrors.XError) xerrors.XError {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	return iterateItems(ledger.finalityItems.gotItems, cb)
}

func (ledger *FinalityLedger[T]) IterateFinalityUpdatedItems(cb func(T) xerrors.XError) xerrors.XError {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	return iterateItems(ledger.finalityItems.updatedItems, cb)
}

func (ledger *FinalityLedger[T]) Commit() ([]byte, int64, xerrors.XError) {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	// remove
	for _, k := range ledger.finalityItems.removedKeys {
		var vk LedgerKey
		copy(vk[:], k[:])
		if _, _, err := ledger.tree.Remove(vk[:]); err != nil {
			return nil, -1, xerrors.From(err)
		}

		// issue #58
		// If SetFinality is called again (recreated) after calling DelFinality for the item,
		// the item's key may exist not only `removedKeys` but also `updatedItems`.
		// this item is removed at here but should be added again at the following code to update tree.
		// Thus, the operation for `removeKeys` MUST be run prior to operation for `updatedItems`.

		//delete(ledger.finalityItems.gotItems, vk)
		//delete(ledger.finalityItems.updatedItems, vk)
		//
		//// this item should be removed from SimpleLedger too.
		//delete(ledger.SimpleLedger.cachedItems.gotItems, vk)
		//delete(ledger.SimpleLedger.cachedItems.updatedItems, vk)
	}

	// update
	var keys LedgerKeyList
	for k, _ := range ledger.finalityItems.updatedItems {
		keys = append(keys, k)
	}
	// root hash may be different by update order
	sort.Sort(keys)
	for _, k := range keys {
		_val := ledger.finalityItems.updatedItems[k]
		_key := _val.Key()
		if bz, err := _val.Encode(); err != nil {
			return nil, -1, err
		} else if _, err := ledger.tree.Set(_key[:], bz); err != nil {
			return nil, -1, xerrors.From(err)
		}
	}

	if r1, r2, err := ledger.tree.SaveVersion(); err != nil {
		return r1, r2, xerrors.From(err)
	} else {
		ledger.SimpleLedger.cachedItems.reset()
		ledger.finalityItems.refresh()
		return r1, r2, nil
	}
}

var _ IFinalityLedger[ILedgerItem] = (*FinalityLedger[ILedgerItem])(nil)
