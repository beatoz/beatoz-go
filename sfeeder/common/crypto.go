package common

import (
	"crypto/aes"
	"crypto/cipher"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/crypto"
)

func Enc(sk, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(sk)
	if err != nil {
		return nil, err
	}
	iv := bytes.ZeroBytes(block.BlockSize())
	cbc := cipher.NewCBCEncrypter(block, iv)
	plaintext, err = crypto.PKCS7Padding(plaintext, block.BlockSize())
	if err != nil {
		return nil, err
	}
	ciphertext := make([]byte, len(plaintext))
	cbc.CryptBlocks(ciphertext, plaintext)

	return ciphertext, nil
}

func Dec(sk, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(sk)
	if err != nil {
		return nil, err
	}
	iv := bytes.ZeroBytes(block.BlockSize())
	cbc := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	cbc.CryptBlocks(plaintext, ciphertext)
	plaintext, err = crypto.PKCS7UnPadding(plaintext, block.BlockSize())
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}
