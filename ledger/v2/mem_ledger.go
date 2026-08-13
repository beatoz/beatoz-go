package v2

import (
	"bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/cosmos/iavl"
	tmlog "github.com/tendermint/tendermint/libs/log"
	"sync"
)

// MemLedger cannot be committed, everything else is like MutableLedger.

type MemLedger struct {
	immuTree *iavl.ImmutableTree

	memStorage map[string][]byte
	cache      *cacheContext

	newItemFor FuncNewItemFor
	logger     tmlog.Logger
	mtx        sync.RWMutex
}

func NewMemLedgerAt(ver int64, from IMutable, lg tmlog.Logger) (*MemLedger, xerrors.XError) {
	mutable, ok := from.(*MutableLedger)
	if !ok {
		return nil, xerrors.NewOrdinary("unsupported mutable ledger")
	}

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
		newItemFor: mutable.newItemFor,
		logger:     lg.With("ledger", "MemLedger"),
	}, nil
}

func (ledger *MemLedger) Get(key LedgerKey) (ILedgerItem, xerrors.XError) {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	item, xerr := ledger.get(key)
	if xerr != nil {
		return nil, xerr
	}
	if ledger.cache != nil {
		ledger.cache.set(key, item)
	}
	return item, nil
}

func (ledger *MemLedger) get(key LedgerKey) (ILedgerItem, xerrors.XError) {
	if item, found, xerr := ledger.cache.read(key); found {
		return item, xerr
	}
	if bz, found := ledger.memStorage[string(key)]; found {
		if bz == nil {
			return nil, xerrors.ErrNotFoundResult
		}
		return ledger.decode(key, bz)
	}

	if ledger.immuTree == nil {
		return nil, xerrors.ErrNotFoundResult
	}
	bz, err := ledger.immuTree.Get(key)
	if err != nil {
		return nil, xerrors.From(err)
	}
	if bz == nil {
		ledger.logger.Debug("MemLedger.immuTree.Get returns nil", "key", key)
		return nil, xerrors.ErrNotFoundResult
	}
	return ledger.decode(key, bz)
}

func (ledger *MemLedger) decode(key LedgerKey, bz []byte) (ILedgerItem, xerrors.XError) {
	item := ledger.newItemFor(key)
	if xerr := item.Decode(key, bz); xerr != nil {
		return nil, xerr
	}
	return item, nil
}

func (ledger *MemLedger) Iterate(cb FuncIterate) xerrors.XError {
	return ledger.Seek(nil, true, cb)
}

func (ledger *MemLedger) Seek(prefix []byte, ascending bool, cb FuncIterate) xerrors.XError {
	if cb == nil {
		return xerrors.NewOrdinary("iterate callback is nil")
	}

	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	if len(ledger.memStorage) == 0 && !ledger.cache.hasEntries() {
		return ledger.seekTree(prefix, ascending, cb)
	}
	keys, xerr := ledger.seekKeys(prefix, ascending)
	if xerr != nil {
		return xerr
	}
	for _, key := range keys {
		item, xerr := ledger.get(key)
		if xerr == xerrors.ErrNotFoundResult {
			continue
		}
		if xerr != nil {
			return xerr
		}
		if xerr := cb(key, item); xerr != nil {
			return xerr
		}
	}
	return nil
}

func (ledger *MemLedger) seekTree(prefix []byte, ascending bool, cb FuncIterate) xerrors.XError {
	if ledger.immuTree == nil {
		return nil
	}

	start := bytes.Clone(prefix)
	iter, err := ledger.immuTree.Iterator(start, nil, ascending)
	if err != nil {
		return xerrors.From(err)
	}

	for ; iter.Valid(); iter.Next() {
		iterKey := iter.Key()
		if !bytes.HasPrefix(iterKey, start) {
			if ascending {
				break
			}
			continue
		}

		key := bytes.Clone(iterKey)
		item, xerr := ledger.decode(key, iter.Value())
		if xerr != nil {
			_ = iter.Close()
			return xerr
		}
		if xerr := cb(key, item); xerr != nil {
			_ = iter.Close()
			return xerr
		}
	}
	if err := iter.Error(); err != nil {
		_ = iter.Close()
		return xerrors.From(err)
	}
	if err := iter.Close(); err != nil {
		return xerrors.From(err)
	}
	return nil
}

func (ledger *MemLedger) seekKeys(prefix []byte, ascending bool) ([][]byte, xerrors.XError) {
	keySet := make(map[string][]byte)

	if ledger.immuTree != nil {
		iter, err := ledger.immuTree.Iterator(prefix, nil, ascending)
		if err != nil {
			return nil, xerrors.From(err)
		}
		if xerr := collectKeys(iter, prefix, ascending, keySet); xerr != nil {
			return nil, xerr
		}
	}

	for key := range ledger.memStorage {
		keyBytes := []byte(key)
		if bytes.HasPrefix(keyBytes, prefix) {
			keySet[key] = keyBytes
		}
	}
	if ledger.cache != nil {
		for _, key := range ledger.cache.keysWithPrefix(prefix) {
			keySet[string(key)] = key
		}
	}

	return sortKeys(keySet, ascending), nil
}

func (ledger *MemLedger) Set(key LedgerKey, item ILedgerItem) xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	if item == nil {
		return xerrors.NewOrdinary("ledger item is nil")
	}
	if ledger.cache != nil {
		ledger.cache.set(key, item)
		return nil
	}
	bz, xerr := encodeLedgerItem(item)
	if xerr != nil {
		return xerr
	}
	ledger.memStorage[string(key)] = bz
	return nil
}

func (ledger *MemLedger) Del(key LedgerKey) xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	if ledger.cache != nil {
		ledger.cache.del(key)
	} else {
		ledger.memStorage[string(key)] = nil
	}
	return nil
}

func (ledger *MemLedger) createCache() xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	if ledger.cache != nil {
		return xerrors.NewOrdinary("transaction cache is already active")
	}
	ledger.cache = newCacheContext()
	return nil
}

func (ledger *MemLedger) writeCache() xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	if ledger.cache == nil {
		return xerrors.NewOrdinary("transaction cache is not active")
	}
	for _, entry := range ledger.cache.orderedEntries() {
		switch entry.op {
		case opSet:
			bz, xerr := encodeLedgerItem(entry.item)
			if xerr != nil {
				return xerr
			}
			ledger.memStorage[string(entry.key)] = bz
		case opDel:
			ledger.memStorage[string(entry.key)] = nil
		}
	}
	return nil
}
func (ledger *MemLedger) clearCache() xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	ledger.cache = nil
	return nil
}

func (ledger *MemLedger) hasActiveCache() bool {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	return ledger.cache != nil
}

var _ IImitable = (*MemLedger)(nil)
