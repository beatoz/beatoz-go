package v1

import (
	"bytes"
	"fmt"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/cosmos/iavl"
	dbm "github.com/cosmos/iavl/db"
	tmlog "github.com/tendermint/tendermint/libs/log"
	"sync"
)

type MutableLedger struct {
	db          dbm.DB
	tree        *iavl.MutableTree
	cacheSize   int
	newItemFunc func() ILedgerItem

	snapshots *snapshotList

	logger tmlog.Logger
	mtx    sync.RWMutex
}

func NewMutableLedger(name, dbDir string, cacheSize int, newItem func() ILedgerItem, lg tmlog.Logger) (*MutableLedger, xerrors.XError) {
	db, err := dbm.NewGoLevelDB(name, dbDir)
	if err != nil {
		return nil, xerrors.From(err)
	}

	tree := iavl.NewMutableTree(db, cacheSize, false, iavl.NewNopLogger(), iavl.SyncOption(true))

	if _, err := tree.Load(); err != nil {
		_ = db.Close()
		return nil, xerrors.From(err)
	}

	return &MutableLedger{
		tree:        tree,
		db:          db,
		cacheSize:   cacheSize,
		newItemFunc: newItem,
		snapshots:   NewSnapshotList(),
		logger:      lg,
	}, nil
}

func (ledger *MutableLedger) DB() dbm.DB {
	return ledger.db
}

func (ledger *MutableLedger) CacheSize() int {
	return ledger.cacheSize
}

func (ledger *MutableLedger) NewItemFunc() func() ILedgerItem {
	return ledger.newItemFunc
}

func (ledger *MutableLedger) Version() int64 {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	return ledger.tree.Version()
}

func (ledger *MutableLedger) Get(key LedgerKey) (ILedgerItem, xerrors.XError) {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	return ledger.get(key)
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

func (ledger *MutableLedger) Set(item ILedgerItem) xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	return ledger.set(item)
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
		// if `oldVal` is `nil`, it will be removed in reverting.
		// if `oldVal` is not equal to `newVal`, `oldVal` will be restored in reverting.
		ledger.snapshots.set(key, oldVal)
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
		ledger.snapshots.set(key, oldVal)
	}
	return nil
}

func (ledger *MutableLedger) Iterate(cb func(ILedgerItem) xerrors.XError) xerrors.XError {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	var xerrStop xerrors.XError
	stopped, err := ledger.tree.Iterate(func(key []byte, value []byte) bool {
		item := ledger.newItemFunc()
		if xerr := item.Decode(value); xerr != nil {
			xerrStop = xerr
			return true // stop
		} else if bytes.Compare(item.Key(), key) != 0 {
			xerrStop = xerrors.From(fmt.Errorf("MutableLedger: the key is compromised - the requested key(%x) is not equal to the key(%x) decoded in value", key, item.Key()))
			return true // stop
		}

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

func (ledger *MutableLedger) Commit() ([]byte, int64, xerrors.XError) {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	ledger.tree.SetCommitting()
	defer ledger.tree.UnsetCommitting()

	if r1, r2, err := ledger.tree.SaveVersion(); err != nil {
		return r1, r2, xerrors.From(err)
	} else {
		ledger.snapshots.reset()
		return r1, r2, nil
	}
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

	ledger.snapshots.reset()

	return nil
}

func (ledger *MutableLedger) Snapshot() int {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	return ledger.snapshots.snapshot()
}

func (ledger *MutableLedger) RevertToSnapshot(snap int) xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	restores := ledger.snapshots.revisions[snap:]
	for i := len(restores) - 1; i >= 0; i-- {
		item := restores[i]
		if item.val != nil {
			if _, err := ledger.tree.Set(item.key, item.val); err != nil {
				return xerrors.From(err)
			}
		} else {
			if _, _, err := ledger.tree.Remove(item.key); err != nil {
				return xerrors.From(err)
			}
		}
	}
	ledger.snapshots.revert(snap)
	return nil
}

//func (ledger *MutableLedger) ImmutableLedgerAt(ver int64) (ILedger, xerrors.XError) {
//	ledger.mtx.RLock()
//	defer ledger.mtx.RUnlock()
//
//	immuTree, err := ledger.tree.GetImmutable(ver)
//	if err != nil {
//		return nil, xerrors.From(err)
//	}
//
//	_ledger := newImmutableLedger(immuTree, ledger.newItemFunc, ledger.logger)
//	return _ledger, nil
//}
//
//func (ledger *MutableLedger) MempoolLedgerAt(ver int64) (ILedger, xerrors.XError) {
//	ledger.mtx.RLock()
//	defer ledger.mtx.RUnlock()
//
//	return NewMempoolLedger(ledger.db, ledger.cacheSize, ledger.newItemFunc, ledger.logger, ver)
//}

var _ ILedger = (*MutableLedger)(nil)
