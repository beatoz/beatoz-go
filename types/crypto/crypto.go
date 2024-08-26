package crypto

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"github.com/beatoz/beatoz-go/types"
	abytes "github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	tmsecp256k1 "github.com/tendermint/tendermint/crypto/secp256k1"
	"hash"
)

func NewPrvKey() (*ecdsa.PrivateKey, error) {
	return ethcrypto.GenerateKey()
}

func ImportPrvKey(d []byte) (*ecdsa.PrivateKey, error) {
	return ethcrypto.ToECDSA(d)
}

func ImportPrvKeyHex(d string) (*ecdsa.PrivateKey, error) {
	return ethcrypto.HexToECDSA(d)
}

func Sign(msg []byte, prv *ecdsa.PrivateKey) ([]byte, error) {
	hmsg := DefaultHash(msg)
	return ethcrypto.Sign(hmsg, prv)
}

func VerifySig(pubkey, msg, sig []byte) bool {
	hmsg := DefaultHash(msg)
	if len(sig) == ethcrypto.SignatureLength {
		sig = sig[:64]
	}
	return ethcrypto.VerifySignature(pubkey, hmsg, sig)
}

func Pub2Addr(pub *ecdsa.PublicKey) types.Address {
	// todo: generate address like as ethereum style

	//a := ethcrypto.PubkeyToAddress(*pub)
	//return a[:]

	pubKeyBytes := CompressPubkey(pub)
	ret, _ := PubBytes2Addr(pubKeyBytes)
	return ret
}

// pubBytes is 33 bytes compressed format
func PubBytes2Addr(pubBytes []byte) (types.Address, xerrors.XError) {
	// ethereum style
	//pub, err := ethcrypto.DecompressPubkey(pubBytes)
	//if err != nil {
	//	return nil, err
	//}
	//a := ethcrypto.PubkeyToAddress(*pub)
	//return a[:], nil

	return abytes.HexBytes(tmsecp256k1.PubKey(pubBytes).Address()), nil
}

func CompressPubkey(pub *ecdsa.PublicKey) abytes.HexBytes {
	return ethcrypto.CompressPubkey(pub)
}

func DecompressPubkey(bz []byte) (*ecdsa.PublicKey, xerrors.XError) {
	if pubKey, err := ethcrypto.DecompressPubkey(bz); err != nil {
		return nil, xerrors.From(err)
	} else {
		return pubKey, nil
	}
}

func Sig2Addr(msg, sig []byte) (types.Address, abytes.HexBytes, xerrors.XError) {
	hmsg := DefaultHash(msg)
	pubKey, err := ethcrypto.SigToPub(hmsg, sig)
	if err != nil {
		return nil, nil, xerrors.From(err)
	}

	return Pub2Addr(pubKey), CompressPubkey(pubKey), nil
}

func DefaultHash(datas ...[]byte) []byte {
	hasher := DefaultHasher()
	for _, bz := range datas {
		hasher.Write(bz)
	}
	return hasher.Sum(nil)
}

func DefaultHasher() hash.Hash {
	//return ethcrypto.NewKeccakState()
	return sha256.New()
}

func DefaultHasherName() string {
	//return "keccak256"
	return "sha256"
}

//
// for padding
//

var (
	// ErrInvalidBlockSize indicates hash blocksize <= 0.
	ErrInvalidBlockSize = xerrors.NewOrdinary("invalid blocksize")

	// ErrInvalidPKCS5Data indicates bad input to PKCS7 pad or unpad.
	ErrInvalidPKCS5Data = xerrors.NewOrdinary("invalid PKCS5 data (empty or not padded)")
	// ErrInvalidPKCS5Padding indicates PKCS5 unpad fails to bad input.
	ErrInvalidPKCS5Padding = xerrors.NewOrdinary("invalid PKCS5 padding on input")

	// ErrInvalidPKCS7Data indicates bad input to PKCS7 pad or unpad.
	ErrInvalidPKCS7Data = xerrors.NewOrdinary("invalid PKCS7 data (empty or not padded)")
	// ErrInvalidPKCS7Padding indicates PKCS7 unpad fails to bad input.
	ErrInvalidPKCS7Padding = xerrors.NewOrdinary("invalid PKCS7 padding on input")
)

func PKCS5Padding(b []byte, blocksize int) ([]byte, error) {
	if blocksize%8 != 0 {
		return nil, ErrInvalidBlockSize
	}
	if b == nil || len(b) == 0 {
		return nil, ErrInvalidPKCS5Data
	}

	pad := blocksize - len(b)%blocksize
	padbuf := bytes.Repeat([]byte{byte(pad)}, pad)
	return append(b, padbuf...), nil
}

func PKCS5UnPadding(b []byte, blocksize int) ([]byte, error) {
	if blocksize%8 != 0 {
		return nil, ErrInvalidBlockSize
	}
	if b == nil || len(b) == 0 {
		return nil, ErrInvalidPKCS5Data
	}
	if len(b)%8 != 0 {
		return nil, ErrInvalidPKCS5Data
	}

	pad := b[len(b)-1]
	padlen := int(pad)
	if padlen == 0 || len(b) < padlen || padlen > blocksize {
		return nil, ErrInvalidPKCS5Data
	}
	for i := 0; i < padlen; i++ {
		if b[len(b)-padlen+i] != pad {
			return nil, ErrInvalidPKCS5Padding
		}
	}
	return b[:(len(b) - padlen)], nil
}

// pkcs7Padding right-pads the given byte slice with 1 to n bytes, where
// n is the block size. The size of the result is x times n, where x
// is at least 1.
func PKCS7Padding(b []byte, blocksize int) ([]byte, error) {
	if blocksize <= 0 {
		return nil, ErrInvalidBlockSize
	}
	if b == nil || len(b) == 0 {
		return nil, ErrInvalidPKCS7Data
	}

	pad := blocksize - (len(b) % blocksize)
	pb := bytes.Repeat([]byte{byte(pad)}, pad)
	return append(b, pb...), nil
}

// pkcs7UnPadding validates and unpads data from the given bytes slice.
// The returned value will be 1 to n bytes smaller depending on the
// amount of padding, where n is the block size.
func PKCS7UnPadding(b []byte, blocksize int) ([]byte, error) {
	if blocksize <= 0 {
		return nil, ErrInvalidBlockSize
	}
	if b == nil || len(b) == 0 {
		return nil, ErrInvalidPKCS7Data
	}
	if len(b)%blocksize != 0 {
		return nil, ErrInvalidPKCS7Padding
	}
	c := b[len(b)-1]
	n := int(c)
	if n == 0 || n > len(b) || n > blocksize {
		return nil, ErrInvalidPKCS7Padding
	}
	for i := 0; i < n; i++ {
		if b[len(b)-n+i] != c {
			return nil, ErrInvalidPKCS7Padding
		}
	}
	return b[:len(b)-n], nil
}
