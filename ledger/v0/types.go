package v0

import (
	"bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"sort"
)

const LEDGERKEYSIZE = 32

type LedgerKey = [32]byte

func ToLedgerKey(s []byte) LedgerKey {
	var ret LedgerKey
	n := len(s)
	if n > LEDGERKEYSIZE {
		n = LEDGERKEYSIZE
	}
	copy(ret[:], s[:n])
	return ret
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

type LedgerStrKey = string

type ILedgerItem interface {
	Key() LedgerKey
	Encode() ([]byte, xerrors.XError)
	Decode([]byte) xerrors.XError
}

type ILedger[T ILedgerItem] interface {
	Version() int64
	Set(T) xerrors.XError
	CancelSet(LedgerKey) xerrors.XError
	Get(LedgerKey) (T, xerrors.XError)
	Del(LedgerKey) (T, xerrors.XError)
	CancelDel(LedgerKey) xerrors.XError
	Read(LedgerKey) (T, xerrors.XError)
	IterateReadAllItems(func(T) xerrors.XError) xerrors.XError
	IterateGotItems(func(T) xerrors.XError) xerrors.XError
	IterateUpdatedItems(func(T) xerrors.XError) xerrors.XError
	Commit() ([]byte, int64, xerrors.XError)
	Clone() ILedger[T]
	Close() xerrors.XError
}

type IFinalityLedger[T ILedgerItem] interface {
	ILedger[T]
	SetFinality(T) xerrors.XError
	CancelSetFinality(LedgerKey) xerrors.XError
	GetFinality(LedgerKey) (T, xerrors.XError)
	DelFinality(LedgerKey) (T, xerrors.XError)
	CancelDelFinality(LedgerKey) xerrors.XError
	IterateReadAllFinalityItems(func(T) xerrors.XError) xerrors.XError
	IterateFinalityGotItems(func(T) xerrors.XError) xerrors.XError
	IterateFinalityUpdatedItems(func(T) xerrors.XError) xerrors.XError
	ImmutableLedgerAt(int64, int) (ILedger[T], xerrors.XError)
}
