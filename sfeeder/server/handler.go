package server

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"github.com/beatoz/beatoz-go/libs"
	common2 "github.com/beatoz/beatoz-go/sfeeder/common"
	"github.com/beatoz/beatoz-go/types/crypto"
	grpc_tags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	"github.com/sirupsen/logrus"
)

type SecretFeederHandler struct {
	storePath string

	tempPrvKey  *ecdsa.PrivateKey
	sessionKeys map[string][]byte
	//secrets    map[string][]byte
	getPass func() []byte

	logger *logrus.Entry
}

func NewSecretFeederHandler(storePath string, cb func() []byte, logger *logrus.Entry) *SecretFeederHandler {
	_privKey, _ := crypto.NewPrvKey()
	return &SecretFeederHandler{
		storePath:   storePath,
		tempPrvKey:  _privKey,
		sessionKeys: make(map[string][]byte),
		getPass:     cb,
		logger:      logger,
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
	x, y := handler.tempPrvKey.Curve.ScalarMult(pubC.X, pubC.Y, handler.tempPrvKey.D.Bytes())
	handler.sessionKeys[req.Id] = crypto.DefaultHash(x.Bytes(), y.Bytes())

	feederPubBytes := crypto.CompressPubkey(&handler.tempPrvKey.PublicKey)

	return &RespHandshake{
		Pub: feederPubBytes,
	}, nil
}

// DISABLED!!!
func (handler *SecretFeederHandler) NewSecret(ctx context.Context, req *ReqNewSecret) (*RespNewSecret, error) {
	return &RespNewSecret{Result: true}, nil

	//sk, ok := handler.sessionKeys[req.Id]
	//if !ok {
	//	return nil, fmt.Errorf("not found key for client(%s)", req.Id)
	//}
	//defer func() { handler.sessionKeys[req.Id] = nil }()
	//
	//tags := grpc_tags.Extract(ctx)
	//tags.Set("requester", req.Id)
	//tags.Set("address", fmt.Sprintf("%X", req.Address))
	//tags.Set("secret", fmt.Sprintf("%d", len(req.Secret)))
	//
	//// decrypt
	//plaintext, err := common.Dec(sk, req.Secret)
	//if err != nil {
	//	return nil, err
	//}
	//
	//haddr := crypto.DefaultHash(req.Address)
	//
	////path := filepath.Join(handler.storePath, hex.EncodeToString(haddr))
	////f := libs.NewFileWriter(path)
	////_, err = f.Write(plaintext)
	////if err != nil {
	////	return nil, err
	////}
	//handler.secrets[hex.EncodeToString(haddr)] = plaintext
	//return &RespNewSecret{Result: true}, nil
}

func (handler *SecretFeederHandler) GetSecret(ctx context.Context, req *ReqGetSecret) (*RespGetSecret, error) {
	sk, ok := handler.sessionKeys[req.Id]
	if !ok {
		return nil, fmt.Errorf("not found key for client(%s)", req.Id)
	}
	defer func() { handler.sessionKeys[req.Id] = nil }()

	tags := grpc_tags.Extract(ctx)
	tags.Set("requester", req.Id)
	tags.Set("address", fmt.Sprintf("%X", req.Address))

	plaintext, err := common2.ReadSecret(req.Address, handler.getPass(), handler.storePath)
	defer libs.ClearCredential(plaintext)
	if err != nil {
		return nil, err
	}

	// encrypt
	ciphertext, err := common2.Enc(sk, plaintext)
	if err != nil {
		return nil, err
	}

	resp := &RespGetSecret{
		Address: req.Address,
		Secret:  ciphertext,
	}
	tags.Set("secret", fmt.Sprintf("%d bytes", len(plaintext)))

	return resp, nil
}

func (handler *SecretFeederHandler) UpdateSecret(ctx context.Context, req *ReqUpdateSecret) (*RespUpdateSecret, error) {
	panic("not implemented")
}

var _ SecretFeederSvcServer = (*SecretFeederHandler)(nil)
