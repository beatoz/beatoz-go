package client

import (
	"context"
	"github.com/beatoz/beatoz-go/libs"
	"github.com/beatoz/beatoz-go/sfeeder/common"
	server2 "github.com/beatoz/beatoz-go/sfeeder/server"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/crypto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"time"
)

func ReadRemoteCredential(nodeid string, serverAddr string, target types.Address) ([]byte, error) {
	conn, ctx, cancelFunc, err := Connect(serverAddr)
	if err != nil {
		return nil, err
	}
	defer CloseConnect(conn, cancelFunc)

	grpcClient := server2.NewSecretFeederSvcClient(conn)
	sk, err := Handshake(ctx, grpcClient, nodeid)
	defer libs.ClearCredential(sk)

	req := &server2.ReqGetSecret{
		Id:      nodeid,
		Address: target,
	}

	resp, err := grpcClient.GetSecret(ctx, req)
	if err != nil {
		return nil, err
	}

	secret, err := common.Dec(sk, resp.Secret)
	if err != nil {
		return nil, err
	}
	return secret, nil
}

func Connect(serverAddr string) (*grpc.ClientConn, context.Context, context.CancelFunc, error) {
	conn, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		return nil, nil, nil, err
	}

	ctx, cancelFunc := context.WithTimeout(context.Background(), 60*time.Second)
	return conn, ctx, cancelFunc, nil
}

func CloseConnect(conn *grpc.ClientConn, cancelFunc context.CancelFunc) {
	_ = conn.Close()
	cancelFunc()
}

func Handshake(ctx context.Context, grpcClient server2.SecretFeederSvcClient, id string) ([]byte, error) {
	prv0, _ := crypto.NewPrvKey()
	pub0 := prv0.PublicKey
	pubBytes := crypto.CompressPubkey(&pub0)
	reqReg0 := &server2.ReqHandshake{
		Id:  id,
		Pub: pubBytes,
	}
	respReg0, err := grpcClient.Handshake(ctx, reqReg0)
	if err != nil {
		return nil, err
	}

	pubR, xerr := crypto.DecompressPubkey(respReg0.Pub)
	if xerr != nil {
		return nil, xerr
	}

	x, y := prv0.Curve.ScalarMult(pubR.X, pubR.Y, prv0.D.Bytes())
	sk0 := crypto.DefaultHash(x.Bytes(), y.Bytes())

	return sk0, nil
}
