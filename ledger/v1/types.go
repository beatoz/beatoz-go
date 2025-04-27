package v1

import (
	"bytes"
	"github.com/beatoz/beatoz-go/types"
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
	//Key() LedgerKey
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
var (
	KeyPrefixAccount    = []byte{0x00}
	KeyPrefixGovParams  = []byte{0x10}
	KeyPrefixProposal   = []byte{0x11}
	KeyPrefixFrozenProp = []byte{0x12}
	KeyPrefixDelegatee  = []byte{0x20}
	KeyPrefixVPower     = []byte{0x21}
)

func LedgerKeyProposal(txhash []byte) LedgerKey {
	_key := make([]byte, len(KeyPrefixProposal)+len(txhash))
	copy(_key, append(KeyPrefixProposal, txhash...))
	return _key
}

func LedgerKeyFrozenProp(txhash []byte) LedgerKey {
	_key := make([]byte, len(KeyPrefixFrozenProp)+len(txhash))
	copy(_key, append(KeyPrefixFrozenProp, txhash...))
	return _key
}

func LedgerKeyAccount(addr types.Address) LedgerKey {
	key := make([]byte, len(KeyPrefixAccount)+len(addr))
	copy(key, append(KeyPrefixAccount, addr...))
	return key
}

func LedgerKeyGovParams() LedgerKey {
	_key := make([]byte, len(KeyPrefixGovParams))
	copy(_key, KeyPrefixGovParams)
	return _key
}

func LedgerKeyVPower(k0, k1 []byte) LedgerKey {
	k := make([]byte, len(KeyPrefixVPower)+len(k0)+len(k1))
	copy(k, KeyPrefixVPower)
	copy(k[len(KeyPrefixVPower):], append(k0, k1...))
	return k
}

func LedgerKeyDelegatee(k0, k1 []byte) LedgerKey {
	k := make([]byte, len(KeyPrefixDelegatee)+len(k0)+len(k1))
	copy(k, KeyPrefixDelegatee)
	copy(k[len(KeyPrefixDelegatee):], append(k0, k1...))
	return k
}
