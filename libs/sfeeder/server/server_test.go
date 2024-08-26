package server_test

import (
	"github.com/beatoz/beatoz-go/libs/sfeeder/client"
	"github.com/beatoz/beatoz-go/libs/sfeeder/common"
	"github.com/beatoz/beatoz-go/libs/sfeeder/server"
	"github.com/beatoz/beatoz-go/types"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

var (
	serverAddr = "127.0.0.1:62667"
	grpcClient server.SecretFeederSvcClient
)

func init() {
	go func() {
		storeRootDir := filepath.Join(os.TempDir(), "sf_store")
		server.Start(serverAddr, storeRootDir)
	}()
}

func TestSecretFeeder(t *testing.T) {
	conn, ctx, cancelFunc, err := client.Connect(serverAddr)
	require.NoError(t, err)
	defer client.CloseConnect(conn, cancelFunc)

	grpcClient = server.NewSecretFeederSvcClient(conn)

	//
	// clinet_0 register
	sk0, err := client.Handshake(ctx, grpcClient, "test_client_0")
	require.NoError(t, err)
	//
	//

	addr := types.RandAddress()
	secret0 := []byte("test passphrase")
	_secret0, err := common.Enc(sk0, secret0)
	require.NoError(t, err)

	// new secret
	reqNewSecret := &server.ReqNewSecret{
		Id:      "test_client_0",
		Address: addr,
		Secret:  _secret0, // encrypted
	}
	respNewSecret, err := grpcClient.NewSecret(ctx, reqNewSecret)
	require.NoError(t, err)
	require.True(t, respNewSecret.Result)

	//
	// clinet_1 (getter) register
	sk1, err := client.Handshake(ctx, grpcClient, "test_client_1")
	require.NoError(t, err)
	require.NotEqual(t, sk0, sk1)
	//
	//

	respGetSecret, err := grpcClient.GetSecret(ctx, &server.ReqGetSecret{Id: "test_client_1", Address: addr})
	require.NoError(t, err)

	secret1, err := common.Dec(sk1, respGetSecret.Secret)
	require.NoError(t, err)
	require.Equal(t, secret0, secret1)
}
