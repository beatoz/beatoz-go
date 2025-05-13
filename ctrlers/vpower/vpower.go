package vpower

import (
	"fmt"
	"github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"google.golang.org/protobuf/proto"
)

type VPower struct {
	VPowerProto
	from types.Address
	to   types.Address
	key  v1.LedgerKey
}

func NewVPower(from types.Address, to types.Address) *VPower {
	ret := &VPower{}
	ret.from = from
	ret.to = to
	ret.key = v1.LedgerKeyVPower(ret.from, ret.to)
	return ret
}

func (x *VPower) Encode() ([]byte, xerrors.XError) {
	if d, err := proto.Marshal(x); err != nil {
		return nil, xerrors.From(err)
	} else {
		return d, nil
	}
}

func (x *VPower) Decode(k, v []byte) xerrors.XError {
	if err := proto.Unmarshal(v, x); err != nil {
		return xerrors.From(err)
	}
	// k is `prefix + from_address + to_address`
	from_to := v1.UnwrapKeyPrefix(k)
	x.from = from_to[:20]
	x.to = from_to[20:]
	x.key = k
	return nil
}

var _ v1.ILedgerItem = (*VPower)(nil)

func (x *VPower) IsSelfPower() bool {
	return bytes.Equal(x.from, x.to)
}

func (x *VPower) findPowerChunk(txhash bytes.HexBytes) *PowerChunkProto {
	for _, pc := range x.PowerChunks {
		if bytes.Equal(pc.TxHash, txhash) {
			return pc
		}
	}
	return nil
}

// DEPRECATED
func (x *VPower) addPowerChunk(pow, height int64) *PowerChunkProto {
	added := &PowerChunkProto{Power: pow, Height: height}
	x.PowerChunks = append(x.PowerChunks, added)
	x.SumPower += added.Power
	return added
}

// DEPRECATED
func (x *VPower) delPowerChunk(idx int) *PowerChunkProto {
	removed := x.PowerChunks[idx]
	x.PowerChunks = append(x.PowerChunks[:idx], x.PowerChunks[idx+1:]...)
	x.SumPower -= removed.Power
	return removed
}

func (x *VPower) AddPowerWithTxHash(pow, height int64, txhash []byte) *PowerChunkProto {
	return x.addPowerWithTxHash(pow, height, txhash)
}

func (x *VPower) addPowerWithTxHash(pow, height int64, txhash []byte) *PowerChunkProto {
	added := &PowerChunkProto{Power: pow, Height: height, TxHash: txhash}
	x.PowerChunks = append(x.PowerChunks, added)
	x.SumPower += added.Power
	return added
}

func (x *VPower) DelPowerWithTxHash(txhash []byte) *PowerChunkProto {
	return x.delPowerWithTxHash(txhash)
}

func (x *VPower) delPowerWithTxHash(txhash []byte) *PowerChunkProto {
	for i, c := range x.PowerChunks {
		if bytes.Equal(txhash, c.TxHash) {
			return x.delPowerChunk(i)
		}
	}
	return nil
}

// sumPowerChunk is used for test
func (x *VPower) sumPowerChunk() int64 {
	ret := int64(0)
	for _, pc := range x.PowerChunks {
		ret += pc.Power
	}
	return ret
}

func (x *VPower) Clone() *VPower {
	copiedChunks := make([]*PowerChunkProto, len(x.PowerChunks))
	for i, c := range x.PowerChunks {
		copiedChunks[i] = &PowerChunkProto{Power: c.Power, Height: c.Height, TxHash: bytes.Copy(c.TxHash)}
	}
	return &VPower{
		VPowerProto: VPowerProto{
			SumPower:    x.SumPower,
			PowerChunks: copiedChunks,
		},
		to: bytes.Copy(x.to),
	}
}

func (x *VPower) String() string {
	pcstr := ""
	for _, pc := range x.PowerChunks {
		pcstr += fmt.Sprintf("[power:%v, height:%v, txhash:%x]", pc.Power, pc.Height, pc.TxHash)
	}
	return fmt.Sprintf("from:%v, to:%v, powerChunks: %v", x.from, x.to, pcstr)
}
