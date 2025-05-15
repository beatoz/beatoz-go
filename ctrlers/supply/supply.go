package supply

import (
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	"google.golang.org/protobuf/proto"
)

type Supply struct {
	_proto       SupplyProto
	totalSupply  *uint256.Int
	adjustSupply *uint256.Int
	changed      bool
}

func NewSupply() *Supply {
	ret := &Supply{}
	ret.fromProto()
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
	s.totalSupply = new(uint256.Int).SetBytes(s._proto.XTotalSupply)
	s.adjustSupply = new(uint256.Int).SetBytes(s._proto.XAdjustSupply)
}

func (s *Supply) toProto() {
	s._proto.XTotalSupply = s.totalSupply.Bytes()
	s._proto.XAdjustSupply = s.adjustSupply.Bytes()
}

func (s *Supply) GetTotalSupply() *uint256.Int {
	return s.totalSupply.Clone()
}

func (s *Supply) GetAdjustSupply() *uint256.Int {
	return s.adjustSupply.Clone()
}

func (s *Supply) GetHeight() int64 {
	return s._proto.Height
}

func (s *Supply) GetAdjustHeight() int64 {
	return s._proto.AdjustHeight
}

func (s *Supply) Add(height int64, amt *uint256.Int) {
	_ = s.totalSupply.Add(s.totalSupply, amt)
	s._proto.Height = height
	s.changed = true
}
func (s *Supply) Sub(height int64, amt *uint256.Int) {
	_ = s.totalSupply.Sub(s.totalSupply, amt)
	s._proto.Height = height
	s.changed = true
}

func (s *Supply) AdjustAdd(height int64, amt *uint256.Int) {
	s.Add(height, amt)
	s.adjustSupply = s.totalSupply.Clone()
	s._proto.AdjustHeight = height
}

func (s *Supply) AdjustSub(height int64, amt *uint256.Int) {
	s.Sub(height, amt)
	s.adjustSupply = s.totalSupply.Clone()
	s._proto.AdjustHeight = height
}
func (s *Supply) IsChanged() bool {
	return s.changed
}
func (s *Supply) ResetChanged() {
	s.changed = false
}

var _ v1.ILedgerItem = (*Supply)(nil)
