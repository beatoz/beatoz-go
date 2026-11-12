package rpc

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"
	tmrpccore "github.com/tendermint/tendermint/rpc/core"
	tmrpcserver "github.com/tendermint/tendermint/rpc/jsonrpc/server"
)

func ethTestLegacyRPCHandler(t *testing.T) http.Handler {
	t.Helper()
	previous := tmrpccore.Routes
	t.Cleanup(func() { tmrpccore.Routes = previous })
	// Isolate AddEthRoutes: Tendermint's native net_info also has a net_ prefix.
	tmrpccore.Routes = make(map[string]*tmrpcserver.RPCFunc)
	AddEthRoutes()
	mux := http.NewServeMux()
	tmrpcserver.RegisterRPCFuncs(mux, tmrpccore.Routes, log.NewNopLogger())
	return mux
}

// TestLegacyEthRouteSurface pins AddEthRoutes registrations on an isolated map,
// since Tendermint's own net_info shares the net_ prefix. Routes added directly
// in AddRoutes are not caught here.
func TestLegacyEthRouteSurface(t *testing.T) {
	ethTestLegacyRPCHandler(t)
	var methods []string
	for name := range tmrpccore.Routes {
		if strings.HasPrefix(name, "eth_") || strings.HasPrefix(name, "net_") || strings.HasPrefix(name, "web3_") {
			methods = append(methods, name)
		}
	}
	require.ElementsMatch(t, []string{"eth_chainId", "eth_blockNumber", "eth_getBlockByNumber"}, methods)
}

func TestLegacyEthBlockNumberIsDecimal(t *testing.T) {
	_, fixture := ethTestRPCHandler(t)
	fixture.appHeight = 12345
	handler := ethTestLegacyRPCHandler(t)
	response := ethServerRequest(t, handler, "eth_blockNumber", `[]`)
	require.Nil(t, response.Error)
	require.JSONEq(t, `"12345"`, string(response.Result))
}
