package v2

import (
	"github.com/beatoz/beatoz-go/ledger/common"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/cosmos/iavl"
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

type ICacheable interface {
	createCache() xerrors.XError
	writeCache() xerrors.XError
	clearCache() xerrors.XError
}

type IImitable interface {
	IGettable
	ISettable
	ICacheable
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
	Del(LedgerKey, bool) xerrors.XError
	CreateCache(bool) xerrors.XError
	WriteCache(bool) xerrors.XError
	ClearCache(bool) xerrors.XError
	Commit() ([]byte, int64, xerrors.XError)
	Close() xerrors.XError
	ImitableLedgerAt(int64) (common.IImitable, xerrors.XError)
}
