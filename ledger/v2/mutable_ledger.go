package v2

import (
	"bytes"
	"sync"

	bytes2 "github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/cosmos/iavl"
	dbm "github.com/cosmos/iavl/db"
	tmlog "github.com/tendermint/tendermint/libs/log"
)

type MutableLedger struct {
	db    dbm.DB
	tree  *iavl.MutableTree
	cache *cacheContext

	newItemFor FuncNewItemFor

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
		newItemFor: newItem,
		logger:     lg.With("ledger", "MutableLedger"),
	}, nil
}

func (ledger *MutableLedger) Get(key LedgerKey) (ILedgerItem, xerrors.XError) {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	if ledger.closed() {
		return nil, xerrors.NewOrdinary("mutable ledger is closed")
	}
	item, xerr := ledger.get(key)
	if xerr != nil {
		return nil, xerr
	}
	if ledger.cache != nil {
		ledger.cache.set(key, item)
	}
	return item, nil
}

func (ledger *MutableLedger) get(key LedgerKey) (ILedgerItem, xerrors.XError) {
	if item, found, xerr := ledger.cache.read(key); found {
		return item, xerr
	}
	bz, err := ledger.tree.Get(key)
	if err != nil {
		return nil, xerrors.From(err)
	}
	if bz == nil {
		ledger.logger.Debug("MutableLedger.tree.Get returns nil", "key", bytes2.HexBytes(key))
		return nil, xerrors.ErrNotFoundResult
	}
	return ledger.decode(key, bz)
}

func (ledger *MutableLedger) decode(key LedgerKey, bz []byte) (ILedgerItem, xerrors.XError) {
	item := ledger.newItemFor(key)
	if xerr := item.Decode(key, bz); xerr != nil {
		return nil, xerr
	}
	return item, nil
}

func (ledger *MutableLedger) Iterate(cb FuncIterate) xerrors.XError {
	return ledger.Seek(nil, true, cb)
}

func (ledger *MutableLedger) Seek(prefix []byte, ascending bool, cb FuncIterate) xerrors.XError {
	if cb == nil {
		return xerrors.NewOrdinary("iterate callback is nil")
	}

	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	if ledger.closed() {
		return xerrors.NewOrdinary("mutable ledger is closed")
	}
	if !ledger.cache.hasEntries() {
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

func (ledger *MutableLedger) seekTree(prefix []byte, ascending bool, cb FuncIterate) xerrors.XError {
	start := bytes.Clone(prefix)
	iter, err := ledger.tree.Iterator(start, nil, ascending)
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

func (ledger *MutableLedger) seekKeys(prefix []byte, ascending bool) ([][]byte, xerrors.XError) {
	keySet := make(map[string][]byte)

	iter, err := ledger.tree.Iterator(prefix, nil, ascending)
	if err != nil {
		return nil, xerrors.From(err)
	}
	if xerr := collectKeys(iter, prefix, ascending, keySet); xerr != nil {
		return nil, xerr
	}

	if ledger.cache != nil {
		for _, key := range ledger.cache.keysWithPrefix(prefix) {
			keySet[string(key)] = key
		}
	}

	return sortKeys(keySet, ascending), nil
}

func (ledger *MutableLedger) Set(key LedgerKey, item ILedgerItem) xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	if ledger.closed() {
		return xerrors.NewOrdinary("mutable ledger is closed")
	}
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

	if _, err := ledger.tree.Set(key, bz); err != nil {
		return xerrors.From(err)
	}
	return nil
}

func (ledger *MutableLedger) Del(key LedgerKey) xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	if ledger.closed() {
		return xerrors.NewOrdinary("mutable ledger is closed")
	}
	if ledger.cache != nil {
		ledger.cache.del(key)
		return nil
	}

	_, _, err := ledger.tree.Remove(key)
	if err != nil {
		return xerrors.From(err)
	}
	return nil
}

func (ledger *MutableLedger) createCache() xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	if ledger.closed() {
		return xerrors.NewOrdinary("mutable ledger is closed")
	}
	if ledger.cache != nil {
		return xerrors.NewOrdinary("transaction cache is already active")
	}

	ledger.cache = newCacheContext()
	return nil
}

func (ledger *MutableLedger) writeCache() xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	if ledger.closed() {
		return xerrors.NewOrdinary("mutable ledger is closed")
	}
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
			if _, err := ledger.tree.Set(entry.key, bz); err != nil {
				return xerrors.From(err)
			}
		case opDel:
			if _, _, err := ledger.tree.Remove(entry.key); err != nil {
				return xerrors.From(err)
			}
		default:
			return xerrors.NewOrdinary("invalid transaction cache operation")
		}
	}
	return nil
}
func encodeLedgerItem(item ILedgerItem) ([]byte, xerrors.XError) {
	if item == nil {
		return nil, xerrors.NewOrdinary("ledger item is nil")
	}
	bz, xerr := item.Encode()
	if xerr != nil {
		return nil, xerr
	}
	if bz == nil {
		return nil, xerrors.NewOrdinary("ledger item encoded value is nil")
	}
	return bz, nil
}

func (ledger *MutableLedger) clearCache() xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	ledger.cache = nil
	return nil
}

func (ledger *MutableLedger) hasActiveCache() bool {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	return ledger.cache != nil
}

func (ledger *MutableLedger) Commit() ([]byte, int64, xerrors.XError) {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	if ledger.closed() {
		return nil, 0, xerrors.NewOrdinary("mutable ledger is closed")
	}
	if ledger.cache != nil {
		return nil, 0, xerrors.NewOrdinary("active transaction cache remains")
	}

	ledger.tree.SetCommitting()
	defer ledger.tree.UnsetCommitting()

	hash, version, err := ledger.tree.SaveVersion()
	if err != nil {
		return hash, version, xerrors.From(err)
	}

	ledger.logger.Debug("tree save version", "hash", hash, "version", version)
	return hash, version, nil
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

	if ledger.cache != nil {
		return xerrors.NewOrdinary("active transaction cache remains")
	}
	if ledger.closed() {
		return nil
	}

	if err := ledger.tree.Close(); err != nil {
		return xerrors.From(err)
	}
	ledger.tree = nil

	if ledger.db != nil {
		if err := ledger.db.Close(); err != nil {
			return xerrors.From(err)
		}
	}
	ledger.db = nil
	return nil
}

func (ledger *MutableLedger) closed() bool {
	return ledger.tree == nil
}

var _ IMutable = (*MutableLedger)(nil)
