package v1

import (
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/cosmos/iavl"
	dbm "github.com/cosmos/iavl/db"
	tmlog "github.com/tendermint/tendermint/libs/log"
	"sync"
)

// MempoolLedger cannot be committed, everything else is like MutableLedger.
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
			MutableTree: tree,
			// Do not set the `db` field
			// This field is only add in the original MutableLedger instance.
			// Because the 'db' field is nil, MempoolLedger.Close() can not close the 'db'.
			// The `db` can only be closed by the original MutableLedger.Close().
			db:          nil,
			cacheSize:   cacheSize,
			newItemFunc: newItem,
			cachedItems: make(map[string]ILedgerItem),
			snapshots:   NewSnapshotList(),
			logger:      lg,
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
