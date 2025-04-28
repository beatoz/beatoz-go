package v1

import (
	"bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/cosmos/iavl"
	"sort"
)

type FuncNewItemFor func(LedgerKey) ILedgerItem
type FuncIterate func(LedgerKey, ILedgerItem) xerrors.XError

type IGettable interface {
	Get(LedgerKey) (ILedgerItem, xerrors.XError)
	Iterate(FuncIterate) xerrors.XError
	Seek([]byte, bool, FuncIterate) xerrors.XError
}

type ISettable interface {
	Set(LedgerKey, ILedgerItem) xerrors.XError
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

type IStateLedger interface {
	Version() int64
	Get(LedgerKey, bool) (ILedgerItem, xerrors.XError)
	Iterate(FuncIterate, bool) xerrors.XError
	Seek([]byte, bool, FuncIterate, bool) xerrors.XError
	Set(LedgerKey, ILedgerItem, bool) xerrors.XError
	Snapshot(bool) int
	RevertToSnapshot(int, bool) xerrors.XError
	Del(LedgerKey, bool) xerrors.XError
	Commit() ([]byte, int64, xerrors.XError)
	Close() xerrors.XError
	ImitableLedgerAt(int64) (IImitable, xerrors.XError)
}

type ILedgerItem interface {
	//Key() LedgerKey
	Encode() ([]byte, xerrors.XError)
	Decode([]byte) xerrors.XError
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
