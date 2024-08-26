package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/beatoz/beatoz-go/libs"
	"github.com/beatoz/beatoz-go/libs/sfeeder/client"
	"github.com/beatoz/beatoz-go/libs/sfeeder/common"
	"github.com/beatoz/beatoz-go/libs/sfeeder/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"os"
	"strings"
	"time"
)

func main() {
	serverAddr := "127.0.0.1:9900"
	if len(os.Args) == 2 {
		serverAddr = os.Args[1]
	}

	run := true
	for run {
		_cmd := readFromShell("sfeeder-client$ ")
		cmd := strings.ToLower(string(_cmd))

		// connection
		conn, ctx, cancelFunc, err := connect(serverAddr)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		grpcClient := server.NewSecretFeederSvcClient(conn)

		switch cmd {
		case "new":
			if err := newSecret(grpcClient, ctx); err != nil {
				fmt.Println(err)
			}
		case "get":
			if s, err := getSecret(grpcClient, ctx); err != nil {
				fmt.Println(err)
			} else {
				fmt.Println(string(s))
			}

		case "exit":
			run = false
		}

		// close connection
		closeConnect(conn, cancelFunc)
	}
}

func connect(serverAddr string) (*grpc.ClientConn, context.Context, context.CancelFunc, error) {
	conn, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		return nil, nil, nil, err
	}

	ctx, cancelFunc := context.WithTimeout(context.Background(), 60*time.Second)
	return conn, ctx, cancelFunc, nil
}

func closeConnect(conn *grpc.ClientConn, cancelFunc context.CancelFunc) {
	_ = conn.Close()
	cancelFunc()
}

func readFromShell(prompt string) []byte {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	text, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("ERROR:", err.Error())
		os.Exit(1)
	}
	return bytes.TrimSpace([]byte(text))
}

func newSecret(grpClient server.SecretFeederSvcClient, ctx context.Context) error {
	hexAddr := readFromShell("Enter address: ")
	addr, err := hex.DecodeString(string(hexAddr))
	if err != nil {
		return err
	}
	secret := libs.ReadCredential("Enter passphrase: ")

	sk, err := client.Handshake(ctx, grpClient, "sfsh")
	if err != nil {
		return err
	}
	_secret, err := common.Enc(sk, secret)
	if err != nil {
		return err
	}

	req := &server.ReqNewSecret{
		Id:      "sfsh",
		Address: addr,
		Secret:  _secret,
	}

	resp, err := grpClient.NewSecret(ctx, req)
	if err != nil {
		return err
	}

	if !resp.Result {
		return errors.New("NewSecret fails")
	}
	return nil
}

func getSecret(grpClient server.SecretFeederSvcClient, ctx context.Context) ([]byte, error) {
	hexAddr := readFromShell("Enter address: ")
	addr, err := hex.DecodeString(string(hexAddr))
	if err != nil {
		return nil, err
	}

	sk, err := client.Handshake(ctx, grpClient, "sfsh")
	if err != nil {
		return nil, err
	}

	req := &server.ReqGetSecret{
		Id:      "sfsh",
		Address: addr,
	}

	resp, err := grpClient.GetSecret(ctx, req)
	if err != nil {
		return nil, err
	}

	secret, err := common.Dec(sk, resp.Secret)
	if err != nil {
		return nil, err
	}

	return secret, nil
}
