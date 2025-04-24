package v1

import (
	"bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/cosmos/iavl"
	tmlog "github.com/tendermint/tendermint/libs/log"
	"sync"
	"unsafe"
)

// MemLedger cannot be committed, everything else is like MutableLedger.

type MemLedger struct {
	immuTree   *iavl.ImmutableTree
	memStorage map[string][]byte
	revisions  *revisionList[[]byte]
	newItemFor FuncNewItemFor
	logger     tmlog.Logger
	mtx        sync.RWMutex
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
		immuTree:   tree,
		memStorage: make(map[string][]byte),
		revisions:  newSnapshotList[[]byte](),
		newItemFor: from.(*MutableLedger).newItemFor,
		logger:     lg.With("ledger", "MemLedger"),
	}, nil
}

func (ledger *MemLedger) Get(key LedgerKey) (ILedgerItem, xerrors.XError) {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	return ledger.get(key)
}

func (ledger *MemLedger) get(key LedgerKey) (ILedgerItem, xerrors.XError) {
	bz, xerr := ledger.findRawBytes(key)
	if xerr != nil {
		return nil, xerr
	}

	item := ledger.newItemFor(key)
	if xerr := item.Decode(bz); xerr != nil {
		return nil, xerr
	}
	return item, nil
}

func (ledger *MemLedger) findRawBytes(key LedgerKey) ([]byte, xerrors.XError) {
	keystr := unsafe.String(&key[0], len(key))
	bz, ok := ledger.memStorage[keystr]
	if !ok {
		_bz, err := ledger.immuTree.Get(key)
		if err != nil {
			return nil, xerrors.From(err)
		}
		bz = _bz
	}

	if bz == nil {
		return nil, xerrors.ErrNotFoundResult
	}
	return bz, nil
}

// Iterate do not travel the elements created or updated by Set
func (ledger *MemLedger) Iterate(cb FuncIterate) xerrors.XError {
	if ledger.immuTree != nil {
		ledger.mtx.RLock()
		defer ledger.mtx.RUnlock()

		var xerrStop xerrors.XError
		stopped, err := ledger.immuTree.Iterate(func(key []byte, value []byte) bool {
			item := ledger.newItemFor(key)
			if xerr := item.Decode(value); xerr != nil {
				xerrStop = xerr
				return true // stop
			}

			//// todo: the following unlock code must not be allowed.
			//// this allows the callee to access the ledger's other method, which may update key or value of the tree.
			//// However, in iterating, the key and value MUST not updated.
			//ledger.mtx.RUnlock()
			//defer ledger.mtx.RLock()

			if xerr := cb(key, item); xerr != nil {
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

func (ledger *MemLedger) Seek(prefix []byte, ascending bool, cb FuncIterate) xerrors.XError {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	if ledger.immuTree == nil {
		return xerrors.ErrNotFoundResult
	}

	iter, err := ledger.immuTree.Iterator(prefix, nil, ascending)
	if err != nil {
		return xerrors.From(err)
	}

	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		if !bytes.HasPrefix(key, prefix) {
			break
		}

		value := iter.Value()
		item := ledger.newItemFor(key)
		if xerr := item.Decode(value); xerr != nil {
			return xerr
		}

		if xerr := cb(key, item); xerr != nil {
			return xerr
		}
	}

	return nil
}

// Set:
//   memStorage 에서 조회
//     없으면 immuTree 에서 조회
//       없으면 revisions 에 nil set
//     	 있으면 revisions 에 immuTree.value 를 set.
//     있으면 revisions 에 memStorage.value를 set.
//   memStroage 에 새로운 value set

func (ledger *MemLedger) Set(key LedgerKey, item ILedgerItem) xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	keystr := unsafe.String(&key[0], len(key))
	oldVal, ok := ledger.memStorage[keystr]
	if !ok {
		_old, err := ledger.immuTree.Get(key)
		if err != nil {
			return xerrors.From(err)
		}
		oldVal = _old // if _old is nil, it means deleted.
	}

	newVal, xerr := item.Encode()
	if xerr != nil {
		return xerr
	}

	if bytes.Compare(oldVal, newVal) != 0 {
		// if `oldVal` is `nil`, it means that the item is created, and it should be removed in reverting.
		// if `oldVal` is not equal to `newVal`, it means that the item is updated, and `oldVal` will be restored in reverting.
		ledger.revisions.set(key, oldVal)
	}

	ledger.memStorage[keystr] = newVal
	return nil
}

func (ledger *MemLedger) Del(key LedgerKey) xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	keystr := unsafe.String(&key[0], len(key))
	if oldVal, ok := ledger.memStorage[keystr]; ok {
		ledger.revisions.set(key, oldVal)
	}

	ledger.memStorage[keystr] = nil // delete from memory. don't read from immuTree.

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
		keystr := unsafe.String(&kv.key[0], len(kv.key))
		ledger.memStorage[keystr] = kv.val
	}
	ledger.revisions.revert(snap)
	return nil
}
