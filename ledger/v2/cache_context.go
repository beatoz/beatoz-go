package v2

import "github.com/beatoz/beatoz-go/types/xerrors"

type cacheStore interface {
	getRaw(key []byte) ([]byte, bool, xerrors.XError)
	setRaw(key, value []byte) xerrors.XError
	deleteRaw(key []byte) xerrors.XError
}

type cacheContext struct {
	parent cacheStore
	writes *writeSet
}

func newCacheContext(parent cacheStore) *cacheContext {
	return &cacheContext{
		parent: parent,
		writes: newWriteSet(),
	}
}

func (ctx *cacheContext) CacheWrap() *cacheContext {
	return newCacheContext(ctx)
}

func (ctx *cacheContext) parentMissing() bool {
	return ctx.parent == nil
}

func (ctx *cacheContext) Write() xerrors.XError {
	if ctx.parentMissing() {
		return xerrors.NewOrdinary("cache context parent is nil")
	}
	for _, entry := range ctx.dirtyEntries(true) {
		switch entry.op {
		case opSet:
			if xerr := ctx.parent.setRaw(entry.key, entry.val); xerr != nil {
				return xerr
			}
		case opDel:
			if xerr := ctx.parent.deleteRaw(entry.key); xerr != nil {
				return xerr
			}
		}
	}
	ctx.writes.reset()
	return nil
}

func (ctx *cacheContext) Get(key []byte) ([]byte, bool, xerrors.XError) {
	if entry, ok := ctx.writes.get(key); ok {
		return entry.value()
	}
	if ctx.parentMissing() {
		return nil, false, xerrors.NewOrdinary("cache context parent is nil")
	}
	return ctx.parent.getRaw(key)
}

func (ctx *cacheContext) Set(key, val []byte) xerrors.XError {
	ctx.writes.set(key, val)
	return nil
}

func (ctx *cacheContext) Delete(key []byte) xerrors.XError {
	ctx.writes.del(key)
	return nil
}

func (ctx *cacheContext) dirtyEntries(ascending bool) []*writeEntry {
	return ctx.writes.orderedEntries(ascending)
}

func (ctx *cacheContext) keysWithPrefix(prefix []byte) [][]byte {
	return ctx.writes.keysWithPrefix(prefix)
}

func (ctx *cacheContext) keysWithPrefixDeep(prefix []byte) [][]byte {
	keys := make(map[string][]byte)
	ctx.collectKeysWithPrefix(prefix, keys)

	ret := make([][]byte, 0, len(keys))
	for _, key := range keys {
		ret = append(ret, cloneBytes(key))
	}
	sortLedgerKeys(ret, true)
	return ret
}

func (ctx *cacheContext) collectKeysWithPrefix(prefix []byte, keys map[string][]byte) {
	if parent, ok := ctx.parent.(*cacheContext); ok {
		parent.collectKeysWithPrefix(prefix, keys)
	}
	for _, key := range ctx.keysWithPrefix(prefix) {
		keys[string(key)] = cloneBytes(key)
	}
}

func (ctx *cacheContext) getRaw(key []byte) ([]byte, bool, xerrors.XError) {
	return ctx.Get(key)
}

func (ctx *cacheContext) setRaw(key, value []byte) xerrors.XError {
	return ctx.Set(key, value)
}

func (ctx *cacheContext) deleteRaw(key []byte) xerrors.XError {
	return ctx.Delete(key)
}

func (entry *writeEntry) value() ([]byte, bool, xerrors.XError) {
	if entry.op == opDel {
		return nil, false, nil
	}
	return cloneBytes(entry.val), true, nil
}

var _ cacheStore = (*cacheContext)(nil)
