package main_test

import (
	"github.com/beatoz/beatoz-go/sfeeder/client"
	"github.com/beatoz/beatoz-go/sfeeder/common"
	"github.com/beatoz/beatoz-go/sfeeder/server"
	"github.com/beatoz/beatoz-go/types"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

var (
	serverAddr = "127.0.0.1:9900"
	grpcClient server.SecretFeederSvcClient

	addr0   = types.RandAddress()
	secret0 = []byte("dflasdjflsdjf;lsdjf")
)

func init() {
	pwd := []byte("1111")
	dataDir := filepath.Join(os.TempDir(), "sfeeder_test")

	os.MkdirAll(dataDir, 0o700)
	_, err := common.WriteSecret(addr0, secret0, pwd, dataDir)
	if err != nil {
		panic(err)
	}

	go func() {
		server.Start(serverAddr, dataDir, func() []byte { return []byte("1111") })
	}()
}

func TestSecretFeeder(t *testing.T) {
	conn, ctx, cancelFunc, err := client.Connect(serverAddr)
	require.NoError(t, err)
	defer client.CloseConnect(conn, cancelFunc)

	grpcClient = server.NewSecretFeederSvcClient(conn)

	sk1, err := client.Handshake(ctx, grpcClient, "test_client_1")
	require.NoError(t, err)
	respGetSecret1, err := grpcClient.GetSecret(ctx, &server.ReqGetSecret{Id: "test_client_1", Address: addr0})
	require.NoError(t, err)
	require.NotEqual(t, secret0, respGetSecret1.Secret)

	secret1, err := common.Dec(sk1, respGetSecret1.Secret)
	require.NoError(t, err)
	require.Equal(t, secret0, secret1)

	sk2, err := client.Handshake(ctx, grpcClient, "test_client_1")
	require.NoError(t, err)
	respGetSecret2, err := grpcClient.GetSecret(ctx, &server.ReqGetSecret{Id: "test_client_1", Address: addr0})
	require.NoError(t, err)
	require.Equal(t, len(respGetSecret1.Secret), len(respGetSecret2.Secret))
	// ciphertext should be different
	require.NotEqual(t, respGetSecret1.Secret, respGetSecret2.Secret)

	secret2, err := common.Dec(sk2, respGetSecret2.Secret)
	require.NoError(t, err)
	require.Equal(t, secret0, secret2)

	server.Stop()
}
