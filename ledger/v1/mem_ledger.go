package v1

import (
	"bytes"
	"fmt"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/cosmos/iavl"
	tmlog "github.com/tendermint/tendermint/libs/log"
	"sync"
	"unsafe"
)

// MemLedger cannot be committed, everything else is like MutableLedger.
type MemLedger struct {
	immuTree    *iavl.ImmutableTree
	items       map[string]ILedgerItem
	revisions   *revisionList[ILedgerItem]
	newItemFunc func() ILedgerItem
	logger      tmlog.Logger
	mtx         sync.RWMutex
}

var _ IImitable = (*MemLedger)(nil)

func NewMemLedgerAt(ver int64, from IMutable, lg tmlog.Logger) (*MemLedger, xerrors.XError) {
	var tree *iavl.ImmutableTree
	if ver > 0 {
		_tree, xerr := from.GetReadOnlyTree(ver)
		if xerr != nil {
			return nil, xerr
		}
		tree = _tree
	}

	return &MemLedger{
		immuTree:    tree,
		items:       make(map[string]ILedgerItem),
		revisions:   newSnapshotList[ILedgerItem](),
		newItemFunc: from.(*MutableLedger).newItemFunc,
		logger:      lg.With("ledger", "MemLedger"),
	}, nil
}

func (ledger *MemLedger) Get(key LedgerKey) (ILedgerItem, xerrors.XError) {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	return ledger.get(key)
}

func (ledger *MemLedger) get(key LedgerKey) (ILedgerItem, xerrors.XError) {
	item, ok := ledger.items[unsafe.String(&key[0], len(key))]
	if ok {
		if item != nil {
			return item, nil
		} else {
			// the `item` maybe deleted on MemLedger.
			return nil, xerrors.ErrNotFoundResult
		}
	}

	if ledger.immuTree != nil {
		bz, err := ledger.immuTree.Get(key)
		if err != nil {
			return nil, xerrors.From(err)
		} else if bz == nil {
			return nil, xerrors.ErrNotFoundResult
		}

		item = ledger.newItemFunc()
		if xerr := item.Decode(bz); xerr != nil {
			return nil, xerr
		} else if bytes.Compare(item.Key(), key) != 0 {
			return nil, xerrors.From(fmt.Errorf("MemLedger: the key is compromised - the requested key(%x) is not equal to the key(%x) decoded in value", key, item.Key()))
		}

		ledger.items[unsafe.String(&key[0], len(key))] = item
	} else {
		return nil, xerrors.ErrNotFoundResult
	}
	return item, nil

}

func (ledger *MemLedger) Iterate(cb func(ILedgerItem) xerrors.XError) xerrors.XError {
	if ledger.immuTree != nil {
		ledger.mtx.RLock()
		defer ledger.mtx.RUnlock()

		var xerrStop xerrors.XError
		stopped, err := ledger.immuTree.Iterate(func(key []byte, value []byte) bool {
			item, ok := ledger.items[unsafe.String(&key[0], len(key))]
			if ok && item == nil {
				// item maybe deleted.
				return false // continue iteration
			}
			if !ok || item == nil {
				item = ledger.newItemFunc()
				if xerr := item.Decode(value); xerr != nil {
					xerrStop = xerr
					return true // stop
				} else if bytes.Compare(item.Key(), key) != 0 {
					xerrStop = xerrors.From(fmt.Errorf("MemLedger: the key is compromised - the requested key(%x) is not equal to the key(%x) decoded in value", key, item.Key()))
					return true // stop
				}
			}

			ledger.mtx.RUnlock()
			defer ledger.mtx.RLock()

			if xerr := cb(item); xerr != nil {
				xerrStop = xerr
				return true // stop
			}
			return false // continue iteration
		})

		if err != nil {
			return xerrors.From(err)
		} else if stopped {
			return xerrStop
		}

		return nil
	}

	return xerrors.ErrNotFoundResult
}

func (ledger *MemLedger) Set(item ILedgerItem) xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	_key := item.Key()
	_keyStr := unsafe.String(&_key[0], len(_key))

	oldItem, ok := ledger.items[_keyStr]
	ledger.items[_keyStr] = item

	if ok {
		// exists.
		// when reverting, it should be restored as 'old'
		ledger.revisions.set(_key, oldItem)
	} else {
		// not exists.
		// when reverting, it should be removed.
		ledger.revisions.set(_key, nil)
	}

	return nil
}

func (ledger *MemLedger) Del(key LedgerKey) xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	if oldItem, ok := ledger.items[unsafe.String(&key[0], len(key))]; ok {
		ledger.items[unsafe.String(&key[0], len(key))] = nil
		ledger.revisions.set(key, oldItem)
	}
	return nil
}

func (ledger *MemLedger) Snapshot() int {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	return ledger.revisions.snapshot()
}

func (ledger *MemLedger) RevertToSnapshot(snap int) xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	restores := ledger.revisions.revs[snap:]
	for i := len(restores) - 1; i >= 0; i-- {
		kv := restores[i]
		if kv.val != nil {
			_item := kv.val.(ILedgerItem)
			_key := _item.Key()
			ledger.items[unsafe.String(&_key[0], len(_key))] = _item
		} else {
			_key := kv.key
			ledger.items[unsafe.String(&_key[0], len(_key))] = nil
		}
	}
	ledger.revisions.revert(snap)
	return nil
}
