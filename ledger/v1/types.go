package v1

import (
	"bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"sort"
)

type ILedgerItem interface {
	Key() LedgerKey
	Encode() ([]byte, xerrors.XError)
	Decode([]byte) xerrors.XError
}

type ILedger interface {
	Version() int64
	Set(ILedgerItem) xerrors.XError
	Get(LedgerKey) (ILedgerItem, xerrors.XError)
	Del(LedgerKey)
	Iterate(cb func(ILedgerItem) xerrors.XError) xerrors.XError
	Commit() ([]byte, int64, xerrors.XError)
	Close() xerrors.XError

	Snapshot() int
	RevertToSnapshot(snap int)
	RevertAll()
	ApplyRevisions() xerrors.XError
	ImmutableLedgerAt(int64) (ILedger, xerrors.XError)
	MempoolLedgerAt(int64) (ILedger, xerrors.XError)
}

type LedgerKey = []byte
type LedgerKeyList []LedgerKey

func (a LedgerKeyList) Len() int {
	return len(a)
}
func (a LedgerKeyList) Less(i, j int) bool {
	ret := bytes.Compare(a[i][:], a[j][:])
	return ret > 0
}
func (a LedgerKeyList) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

var _ sort.Interface = LedgerKeyList(nil)
