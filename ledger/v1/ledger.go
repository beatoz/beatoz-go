package v1

import (
	"bytes"
	"fmt"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/cosmos/iavl"
	dbm "github.com/cosmos/iavl/db"
	"sync"
)

type Ledger struct {
	*iavl.MutableTree
	db          dbm.DB
	cacheSize   int
	newItemFunc func() ILedgerItem

	revisions []*kvRevision

	mtx sync.RWMutex
}

func NewLedger(name, dbDir string, cacheSize int, newItem func() ILedgerItem) (*Ledger, xerrors.XError) {
	db, err := dbm.NewGoLevelDB(name, dbDir)
	if err != nil {
		return nil, xerrors.From(err)
	}

	tree := iavl.NewMutableTree(db, cacheSize, false, iavl.NewNopLogger(), func(opt *iavl.Options) { opt.Sync = true })
	if _, err := tree.Load(); err != nil {
		_ = db.Close()
		return nil, xerrors.From(err)
	}

	return &Ledger{
		MutableTree: tree,
		db:          db,
		cacheSize:   cacheSize,
		newItemFunc: newItem,
		revisions:   make([]*kvRevision, 0),
	}, nil
}

func (ledger *Ledger) DB() dbm.DB {
	return ledger.db
}

func (ledger *Ledger) CacheSize() int {
	return ledger.cacheSize
}

func (ledger *Ledger) NewItemFunc() func() ILedgerItem {
	return ledger.newItemFunc
}

func (ledger *Ledger) Version() int64 {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	return ledger.MutableTree.Version()
}

func (ledger *Ledger) Set(item ILedgerItem) xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	key := item.Key()
	val, xerr := item.Encode()
	if xerr != nil {
		return xerr
	}

	ledger.revisionForSet(key[:], val)
	return nil
}

func (ledger *Ledger) Get(key LedgerKey) (ILedgerItem, xerrors.XError) {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	var bzVal []byte
	if rev := ledger.findRevision(key); rev != nil {
		bzVal = rev.Value
	} else if bz, err := ledger.ImmutableTree.Get(key[:]); err != nil {
		return nil, xerrors.From(err)
	} else if bz == nil {
		return nil, xerrors.ErrNotFoundResult
	} else {
		bzVal = bz
	}

	item := ledger.newItemFunc()
	if xerr := item.Decode(bzVal); xerr != nil {
		return nil, xerr
	} else if bytes.Compare(item.Key(), key) != 0 {
		return nil, xerrors.NewOrdinary("Ledger: the key is compromised - the requested key is not equal to the key encoded in value")
	}

	return item, nil
}

func (ledger *Ledger) Del(key LedgerKey) {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	ledger.revisionForDel(key)
}

func (ledger *Ledger) Iterate(cb func(ILedgerItem) xerrors.XError) xerrors.XError {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	var xerrStop xerrors.XError
	stopped, err := ledger.MutableTree.Iterate(func(key []byte, value []byte) bool {
		item := ledger.newItemFunc()
		if xerr := item.Decode(value); xerr != nil {
			xerrStop = xerrors.NewOrdinary(fmt.Sprintf("item decode error - Key:%X vs. err:%v", key, xerr))
			return true
		} else if bytes.Compare(item.Key(), key) != 0 {
			xerrStop = xerrors.NewOrdinary(fmt.Sprintf("wrong key - Key:%X vs. stake's txhash:%X", key, item.Key()))
			return true
		} else if xerr := cb(item); xerr != nil {
			xerrStop = xerr
			return true
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

func (ledger *Ledger) Commit() ([]byte, int64, xerrors.XError) {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	ledger.MutableTree.SetCommitting()
	defer ledger.MutableTree.UnsetCommitting()

	// apply revisions
	if xerr := ledger.applyRevisions(); xerr != nil {
		return nil, 0, xerr
	}

	if r1, r2, err := ledger.MutableTree.SaveVersion(); err != nil {
		return r1, r2, xerrors.From(err)
	} else {
		return r1, r2, nil
	}
}

func (ledger *Ledger) Close() xerrors.XError {
	if ledger.db != nil {
		if err := ledger.db.Close(); err != nil {
			return xerrors.From(err)
		}
	}
	ledger.db = nil

	if ledger.MutableTree != nil {
		if err := ledger.MutableTree.Close(); err != nil {
			return xerrors.From(err)
		}
	}
	ledger.MutableTree = nil
	return nil
}

func (ledger *Ledger) ImmutableLedgerAt(ver int64) (ILedger, xerrors.XError) {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	immuTree, err := ledger.MutableTree.GetImmutable(ver)
	if err != nil {
		return nil, xerrors.From(err)
	}

	_ledger := newImmutableLedger(immuTree, ledger.newItemFunc)
	return _ledger, nil
}

func (ledger *Ledger) MempoolLedgerAt(ver int64) (ILedger, xerrors.XError) {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	return NewMempoolLedger(ledger.db, ledger.cacheSize, ledger.newItemFunc, ver)
}

var _ ILedger = (*Ledger)(nil)
