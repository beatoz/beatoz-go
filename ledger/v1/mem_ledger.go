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
	if xerr := item.Decode(key, bz); xerr != nil {
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
		ledger.logger.Debug("MemLedger.immuTree.Get returns nil", "key", key)
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
			if xerr := item.Decode(key, value); xerr != nil {
				xerrStop = xerr
				return true // stop
			}

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
	ledger.logger.Debug("MemLedger.immuTree in Iterate is nil")
	return xerrors.ErrNotFoundResult
}

func (ledger *MemLedger) Seek(prefix []byte, ascending bool, cb FuncIterate) xerrors.XError {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	if ledger.immuTree == nil {
		ledger.logger.Debug("MemLedger.immuTree is nil in Seek", "prefix", prefix, "ascending", ascending)
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
		if xerr := item.Decode(key, value); xerr != nil {
			return xerr
		}

		if xerr := cb(key, item); xerr != nil {
			return xerr
		}
	}

	return nil
}

func (ledger *MemLedger) Set(key LedgerKey, item ILedgerItem) xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	newVal, xerr := item.Encode()
	if xerr != nil {
		return xerr
	}
	ledger.memStorage[unsafe.String(&key[0], len(key))] = newVal
	return nil
}

func (ledger *MemLedger) Del(key LedgerKey) xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	ledger.memStorage[unsafe.String(&key[0], len(key))] = nil
	return nil
}
