package rpc

import (
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/p2p/conn"
)

type ethRPCPeer struct {
	p2p.Peer
	info p2p.NodeInfo
}

func (p ethRPCPeer) NodeInfo() p2p.NodeInfo      { return p.info }
func (ethRPCPeer) IsOutbound() bool              { return false }
func (ethRPCPeer) Status() conn.ConnectionStatus { return conn.ConnectionStatus{} }
func (ethRPCPeer) RemoteIP() net.IP              { return net.IPv4(127, 0, 0, 1) }

func TestEthAccounts(t *testing.T) {
	handler, fixture := ethTestRPCHandler(t)
	fixture.queryError = fmt.Errorf("query unavailable")
	response := ethServerRequest(t, handler, "eth_accounts", `[]`)
	require.Nil(t, response.Error)
	require.Equal(t, "[]", string(response.Result))
	require.Empty(t, fixture.queries)
}

func TestEthNetworkInfo(t *testing.T) {
	handler, fixture := ethTestRPCHandler(t)
	for _, tt := range []struct {
		listening bool
		peers     int
		wantCount string
	}{
		{false, 0, `"0x0"`}, {true, 0, `"0x0"`}, {false, 2, `"0x2"`}, {true, 16, `"0x10"`},
	} {
		t.Run(fmt.Sprintf("listening=%t/peers=%d", tt.listening, tt.peers), func(t *testing.T) {
			fixture.env.P2PTransport = ethRPCTransport{listening: tt.listening}
			peers := &ethRPCPeers{}
			for i := 0; i < tt.peers; i++ {
				peers.list = append(peers.list, ethRPCPeer{info: p2p.DefaultNodeInfo{DefaultNodeID: p2p.ID(fmt.Sprintf("peer-%d", i))}})
			}
			fixture.env.P2PPeers = peers
			response := ethServerRequest(t, handler, "net_listening", `[]`)
			require.Nil(t, response.Error)
			require.Equal(t, fmt.Sprint(tt.listening), string(response.Result))
			response = ethServerRequest(t, handler, "net_peerCount", `[]`)
			require.Nil(t, response.Error)
			require.Equal(t, tt.wantCount, string(response.Result))
		})
	}
	require.Empty(t, fixture.queries)
}

func TestEthNetworkInfoBackendErrors(t *testing.T) {
	handler, fixture := ethTestRPCHandler(t)
	// NetInfo rejects peers that do not provide DefaultNodeInfo.
	fixture.env.P2PPeers = &ethRPCPeers{list: []p2p.Peer{ethRPCPeer{}}}
	for _, method := range []string{"net_listening", "net_peerCount"} {
		response := ethServerRequest(t, handler, method, `[]`)
		require.NotNil(t, response.Error)
		require.Equal(t, -32000, response.Error.Code)
		require.Equal(t, "peer.NodeInfo() is not DefaultNodeInfo", response.Error.Message)
		require.Empty(t, response.Result)
	}
}

func TestEthWeb3Sha3(t *testing.T) {
	handler, fixture := ethTestRPCHandler(t)
	for _, tt := range []struct{ input, want string }{
		{"0x68656c6c6f20776f726c64", "0x47173285a8d7341e5e972fc677286384f802f8ef42a5ec5f03bbfa254cb01fad"},
		{"0x", "0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"},
		{"", "0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"},
	} {
		response := ethServerRequest(t, handler, "web3_sha3", fmt.Sprintf(`[%q]`, tt.input))
		require.Nil(t, response.Error)
		require.Equal(t, fmt.Sprintf(`%q`, tt.want), string(response.Result))
	}
	for _, params := range []string{`["hello world"]`, `["0x0"]`, `["0xgg"]`, `["68656c6c6f"]`, `[0]`, `[null]`, `[{}]`, `[]`} {
		response := ethServerRequest(t, handler, "web3_sha3", params)
		require.NotNil(t, response.Error, params)
		require.Equal(t, -32602, response.Error.Code, params)
		require.Empty(t, response.Result)
	}
	require.Empty(t, fixture.queries)
}
