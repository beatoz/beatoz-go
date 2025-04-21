package vpower

import (
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/stretchr/testify/require"
	"math/rand"
	"testing"
)

func Test_Codec(t *testing.T) {
	vpow := &VPower{
		VPowerProto: VPowerProto{
			SumPower: rand.Int63(),
			PowerChunks: []*PowerChunk{
				{Power: rand.Int63(), Height: rand.Int63(), TxHash: bytes.RandBytes(32)},
				{Power: rand.Int63(), Height: rand.Int63(), TxHash: bytes.RandBytes(32)},
				{Power: rand.Int63(), Height: rand.Int63(), TxHash: bytes.RandBytes(32)},
				{Power: rand.Int63(), Height: rand.Int63(), TxHash: bytes.RandBytes(32)},
				{Power: rand.Int63(), Height: rand.Int63(), TxHash: bytes.RandBytes(32)},
			},
		},
		from: types.RandAddress(),
		to:   types.RandAddress(),
	}
	k0 := vpow.Key()
	require.EqualValues(t, []byte(prefixVPowerProto), k0[:len(prefixDelegateeProto)])
	require.Equal(t, k0[len(prefixDelegateeProto):len(prefixDelegateeProto)+20], vpow.from)
	require.Equal(t, k0[len(prefixDelegateeProto)+20:], vpow.to)
}

func Test_Key(t *testing.T) {
	//h := rand.Int63()
	//k := frozenKey(h)
	//
	//expected := []byte(fmt.Sprintf("%s%d", prefixFrozenVPowerProto, h))
	//require.EqualValues(t, expected, k, "key", string(k))
	//
	//from := types.RandAddress()
	//to := types.RandAddress()
	//k = vpowerProtoKey(from, to)
	//expected = append([]byte(prefixVPowerProto), append(from, to...)...)
	//require.EqualValues(t, expected, k)
}
