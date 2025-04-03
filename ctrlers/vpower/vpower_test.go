package vpower

import (
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/stretchr/testify/require"
	"math/rand"
	"testing"
)

func Test_Codec(t *testing.T) {
	vpow := &VPowerProto{
		From:     types.RandAddress(),
		To:       types.RandAddress(),
		SumPower: 100,
		PowerChunks: []*PowerChunk{
			{Power: rand.Int63(), Height: rand.Int63(), TxHash: bytes.RandBytes(32)},
			{Power: rand.Int63(), Height: rand.Int63(), TxHash: bytes.RandBytes(32)},
			{Power: rand.Int63(), Height: rand.Int63(), TxHash: bytes.RandBytes(32)},
			{Power: rand.Int63(), Height: rand.Int63(), TxHash: bytes.RandBytes(32)},
			{Power: rand.Int63(), Height: rand.Int63(), TxHash: bytes.RandBytes(32)},
		},
	}
	k0 := vpow.Key()
	require.EqualValues(t, []byte(prefixVPowerProto), k0[:len(prefixDelegateeProto)])
	require.Equal(t, k0[len(prefixDelegateeProto):len(prefixDelegateeProto)+20], vpow.From)
	require.Equal(t, k0[len(prefixDelegateeProto)+20:], vpow.To)
}
