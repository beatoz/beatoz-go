package supply

import (
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"google.golang.org/protobuf/proto"
)

type Supply struct {
	SupplyProto
	key v1.LedgerKey
}

func (s *Supply) Encode() ([]byte, xerrors.XError) {
	if d, err := proto.Marshal(s); err != nil {
		return nil, xerrors.From(err)
	} else {
		return d, nil
	}
}

func (s *Supply) Decode(k, v []byte) xerrors.XError {
	if err := proto.Unmarshal(v, s); err != nil {
		return xerrors.From(err)
	}
	s.key = k
	return nil
}

var _ v1.ILedgerItem = (*Supply)(nil)
