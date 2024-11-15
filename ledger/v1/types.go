package v1

import (
	"bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/cosmos/iavl"
	"sort"
)

type IGettable interface {
	Get(LedgerKey) (ILedgerItem, xerrors.XError)
	Iterate(cb func(ILedgerItem) xerrors.XError) xerrors.XError
}

type ISettable interface {
	Set(ILedgerItem) xerrors.XError
	Del(LedgerKey) xerrors.XError
	Snapshot() int
	RevertToSnapshot(int) xerrors.XError
}

type ICommittable interface {
	Commit() ([]byte, int64, xerrors.XError)
}

type IImitable interface {
	IGettable
	ISettable
}

type IMutable interface {
	IGettable
	ISettable
	ICommittable
	Version() int64
	GetReadOnlyTree(int64) (*iavl.ImmutableTree, xerrors.XError)
	Close() xerrors.XError
}

type IStateLedger[T ILedgerItem] interface {
	Version() int64
	Get(LedgerKey, bool) (T, xerrors.XError)
	Iterate(func(T) xerrors.XError, bool) xerrors.XError
	Set(T, bool) xerrors.XError
	Snapshot(bool) int
	RevertToSnapshot(int, bool) xerrors.XError
	Del(LedgerKey, bool) xerrors.XError
	Commit() ([]byte, int64, xerrors.XError)
	Close() xerrors.XError
	ImitableLedgerAt(int64) (IImitable, xerrors.XError)

	//RevertAll()
	//ApplyRevisions() xerrors.XError
	//ImitableLedgerAt(int64) (ILedger, xerrors.XError)
	//MempoolLedgerAt(int64) (ILedger, xerrors.XError)
}

//type ILedger interface {
//	Version() int64
//	Set(ILedgerItem) xerrors.XError
//	Get(LedgerKey) (ILedgerItem, xerrors.XError)
//	Del(LedgerKey) xerrors.XError
//	Iterate(cb func(ILedgerItem) xerrors.XError) xerrors.XError
//	Commit() ([]byte, int64, xerrors.XError)
//	Close() xerrors.XError
//
//	Snapshot() int
//	RevertToSnapshot(int) xerrors.XError
//	//RevertAll()
//	//ApplyRevisions() xerrors.XError
//	//ImitableLedgerAt(int64) (ILedger, xerrors.XError)
//	//MempoolLedgerAt(int64) (ILedger, xerrors.XError)
//}

type ILedgerItem interface {
	Key() LedgerKey
	Encode() ([]byte, xerrors.XError)
	Decode([]byte) xerrors.XError
}

type LedgerKey = []byte

//const LEDGERKEYSIZE = 32
//
//type LedgerKey = [LEDGERKEYSIZE]byte
//
//func ToLedgerKey(s []byte) LedgerKey {
//	var ret LedgerKey
//	n := len(s)
//	if n > LEDGERKEYSIZE {
//		n = LEDGERKEYSIZE
//	}
//	copy(ret[:], s[:n])
//	return ret
//}

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
