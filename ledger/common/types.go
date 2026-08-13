package common

import "github.com/beatoz/beatoz-go/types/xerrors"

type LedgerVersion uint8

const (
	LedgerNone LedgerVersion = iota
	LedgerV1
	LedgerV2
)

type LedgerKey = []byte

type ILedgerItem interface {
	Encode() ([]byte, xerrors.XError)
	Decode([]byte, []byte) xerrors.XError
}

type FuncNewItemFor func(LedgerKey) ILedgerItem
type FuncIterate func(LedgerKey, ILedgerItem) xerrors.XError

type IImitable interface {
	Get(LedgerKey) (ILedgerItem, xerrors.XError)
	Iterate(FuncIterate) xerrors.XError
	Seek([]byte, bool, FuncIterate) xerrors.XError
	Set(LedgerKey, ILedgerItem) xerrors.XError
	Del(LedgerKey) xerrors.XError
}
