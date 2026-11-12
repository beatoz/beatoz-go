package rpc

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	tmrpccore "github.com/tendermint/tendermint/rpc/core"
)

func TestEthServerAccountQueries(t *testing.T) {
	_, fixture := ethTestRPCHandler(t)
	handler, err := newEthRPCHandler(nil)
	require.NoError(t, err)
	address := "0x" + strings.Repeat("aB", 20)
	fixture.accounts[string(bytes.Repeat([]byte{0xab}, 20))] = []byte(`{"address":"` + strings.Repeat("AB", 20) + `","nonce":"42","balance":"1000000000000000000","code":"6080ABFF"}`)
	for _, tt := range []struct{ method, want, zero string }{
		{"eth_getBalance", "0xde0b6b3a7640000", "0x0"},
		{"eth_getTransactionCount", "0x2a", "0x0"},
		{"eth_getCode", "0x6080abff", "0x"},
	} {
		t.Run(tt.method, func(t *testing.T) {
			response := ethServerRequest(t, handler, tt.method, `["`+address+`"]`)
			require.Nil(t, response.Error)
			require.JSONEq(t, fmt.Sprintf("%q", tt.want), string(response.Result))
			query := fixture.queries[len(fixture.queries)-1]
			require.Equal(t, "account", query.Path)
			require.Equal(t, bytes.Repeat([]byte{0xab}, 20), query.Data)
			require.Zero(t, query.Height)
			for _, unknown := range []string{strings.Repeat("00", 20), strings.Repeat("12", 20)} {
				response := ethServerRequest(t, handler, tt.method, `["0x`+unknown+`"]`)
				require.Nil(t, response.Error)
				require.JSONEq(t, fmt.Sprintf("%q", tt.zero), string(response.Result))
			}
			response = ethServerRequest(t, handler, tt.method, `["0x1234"]`)
			require.NotNil(t, response.Error)
			require.Equal(t, -32602, response.Error.Code)
		})
	}
	fixture.accounts[string(bytes.Repeat([]byte{0xab}, 20))] = []byte(`{"nonce":"18446744073709551615","balance":"115792089237316195423570985008687907853269984665640564039457584007913129639935"}`)
	for _, tt := range []struct{ method, want string }{
		{"eth_getBalance", "0x" + strings.Repeat("f", 64)},
		{"eth_getTransactionCount", "0xffffffffffffffff"},
	} {
		response := ethServerRequest(t, handler, tt.method, `["`+address+`"]`)
		require.Nil(t, response.Error)
		require.JSONEq(t, fmt.Sprintf("%q", tt.want), string(response.Result))
	}
}

func TestEthServerQueryHeightsAndErrors(t *testing.T) {
	_, fixture := ethTestRPCHandler(t)
	handler, err := newEthRPCHandler(nil)
	require.NoError(t, err)
	address := `"0x` + strings.Repeat("ab", 20) + `"`
	for _, tt := range []struct{ method, params, path string }{
		{"eth_getBalance", address, "account"},
		{"eth_getTransactionCount", address, "account"},
		{"eth_getCode", address, "account"},
		{"eth_getStorageAt", address + `,"0x1"`, "eth_getStorageAt"},
		{"eth_call", `{}`, "eth_call"},
		{"eth_estimateGas", `{}`, "eth_estimateGas"},
	} {
		t.Run(tt.method, func(t *testing.T) {
			// These methods belong only to the standalone Ethereum server.
			require.NotContains(t, tmrpccore.Routes, tt.method)
			for _, tag := range []struct {
				suffix string
				height int64
			}{
				{"", 0}, {`,null`, 0}, {`,"latest"`, 0}, {`,"pending"`, 0}, {`,"safe"`, 0}, {`,"finalized"`, 0},
				{`,"earliest"`, 10}, {`,{"blockNumber":"earliest"}`, 10}, {`,"0xb"`, 11}, {`,"0Xb"`, 11}, {`,"0xc"`, 12},
			} {
				fixture.queries = nil
				response := ethServerRequest(t, handler, tt.method, `[`+tt.params+tag.suffix+`]`)
				require.Nil(t, response.Error, tag.suffix)
				height := tag.height
				if height == 0 && (tt.method == "eth_call" || tt.method == "eth_estimateGas") {
					height = fixture.appHeight
				}
				if tt.method == "eth_call" || tt.method == "eth_estimateGas" {
					require.Len(t, fixture.queries, 3)
					require.Equal(t, "block_height", fixture.queries[0].Path)
					require.Equal(t, "gov_params", fixture.queries[1].Path)
					require.Equal(t, height, fixture.queries[1].Height)
				} else if height > 0 {
					require.Len(t, fixture.queries, 2)
					require.Equal(t, "block_height", fixture.queries[0].Path)
				} else {
					require.Len(t, fixture.queries, 1)
				}
				query := fixture.queries[len(fixture.queries)-1]
				require.Equal(t, tt.path, query.Path)
				require.Equal(t, height, query.Height)
				require.False(t, query.Prove)
			}
			for _, tag := range []struct {
				selector string
				queries  int
			}{
				{`"0x0"`, 0}, {`"0X0"`, 0}, {`"\u0030x0"`, 0},
				{`{"blockNumber":"0x0"}`, 0}, {`{"blockNumber":"0X0"}`, 0},
				{`{"BLOCKNUMBER":"0x0"}`, 0},
				{`{"blockNumber":"earliest","BLOCKNUMBER":"0x0"}`, 0},
				{`"0xd"`, 1}, {`"0x7fffffffffffffff"`, 1},
			} {
				fixture.queries = nil
				response := ethServerRequest(t, handler, tt.method, `[`+tt.params+`,`+tag.selector+`]`)
				require.NotNil(t, response.Error, tag.selector)
				require.Equal(t, -32000, response.Error.Code)
				require.Equal(t, "header not found", response.Error.Message)
				require.Len(t, fixture.queries, tag.queries)
				for _, query := range fixture.queries {
					require.Equal(t, "block_height", query.Path)
				}
			}
			for _, tag := range []string{`"bad"`, `"Earliest"`, `"EARLIEST"`, `"0x00"`, `"0x8000000000000000"`, `12`, `{}`, `[]`, `true`} {
				fixture.queries = nil
				response := ethServerRequest(t, handler, tt.method, `[`+tt.params+`,`+tag+`]`)
				require.NotNil(t, response.Error)
				require.Equal(t, -32602, response.Error.Code, tag)
				require.Empty(t, fixture.queries)
			}
			fixture.queryCode = 7
			fixture.codePath = tt.path
			for _, log := range []string{"query rejected", ""} {
				fixture.queryLog = log
				fixture.queries = nil
				response := ethServerRequest(t, handler, tt.method, `[`+tt.params+`]`)
				require.NotNil(t, response.Error)
				require.Equal(t, -32000, response.Error.Code)
				want := log
				if want == "" {
					want = tt.path + " failed with code 7"
				}
				if tt.method == "eth_call" || tt.method == "eth_estimateGas" {
					require.Len(t, fixture.queries, 3)
					require.Equal(t, "gov_params", fixture.queries[1].Path)
					require.Equal(t, tt.path, fixture.queries[2].Path)
				}
				require.Equal(t, want, response.Error.Message)
			}
			fixture.queryCode, fixture.queryLog, fixture.codePath = 0, "", ""
			fixture.queryError = fmt.Errorf("fixture unavailable")
			response := ethServerRequest(t, handler, tt.method, `[`+tt.params+`]`)
			require.NotNil(t, response.Error)
			require.Equal(t, -32000, response.Error.Code)
			require.Equal(t, fixture.queryError.Error(), response.Error.Message)
			fixture.queryError = nil
		})
	}
}

func TestEthServerStorageAt(t *testing.T) {
	_, fixture := ethTestRPCHandler(t)
	handler, err := newEthRPCHandler(nil)
	require.NoError(t, err)
	address := "0x" + strings.Repeat("ab", 20)
	for _, slot := range []string{"0X1", "0x1", "0x001", "0x" + strings.Repeat("0", 63) + "1"} {
		fixture.storageValue = append(make([]byte, 31), 0xab)
		response := ethServerRequest(t, handler, "eth_getStorageAt", `["`+address+`","`+slot+`"]`)
		require.Nil(t, response.Error)
		require.JSONEq(t, `"0x`+strings.Repeat("00", 31)+`ab"`, string(response.Result))
		query := fixture.queries[len(fixture.queries)-1]
		require.Equal(t, "eth_getStorageAt", query.Path)
		require.Equal(t, append(bytes.Repeat([]byte{0xab}, 20), append(make([]byte, 31), 1)...), query.Data)
	}
	fixture.storageValue = nil
	for _, slot := range []string{`"0x0"`, `"0x"`, `"0X"`} {
		response := ethServerRequest(t, handler, "eth_getStorageAt", `["`+address+`",`+slot+`]`)
		require.Nil(t, response.Error)
		require.JSONEq(t, `"0x`+strings.Repeat("00", 32)+`"`, string(response.Result))
		require.Equal(t, append(bytes.Repeat([]byte{0xab}, 20), make([]byte, 32)...), fixture.queries[len(fixture.queries)-1].Data)
	}
	for _, size := range []int{1, 31, 33} {
		fixture.storageValue = make([]byte, size)
		response := ethServerRequest(t, handler, "eth_getStorageAt", `["`+address+`","0x0"]`)
		require.NotNil(t, response.Error)
		require.Equal(t, -32000, response.Error.Code)
		require.Contains(t, response.Error.Message, "value length")
	}
	for _, params := range []string{
		`["bad","0x1"]`, `["` + address + `","0x` + strings.Repeat("ab", 33) + `"]`, `["` + address + `","0xgg"]`,
		`["` + address + `"]`, `["` + address + `",null]`, `["` + address + `",""]`,
		`["` + address + `","10"]`, `["` + address + `","zz"]`, `["` + address + `","1"]`,
		`["` + address + `",10]`, `["` + address + `",true]`, `["` + address + `",{}]`,
	} {
		fixture.queries = nil
		response := ethServerRequest(t, handler, "eth_getStorageAt", params)
		require.NotNil(t, response.Error, params)
		require.Equal(t, -32602, response.Error.Code)
		require.Empty(t, fixture.queries)
	}
}
