package vpower

import (
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/crypto"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"google.golang.org/protobuf/proto"
	"sort"
)

var (
	prefixDelegateeProto = "dg"
)

type DelegateeV1 struct {
	DelegateeProto
	addr types.Address
}

func dgteeProtoKey(addr types.Address) v1.LedgerKey {
	return append([]byte(prefixDelegateeProto), addr...)
}

func newDelegateeV1(pubKey bytes.HexBytes) *DelegateeV1 {
	return &DelegateeV1{
		DelegateeProto: DelegateeProto{
			PubKey: pubKey,
		},
		addr: crypto.PubKeyBytes2Addr(pubKey),
	}
}

func LoadAllDelegateeV1(ledger v1.IStateLedger[*DelegateeV1]) ([]*DelegateeV1, xerrors.XError) {
	var dgtees []*DelegateeV1
	if xerr := ledger.Iterate(func(elem *DelegateeV1) xerrors.XError {
		elem.addr = crypto.PubKeyBytes2Addr(elem.PubKey)
		dgtees = append(dgtees, elem)
		return nil
	}, true); xerr != nil {
		return nil, xerr
	}
	return dgtees, nil
}

func (x *DelegateeV1) Key() v1.LedgerKey {
	return dgteeProtoKey(x.addr)
}

func (x *DelegateeV1) Encode() ([]byte, xerrors.XError) {
	d, err := proto.Marshal(x)
	if err != nil {
		return nil, xerrors.From(err)
	}
	return d, nil
}

func (x *DelegateeV1) Decode(d []byte) xerrors.XError {
	if err := proto.Unmarshal(d, x); err != nil {
		return xerrors.From(err)
	}
	x.addr = crypto.PubKeyBytes2Addr(x.PubKey)
	return nil
}

func (x *DelegateeV1) addPower(from types.Address, pow int64) {
	x.TotalPower += pow
	if bytes.Equal(from, x.addr) {
		x.SelfPower += pow
	}
}

func (x *DelegateeV1) delPower(from types.Address, pow int64) {
	x.TotalPower -= pow
	if bytes.Equal(from, x.addr) {
		x.SelfPower -= pow
	}
}

func (x *DelegateeV1) hasDelegator(from types.Address) bool {
	for _, d := range x.Delegators {
		if bytes.Equal(d, from) {
			return true
		}
	}
	return false
}
func (x *DelegateeV1) addDelegator(from types.Address) {
	if !x.hasDelegator(from) {
		x.Delegators = append(x.Delegators, from)
	}
}
func (x *DelegateeV1) delDelegator(from types.Address) {
	for i := len(x.Delegators) - 1; i >= 0; i-- {
		if bytes.Equal(x.Delegators[i], from) {
			x.Delegators = append(x.Delegators[:i], x.Delegators[i+1:]...)
			return
		}
	}
	return
}

func (x *DelegateeV1) Clone() *DelegateeV1 {
	return &DelegateeV1{
		DelegateeProto: DelegateeProto{
			PubKey:      bytes.Copy(x.PubKey),
			TotalPower:  x.TotalPower,
			SelfPower:   x.SelfPower,
			MaturePower: x.MaturePower,
		},
		addr: bytes.Copy(x.addr),
	}
}

func copyDelegateeV1Array(src []*DelegateeV1) []*DelegateeV1 {
	dst := make([]*DelegateeV1, len(src))
	for i, d := range src {
		dst[i] = d.Clone()
	}
	return dst
}
func findDelegateeV1ByAddr(addr types.Address, dgtees []*DelegateeV1) *DelegateeV1 {
	for _, v := range dgtees {
		if bytes.Equal(v.addr, addr) {
			return v
		}
	}
	return nil
}

func findDelegateeV1ByPubKey(pubKey bytes.HexBytes, dgtees []*DelegateeV1) *DelegateeV1 {
	for _, v := range dgtees {
		if bytes.Equal(v.PubKey, pubKey) {
			return v
		}
	}
	return nil
}

type orderByPowerDelegateeV1 []*DelegateeV1

func (dgtees orderByPowerDelegateeV1) Len() int {
	return len(dgtees)
}

// descending order by TotalPower
func (dgtees orderByPowerDelegateeV1) Less(i, j int) bool {
	if dgtees[i].TotalPower != dgtees[j].TotalPower {
		return dgtees[i].TotalPower > dgtees[j].TotalPower
	}
	if dgtees[i].SelfPower != dgtees[j].SelfPower {
		return dgtees[i].SelfPower > dgtees[j].SelfPower
	}
	if dgtees[i].MaturePower != dgtees[j].MaturePower {
		return dgtees[i].MaturePower > dgtees[j].MaturePower
	}
	if bytes.Compare(dgtees[i].PubKey, dgtees[j].PubKey) > 0 {
		return true
	}
	return false
}

func (dgtees orderByPowerDelegateeV1) Swap(i, j int) {
	dgtees[i], dgtees[j] = dgtees[j], dgtees[i]
}

var _ sort.Interface = (orderByPowerDelegateeV1)(nil)

type orderByPubDelegateeV1 []*DelegateeV1

func (dgtees orderByPubDelegateeV1) Len() int {
	return len(dgtees)
}

// ascending order by address
func (dgtees orderByPubDelegateeV1) Less(i, j int) bool {
	return bytes.Compare(dgtees[i].PubKey, dgtees[j].PubKey) < 0
}

func (dgtees orderByPubDelegateeV1) Swap(i, j int) {
	dgtees[i], dgtees[j] = dgtees[j], dgtees[i]
}

var _ sort.Interface = (orderByPubDelegateeV1)(nil)
