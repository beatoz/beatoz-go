package v2

import (
	"sync"

	"github.com/beatoz/beatoz-go/types/xerrors"
	tmlog "github.com/tendermint/tendermint/libs/log"
)

type StateLedger struct {
	commitLedger   *MutableLedger
	imitableLedger *MemLedger

	logger tmlog.Logger
	mtx    sync.RWMutex
}

var _ IStateLedger = (*StateLedger)(nil)

func NewStateLedger(name, dbDir string, cacheSize int, newItem FuncNewItemFor, lg tmlog.Logger) (*StateLedger, xerrors.XError) {
	commitLedger, xerr := NewMutableLedger(name, dbDir, cacheSize, newItem, lg)
	if xerr != nil {
		return nil, xerr
	}

	imitableLedger, xerr := NewMemLedgerAt(commitLedger.Version(), commitLedger, lg)
	if xerr != nil {
		_ = commitLedger.Close()
		return nil, xerr
	}

	return newStateLedger(commitLedger, imitableLedger, lg), nil
}

func newStateLedger(commitLedger *MutableLedger, imitableLedger *MemLedger, lg tmlog.Logger) *StateLedger {
	return &StateLedger{
		commitLedger:   commitLedger,
		imitableLedger: imitableLedger,
		logger:         lg.With("ledger", "StateLedger"),
	}
}

func (ledger *StateLedger) getLedger(exec bool) IImitable {
	if exec {
		return ledger.commitLedger
	}
	return ledger.imitableLedger
}

func (ledger *StateLedger) closed() bool {
	return ledger.commitLedger == nil || ledger.imitableLedger == nil
}

func (ledger *StateLedger) Version() int64 {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	if ledger.closed() {
		return 0
	}
	return ledger.commitLedger.Version()
}

func (ledger *StateLedger) Get(key LedgerKey, exec bool) (ILedgerItem, xerrors.XError) {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	if ledger.closed() {
		return nil, xerrors.NewOrdinary("state ledger is closed")
	}
	return ledger.getLedger(exec).Get(key)
}

func (ledger *StateLedger) Iterate(cb FuncIterate, exec bool) xerrors.XError {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	if ledger.closed() {
		return xerrors.NewOrdinary("state ledger is closed")
	}
	return ledger.getLedger(exec).Iterate(cb)
}

func (ledger *StateLedger) Seek(prefix []byte, ascending bool, cb FuncIterate, exec bool) xerrors.XError {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	if ledger.closed() {
		return xerrors.NewOrdinary("state ledger is closed")
	}
	return ledger.getLedger(exec).Seek(prefix, ascending, cb)
}

func (ledger *StateLedger) Set(key LedgerKey, item ILedgerItem, exec bool) xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	if ledger.closed() {
		return xerrors.NewOrdinary("state ledger is closed")
	}
	return ledger.getLedger(exec).Set(key, item)
}

func (ledger *StateLedger) Del(key LedgerKey, exec bool) xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	if ledger.closed() {
		return xerrors.NewOrdinary("state ledger is closed")
	}
	return ledger.getLedger(exec).Del(key)
}

func (ledger *StateLedger) CacheContext(exec bool) (IStateLedger, func() xerrors.XError) {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	if ledger.closed() {
		return nil, func() xerrors.XError {
			return xerrors.NewOrdinary("state ledger is closed")
		}
	}
	if exec {
		cachedCommitLedger := ledger.commitLedger.CacheWrap()
		if cachedCommitLedger == nil {
			return nil, func() xerrors.XError {
				return xerrors.NewOrdinary("state ledger commit cache is unavailable")
			}
		}
		return newStateLedger(cachedCommitLedger, ledger.imitableLedger, ledger.logger), cachedCommitLedger.Write
	}

	cachedImitableLedger := ledger.imitableLedger.CacheWrap()
	if cachedImitableLedger == nil {
		return nil, func() xerrors.XError {
			return xerrors.NewOrdinary("state ledger imitable cache is unavailable")
		}
	}
	return newStateLedger(ledger.commitLedger, cachedImitableLedger, ledger.logger), cachedImitableLedger.Write
}

func (ledger *StateLedger) Commit() ([]byte, int64, xerrors.XError) {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	if ledger.closed() {
		return nil, 0, xerrors.NewOrdinary("state ledger is closed")
	}
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

func (ledger *StateLedger) ImitableLedgerAt(height int64) (IImitable, xerrors.XError) {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	if ledger.closed() {
		return nil, xerrors.NewOrdinary("state ledger is closed")
	}
	return NewMemLedgerAt(height, ledger.commitLedger, ledger.logger)
}
