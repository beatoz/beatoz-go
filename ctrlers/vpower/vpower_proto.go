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
	prefixVPowerProto = "vp"
)

func vpowKey(prefix, k0, k1 []byte) v1.LedgerKey {
	k := make([]byte, len(prefix)+len(k0)+len(k1))
	copy(k, prefix)
	copy(k[len(prefix):], append(k0, k1...))
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
	return vpowKey([]byte(prefixVPowerProto), x.From, x.To)
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

func (x *VPowerProto) addPowerChunk(pow, height int64) {
	x.SumPower += pow
	x.PowerChunks = append(x.PowerChunks, &PowerChunk{Power: pow, Height: height})
}

func (x *VPowerProto) delPowerChunk(idx int) {
	x.SumPower -= x.PowerChunks[idx].Power
	x.PowerChunks = append(x.PowerChunks[:idx], x.PowerChunks[idx+1:]...)
}

func (x *VPowerProto) addPowerWithTxHash(pow, height int64, txhash []byte) {
	x.SumPower += pow
	x.PowerChunks = append(x.PowerChunks, &PowerChunk{Power: pow, Height: height, TxHash: txhash})
}

func (x *VPowerProto) delPowerWithTxHash(txhash []byte) {
	for i, c := range x.PowerChunks {
		if bytes.Equal(txhash, c.TxHash) {
			x.delPowerChunk(i)
			return
		}
	}
	return
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
