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
	ret := &Supply{
		_proto: SupplyProto{
			Height:  height,
			XSupply: supply.Bytes(),
			XChange: change.Bytes(),
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

func (rwd *Supply) fromProto() {
	rwd.supply = new(uint256.Int).SetBytes(rwd._proto.XSupply)
	rwd.change = new(uint256.Int).SetBytes(rwd._proto.XChange)
}

func (rwd *Supply) toProto() {
	rwd._proto.XSupply = rwd.supply.Bytes()
	rwd._proto.XChange = rwd.change.Bytes()
}

func (rwd *Supply) Supply() *uint256.Int {
	return rwd.supply.Clone()
}

func (rwd *Supply) Change() *uint256.Int {
	return rwd.change.Clone()
}

func (rwd *Supply) Height() int64 {
	return rwd._proto.Height
}

var _ v1.ILedgerItem = (*Supply)(nil)
