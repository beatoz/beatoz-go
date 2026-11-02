package rpc

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/big"
	"runtime"
	"strings"
	"testing"

	"github.com/beatoz/beatoz-go/cmd/version"
	"github.com/stretchr/testify/require"
)

func TestEthQuantities(t *testing.T) {
	for _, tt := range []struct {
		n    uint64
		want string
	}{
		{0, "0x0"},
		{1, "0x1"},
		{2, "0x2"},
		{15, "0xf"},
		{16, "0x10"},
		{255, "0xff"},
		{math.MaxInt64, "0x7fffffffffffffff"},
		{math.MaxUint64, "0xffffffffffffffff"},
	} {
		t.Run(tt.want, func(t *testing.T) {
			require.Equal(t, tt.want, ethQuantityUint64(tt.n))
			got, err := ethQuantityBig(new(big.Int).SetUint64(tt.n))
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
			if tt.n <= math.MaxInt64 {
				got, err = ethQuantityInt64(int64(tt.n))
				require.NoError(t, err)
				require.Equal(t, tt.want, got)
			}
		})
	}

	n := new(big.Int).SetBytes(bytes.Repeat([]byte{0xff}, 32))
	got, err := ethQuantityBig(n)
	require.NoError(t, err)
	require.Equal(t, "0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", got)
	_, err = ethQuantityInt64(-1)
	require.Error(t, err)
	_, err = ethQuantityInt64(math.MinInt64)
	require.Error(t, err)
	_, err = ethQuantityBig(big.NewInt(-1))
	require.Error(t, err)
	_, err = ethQuantityBig(nil)
	require.Error(t, err)
}

func TestParseEthBlockTag(t *testing.T) {
	for _, tag := range []string{"latest", "pending", "safe", "finalized", "earliest"} {
		t.Run(tag, func(t *testing.T) {
			height, earliest, err := parseEthBlockTag(tag)
			require.NoError(t, err)
			require.Nil(t, height)
			require.Equal(t, tag == "earliest", earliest)
		})
	}
	for _, tt := range []struct {
		tag  string
		want int64
	}{
		{"0x0", 0},
		{"0x1", 1},
		{"0x1234", 4660},
		{"0xaB", 171},
		{"0x7fffffffffffffff", math.MaxInt64},
	} {
		t.Run(tt.tag, func(t *testing.T) {
			height, earliest, err := parseEthBlockTag(tt.tag)
			require.NoError(t, err)
			require.NotNil(t, height)
			require.Equal(t, tt.want, *height)
			require.False(t, earliest)
		})
	}
	for _, tag := range []string{
		"", "0", "123", "LATEST", "unknown", "0x", "0X1", "0x01", "0x00",
		"0xg", "0x1g", "0x-1", "0x+1", "-0x1", "0x1_0", " 0x1", "0x1 ",
		"0x8000000000000000", "0xffffffffffffffff", "0x10000000000000000",
	} {
		t.Run("invalid/"+tag, func(t *testing.T) {
			height, earliest, err := parseEthBlockTag(tag)
			require.ErrorContains(t, err, "invalid Ethereum block tag")
			require.Nil(t, height)
			require.False(t, earliest)
		})
	}
}

func TestFormatEthBlockNumber(t *testing.T) {
	for _, tt := range []struct {
		height uint64
		want   string
	}{
		{0, "0x0"},
		{2, "0x2"},
		{16, "0x10"},
		{math.MaxUint64, "0xffffffffffffffff"},
	} {
		t.Run(tt.want, func(t *testing.T) {
			value := make([]byte, 8)
			binary.BigEndian.PutUint64(value, tt.height)
			got, err := formatEthBlockNumber(value)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
	for _, size := range []int{0, 1, 7, 9} {
		_, err := formatEthBlockNumber(make([]byte, size))
		require.Error(t, err)
	}
}

func TestFormatNetVersion(t *testing.T) {
	for _, tt := range []struct {
		chainID []byte
		want    string
	}{
		{nil, "0"},
		{[]byte{0}, "0"},
		{[]byte{0x12, 0x34}, "4660"},
		{[]byte{0, 0x12, 0x34}, "4660"},
		{bytes.Repeat([]byte{0xff}, 32), "115792089237316195423570985008687907853269984665640564039457584007913129639935"},
	} {
		require.Equal(t, tt.want, formatNetVersion(tt.chainID))
	}
}

func TestWeb3ClientVersion(t *testing.T) {
	got, err := Web3ClientVersion(nil)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(got, "beatoz/"+version.String()+"/"))
	require.Contains(t, got, runtime.GOOS+"-"+runtime.GOARCH)
	require.True(t, strings.HasSuffix(got, runtime.Version()))
}
