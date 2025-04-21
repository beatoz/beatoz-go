package v1

import (
	"github.com/beatoz/beatoz-go/types/xerrors"
	tmlog "github.com/tendermint/tendermint/libs/log"
	"sync"
)

type StateLedger[T ILedgerItem] struct {
	commitLedger   IMutable
	imitableLedger IImitable

	logger tmlog.Logger
	mtx    sync.RWMutex
}

var _ IStateLedger[ILedgerItem] = (*StateLedger[ILedgerItem])(nil)

func NewStateLedger[T ILedgerItem](name, dbDir string, cacheSize int, newItem func() ILedgerItem, lg tmlog.Logger) (*StateLedger[T], xerrors.XError) {
	newItemFunc := func() ILedgerItem { return newItem() }
	_commitLedger, xerr := NewMutableLedger(name, dbDir, cacheSize, newItemFunc, lg)
	if xerr != nil {
		return nil, xerr
	}
	_imitableLedger, xerr := NewMemLedgerAt(_commitLedger.Version(), _commitLedger, lg)
	if xerr != nil {
		_ = _commitLedger.Close()
		return nil, xerr
	}

	return &StateLedger[T]{
		commitLedger:   _commitLedger,
		imitableLedger: _imitableLedger,
		logger:         lg.With("ledger", "StateLedger"),
	}, nil
}

func (ledger *StateLedger[T]) getLedger(exec bool) IImitable {
	if exec == true {
		return ledger.commitLedger
	}
	return ledger.imitableLedger
}

func (ledger *StateLedger[T]) Version() int64 {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	return ledger.commitLedger.Version()
}

func (ledger *StateLedger[T]) Get(key LedgerKey, exec bool) (T, xerrors.XError) {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	var emptyNil T
	item, xerr := ledger.getLedger(exec).Get(key)
	if item == nil {
		item = emptyNil
	}

	return item.(T), xerr
}

func (ledger *StateLedger[T]) Iterate(cb func(T) xerrors.XError, exec bool) xerrors.XError {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	return ledger.getLedger(exec).Iterate(func(item ILedgerItem) xerrors.XError {
		// todo: the following unlock code must not be allowed.
		// this allows the callee to access the ledger's other method, which may update key or value of the tree.
		// However, in iterating, the key and value MUST not updated.
		ledger.mtx.RUnlock()
		defer ledger.mtx.RLock()

		return cb(item.(T))
	})
}

func (ledger *StateLedger[T]) Seek(prefix []byte, ascending bool, cb func(T) xerrors.XError, exec bool) xerrors.XError {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	return ledger.getLedger(exec).Seek(prefix, ascending, func(item ILedgerItem) xerrors.XError {
		return cb(item.(T))
	})
}

func (ledger *StateLedger[T]) Set(key LedgerKey, item T, exec bool) xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	if xerr := ledger.getLedger(exec).Set(key, item); xerr != nil {
		return xerr
	}
	return nil
}

func (ledger *StateLedger[T]) Del(key LedgerKey, exec bool) xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	if xerr := ledger.getLedger(exec).Del(key); xerr != nil {
		return xerr
	}

	return nil
}

func (ledger *StateLedger[T]) Snapshot(exec bool) int {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	return ledger.getLedger(exec).Snapshot()
}

func (ledger *StateLedger[T]) RevertToSnapshot(snap int, exec bool) xerrors.XError {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	return ledger.getLedger(exec).RevertToSnapshot(snap)
}

func (ledger *StateLedger[T]) Commit() ([]byte, int64, xerrors.XError) {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	hash, ver, xerr := ledger.commitLedger.Commit()
	if xerr != nil {
		return nil, 0, xerr
	}

	ledger.imitableLedger, xerr = NewMemLedgerAt(ver, ledger.commitLedger, ledger.logger)
	if xerr != nil {
		return nil, 0, xerr
	}

	return hash, ver, nil
}

func (ledger *StateLedger[T]) Close() xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	if ledger.commitLedger != nil {
		if xerr := ledger.commitLedger.Close(); xerr != nil {
			return xerr
		}
		ledger.commitLedger = nil
	}
	ledger.imitableLedger = nil
	return nil
}

// ImitableLedgerAt returns the ledger that is immutable and not committable.
func (ledger *StateLedger[T]) ImitableLedgerAt(height int64) (IImitable, xerrors.XError) {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	return NewMemLedgerAt(height, ledger.commitLedger, ledger.logger)
}
