package v1

import (
	"bytes"
	"fmt"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/cosmos/iavl"
	"sync"
)

type ImmutableLedger struct {
	*iavl.ImmutableTree
	newItemFunc func() ILedgerItem

	mtx sync.RWMutex
}

func newImmutableLedger(immuTree *iavl.ImmutableTree, newItem func() ILedgerItem) *ImmutableLedger {
	return &ImmutableLedger{
		ImmutableTree: immuTree,
		newItemFunc:   newItem,
	}
}

func (ledger *ImmutableLedger) Set(item ILedgerItem) xerrors.XError {
	panic("ImmutableLedger can not have this method")
}

func (ledger *ImmutableLedger) Get(key LedgerKey) (ILedgerItem, xerrors.XError) {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	val, err := ledger.ImmutableTree.Get(key)
	if err != nil {
		return nil, xerrors.From(err)
	} else if val == nil {
		return nil, xerrors.ErrNotFoundResult
	}

	item := ledger.newItemFunc()
	if xerr := item.Decode(val); xerr != nil {
		return nil, xerr
	} else if bytes.Compare(item.Key(), key) != 0 {
		return nil, xerrors.NewOrdinary("ImmutableLedger: the key is compromised - the requested key is not equal to the key encoded in value")
	}

	return item, nil
}

func (ledger *ImmutableLedger) Del(key LedgerKey) {
	panic("ImmutableLedger can not have this method")
}

func (ledger *ImmutableLedger) Iterate(cb func(ILedgerItem) xerrors.XError) xerrors.XError {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	var xerrStop xerrors.XError
	stopped, err := ledger.ImmutableTree.Iterate(func(key []byte, value []byte) bool {
		item := ledger.newItemFunc()
		if xerr := item.Decode(value); xerr != nil {
			xerrStop = xerrors.NewOrdinary(fmt.Sprintf("item decode error - Key:%X vs. err:%v", key, xerr))
			return true
		} else if bytes.Compare(item.Key(), key) != 0 {
			xerrStop = xerrors.NewOrdinary(fmt.Sprintf("wrong key - Key:%X vs. stake's txhash:%X", key, item.Key()))
			return true
		} else if xerr := cb(item); xerr != nil {
			xerrStop = xerr
			return true
		}
		return false // continue iteration
	})

	if err != nil {
		return xerrors.From(err)
	} else if stopped {
		return xerrStop
	}
	return nil
}

func (ledger *ImmutableLedger) Commit() ([]byte, int64, xerrors.XError) {
	panic("ImmutableLedger can not have this method")
}

func (ledger *ImmutableLedger) Close() xerrors.XError {
	panic("ImmutableLedger can not have this method")
}

func (ledger *ImmutableLedger) Snapshot() int {
	panic("ImmutableLedger can not have this method")
}

func (ledger *ImmutableLedger) RevertToSnapshot(snap int) {
	panic("ImmutableLedger can not have this method")
}

func (ledger *ImmutableLedger) RevertAll() {
	panic("ImmutableLedger can not have this method")
}

func (ledger *ImmutableLedger) ApplyRevisions() xerrors.XError {
	panic("ImmutableLedger can not have this method")
}

func (ledger *ImmutableLedger) ImmutableLedgerAt(i int64) (ILedger, xerrors.XError) {
	panic("ImmutableLedger can not have this method")
}
func (ledger *ImmutableLedger) MempoolLedgerAt(i int64) (ILedger, xerrors.XError) {
	panic("ImmutableLedger can not have this method")
}

var _ ILedger = (*ImmutableLedger)(nil)
