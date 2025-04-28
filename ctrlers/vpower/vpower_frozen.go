package vpower

import (
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"google.golang.org/protobuf/proto"
)

type FrozenVPower struct {
	FrozenVPowerProto
}

func newFrozenVPower(power int64) *FrozenVPower {
	ret := &FrozenVPower{
		FrozenVPowerProto: FrozenVPowerProto{
			RefundPower: power,
		},
	}
	return ret
}

func (x *FrozenVPower) Encode() ([]byte, xerrors.XError) {
	if d, err := proto.Marshal(x); err != nil {
		return nil, xerrors.From(err)
	} else {
		return d, nil
	}
}

func (x *FrozenVPower) Decode(d []byte) xerrors.XError {
	if err := proto.Unmarshal(d, x); err != nil {
		return xerrors.From(err)
	}
	return nil
}

func (x *FrozenVPower) appendPowerChunks(powChunks ...*PowerChunkProto) {
	for _, pc := range powChunks {
		x.RefundPower += pc.Power
		x.PowerChunks = append(x.PowerChunks, pc)
	}
}

var _ v1.ILedgerItem = (*FrozenVPower)(nil)
