package v1

import (
	"sync/atomic"

	"github.com/beatoz/beatoz-go/types/xerrors"
)

type kvPair[T any] struct {
	key []byte
	val T
}

type revisionList[T any] struct {
	revs       []*kvPair[T]
	ledgerID   uint64
	generation uint64
}

// nextLedgerID issues process-local IDs used only to reject snapshots
// created by a different ledger instance.
var nextLedgerID uint64

func newSnapshotList[T any]() *revisionList[T] {
	return &revisionList[T]{
		revs:       make([]*kvPair[T], 0),
		ledgerID:   atomic.AddUint64(&nextLedgerID, 1),
		generation: 1,
	}
}

func (revlist *revisionList[T]) set(key []byte, val T) {
	revlist.revs = append(revlist.revs, &kvPair[T]{
		key: key,
		val: val,
	})
}

func (revlist revisionList[T]) snapshot() Snapshot {
	return Snapshot{
		ledgerID:   revlist.ledgerID,
		generation: revlist.generation,
		revision:   len(revlist.revs),
	}
}

func (revlist revisionList[T]) validateSnapshot(snap Snapshot) xerrors.XError {
	if snap.ledgerID != revlist.ledgerID {
		return xerrors.ErrInvalidSnapshot.Wrapf(
			"ledger ID mismatch: snapshot=%d, ledger=%d",
			snap.ledgerID,
			revlist.ledgerID,
		)
	}
	if snap.generation != revlist.generation {
		return xerrors.ErrInvalidSnapshot.Wrapf(
			"generation mismatch: snapshot=%d, ledger=%d",
			snap.generation,
			revlist.generation,
		)
	}
	if snap.revision < 0 || snap.revision > len(revlist.revs) {
		return xerrors.ErrInvalidSnapshot.Wrapf(
			"revision out of range: snapshot=%d, revisions=%d",
			snap.revision,
			len(revlist.revs),
		)
	}
	return nil
}

func (revlist *revisionList[T]) revert(snap Snapshot) {
	revlist.revs = revlist.revs[:snap.revision]
}

func (revlist *revisionList[T]) reset() {
	revlist.revs = revlist.revs[:0]
	revlist.generation++
}

func (revlist revisionList[T]) iterate(cb func(idx int, kv *kvPair[T]) bool) {
	for i, kv := range revlist.revs {
		if cb(i, kv) == false {
			break
		}
	}
}
