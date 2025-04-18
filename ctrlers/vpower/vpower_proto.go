package vpower

import (
	"fmt"
	"github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"google.golang.org/protobuf/proto"
)

var (
	prefixVPowerProto       = "vp"
	prefixFrozenVPowerProto = "fz"
)

func vpowerProtoKey(k0, k1 []byte) v1.LedgerKey {
	k := make([]byte, len(prefixVPowerProto)+len(k0)+len(k1))
	copy(k, prefixVPowerProto)
	copy(k[len(prefixVPowerProto):], append(k0, k1...))
	return k
}

func newVPower(from, to types.Address, pow, height int64) *VPowerProto {
	ret := &VPowerProto{
		From: from,
		To:   to,
	}

	if pow > 0 && height > 0 {
		ret.addPowerChunk(pow, height)
	}

	return ret
}

func newVPowerWithTxHash(from, to types.Address, pow, height int64, txhash []byte) *VPowerProto {
	ret := &VPowerProto{
		From: from,
		To:   to,
	}

	if pow > 0 && height > 0 && len(txhash) > 0 {
		ret.addPowerWithTxHash(pow, height, txhash)
	} else {
		panic(fmt.Errorf("negative. from:%v, to:%v, pow:%v, height:%v, txhash:%x", from, to, pow, height, txhash))
	}

	return ret
}

func (x *VPowerProto) Key() v1.LedgerKey {
	return vpowerProtoKey(x.From, x.To)
}

func (x *VPowerProto) Encode() ([]byte, xerrors.XError) {
	if d, err := proto.Marshal(x); err != nil {
		return nil, xerrors.From(err)
	} else {
		return d, nil
	}
}

func (x *VPowerProto) Decode(d []byte) xerrors.XError {
	if err := proto.Unmarshal(d, x); err != nil {
		return xerrors.From(err)
	}
	return nil
}

var _ v1.ILedgerItem = (*VPowerProto)(nil)

func (x *VPowerProto) IsSelfPower() bool {
	return bytes.Equal(x.From, x.To)
}

func (x *VPowerProto) findPowerChunk(txhash bytes.HexBytes) *PowerChunk {
	for _, pc := range x.PowerChunks {
		if bytes.Equal(pc.TxHash, txhash) {
			return pc
		}
	}
	return nil
}

func (x *VPowerProto) addPowerChunk(pow, height int64) *PowerChunk {
	added := &PowerChunk{Power: pow, Height: height}
	x.PowerChunks = append(x.PowerChunks, added)
	x.SumPower += added.Power
	return added
}

func (x *VPowerProto) delPowerChunk(idx int) *PowerChunk {
	removed := x.PowerChunks[idx]
	x.PowerChunks = append(x.PowerChunks[:idx], x.PowerChunks[idx+1:]...)
	x.SumPower -= removed.Power
	return removed
}

func (x *VPowerProto) addPowerWithTxHash(pow, height int64, txhash []byte) *PowerChunk {
	added := &PowerChunk{Power: pow, Height: height, TxHash: txhash}
	x.PowerChunks = append(x.PowerChunks, added)
	x.SumPower += added.Power
	return added
}

func (x *VPowerProto) delPowerWithTxHash(txhash []byte) *PowerChunk {
	for i, c := range x.PowerChunks {
		if bytes.Equal(txhash, c.TxHash) {
			return x.delPowerChunk(i)
		}
	}
	return nil
}

// sumPowerChunk is used for test
func (x *VPowerProto) sumPowerChunk() int64 {
	ret := int64(0)
	for _, pc := range x.PowerChunks {
		ret += pc.Power
	}
	return ret
}

func (x *VPowerProto) Clone() *VPowerProto {
	copiedChunks := make([]*PowerChunk, len(x.PowerChunks))
	for i, c := range x.PowerChunks {
		copiedChunks[i] = &PowerChunk{Power: c.Power, Height: c.Height, TxHash: bytes.Copy(c.TxHash)}
	}
	return &VPowerProto{
		From:        bytes.Copy(x.From),
		To:          bytes.Copy(x.To),
		PowerChunks: copiedChunks,
	}
}
