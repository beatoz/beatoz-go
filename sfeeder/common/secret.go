package common

import (
	"fmt"
	"github.com/beatoz/beatoz-go/libs"
	"github.com/beatoz/beatoz-go/types/crypto"
	"path/filepath"
)

func WriteSecret(addr, secretVal, pass []byte, dataDir string) (int, error) {

	hashedAddr := crypto.DefaultHash(addr)

	plaintext := secretVal
	if plaintext == nil {
		plaintext = libs.ReadCredential(fmt.Sprintf("Enter secret for %X: ", addr))
	}
	sk := crypto.DefaultHash(hashedAddr, []byte(pass))
	ciphertext, err := Enc(sk, plaintext)
	if err != nil {
		libs.ClearCredential(sk)
		return 0, err
	}
	libs.ClearCredential(sk)

	fd := libs.NewFileWriter(filepath.Join(dataDir, fmt.Sprintf("%X", hashedAddr)))
	return fd.Write(ciphertext)
}

func ReadSecret(addr, pass []byte, dataDir string) ([]byte, error) {
	hashedAddr := crypto.DefaultHash(addr)

	fd := libs.NewFileReader(filepath.Join(dataDir, fmt.Sprintf("%X", hashedAddr)))
	buf := make([]byte, 1024)
	n, err := fd.Read(buf)
	if err != nil {
		return nil, err
	}

	ciphertext := buf[:n]
	sk := crypto.DefaultHash(hashedAddr, pass)
	defer libs.ClearCredential(sk)

	return Dec(sk, ciphertext)
}

func ReadPlainSecret(addr []byte, dataDir string) ([]byte, error) {
	hashedAddr := crypto.DefaultHash(addr)

	fd := libs.NewFileReader(filepath.Join(dataDir, fmt.Sprintf("%X", hashedAddr)))
	buf := make([]byte, 1024)
	n, err := fd.Read(buf)
	if err != nil {
		return nil, err
	}

	return buf[:n], nil
}
