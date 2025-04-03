package vpower

import (
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/crypto"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"google.golang.org/protobuf/proto"
)

var (
	prefixDelegateeProto = "dg"
)

func newDelegateeProto(pub bytes.HexBytes) *DelegateeProto {
	return &DelegateeProto{
		PubKey: pub,
	}
}

func (x *DelegateeProto) Key() v1.LedgerKey {
	return append([]byte(prefixDelegateeProto), crypto.PubKeyBytes2Addr(x.PubKey)...)
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
}

func (x *DelegateeProto) DelPower(from types.Address, pow int64) {
	x.TotalPower -= pow
}
