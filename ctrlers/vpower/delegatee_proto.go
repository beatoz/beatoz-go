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

type DelegateeV1 struct {
	DelegateeProto
	addr types.Address
	key  v1.LedgerKey
}

func newDelegateeV1(pubKey bytes.HexBytes) *DelegateeV1 {
	ret := &DelegateeV1{
		DelegateeProto: DelegateeProto{
			PubKey: pubKey,
		},
	}
	ret.addr = crypto.PubKeyBytes2Addr(pubKey)
	ret.key = v1.LedgerKeyDelegatee(ret.addr, nil)
	return ret
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
	x.key = v1.LedgerKeyDelegatee(x.addr, nil)
	return nil
}

func (x *DelegateeV1) addPower(from types.Address, pow int64) {
	x.SumPower += pow
	if bytes.Equal(from, x.addr) {
		x.SelfPower += pow
	}
}

func (x *DelegateeV1) delPower(from types.Address, pow int64) {
	x.SumPower -= pow
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
			PubKey:    bytes.Copy(x.PubKey),
			SumPower:  x.SumPower,
			SelfPower: x.SelfPower,
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
	if dgtees[i].SumPower != dgtees[j].SumPower {
		return dgtees[i].SumPower > dgtees[j].SumPower
	}
	if dgtees[i].SelfPower != dgtees[j].SelfPower {
		return dgtees[i].SelfPower > dgtees[j].SelfPower
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
