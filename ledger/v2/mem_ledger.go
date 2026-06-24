package v2

import (
	"bytes"
	"sort"
	"sync"

	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/cosmos/iavl"
	tmlog "github.com/tendermint/tendermint/libs/log"
)

type immutableTreeStore struct {
	tree *iavl.ImmutableTree
}

func (store *immutableTreeStore) getRaw(key []byte) ([]byte, bool, xerrors.XError) {
	if store == nil || store.tree == nil {
		return nil, false, nil
	}
	bz, err := store.tree.Get(key)
	if err != nil {
		return nil, false, xerrors.From(err)
	}
	if bz == nil {
		return nil, false, nil
	}
	return cloneBytes(bz), true, nil
}

func (store *immutableTreeStore) setRaw(_, _ []byte) xerrors.XError {
	return xerrors.NewOrdinary("immutable tree store is read-only")
}

func (store *immutableTreeStore) deleteRaw(_ []byte) xerrors.XError {
	return xerrors.NewOrdinary("immutable tree store is read-only")
}

type MemLedger struct {
	immuTree   *iavl.ImmutableTree
	cache      *cacheContext
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

	return newMemLedger(tree, newCacheContext(&immutableTreeStore{tree: tree}), from.NewItem, lg), nil
}

func newMemLedger(tree *iavl.ImmutableTree, cache *cacheContext, newItem FuncNewItemFor, lg tmlog.Logger) *MemLedger {
	return &MemLedger{
		immuTree:   tree,
		cache:      cache,
		newItemFor: newItem,
		logger:     lg.With("ledger", "MemLedger"),
	}
}

func (ledger *MemLedger) closed() bool {
	return ledger.cache == nil
}

func (ledger *MemLedger) CacheWrap() *MemLedger {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	if ledger.closed() {
		return nil
	}
	return newMemLedger(ledger.immuTree, ledger.cache.CacheWrap(), ledger.newItemFor, ledger.logger)
}

func (ledger *MemLedger) Write() xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	if ledger.closed() {
		return xerrors.NewOrdinary("mem ledger is closed")
	}
	return ledger.cache.Write()
}

func (ledger *MemLedger) Get(key LedgerKey) (ILedgerItem, xerrors.XError) {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	if ledger.closed() {
		return nil, xerrors.NewOrdinary("mem ledger is closed")
	}
	bz, ok, xerr := ledger.cache.Get(key)
	if xerr != nil {
		return nil, xerr
	}
	if !ok {
		return nil, xerrors.ErrNotFoundResult
	}

	item := ledger.newItemFor(key)
	if item == nil {
		return nil, xerrors.NewOrdinary("mem ledger item factory returned nil")
	}
	if xerr := item.Decode(key, bz); xerr != nil {
		return nil, xerr
	}
	return item, nil
}

func (ledger *MemLedger) Set(key LedgerKey, item ILedgerItem) xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	if ledger.closed() {
		return xerrors.NewOrdinary("mem ledger is closed")
	}
	if item == nil {
		return xerrors.NewOrdinary("mem ledger item is nil")
	}
	bz, xerr := item.Encode()
	if xerr != nil {
		return xerr
	}
	return ledger.cache.Set(key, bz)
}

func (ledger *MemLedger) Del(key LedgerKey) xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	if ledger.closed() {
		return xerrors.NewOrdinary("mem ledger is closed")
	}
	return ledger.cache.Delete(key)
}

func (ledger *MemLedger) Iterate(cb FuncIterate) xerrors.XError {
	return ledger.Seek(nil, true, cb)
}

func (ledger *MemLedger) Seek(prefix []byte, ascending bool, cb FuncIterate) xerrors.XError {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	if ledger.closed() {
		return xerrors.NewOrdinary("mem ledger is closed")
	}
	if cb == nil {
		return xerrors.NewOrdinary("mem ledger iterator callback is nil")
	}
	keys, xerr := ledger.seekKeys(prefix, ascending)
	if xerr != nil {
		return xerr
	}

	for _, key := range keys {
		bz, ok, xerr := ledger.cache.Get(key)
		if xerr != nil {
			return xerr
		}
		if !ok {
			continue
		}

		item := ledger.newItemFor(key)
		if item == nil {
			return xerrors.NewOrdinary("mem ledger item factory returned nil")
		}
		if xerr := item.Decode(key, bz); xerr != nil {
			return xerr
		}
		if xerr := cb(key, item); xerr != nil {
			return xerr
		}
	}
	return nil
}

func (ledger *MemLedger) seekKeys(prefix []byte, ascending bool) ([][]byte, xerrors.XError) {
	keys := make(map[string][]byte)
	if ledger.immuTree != nil {
		iter, err := ledger.immuTree.Iterator(prefix, nil, true)
		if err != nil {
			return nil, xerrors.From(err)
		}
		defer func() {
			_ = iter.Close()
		}()

		for ; iter.Valid(); iter.Next() {
			key := iter.Key()
			if !bytes.HasPrefix(key, prefix) {
				break
			}
			keys[string(key)] = cloneBytes(key)
		}
	}

	for _, key := range ledger.cache.keysWithPrefixDeep(prefix) {
		keys[string(key)] = cloneBytes(key)
	}

	ret := make([][]byte, 0, len(keys))
	for _, key := range keys {
		ret = append(ret, key)
	}
	sortLedgerKeys(ret, ascending)
	return ret, nil
}

func sortLedgerKeys(keys [][]byte, ascending bool) {
	sort.Slice(keys, func(i, j int) bool {
		cmp := bytes.Compare(keys[i], keys[j])
		if ascending {
			return cmp < 0
		}
		return cmp > 0
	})
}
