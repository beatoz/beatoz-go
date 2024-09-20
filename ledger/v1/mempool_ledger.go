package v1

import (
	"bytes"
	"fmt"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/cosmos/iavl"
	dbm "github.com/cosmos/iavl/db"
	tmlog "github.com/tendermint/tendermint/libs/log"
	"sync"
)

// MempoolLedger cannot be committed, everything else is like MutableLedger.
// DEPRECATED
type MempoolLedger struct {
	*MutableLedger
	mtx sync.RWMutex
}

func NewMempoolLedger(db dbm.DB, cacheSize int, newItem func() ILedgerItem, lg tmlog.Logger, ver int64) (*MempoolLedger, xerrors.XError) {
	tree := iavl.NewMutableTree(db, cacheSize, false, iavl.NewNopLogger())
	if _, err := tree.LoadVersion(ver); err != nil {
		return nil, xerrors.From(err)
	}

	return &MempoolLedger{
		MutableLedger: &MutableLedger{
			tree: tree,
			// Do not set the `db` field
			// This field is only add in the original MutableLedger instance.
			// Because the 'db' field is nil, MempoolLedger.Close() can not close the 'db'.
			// The `db` can only be closed by the original MutableLedger.Close().
			db:          nil,
			cacheSize:   cacheSize,
			newItemFunc: newItem,
			//usedItems:   make(map[string]ILedgerItem),
			snapshots: NewSnapshotList(),
			logger:    lg,
		},
	}, nil
}

func (ledger *MempoolLedger) Iterate(cb func(ILedgerItem) xerrors.XError) xerrors.XError {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	var xerrStop xerrors.XError
	stopped, err := ledger.tree.Iterate(func(key []byte, value []byte) bool {

		// if the item is cached, return it to `cb`.
		//if item, ok := ledger.usedItems[unsafe.String(&key[0], len(key))]; ok {
		//	if xerr := cb(item); xerr != nil {
		//		xerrStop = xerr
		//		return true // stop
		//	}
		//}

		item := ledger.newItemFunc()
		if xerr := item.Decode(value); xerr != nil {
			xerrStop = xerr
			return true // stop
		} else if bytes.Compare(item.Key(), key) != 0 {
			xerrStop = xerrors.From(fmt.Errorf("MutableLedger: the key is compromised - the requested key(%x) is not equal to the key(%x) decoded in value", key, item.Key()))
			return true // stop
		} else if xerr := cb(item); xerr != nil {
			xerrStop = xerr
			return true // stop
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

//func (ledger *MempoolLedger) Commit() ([]byte, int64, xerrors.XError) {
//	panic("MempoolLedger can not have the method `Commit`")
//}
//
//func (ledger *MempoolLedger) ImmutableLedgerAt(ver int64) (ILedger, xerrors.XError) {
//	panic("MempoolLedger can not have the method `ImmutableLedgerAt`")
//}
//
//func (ledger *MempoolLedger) MempoolLedgerAt(ver int64) (ILedger, xerrors.XError) {
//	panic("MempoolLedger can not have the method `MempoolLedgerAt`")
//}
//
//var _ ILedger = (*MempoolLedger)(nil)
