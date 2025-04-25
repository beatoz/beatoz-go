package v1

import (
	"github.com/beatoz/beatoz-go/types/xerrors"
	tmlog "github.com/tendermint/tendermint/libs/log"
	"sync"
)

type StateLedger struct {
	commitLedger   IMutable
	imitableLedger IImitable

	logger tmlog.Logger
	mtx    sync.RWMutex
}

var _ IStateLedger = (*StateLedger)(nil)

func NewStateLedger(name, dbDir string, cacheSize int, newItem FuncNewItemFor, lg tmlog.Logger) (*StateLedger, xerrors.XError) {
	_commitLedger, xerr := NewMutableLedger(name, dbDir, cacheSize, newItem, lg)
	if xerr != nil {
		return nil, xerr
	}
	_imitableLedger, xerr := NewMemLedgerAt(_commitLedger.Version(), _commitLedger, lg)
	if xerr != nil {
		_ = _commitLedger.Close()
		return nil, xerr
	}

	return &StateLedger{
		commitLedger:   _commitLedger,
		imitableLedger: _imitableLedger,
		logger:         lg.With("ledger", "StateLedger"),
	}, nil
}

func (ledger *StateLedger) getLedger(exec bool) IImitable {
	if exec == true {
		return ledger.commitLedger
	}
	return ledger.imitableLedger
}

func (ledger *StateLedger) Version() int64 {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	return ledger.commitLedger.Version()
}

func (ledger *StateLedger) Get(key LedgerKey, exec bool) (ILedgerItem, xerrors.XError) {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	return ledger.getLedger(exec).Get(key)
}

func (ledger *StateLedger) Iterate(cb FuncIterate, exec bool) xerrors.XError {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	return ledger.getLedger(exec).Iterate(func(key LedgerKey, item ILedgerItem) xerrors.XError {
		return cb(key, item)
	})
}

func (ledger *StateLedger) Seek(prefix []byte, ascending bool, cb FuncIterate, exec bool) xerrors.XError {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	return ledger.getLedger(exec).Seek(prefix, ascending, func(key LedgerKey, item ILedgerItem) xerrors.XError {
		return cb(key, item)
	})
}

func (ledger *StateLedger) Set(key LedgerKey, item ILedgerItem, exec bool) xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	if xerr := ledger.getLedger(exec).Set(key, item); xerr != nil {
		return xerr
	}
	return nil
}

func (ledger *StateLedger) Del(key LedgerKey, exec bool) xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	if xerr := ledger.getLedger(exec).Del(key); xerr != nil {
		return xerr
	}

	return nil
}

func (ledger *StateLedger) Snapshot(exec bool) int {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	return ledger.getLedger(exec).Snapshot()
}

func (ledger *StateLedger) RevertToSnapshot(snap int, exec bool) xerrors.XError {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	return ledger.getLedger(exec).RevertToSnapshot(snap)
}

func (ledger *StateLedger) Commit() ([]byte, int64, xerrors.XError) {
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

func (ledger *StateLedger) Close() xerrors.XError {
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
func (ledger *StateLedger) ImitableLedgerAt(height int64) (IImitable, xerrors.XError) {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	return NewMemLedgerAt(height, ledger.commitLedger, ledger.logger)
}
