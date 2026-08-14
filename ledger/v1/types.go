package v1

import (
	"bytes"
	"github.com/beatoz/beatoz-go/ledger/common"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/cosmos/iavl"
	"sort"
)

type LedgerKey = common.LedgerKey
type ILedgerItem = common.ILedgerItem
type FuncNewItemFor = common.FuncNewItemFor
type FuncIterate = common.FuncIterate

type IGettable interface {
	Get(LedgerKey) (ILedgerItem, xerrors.XError)
	Iterate(FuncIterate) xerrors.XError
	Seek([]byte, bool, FuncIterate) xerrors.XError
}

type ISettable interface {
	Set(LedgerKey, ILedgerItem) xerrors.XError
	Del(LedgerKey) xerrors.XError
}

type ICommittable interface {
	Commit() ([]byte, int64, xerrors.XError)
}

type IImitable = common.IImitable

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
	Del(LedgerKey, bool) xerrors.XError
	CreateCache(bool) xerrors.XError
	WriteCache(bool) xerrors.XError
	ClearCache(bool) xerrors.XError
	Commit() ([]byte, int64, xerrors.XError)
	Close() xerrors.XError
	ImitableLedgerAt(int64) (IImitable, xerrors.XError)
}

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
