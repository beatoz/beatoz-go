package v1

import (
	"bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/cosmos/iavl"
	dbm "github.com/cosmos/iavl/db"
	tmlog "github.com/tendermint/tendermint/libs/log"
	"sync"
	"unsafe"
)

type MutableLedger struct {
	db         dbm.DB
	tree       *iavl.MutableTree
	revisions  *revisionList[[]byte]
	cachedObjs map[string]ILedgerItem

	newItemFor FuncNewItemFor
	cacheSize  int

	logger tmlog.Logger
	mtx    sync.RWMutex
}

func NewMutableLedger(name, dbDir string, cacheSize int, newItem FuncNewItemFor, lg tmlog.Logger) (*MutableLedger, xerrors.XError) {
	db, err := dbm.NewGoLevelDB(name, dbDir)
	if err != nil {
		return nil, xerrors.Wrap(err, "goleveldb open failed")
	}

	tree := iavl.NewMutableTree(db, cacheSize, false, iavl.NewNopLogger(), iavl.SyncOption(true))
	if _, err := tree.LoadVersion(0); err != nil {
		_ = tree.Close()
		return nil, xerrors.Wrap(err, "tree's LoadVersion failed")
	}

	return &MutableLedger{
		db:         db,
		tree:       tree,
		revisions:  newSnapshotList[[]byte](),
		cachedObjs: make(map[string]ILedgerItem),
		newItemFor: newItem,
		cacheSize:  cacheSize,
		logger:     lg.With("ledger", "MutableLedger"),
	}, nil
}

func (ledger *MutableLedger) Get(key LedgerKey) (ILedgerItem, xerrors.XError) {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	keystr := unsafe.String(&key[0], len(key))
	if obj, ok := ledger.cachedObjs[keystr]; ok {
		return obj, nil
	}

	item, xerr := ledger.get(key)
	if xerr != nil {
		return nil, xerr
	}
	ledger.cachedObjs[keystr] = item
	return item, nil
}

func (ledger *MutableLedger) get(key LedgerKey) (ILedgerItem, xerrors.XError) {
	if bz, err := ledger.tree.Get(key); err != nil {
		return nil, xerrors.From(err)
	} else if bz == nil {
		return nil, xerrors.ErrNotFoundResult
	} else {
		item := ledger.newItemFor(key)
		if xerr := item.Decode(bz); xerr != nil {
			return nil, xerr
		}
		return item, nil
	}
}

// Iterate do not travel the elements not committed.
func (ledger *MutableLedger) Iterate(cb FuncIterate) xerrors.XError {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	var xerrStop xerrors.XError
	stopped, err := ledger.tree.Iterate(func(key []byte, value []byte) bool {
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

func (ledger *MutableLedger) Seek(prefix []byte, ascending bool, cb FuncIterate) xerrors.XError {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	iter, err := ledger.tree.Iterator(prefix, nil, ascending)
	if err != nil {
		return xerrors.From(err)
	}
	defer func() {
		_ = iter.Close()
	}()

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

func (ledger *MutableLedger) Set(key LedgerKey, item ILedgerItem) xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	if xerr := ledger.set(key, item); xerr != nil {
		return xerr
	}

	ledger.cachedObjs[unsafe.String(&key[0], len(key))] = item
	return nil
}

func (ledger *MutableLedger) set(key LedgerKey, item ILedgerItem) xerrors.XError {
	oldVal, err := ledger.tree.Get(key)
	if err != nil {
		return xerrors.From(err)
	}
	newVal, xerr := item.Encode()
	if xerr != nil {
		return xerr
	}

	_, err = ledger.tree.Set(key, newVal)
	if err != nil {
		return xerrors.From(err)
	}

	ledger.logger.Debug("set item to tree", "key", key, "oldVal", oldVal, "newVal", newVal)

	if bytes.Compare(oldVal, newVal) != 0 {
		// if `oldVal` is `nil`, it means that the item is created, and it should be removed in reverting.
		// if `oldVal` is not equal to `newVal`, it means that the item is updated, and `oldVal` will be restored in reverting.
		ledger.revisions.set(key, oldVal)
	}
	return nil
}

func (ledger *MutableLedger) Del(key LedgerKey) xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	oldVal, removed, err := ledger.tree.Remove(key)
	if err != nil {
		return xerrors.From(err)
	}
	ledger.logger.Debug("delete item from tree", "key", key, "value", oldVal, "removed", removed)

	//if oldVal != nil && removed {
	if removed {
		// In reverting, `oldVal` will be restored.
		ledger.revisions.set(key, oldVal)
	}

	delete(ledger.cachedObjs, unsafe.String(&key[0], len(key)))
	return nil
}

func (ledger *MutableLedger) Snapshot() int {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	return ledger.revisions.snapshot()
}

func (ledger *MutableLedger) RevertToSnapshot(snap int) xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	restores := ledger.revisions.revs[snap:]
	for i := len(restores) - 1; i >= 0; i-- {
		kv := restores[i]
		if kv.val != nil {
			if _, err := ledger.tree.Set(kv.key, kv.val); err != nil {
				return xerrors.From(err)
			}
			restoreItem := ledger.newItemFor(kv.key)
			if xerr := restoreItem.Decode(kv.val); xerr != nil {
				return xerr
			}
			ledger.cachedObjs[unsafe.String(&kv.key[0], len(kv.key))] = restoreItem
		} else {
			if _, _, err := ledger.tree.Remove(kv.key); err != nil {
				return xerrors.From(err)
			}
			delete(ledger.cachedObjs, unsafe.String(&kv.key[0], len(kv.key)))
		}
	}
	ledger.revisions.revert(snap)
	return nil
}

func (ledger *MutableLedger) Commit() ([]byte, int64, xerrors.XError) {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	ledger.tree.SetCommitting()
	defer ledger.tree.UnsetCommitting()

	r1, r2, err := ledger.tree.SaveVersion()
	if err != nil {
		return r1, r2, xerrors.From(err)
	}

	ledger.logger.Debug("tree save version", "hash", r1, "version", r2)

	ledger.revisions.reset()
	ledger.cachedObjs = make(map[string]ILedgerItem)
	return r1, r2, nil
}

func (ledger *MutableLedger) Version() int64 {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	return ledger.tree.Version()
}

func (ledger *MutableLedger) GetReadOnlyTree(ver int64) (*iavl.ImmutableTree, xerrors.XError) {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	tree, err := ledger.tree.GetImmutable(ver)
	if err != nil {
		return nil, xerrors.From(err)
	}
	return tree, nil
}

func (ledger *MutableLedger) Close() xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	if ledger.tree != nil {
		if err := ledger.tree.Close(); err != nil {
			return xerrors.From(err)
		}
	}
	ledger.tree = nil

	if ledger.db != nil {
		if err := ledger.db.Close(); err != nil {
			return xerrors.From(err)
		}
	}
	ledger.db = nil

	ledger.revisions.reset()

	return nil
}

var _ IMutable = (*MutableLedger)(nil)
