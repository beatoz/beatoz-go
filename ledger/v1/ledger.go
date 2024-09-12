package v1

import (
	"github.com/beatoz/beatoz-go/types/xerrors"
	tmlog "github.com/tendermint/tendermint/libs/log"
	"sync"
)

type Ledger[T ILedgerItem] struct {
	consensusLedger *MutableLedger
	mempoolLedger   *MempoolLedger

	logger tmlog.Logger
	mtx    sync.RWMutex
}

func NewLedger[T ILedgerItem](name, dbDir string, cacheSize int, newItem func() T, lg tmlog.Logger) (*Ledger[T], xerrors.XError) {
	newItemFunc := func() ILedgerItem { return newItem() }
	_consensusLedger, xerr := NewMutableLedger(name, dbDir, cacheSize, newItemFunc, lg)
	if xerr != nil {
		return nil, xerr
	}
	_mempoolLedger, xerr := NewMempoolLedger(
		_consensusLedger.DB(), cacheSize, newItemFunc, lg, 0)
	if xerr != nil {
		_ = _consensusLedger.Close()
		return nil, xerr
	}

	return &Ledger[T]{
		consensusLedger: _consensusLedger,
		mempoolLedger:   _mempoolLedger,
		logger:          lg,
	}, nil
}

func (ledger *Ledger[T]) GetLedger(exec bool) ILedger {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	return ledger.getLedger(exec)

}

func (ledger *Ledger[T]) getLedger(exec bool) ILedger {
	if exec == true {
		return ledger.consensusLedger
	}
	return ledger.mempoolLedger
}

func (ledger *Ledger[T]) Write(item T, exec bool) xerrors.XError {
	return ledger.GetLedger(exec).Set(item)
}

func (ledger *Ledger[T]) Read(key LedgerKey, exec bool) (T, xerrors.XError) {
	v, xerr := ledger.GetLedger(exec).Get(key)
	return v.(T), xerr
}

func (ledger *Ledger[T]) Iterate(cb func(T) xerrors.XError, exec bool) xerrors.XError {
	return ledger.GetLedger(exec).Iterate(func(item ILedgerItem) xerrors.XError {
		return cb(item.(T))
	})
}

func (ledger *Ledger[T]) Commit() ([]byte, int64, xerrors.XError) {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	hash, ver, xerr := ledger.consensusLedger.Commit()
	if xerr != nil {
		return nil, 0, xerr
	}

	ledger.mempoolLedger, xerr = NewMempoolLedger(
		ledger.consensusLedger.DB(),
		ledger.consensusLedger.CacheSize(),
		ledger.consensusLedger.NewItemFunc(), ledger.logger, ver)
	if xerr != nil {
		return nil, 0, xerr
	}
	return hash, ver, nil
}

func (ledger *Ledger[T]) Close() xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	if ledger.mempoolLedger != nil {
		if xerr := ledger.mempoolLedger.Close(); xerr != nil {
			return xerr
		}
		ledger.mempoolLedger = nil
	}
	if ledger.consensusLedger != nil {
		if xerr := ledger.consensusLedger.Close(); xerr != nil {
			return xerr
		}
		ledger.consensusLedger = nil
	}

	return nil
}
