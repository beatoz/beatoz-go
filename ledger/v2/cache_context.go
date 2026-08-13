package v2

import "github.com/beatoz/beatoz-go/types/xerrors"

type cacheContext struct {
	writes *writeSet
}

func newCacheContext() *cacheContext {
	return &cacheContext{
		writes: newWriteSet(),
	}
}

func (ctx *cacheContext) read(key []byte) (ILedgerItem, bool, xerrors.XError) {
	if ctx == nil || ctx.writes == nil {
		return nil, false, nil
	}

	entry, ok := ctx.writes.lookup(key)
	if !ok {
		return nil, false, nil
	}
	switch entry.op {
	case opSet:
		return entry.item, true, nil
	case opDel:
		return nil, true, xerrors.ErrNotFoundResult
	default:
		return nil, true, xerrors.NewOrdinary("invalid transaction cache operation")
	}
}

func (ctx *cacheContext) set(key []byte, item ILedgerItem) {
	ctx.writes.set(key, item)
}

func (ctx *cacheContext) del(key []byte) {
	ctx.writes.del(key)
}

func (ctx *cacheContext) orderedEntries() []*writeEntry {
	return ctx.writes.orderedEntries()
}

func (ctx *cacheContext) keysWithPrefix(prefix []byte) [][]byte {
	return ctx.writes.keysWithPrefix(prefix)
}

func (ctx *cacheContext) hasEntries() bool {
	return ctx != nil && ctx.writes != nil && len(ctx.writes.entries) > 0
}
