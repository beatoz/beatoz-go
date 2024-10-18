package v1

import (
	"bytes"
	"fmt"
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

	newItemFunc func() ILedgerItem
	cacheSize   int

	logger tmlog.Logger
	mtx    sync.RWMutex
}

func NewMutableLedger(name, dbDir string, cacheSize int, newItem func() ILedgerItem, lg tmlog.Logger) (*MutableLedger, xerrors.XError) {
	db, err := dbm.NewGoLevelDB(name, dbDir)
	if err != nil {
		return nil, xerrors.From(err)
	}

	tree := iavl.NewMutableTree(db, cacheSize, false, iavl.NewNopLogger(), iavl.SyncOption(true))
	if _, err := tree.LoadVersion(0); err != nil {
		_ = tree.Close()
		return nil, xerrors.From(err)
	}

	return &MutableLedger{
		db:          db,
		tree:        tree,
		revisions:   newSnapshotList[[]byte](),
		cachedObjs:  make(map[string]ILedgerItem),
		newItemFunc: newItem,
		cacheSize:   cacheSize,
		logger:      lg.With("ledger", "MutableLedger"),
	}, nil
}

func (ledger *MutableLedger) Get(key LedgerKey) (ILedgerItem, xerrors.XError) {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	_keystr := unsafe.String(&key[0], len(key))
	if obj, ok := ledger.cachedObjs[_keystr]; ok {
		return obj, nil
	}

	item, xerr := ledger.get(key)
	if xerr != nil {
		return nil, xerr
	}
	ledger.cachedObjs[_keystr] = item
	return item, nil
}

func (ledger *MutableLedger) get(key LedgerKey) (ILedgerItem, xerrors.XError) {
	if bz, err := ledger.tree.Get(key); err != nil {
		return nil, xerrors.From(err)
	} else if bz == nil {
		return nil, xerrors.ErrNotFoundResult
	} else {
		item := ledger.newItemFunc()
		if xerr := item.Decode(bz); xerr != nil {
			return nil, xerr
		} else if bytes.Compare(item.Key(), key) != 0 {
			return nil, xerrors.From(fmt.Errorf("MutableLedger: the key is compromised - the requested key(%x) is not equal to the key(%x) decoded in value", key, item.Key()))
		}
		return item, nil
	}
}

func (ledger *MutableLedger) Iterate(cb func(ILedgerItem) xerrors.XError) xerrors.XError {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	var xerrStop xerrors.XError
	stopped, err := ledger.tree.Iterate(func(key []byte, value []byte) bool {
		item, ok := ledger.cachedObjs[unsafe.String(&key[0], len(key))]
		if !ok {
			item = ledger.newItemFunc()
			if xerr := item.Decode(value); xerr != nil {
				xerrStop = xerr
				return true // stop
			} else if bytes.Compare(item.Key(), key) != 0 {
				xerrStop = xerrors.From(fmt.Errorf("MutableLedger: the key is compromised - the requested key(%x) is not equal to the key(%x) decoded in value", key, item.Key()))
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

func (ledger *MutableLedger) Set(item ILedgerItem) xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	if xerr := ledger.set(item); xerr != nil {
		return xerr
	}

	_key := item.Key()
	ledger.cachedObjs[unsafe.String(&_key[0], len(_key))] = item
	return nil
}

func (ledger *MutableLedger) set(item ILedgerItem) xerrors.XError {
	key := item.Key()
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

	if oldVal == nil || bytes.Compare(oldVal, newVal) != 0 {
		// if `oldVal` is `nil`, it means that the item is created, and it should be removed in reverting.
		// if `oldVal` is not equal to `newVal`, it means that the item is updated, and `oldVal` will be restored in reverting.
		ledger.revisions.set(key, oldVal)
	}
	return nil
}

func (ledger *MutableLedger) Del(key LedgerKey) xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	if oldVal, removed, err := ledger.tree.Remove(key); err != nil {
		return xerrors.From(err)
	} else if oldVal != nil && removed {
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
			restoreItem := ledger.newItemFunc()
			if xerr := restoreItem.Decode(kv.val); xerr != nil {
				return xerr
			} else if bytes.Compare(restoreItem.Key(), kv.key) != 0 {
				return xerrors.From(fmt.Errorf("MutableLedger: the key is compromised - the requested key(%x) is not equal to the key(%x) decoded in value", kv.key, restoreItem.Key()))
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

	if r1, r2, err := ledger.tree.SaveVersion(); err != nil {
		return r1, r2, xerrors.From(err)
	} else {
		ledger.revisions.reset()
		ledger.cachedObjs = make(map[string]ILedgerItem)
		return r1, r2, nil
	}
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
	if ledger.db != nil {
		if err := ledger.db.Close(); err != nil {
			return xerrors.From(err)
		}
	}
	ledger.db = nil

	if ledger.tree != nil {
		if err := ledger.tree.Close(); err != nil {
			return xerrors.From(err)
		}
	}
	ledger.tree = nil

	ledger.revisions.reset()

	return nil
}

var _ IMutable = (*MutableLedger)(nil)
