package v1

import (
	"github.com/beatoz/beatoz-go/types/xerrors"
	tmlog "github.com/tendermint/tendermint/libs/log"
	"sync"
)

type Ledger struct {
	consensusLedger *MutableLedger
	mempoolLedger   *MempoolLedger

	logger tmlog.Logger
	mtx    sync.Mutex
}

func NewLedger(name, dbDir string, cacheSize int, newItem func() ILedgerItem, lg tmlog.Logger) (*Ledger, xerrors.XError) {
	_consensusLedger, xerr := NewMutableLedger(name, dbDir, cacheSize, newItem, lg)
	if xerr != nil {
		return nil, xerr
	}
	_mempoolLedger, xerr := NewMempoolLedger(
		_consensusLedger.DB(), cacheSize, newItem, lg, 0)
	if xerr != nil {
		_ = _consensusLedger.Close()
		return nil, xerr
	}

	return &Ledger{
		consensusLedger: _consensusLedger,
		mempoolLedger:   _mempoolLedger,
		logger:          lg,
	}, nil
}

func (ledger *Ledger) GetLedger(exec bool) ILedger {
	if exec == true {
		return ledger.consensusLedger
	}
	return ledger.mempoolLedger
}

func (ledger *Ledger) Commit() ([]byte, int64, xerrors.XError) {
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

func (ledger *Ledger) Close() xerrors.XError {
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
