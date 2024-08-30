package v1

import (
	"bytes"
	"fmt"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/cosmos/iavl"
	tmlog "github.com/tendermint/tendermint/libs/log"
	"sync"
)

type ImmutableLedger struct {
	*iavl.ImmutableTree
	newItemFunc func() ILedgerItem

	logger tmlog.Logger
	mtx    sync.RWMutex
}

func newImmutableLedger(immuTree *iavl.ImmutableTree, newItem func() ILedgerItem, lg tmlog.Logger) *ImmutableLedger {
	return &ImmutableLedger{
		ImmutableTree: immuTree,
		newItemFunc:   newItem,
		logger:        lg,
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
		return nil, xerrors.From(fmt.Errorf("ImmutableLedger: the key is compromised - the requested key(%x) is not equal to the key(%x) decoded in value", key, item.Key()))
	}

	return item, nil
}

func (ledger *ImmutableLedger) Del(key LedgerKey) xerrors.XError {
	panic("ImmutableLedger can not have this method")
}

func (ledger *ImmutableLedger) Iterate(cb func(ILedgerItem) xerrors.XError) xerrors.XError {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	var xerrStop xerrors.XError
	stopped, err := ledger.ImmutableTree.Iterate(func(key []byte, value []byte) bool {
		item := ledger.newItemFunc()
		if xerr := item.Decode(value); xerr != nil {
			xerrStop = xerr
			return true
		} else if bytes.Compare(item.Key(), key) != 0 {
			xerrStop = xerrors.From(fmt.Errorf("ImmutableLedger: the key is compromised - the requested key(%x) is not equal to the key(%x) decoded in value", key, item.Key()))
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

func (ledger *ImmutableLedger) RevertToSnapshot(snap int) xerrors.XError {
	panic("ImmutableLedger can not have this method")
}

func (ledger *ImmutableLedger) ImmutableLedgerAt(i int64) (ILedger, xerrors.XError) {
	panic("ImmutableLedger can not have this method")
}
func (ledger *ImmutableLedger) MempoolLedgerAt(i int64) (ILedger, xerrors.XError) {
	panic("ImmutableLedger can not have this method")
}

var _ ILedger = (*ImmutableLedger)(nil)
