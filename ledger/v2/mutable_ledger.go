package v2

import (
	"bytes"
	"sync"

	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/cosmos/iavl"
	dbm "github.com/cosmos/iavl/db"
	tmlog "github.com/tendermint/tendermint/libs/log"
)

type mutableTreeStore struct {
	tree *iavl.MutableTree
}

func (store *mutableTreeStore) getRaw(key []byte) ([]byte, bool, xerrors.XError) {
	if store == nil || store.tree == nil {
		return nil, false, nil
	}
	// bz is the raw encoded ledger item value stored in the IAVL tree.
	bz, err := store.tree.Get(key)
	if err != nil {
		return nil, false, xerrors.From(err)
	}
	if bz == nil {
		return nil, false, nil
	}
	return cloneBytes(bz), true, nil
}

func (store *mutableTreeStore) setRaw(key, value []byte) xerrors.XError {
	if _, err := store.tree.Set(key, value); err != nil {
		return xerrors.From(err)
	}
	return nil
}

func (store *mutableTreeStore) deleteRaw(key []byte) xerrors.XError {
	if _, _, err := store.tree.Remove(key); err != nil {
		return xerrors.From(err)
	}
	return nil
}

type MutableLedger struct {
	db    dbm.DB
	tree  *iavl.MutableTree
	cache *cacheContext

	newItemFor FuncNewItemFor
	cacheSize  int

	logger tmlog.Logger
	mtx    sync.RWMutex
}

var _ IMutable = (*MutableLedger)(nil)

func NewMutableLedger(name, dbDir string, cacheSize int, newItem FuncNewItemFor, lg tmlog.Logger) (*MutableLedger, xerrors.XError) {
	db, err := dbm.NewGoLevelDB(name, dbDir)
	if err != nil {
		return nil, xerrors.Wrap(err, "goleveldb open failed")
	}

	tree := iavl.NewMutableTree(db, cacheSize, false, iavl.NewNopLogger(), iavl.SyncOption(true))
	if _, err := tree.LoadVersion(0); err != nil {
		_ = tree.Close()
		_ = db.Close()
		return nil, xerrors.Wrap(err, "tree's LoadVersion failed")
	}

	return newMutableLedger(db, tree, newCacheContext(&mutableTreeStore{tree: tree}), cacheSize, newItem, lg), nil
}

func newMutableLedger(db dbm.DB, tree *iavl.MutableTree, cache *cacheContext, cacheSize int, newItem FuncNewItemFor, lg tmlog.Logger) *MutableLedger {
	return &MutableLedger{
		db:         db,
		tree:       tree,
		cache:      cache,
		newItemFor: newItem,
		cacheSize:  cacheSize,
		logger:     lg.With("ledger", "MutableLedger"),
	}
}

func (ledger *MutableLedger) closed() bool {
	return ledger.tree == nil || ledger.cache == nil
}

func (ledger *MutableLedger) CacheWrap() *MutableLedger {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	if ledger.closed() {
		return nil
	}
	return newMutableLedger(ledger.db, ledger.tree, ledger.cache.CacheWrap(), ledger.cacheSize, ledger.newItemFor, ledger.logger)
}

func (ledger *MutableLedger) Write() xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	if ledger.closed() {
		return xerrors.NewOrdinary("mutable ledger is closed")
	}
	return ledger.cache.Write()
}

func (ledger *MutableLedger) NewItem(key LedgerKey) ILedgerItem {
	if ledger.newItemFor == nil {
		return nil
	}
	return ledger.newItemFor(key)
}

func (ledger *MutableLedger) Get(key LedgerKey) (ILedgerItem, xerrors.XError) {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	if ledger.closed() {
		return nil, xerrors.NewOrdinary("mutable ledger is closed")
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
		return nil, xerrors.NewOrdinary("mutable ledger item factory returned nil")
	}
	if xerr := item.Decode(key, bz); xerr != nil {
		return nil, xerr
	}
	return item, nil
}

func (ledger *MutableLedger) Set(key LedgerKey, item ILedgerItem) xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	if ledger.closed() {
		return xerrors.NewOrdinary("mutable ledger is closed")
	}
	if item == nil {
		return xerrors.NewOrdinary("mutable ledger item is nil")
	}
	// bz is the encoded form written to the cache layer.
	bz, xerr := item.Encode()
	if xerr != nil {
		return xerr
	}
	return ledger.cache.Set(key, bz)
}

func (ledger *MutableLedger) Del(key LedgerKey) xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	if ledger.closed() {
		return xerrors.NewOrdinary("mutable ledger is closed")
	}
	return ledger.cache.Delete(key)
}

func (ledger *MutableLedger) Iterate(cb FuncIterate) xerrors.XError {
	return ledger.Seek(nil, true, cb)
}

func (ledger *MutableLedger) Seek(prefix []byte, ascending bool, cb FuncIterate) xerrors.XError {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	if ledger.closed() {
		return xerrors.NewOrdinary("mutable ledger is closed")
	}
	if cb == nil {
		return xerrors.NewOrdinary("mutable ledger iterator callback is nil")
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
			return xerrors.NewOrdinary("mutable ledger item factory returned nil")
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

func (ledger *MutableLedger) seekKeys(prefix []byte, ascending bool) ([][]byte, xerrors.XError) {
	keys := make(map[string][]byte)
	iter, err := ledger.tree.Iterator(prefix, nil, true)
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

func (ledger *MutableLedger) Commit() ([]byte, int64, xerrors.XError) {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	if ledger.closed() {
		return nil, 0, xerrors.NewOrdinary("mutable ledger is closed")
	}
	if xerr := ledger.cache.Write(); xerr != nil {
		return nil, 0, xerr
	}

	ledger.tree.SetCommitting()
	defer ledger.tree.UnsetCommitting()

	hash, ver, err := ledger.tree.SaveVersion()
	if err != nil {
		return hash, ver, xerrors.From(err)
	}

	ledger.cache = newCacheContext(&mutableTreeStore{tree: ledger.tree})
	ledger.logger.Debug("tree save version", "hash", hash, "version", ver)
	return hash, ver, nil
}

func (ledger *MutableLedger) Version() int64 {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	if ledger.closed() {
		return 0
	}
	return ledger.tree.Version()
}

func (ledger *MutableLedger) GetReadOnlyTree(ver int64) (*iavl.ImmutableTree, xerrors.XError) {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	if ledger.closed() {
		return nil, xerrors.NewOrdinary("mutable ledger is closed")
	}
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
	ledger.cache = nil
	return nil
}
