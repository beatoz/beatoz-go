package rpc

import (
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

func TestParseEthAddress(t *testing.T) {
	address := "1234567890abcdef1234567890abcdef12345678"

	tests := []struct {
		name  string
		input string
		want  common.Address
	}{
		{name: "with lowercase prefix", input: "0x" + address, want: common.HexToAddress("0x" + address)},
		{name: "without prefix", input: address, want: common.HexToAddress("0x" + address)},
		{name: "with uppercase prefix", input: "0X" + address, want: common.HexToAddress("0x" + address)},
		{name: "trims whitespace", input: "  0x" + address + "  ", want: common.HexToAddress("0x" + address)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseEthAddress(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestParseEthAddressInvalid(t *testing.T) {
	tests := []string{
		"",
		"0x",
		"0x1234",
		"0x0X34567890abcdef1234567890abcdef12345678",
		"0x1234567890abcdef1234567890abcdef1234567890",
		"0x1234567890abcdef1234567890abcdef1234567z",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			_, err := parseEthAddress(input)
			require.Error(t, err)
		})
	}
}

func TestParseEthStorageSlot(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  common.Hash
	}{
		{name: "empty", input: "", want: common.Hash{}},
		{name: "zero prefix only", input: "0x", want: common.Hash{}},
		{name: "single nibble", input: "0xa", want: common.HexToHash("0x0a")},
		{name: "odd length", input: "0xabc", want: common.HexToHash("0x0abc")},
		{name: "full hash", input: "0x" + strings.Repeat("1", 64), want: common.HexToHash("0x" + strings.Repeat("1", 64))},
		{name: "trims whitespace", input: "  0x2a  ", want: common.HexToHash("0x2a")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseEthStorageSlot(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestParseEthStorageSlotInvalid(t *testing.T) {
	tests := []string{
		"0x" + strings.Repeat("1", 65),
		"0x0X921",
		"0xzz",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			_, err := parseEthStorageSlot(input)
			require.Error(t, err)
		})
	}
}

func TestParseEthBlockNumber(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int64
	}{
		{name: "latest", input: "latest", want: 0},
		{name: "latest case and trim", input: "  Latest  ", want: 0},
		{name: "earliest", input: "earliest", want: 1},
		{name: "hex", input: "0xff", want: 255},
		{name: "hex uppercase prefix", input: "0X10", want: 16},
		{name: "decimal", input: "100", want: 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseEthBlockNumber(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestParseEthBlockNumberInvalid(t *testing.T) {
	tests := []string{
		"",
		"pending",
		"safe",
		"finalized",
		"0",
		"0x0",
		"-1",
		"0xzz",
		"0xffffffffffffffff",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			_, err := parseEthBlockNumber(input)
			require.Error(t, err)
		})
	}
}

func TestFormatEthStorageAtResponse(t *testing.T) {
	value := common.HexToHash("0x2a").Bytes()

	got, err := formatEthStorageAtResponse(abcitypes.ResponseQuery{
		Code:  abcitypes.CodeTypeOK,
		Value: value,
	})
	require.NoError(t, err)
	require.Equal(t, "0x000000000000000000000000000000000000000000000000000000000000002a", got)
}

func TestFormatEthStorageAtResponseEmptyValue(t *testing.T) {
	got, err := formatEthStorageAtResponse(abcitypes.ResponseQuery{
		Code: abcitypes.CodeTypeOK,
	})
	require.NoError(t, err)
	require.Equal(t, "0x0000000000000000000000000000000000000000000000000000000000000000", got)
}

func TestFormatEthStorageAtResponseErrorCode(t *testing.T) {
	_, err := formatEthStorageAtResponse(abcitypes.ResponseQuery{
		Code: 1,
		Log:  "state not found",
	})
	require.EqualError(t, err, "state not found")
}

func TestFormatEthStorageAtResponseInvalidValueLength(t *testing.T) {
	_, err := formatEthStorageAtResponse(abcitypes.ResponseQuery{
		Code:  abcitypes.CodeTypeOK,
		Value: []byte{1, 2, 3},
	})
	require.EqualError(t, err, "invalid storage value length: 3")
}
