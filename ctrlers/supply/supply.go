package supply

import (
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	"google.golang.org/protobuf/proto"
)

/*
Height  int64  `protobuf:"varint,1,opt,name=height,proto3" json:"height,omitempty"`

	XSupply []byte `protobuf:"bytes,2,opt,name=_supply,json=Supply,proto3" json:"_supply,omitempty"`
	XChange []byte `protobuf:"bytes,3,opt,name=_change,json=Change,proto3" json:"_change,omitempty"`
	IsMint  bool   `protobuf:"varint,4,opt,name=is_mint,json=isMint,proto3" json:"is_mint,omitempty"`
*/
type Supply struct {
	_proto SupplyProto
	supply *uint256.Int
	change *uint256.Int
}

func NewSupply(height int64, supply, change *uint256.Int) *Supply {
	var xsupply, xchange []byte
	if supply != nil {
		xsupply = supply.Bytes()
	}
	if change != nil {
		xchange = change.Bytes()
	}
	ret := &Supply{
		_proto: SupplyProto{
			Height:  height,
			XSupply: xsupply,
			XChange: xchange,
		},
		supply: supply,
		change: change,
	}
	return ret
}

func (s *Supply) Encode() ([]byte, xerrors.XError) {
	s.toProto()
	if d, err := proto.Marshal(&s._proto); err != nil {
		return nil, xerrors.From(err)
	} else {
		return d, nil
	}
}

func (s *Supply) Decode(k, v []byte) xerrors.XError {
	if err := proto.Unmarshal(v, &s._proto); err != nil {
		return xerrors.From(err)
	}
	s.fromProto()
	return nil
}

func (s *Supply) fromProto() {
	s.supply = new(uint256.Int).SetBytes(s._proto.XSupply)
	s.change = new(uint256.Int).SetBytes(s._proto.XChange)
}

func (s *Supply) toProto() {
	s._proto.XSupply = s.supply.Bytes()
	s._proto.XChange = s.change.Bytes()
}

func (s *Supply) Supply() *uint256.Int {
	return s.supply.Clone()
}

func (s *Supply) Change() *uint256.Int {
	return s.change.Clone()
}

func (s *Supply) Height() int64 {
	return s._proto.Height
}

func (s *Supply) Mint(amt *uint256.Int) {
	_ = s.supply.Add(s.supply, amt)
	_ = s.change.Add(s.change, amt)
}

func (s *Supply) Burn(amt *uint256.Int) {
	_ = s.supply.Sub(s.supply, amt)
	_ = s.change.Add(s.change, amt)
}

var _ v1.ILedgerItem = (*Supply)(nil)
