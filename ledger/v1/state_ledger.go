package v1

import (
	"github.com/beatoz/beatoz-go/types/xerrors"
	"sync"
)

type StateLedger struct {
	consensusLedger *Ledger
	mempoolLedger   *MempoolLedger

	mtx sync.Mutex
}

func NewStateLedger(name, dbDir string, cacheSize int, newItem func() ILedgerItem) (*StateLedger, xerrors.XError) {
	_consensusLedger, xerr := NewLedger(name, dbDir, cacheSize, newItem)
	if xerr != nil {
		return nil, xerr
	}
	_mempoolLedger, xerr := NewMempoolLedger(
		_consensusLedger.DB(), cacheSize, newItem, 0)
	if xerr != nil {
		_ = _consensusLedger.Close()
		return nil, xerr
	}

	return &StateLedger{
		consensusLedger: _consensusLedger,
		mempoolLedger:   _mempoolLedger,
	}, nil
}

func (ledger *StateLedger) GetLedger(exec bool) ILedger {
	if exec == true {
		return ledger.consensusLedger
	}
	return ledger.mempoolLedger
}

func (ledger *StateLedger) Commit() ([]byte, int64, xerrors.XError) {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	hash, ver, xerr := ledger.consensusLedger.Commit()
	if xerr != nil {
		return nil, 0, xerr
	}

	ledger.mempoolLedger, xerr = NewMempoolLedger(
		ledger.consensusLedger.DB(),
		ledger.consensusLedger.CacheSize(),
		ledger.consensusLedger.NewItemFunc(), 0)
	if xerr != nil {
		return nil, 0, xerr
	}

	return hash, ver, nil
}

func (ledger *StateLedger) Close() xerrors.XError {
	if ledger.consensusLedger != nil {
		if xerr := ledger.consensusLedger.Close(); xerr != nil {
			return xerr
		}
		ledger.consensusLedger = nil
	}

	if ledger.mempoolLedger != nil {
		if xerr := ledger.mempoolLedger.Close(); xerr != nil {
			return xerr
		}
		ledger.mempoolLedger = nil
	}

	return nil
}
