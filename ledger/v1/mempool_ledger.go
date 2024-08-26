package v1

import (
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/cosmos/iavl"
	dbm "github.com/cosmos/iavl/db"
	"sync"
)

// MempoolLedger cannot be committed, everything else is like Ledger.
type MempoolLedger struct {
	*Ledger
	mtx sync.RWMutex
}

func NewMempoolLedger(db dbm.DB, cacheSize int, newItem func() ILedgerItem, ver int64) (*MempoolLedger, xerrors.XError) {
	tree := iavl.NewMutableTree(db, cacheSize, false, iavl.NewNopLogger())
	if _, err := tree.LoadVersion(ver); err != nil {
		return nil, xerrors.From(err)
	}

	return &MempoolLedger{
		Ledger: &Ledger{
			MutableTree: tree,
			// Do not set the `db` field
			// This field is only set in the original Ledger instance.
			// Because the 'db' field is nil, MempoolLedger.Close() can not close the 'db'.
			// The `db` can only be closed by the original Ledger.Close().
			db:          nil,
			cacheSize:   cacheSize,
			newItemFunc: newItem,
			revisions:   make([]*kvRevision, 0),
		},
	}, nil
}

func (ledger *MempoolLedger) Commit() ([]byte, int64, xerrors.XError) {
	panic("MempoolLedger can not have the method `Commit`")
}

func (ledger *MempoolLedger) ImmutableLedgerAt(ver int64) (ILedger, xerrors.XError) {
	panic("MempoolLedger can not have the method `ImmutableLedgerAt`")
}

func (ledger *MempoolLedger) MempoolLedgerAt(ver int64) (ILedger, xerrors.XError) {
	panic("MempoolLedger can not have the method `MempoolLedgerAt`")
}

var _ ILedger = (*MempoolLedger)(nil)
