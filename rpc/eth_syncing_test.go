package rpc

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	tmjson "github.com/tendermint/tendermint/libs/json"
	"github.com/tendermint/tendermint/libs/log"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	tmrpcserver "github.com/tendermint/tendermint/rpc/jsonrpc/server"
	tmrpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

func TestEthSyncingEncoding(t *testing.T) {
	for _, tt := range []struct {
		name string
		info coretypes.SyncInfo
		want string
	}{
		{"idle", coretypes.SyncInfo{}, `false`},
		{"caught up", coretypes.SyncInfo{EarliestBlockHeight: 1, LatestBlockHeight: 20}, `false`},
		{"starting", coretypes.SyncInfo{CatchingUp: true}, `{"startingBlock":"0x0","currentBlock":"0x0","highestBlock":"0x0"}`},
		{"catching up", coretypes.SyncInfo{CatchingUp: true, EarliestBlockHeight: 16, LatestBlockHeight: 42}, `{"startingBlock":"0x10","currentBlock":"0x2a","highestBlock":"0x2a"}`},
		{"large height", coretypes.SyncInfo{CatchingUp: true, EarliestBlockHeight: 1, LatestBlockHeight: math.MaxInt64}, `{"startingBlock":"0x1","currentBlock":"0x7fffffffffffffff","highestBlock":"0x7fffffffffffffff"}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := formatEthSyncing(tt.info)
			require.NoError(t, err)
			encoded, err := tmjson.Marshal(&result)
			require.NoError(t, err)
			require.JSONEq(t, tt.want, string(encoded))

			handler := EthSyncing
			handler = func(ctx *tmrpctypes.Context) (json.RawMessage, error) {
				return formatEthSyncing(tt.info)
			}
			mux := http.NewServeMux()
			tmrpcserver.RegisterRPCFuncs(mux, map[string]*tmrpcserver.RPCFunc{
				"eth_syncing": tmrpcserver.NewRPCFunc(handler, ""),
			}, log.NewNopLogger())
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"eth_syncing","params":[]}`))
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, req)
			require.Equal(t, http.StatusOK, recorder.Code)
			require.JSONEq(t, `{"jsonrpc":"2.0","id":1,"result":`+tt.want+`}`, recorder.Body.String())
		})
	}
}

func TestFormatEthSyncingInvalidHeights(t *testing.T) {
	for _, info := range []coretypes.SyncInfo{
		{CatchingUp: true, EarliestBlockHeight: -1},
		{CatchingUp: true, LatestBlockHeight: -1},
	} {
		_, err := formatEthSyncing(info)
		require.Error(t, err)
	}
}
