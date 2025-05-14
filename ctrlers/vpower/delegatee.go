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

type Delegatee struct {
	DelegateeProto
	key  v1.LedgerKey
	addr types.Address
}

func NewDelegatee(pubKey bytes.HexBytes) *Delegatee {
	ret := &Delegatee{
		DelegateeProto: DelegateeProto{
			PubKey: pubKey,
		},
	}
	ret.addr = crypto.PubKeyBytes2Addr(pubKey)
	ret.key = v1.LedgerKeyDelegatee(ret.addr)
	return ret
}

func (x *Delegatee) Encode() ([]byte, xerrors.XError) {
	d, err := proto.Marshal(x)
	if err != nil {
		return nil, xerrors.From(err)
	}
	return d, nil
}

func (x *Delegatee) Decode(k, v []byte) xerrors.XError {
	if err := proto.Unmarshal(v, x); err != nil {
		return xerrors.From(err)
	}
	x.key = k
	x.addr = crypto.PubKeyBytes2Addr(x.PubKey)
	return nil
}

var _ v1.ILedgerItem = (*Delegatee)(nil)

func (x *Delegatee) Address() types.Address {
	return x.addr
}

func (x *Delegatee) PublicKey() bytes.HexBytes {
	return x.PubKey
}

func (x *Delegatee) AddPower(from types.Address, pow int64) {
	x.addPower(from, pow)
}
func (x *Delegatee) addPower(from types.Address, pow int64) {
	x.SumPower += pow
	if bytes.Equal(from, x.addr) {
		x.SelfPower += pow
	}
}
func (x *Delegatee) DelPower(from types.Address, pow int64) {
	x.delPower(from, pow)
}
func (x *Delegatee) delPower(from types.Address, pow int64) {
	x.SumPower -= pow
	if bytes.Equal(from, x.addr) {
		x.SelfPower -= pow
	}
}

func (x *Delegatee) hasDelegator(from types.Address) bool {
	for _, d := range x.Delegators {
		if bytes.Equal(d, from) {
			return true
		}
	}
	return false
}
func (x *Delegatee) AddDelegator(from types.Address) {
	x.addDelegator(from)
}
func (x *Delegatee) addDelegator(from types.Address) {
	if !x.hasDelegator(from) {
		x.Delegators = append(x.Delegators, from)
	}
}

func (x *Delegatee) DelDelegator(from types.Address) {
	x.delDelegator(from)
}
func (x *Delegatee) delDelegator(from types.Address) {
	for i := len(x.Delegators) - 1; i >= 0; i-- {
		if bytes.Equal(x.Delegators[i], from) {
			x.Delegators = append(x.Delegators[:i], x.Delegators[i+1:]...)
			return
		}
	}
	return
}

func (x *Delegatee) Clone() *Delegatee {
	copiedDelegators := make([][]byte, len(x.Delegators))
	for i, inner := range x.Delegators {
		copiedDelegators[i] = make([]byte, len(inner))
		copy(copiedDelegators[i], inner)
	}

	return &Delegatee{
		DelegateeProto: DelegateeProto{
			PubKey:     bytes.Copy(x.PubKey),
			Delegators: copiedDelegators,
			SumPower:   x.SumPower,
			SelfPower:  x.SelfPower,
		},
		key:  bytes.Copy(x.key),
		addr: bytes.Copy(x.addr),
	}
}

func copyDelegateeArray(src []*Delegatee) []*Delegatee {
	dst := make([]*Delegatee, len(src))
	for i, d := range src {
		dst[i] = d.Clone()
	}
	return dst
}
func findDelegateeByAddr(addr types.Address, dgtees []*Delegatee) *Delegatee {
	for _, v := range dgtees {
		if bytes.Equal(v.addr, addr) {
			return v
		}
	}
	return nil
}

func findDelegateeByPubKey(pubKey bytes.HexBytes, dgtees []*Delegatee) *Delegatee {
	for _, v := range dgtees {
		if bytes.Equal(v.PubKey, pubKey) {
			return v
		}
	}
	return nil
}

type OrderByPowerDelegatees []*Delegatee

func (dgtees OrderByPowerDelegatees) Len() int {
	return len(dgtees)
}

// descending order by TotalPower
func (dgtees OrderByPowerDelegatees) Less(i, j int) bool {
	if dgtees[i].SumPower != dgtees[j].SumPower {
		return dgtees[i].SumPower > dgtees[j].SumPower
	}
	if dgtees[i].SelfPower != dgtees[j].SelfPower {
		return dgtees[i].SelfPower > dgtees[j].SelfPower
	}
	// when sorting by PubKey, ascending order is used
	if bytes.Compare(dgtees[i].PubKey, dgtees[j].PubKey) < 0 {
		return true
	}
	return false
}

func (dgtees OrderByPowerDelegatees) Swap(i, j int) {
	dgtees[i], dgtees[j] = dgtees[j], dgtees[i]
}

var _ sort.Interface = (OrderByPowerDelegatees)(nil)

type OrderByPubDelegatees []*Delegatee

func (dgtees OrderByPubDelegatees) Len() int {
	return len(dgtees)
}

// ascending order by address
func (dgtees OrderByPubDelegatees) Less(i, j int) bool {
	return bytes.Compare(dgtees[i].PubKey, dgtees[j].PubKey) < 0
}

func (dgtees OrderByPubDelegatees) Swap(i, j int) {
	dgtees[i], dgtees[j] = dgtees[j], dgtees[i]
}

var _ sort.Interface = (OrderByPubDelegatees)(nil)
