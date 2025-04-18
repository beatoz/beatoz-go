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

func dgteeProtoKey(addr types.Address) v1.LedgerKey {
	return append([]byte(prefixDelegateeProto), addr...)
}
func newDelegateeProto(pub bytes.HexBytes) *DelegateeProto {
	return &DelegateeProto{
		PubKey: pub,
	}
}

func (x *DelegateeProto) Key() v1.LedgerKey {
	return dgteeProtoKey(x.Address())
}

func (x *DelegateeProto) Encode() ([]byte, xerrors.XError) {
	d, err := proto.Marshal(x)
	if err != nil {
		return nil, xerrors.From(err)
	}
	return d, nil
}

func (x *DelegateeProto) Decode(d []byte) xerrors.XError {
	if err := proto.Unmarshal(d, x); err != nil {
		return xerrors.From(err)
	}
	return nil
}

func (x *DelegateeProto) Address() types.Address {
	return crypto.PubKeyBytes2Addr(x.PubKey)
}

func (x *DelegateeProto) AddPower(from types.Address, pow int64) {
	x.TotalPower += pow
	if bytes.Equal(from, x.Address()) {
		x.SelfPower += pow
	}
}

func (x *DelegateeProto) DelPower(from types.Address, pow int64) {
	x.TotalPower -= pow
	if bytes.Equal(from, x.Address()) {
		x.SelfPower -= pow
	}
}

func (x *DelegateeProto) Clone() *DelegateeProto {
	return &DelegateeProto{
		PubKey:      bytes.Copy(x.PubKey),
		TotalPower:  x.TotalPower,
		SelfPower:   x.SelfPower,
		MaturePower: x.MaturePower,
	}
}

func copyDelegateeProtoArray(src []*DelegateeProto) []*DelegateeProto {
	dst := make([]*DelegateeProto, len(src))
	for i, d := range src {
		dst[i] = d.Clone()
	}
	return dst
}
func findDelegateeProtoByAddr(addr types.Address, dgtees []*DelegateeProto) *DelegateeProto {
	for _, v := range dgtees {
		if bytes.Equal(v.Address(), addr) {
			return v
		}
	}
	return nil
}

func findDelegateeProtoByPubKey(pubKey bytes.HexBytes, dgtees []*DelegateeProto) *DelegateeProto {
	for _, v := range dgtees {
		if bytes.Equal(v.PubKey, pubKey) {
			return v
		}
	}
	return nil
}

type orderByPowerDelegateeProtos []*DelegateeProto

func (dgtees orderByPowerDelegateeProtos) Len() int {
	return len(dgtees)
}

// descending order by TotalPower
func (dgtees orderByPowerDelegateeProtos) Less(i, j int) bool {
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

func (dgtees orderByPowerDelegateeProtos) Swap(i, j int) {
	dgtees[i], dgtees[j] = dgtees[j], dgtees[i]
}

var _ sort.Interface = (orderByPowerDelegateeProtos)(nil)

type orderByPubDelegateeProtos []*DelegateeProto

func (dgtees orderByPubDelegateeProtos) Len() int {
	return len(dgtees)
}

// ascending order by address
func (dgtees orderByPubDelegateeProtos) Less(i, j int) bool {
	return bytes.Compare(dgtees[i].PubKey, dgtees[j].PubKey) < 0
}

func (dgtees orderByPubDelegateeProtos) Swap(i, j int) {
	dgtees[i], dgtees[j] = dgtees[j], dgtees[i]
}

var _ sort.Interface = (orderByPubDelegateeProtos)(nil)
