package v1

import (
	"bytes"
	"fmt"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"slices"
)

type kvRevision struct {
	Delete bool
	Key    []byte
	Value  []byte
}

func newRevision(key, val []byte, del bool) *kvRevision {
	return &kvRevision{
		Delete: del,
		Key:    key,
		Value:  val,
	}
}

func (ledger *Ledger) Snapshot() int {
	ledger.mtx.RLock()
	defer ledger.mtx.RUnlock()

	return len(ledger.revisions)
}

func (ledger *Ledger) RevertToSnapshot(snap int) {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	ledger.revisions = ledger.revisions[0:snap]
}

func (ledger *Ledger) RevertAll() {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	ledger.revertAll()
}

func (ledger *Ledger) revertAll() {
	ledger.revisions = ledger.revisions[:0]
}

// Revert DEPRECATED
func (ledger *Ledger) Revert(key LedgerKey) {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	slices.DeleteFunc(ledger.revisions, func(rev *kvRevision) bool {
		return bytes.Compare(rev.Key, key[:]) == 0
	})
}

func (ledger *Ledger) revisionForSet(key, val []byte) {
	ledger.addRevision(newRevision(key, val, false))
}

func (ledger *Ledger) revisionForDel(key LedgerKey) {
	ledger.addRevision(newRevision(key[:], nil, true))
}

func (ledger *Ledger) addRevision(rev *kvRevision) {
	ledger.revisions = append(ledger.revisions, rev)
}

func (ledger *Ledger) findRevision(key LedgerKey) *kvRevision {
	for i := len(ledger.revisions) - 1; i >= 0; i-- {
		rev := ledger.revisions[i]
		if bytes.Compare(rev.Key, key[:]) == 0 {
			return rev
		}
	}
	//for _, rev := range ledger.revisions {
	//	if bytes.Compare(rev.Key, key[:]) == 0 {
	//		return rev
	//	}
	//}
	return nil
}

func (ledger *Ledger) ApplyRevisions() xerrors.XError {
	ledger.mtx.Lock()
	defer ledger.mtx.Unlock()

	return ledger.applyRevisions()
}

func (ledger *Ledger) applyRevisions() xerrors.XError {
	for _, rev := range ledger.revisions {
		if rev.Delete {
			_, removed, err := ledger.MutableTree.Remove(rev.Key)
			if !removed {
				return xerrors.From(fmt.Errorf("attempted to remove non-existent key %s", rev.Key))
			}
			if err != nil {
				return xerrors.From(err)
			}
		} else {
			if _, err := ledger.MutableTree.Set(rev.Key, rev.Value); err != nil {
				return xerrors.From(err)
			}
		}
	}
	ledger.revertAll()

	return nil
}
