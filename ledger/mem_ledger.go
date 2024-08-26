package ledger

import (
	"github.com/beatoz/beatoz-go/types/xerrors"
	"sync"
)

type MemLedger[T ILedgerItem] struct {
	memStorage  map[LedgerKey]T
	cachedItems *memItems[T]
	getNewItem  func() T

	mtx sync.RWMutex
}

func NewMemLedger[T ILedgerItem](name, cb func() T) (*MemLedger[T], xerrors.XError) {
	return &MemLedger[T]{
		memStorage:  make(map[LedgerKey]T),
		cachedItems: newMemItems[T](),
		getNewItem:  cb,
	}, nil
}

func (ledger *MemLedger[T]) Version() int64 {
	return 0
}

func (ledger *MemLedger[T]) Set(item T) xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	ledger.cachedItems.setUpdatedItem(item)
	return nil
}

func (ledger *MemLedger[T]) CancelSet(key LedgerKey) xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	ledger.cachedItems.delUpdatedItem(key)
	return nil
}

func (ledger *MemLedger[T]) Get(key LedgerKey) (T, xerrors.XError) {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	return ledger.get(key)
}

func (ledger *MemLedger[T]) get(key LedgerKey) (T, xerrors.XError) {
	// search in cachedItems
	if item, ok := ledger.cachedItems.getGotItem(key); ok {
		return item, nil
	}

	// search in storage in memory
	if item, xerr := ledger.read(key); xerr != nil {
		return item, xerr
	} else {
		// caching
		ledger.cachedItems.setGotItem(item)
		return item, nil
	}
}

func (ledger *MemLedger[T]) Del(key LedgerKey) (T, xerrors.XError) {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	var emptyNil T

	if item, err := ledger.get(key); err != nil {
		return emptyNil, err
	} else {
		ledger.cachedItems.delGotItem(key)       // delete(ledger.gotItems, key)
		ledger.cachedItems.delUpdatedItem(key)   // delete(ledger.updatedItems, key)
		ledger.cachedItems.appendRemovedKey(key) // ledger.removedKeys = append(ledger.removedKeys, key)
		return item, nil
	}
}

func (ledger *MemLedger[T]) CancelDel(key LedgerKey) xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	ledger.cachedItems.delRemovedKey(key)
	return nil
}

func (ledger *MemLedger[T]) IterateReadAllItems(cb func(T) xerrors.XError) xerrors.XError {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	return iterateItems(ledger.memStorage, cb)
}

func (ledger *MemLedger[T]) IterateGotItems(cb func(T) xerrors.XError) xerrors.XError {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	return iterateItems(ledger.cachedItems.gotItems, cb)
}

func (ledger *MemLedger[T]) IterateUpdatedItems(cb func(T) xerrors.XError) xerrors.XError {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	return iterateItems(ledger.cachedItems.updatedItems, cb)
}

func (ledger *MemLedger[T]) Read(key LedgerKey) (T, xerrors.XError) {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	// search in storage in memory
	var emptyNil T
	if item, xerr := ledger.read(key); xerr != nil {
		return emptyNil, xerr
	} else {
		// caching
		ledger.cachedItems.setGotItem(item)
		return item, nil
	}
}

func (ledger *MemLedger[T]) read(key LedgerKey) (T, xerrors.XError) {
	var emptyNil T
	if item, ok := ledger.memStorage[key]; !ok {
		return item, xerrors.NewOrdinary("simple_ledger: not found")
	} else if key != item.Key() {
		return emptyNil, xerrors.NewOrdinary("simple_ledger: the key is compromised - the requested key is not equal to the key encoded in value")
	} else {
		return item, nil
	}
}

func (ledger *MemLedger[T]) Commit() ([]byte, int64, xerrors.XError) {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	for key, item := range ledger.cachedItems.updatedItems {
		ledger.memStorage[key] = item
	}

	for _, key := range ledger.cachedItems.removedKeys {
		var vk LedgerKey
		copy(vk[:], key[:])
		delete(ledger.memStorage, vk)
	}

	ledger.cachedItems.reset()
	return nil, 0, nil
}

func (ledger *MemLedger[T]) Clone() ILedger[T] {
	return &MemLedger[T]{
		memStorage:  ledger.memStorage,
		cachedItems: newMemItems[T](),
		getNewItem:  ledger.getNewItem,
	}
}

func (ledger *MemLedger[T]) Close() xerrors.XError {
	ledger.memStorage = nil
	ledger.cachedItems.reset()
	return nil
}

var _ ILedger[ILedgerItem] = (*MemLedger[ILedgerItem])(nil)
