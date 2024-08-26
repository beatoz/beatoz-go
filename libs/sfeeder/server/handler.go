package server

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"github.com/beatoz/beatoz-go/libs/sfeeder/common"
	"github.com/beatoz/beatoz-go/types/crypto"
	grpc_tags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	"github.com/sirupsen/logrus"
)

type SecretFeederHandler struct {
	storePath string

	privKey    *ecdsa.PrivateKey
	clientKeys map[string][]byte
	secrets    map[string][]byte

	logger *logrus.Entry
}

func NewSecretFeederHandler(storePath string, logger *logrus.Entry) *SecretFeederHandler {
	_privKey, _ := crypto.NewPrvKey()
	return &SecretFeederHandler{
		storePath:  storePath,
		privKey:    _privKey,
		clientKeys: make(map[string][]byte),
		secrets:    make(map[string][]byte),
		logger:     logger,
	}
}

func (handler *SecretFeederHandler) mustEmbedUnimplementedSecretFeederSvcServer() {
	panic("implement me")
}

func (handler *SecretFeederHandler) Handshake(ctx context.Context, req *ReqHandshake) (*RespHandshake, error) {
	pubC, err := crypto.DecompressPubkey(req.Pub)
	if err != nil {
		return nil, err
	}
	x, y := handler.privKey.Curve.ScalarMult(pubC.X, pubC.Y, handler.privKey.D.Bytes())
	handler.clientKeys[req.Id] = crypto.DefaultHash(x.Bytes(), y.Bytes())

	feederPubBytes := crypto.CompressPubkey(&handler.privKey.PublicKey)

	return &RespHandshake{
		Pub: feederPubBytes,
	}, nil
}

func (handler *SecretFeederHandler) NewSecret(ctx context.Context, req *ReqNewSecret) (*RespNewSecret, error) {
	sk, ok := handler.clientKeys[req.Id]
	if !ok {
		return nil, fmt.Errorf("not found key for client(%s)", req.Id)
	}
	defer func() { handler.clientKeys[req.Id] = nil }()

	tags := grpc_tags.Extract(ctx)
	tags.Set("requester", req.Id)
	tags.Set("address", fmt.Sprintf("%X", req.Address))
	tags.Set("secret", fmt.Sprintf("%d", len(req.Secret)))

	// decrypt
	plaintext, err := common.Dec(sk, req.Secret)
	if err != nil {
		return nil, err
	}

	haddr := crypto.DefaultHash(req.Address)

	//path := filepath.Join(handler.storePath, hex.EncodeToString(haddr))
	//f := libs.NewFileWriter(path)
	//_, err = f.Write(plaintext)
	//if err != nil {
	//	return nil, err
	//}
	handler.secrets[hex.EncodeToString(haddr)] = plaintext
	return &RespNewSecret{Result: true}, nil
}

func (handler *SecretFeederHandler) GetSecret(ctx context.Context, req *ReqGetSecret) (*RespGetSecret, error) {
	sk, ok := handler.clientKeys[req.Id]
	if !ok {
		return nil, fmt.Errorf("not found key for client(%s)", req.Id)
	}
	defer func() { handler.clientKeys[req.Id] = nil }()

	tags := grpc_tags.Extract(ctx)
	tags.Set("requester", req.Id)
	tags.Set("address", fmt.Sprintf("%X", req.Address))

	haddr := crypto.DefaultHash(req.Address)

	//path := filepath.Join(handler.storePath, hex.EncodeToString(haddr))
	//f := libs.NewFileReader(path)
	//buf := make([]byte, 1024)
	//n, err := f.Read(buf)
	//if err != nil {
	//	return nil, err
	//}
	//buf = buf[:n]

	plaintext, ok := handler.secrets[hex.EncodeToString(haddr)]
	if !ok {
		return nil, fmt.Errorf("%X's secret is not found", req.Address)
	}

	// encrypt
	ciphertext, err := common.Enc(sk, plaintext)
	if err != nil {
		return nil, err
	}

	resp := &RespGetSecret{
		Address: req.Address,
		Secret:  ciphertext,
	}
	tags.Set("secret", fmt.Sprintf("%d", len(resp.Secret)))

	return resp, nil
}

func (handler *SecretFeederHandler) UpdateSecret(ctx context.Context, req *ReqUpdateSecret) (*RespUpdateSecret, error) {
	panic("not implemented")
}

var _ SecretFeederSvcServer = (*SecretFeederHandler)(nil)
