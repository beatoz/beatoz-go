package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/beatoz/beatoz-go/libs"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	ethec "github.com/ethereum/go-ethereum/crypto/secp256k1"
	tmsecp256k1 "github.com/tendermint/tendermint/crypto/secp256k1"
	tmjson "github.com/tendermint/tendermint/libs/json"
	"golang.org/x/crypto/pbkdf2"
	"io"
	"path/filepath"
)

const (
	//Aes128CBC = "aes-128-cbc"
	Aes256CBC = "aes-256-cbc"
	//Aes512CBC = "aes-512-cbc"

	Secp256K1 = tmsecp256k1.KeyType

	SymmAlgo = Aes256CBC
	DKLEN    = 32

	AsymmAlgo = Secp256K1
)

type cipherTextParams struct {
	Algo string `json:"ca"`
	Text []byte `json:"ct"`
	Iv   []byte `json:"ci,omitempty"`
	Salt []byte `json:"cs,omitempty"`
}

type dkParams struct {
	Algo  string `json:"ka"`
	Prf   string `json:"kh"`
	Iter  int    `json:"kc"`
	Salt  []byte `json:"ks"`
	DkLen int    `json:"kl"`
}

type WalletKey struct {
	Version          int               `json:"version"`
	Address          types.Address     `json:"address"`
	Algo             string            `json:"algo"`
	CipherTextParams *cipherTextParams `json:"cp"`
	DKParams         *dkParams         `json:"dkp"`

	prvKey []byte
	pubKey []byte
}

func NewWalletKeyWith(keyBytes, pass []byte) *WalletKey {
	var _pubKey, _prvKey []byte
	var _cipherTextParams *cipherTextParams
	var _dkParams *dkParams

	if pass != nil {
		salt := make([]byte, DKLEN)
		rand.Read(salt)
		iter := 1 /*600000*/ + int(binary.BigEndian.Uint16(append([]byte{0x00}, salt[:1]...)))

		sk := pbkdf2.Key(pass, salt, iter, DKLEN, DefaultHasher)

		_dkParams = &dkParams{
			Algo:  "pbkdf2",
			Prf:   DefaultHasherName(),
			Iter:  iter,
			Salt:  salt,
			DkLen: DKLEN,
		}

		block, err := aes.NewCipher(sk)
		if err != nil {
			panic(err)
		}

		iv := make([]byte, block.BlockSize())
		rand.Read(iv)

		plaintext, err := PKCS7Padding(keyBytes, block.BlockSize())
		if err != nil {
			panic(err)
		}

		ciphertext := make([]byte, len(plaintext))
		cbc := cipher.NewCBCEncrypter(block, iv)
		cbc.CryptBlocks(ciphertext, plaintext)
		libs.ClearCredential(plaintext)

		_cipherTextParams = &cipherTextParams{
			Algo: SymmAlgo,
			Text: ciphertext,
			Iv:   iv,
		}
	} else {
		_cipherTextParams = &cipherTextParams{
			Algo: SymmAlgo,
			Text: keyBytes, // plaintext
		}
		_prvKey = append(keyBytes[:0:0], keyBytes...)
	}
	_pubKey = tmsecp256k1.PrivKey(keyBytes).PubKey().Bytes()

	_addr, err := PubBytes2Addr(_pubKey)
	if err != nil {
		panic(err)
	}

	return &WalletKey{
		Version:          1,
		Address:          _addr, //tmsecp256k1.PrivKey(keyBytes).PubKey().Address(),
		Algo:             AsymmAlgo,
		CipherTextParams: _cipherTextParams,
		DKParams:         _dkParams,
		prvKey:           _prvKey,
		pubKey:           _pubKey,
	}
}

func NewWalletKey(s []byte) *WalletKey {
	privKey := tmsecp256k1.GenPrivKey()
	return NewWalletKeyWith(privKey, s)
}

func OpenWalletKey(r io.Reader) (*WalletKey, error) {
	buf := make([]byte, 1000)
	n, err := r.Read(buf)
	if n > 0 {
		wk := &WalletKey{}
		if err := tmjson.Unmarshal(buf[:n], wk); err != nil {
			return nil, err
		}
		return wk, nil
	}
	return nil, err
}

func (wk *WalletKey) Save(wr io.Writer) (int, error) {
	bz, err := tmjson.MarshalIndent(wk, "", "  ")
	if err != nil {
		return 0, err
	}
	return wr.Write(bz)
}

func (wk *WalletKey) IsLock() bool {
	return wk.prvKey == nil
}

func (wk *WalletKey) Lock() {
	libs.ClearCredential(wk.prvKey[:])
	wk.prvKey = nil
}

func (wk *WalletKey) LockWith(pass []byte) {
	if wk.prvKey == nil {
		return
	}
	defer func() {
		libs.ClearCredential(wk.prvKey[:])
		wk.prvKey = nil
	}()

	salt := make([]byte, DKLEN)
	rand.Read(salt)
	iter := 600000 + int(binary.BigEndian.Uint16(append([]byte{0x00}, salt[:1]...)))

	sk := pbkdf2.Key(pass, salt, iter, DKLEN, DefaultHasher)

	_dkParams := &dkParams{
		Algo:  "pbkdf2",
		Prf:   DefaultHasherName(),
		Iter:  iter,
		Salt:  salt,
		DkLen: DKLEN,
	}

	block, err := aes.NewCipher(sk)
	if err != nil {
		panic(err)
	}

	iv := make([]byte, block.BlockSize())
	rand.Read(iv)

	plaintext, err := PKCS7Padding(wk.prvKey, block.BlockSize())
	if err != nil {
		panic(err)
	}

	ciphertext := make([]byte, len(plaintext))
	cbc := cipher.NewCBCEncrypter(block, iv)
	cbc.CryptBlocks(ciphertext, plaintext)
	libs.ClearCredential(plaintext)

	wk.CipherTextParams = &cipherTextParams{
		Algo: SymmAlgo,
		Text: ciphertext,
		Iv:   iv,
	}
	wk.DKParams = _dkParams
}

func (wk *WalletKey) Unlock(s []byte) error {
	if !wk.IsLock() {
		return nil
	}

	if s != nil {
		sk := pbkdf2.Key(s, wk.DKParams.Salt, wk.DKParams.Iter, wk.DKParams.DkLen, DefaultHasher)
		iv := wk.CipherTextParams.Iv
		ciphertext := wk.CipherTextParams.Text
		prvKey := make([]byte, len(ciphertext))

		block, err := aes.NewCipher(sk)
		if err != nil {
			return err
		}

		cbc := cipher.NewCBCDecrypter(block, iv)
		cbc.CryptBlocks(prvKey, ciphertext)

		prvKey, err = PKCS7UnPadding(prvKey, block.BlockSize())
		if prvKey == nil {
			return err
		}
		wk.prvKey = prvKey
	} else if wk.DKParams == nil {
		wk.prvKey = append([]byte(nil), wk.CipherTextParams.Text...)
	} else {
		return errors.New("wrong passphrase: the passphrase can not be empty")
	}
	wk.pubKey = tmsecp256k1.PrivKey(wk.prvKey).PubKey().Bytes()

	return nil
}

func (wk *WalletKey) PrvKey() []byte {
	return wk.prvKey
}

func (wk *WalletKey) PrvKeyClone() []byte {
	if wk.prvKey != nil {
		r := make([]byte, len(wk.prvKey))
		copy(r, wk.prvKey)
		return r
	}
	return nil
}

func (wk *WalletKey) Sign(msg []byte) ([]byte, error) {
	if wk.IsLock() {
		return nil, xerrors.NewOrdinary("error: WalletKey is locked")
	}

	return ethec.Sign(DefaultHash(msg), wk.prvKey)
}

func (wk *WalletKey) VerifySig(msg, sig []byte) bool {
	return ethec.VerifySignature(wk.pubKey, DefaultHash(msg), sig)
}

func (wk *WalletKey) PubKey() []byte {
	return wk.pubKey
}

func (wk *WalletKey) String() string {
	bz, _ := tmjson.MarshalIndent(wk, "", "  ")
	return string(bz)
}

const DefaultWalletKeyDirPerm = 0700
const DefaultWalletKeyDir = "walkeys"
const DefaultValKeyDir = "walkeys/vals"

func createWalletKeyFile(s []byte, dir string) (*WalletKey, error) {
	wk := NewWalletKey(s)
	filePath := filepath.Join(dir, fmt.Sprintf("wk%X.json", wk.Address))
	if _, err := wk.Save(libs.NewFileWriter(filePath)); err != nil {
		return nil, err
	}
	return wk, nil
}

func CreateWalletKeyFiles(s []byte, cnt int, dir string) ([]*WalletKey, error) {
	var wks []*WalletKey
	for i := 0; i < cnt; i++ {
		wk, err := createWalletKeyFile(s, dir)
		if err != nil {
			return nil, err
		}
		wks = append(wks, wk)
	}
	return wks, nil
}
