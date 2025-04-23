package vpower

import (
	"fmt"
	"github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/crypto"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"google.golang.org/protobuf/proto"
)

var (
	prefixVPowerProto       = "vp"
	prefixFrozenVPowerProto = "fz"
)

type VPower struct {
	VPowerProto
	to types.Address
}

func vpowerProtoKey(k0, k1 []byte) v1.LedgerKey {
	k := make([]byte, len(prefixVPowerProto)+len(k0)+len(k1))
	copy(k, prefixVPowerProto)
	copy(k[len(prefixVPowerProto):], append(k0, k1...))
	return k
}

func newVPower(from types.Address, pubKey bytes.HexBytes) *VPower {
	ret := &VPower{
		VPowerProto: VPowerProto{
			From:     from,
			PubKeyTo: pubKey,
		},
		to: crypto.PubKeyBytes2Addr(pubKey),
	}

	return ret
}

func (x *VPower) Key() v1.LedgerKey {
	return vpowerProtoKey(x.From, x.to)
}

func (x *VPower) Encode() ([]byte, xerrors.XError) {
	if d, err := proto.Marshal(x); err != nil {
		return nil, xerrors.From(err)
	} else {
		return d, nil
	}
}

func (x *VPower) Decode(d []byte) xerrors.XError {
	if err := proto.Unmarshal(d, x); err != nil {
		return xerrors.From(err)
	}
	x.to = crypto.PubKeyBytes2Addr(x.PubKeyTo)
	return nil
}

var _ v1.ILedgerItem = (*VPower)(nil)

func (x *VPower) IsSelfPower() bool {
	return bytes.Equal(x.From, x.to)
}

func (x *VPower) findPowerChunk(txhash bytes.HexBytes) *PowerChunk {
	for _, pc := range x.PowerChunks {
		if bytes.Equal(pc.TxHash, txhash) {
			return pc
		}
	}
	return nil
}

func (x *VPower) addPowerChunk(pow, height int64) *PowerChunk {
	added := &PowerChunk{Power: pow, Height: height}
	x.PowerChunks = append(x.PowerChunks, added)
	x.SumPower += added.Power
	return added
}

func (x *VPower) delPowerChunk(idx int) *PowerChunk {
	removed := x.PowerChunks[idx]
	x.PowerChunks = append(x.PowerChunks[:idx], x.PowerChunks[idx+1:]...)
	x.SumPower -= removed.Power
	return removed
}

func (x *VPower) addPowerWithTxHash(pow, height int64, txhash []byte) *PowerChunk {
	added := &PowerChunk{Power: pow, Height: height, TxHash: txhash}
	x.PowerChunks = append(x.PowerChunks, added)
	x.SumPower += added.Power
	return added
}

func (x *VPower) delPowerWithTxHash(txhash []byte) *PowerChunk {
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
	copiedChunks := make([]*PowerChunk, len(x.PowerChunks))
	for i, c := range x.PowerChunks {
		copiedChunks[i] = &PowerChunk{Power: c.Power, Height: c.Height, TxHash: bytes.Copy(c.TxHash)}
	}
	return &VPower{
		VPowerProto: VPowerProto{
			From:        bytes.Copy(x.From),
			PubKeyTo:    bytes.Copy(x.PubKeyTo),
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
	return fmt.Sprintf("from:%v, to:%v, powerChunks: %v", x.From, x.to, pcstr)
}
