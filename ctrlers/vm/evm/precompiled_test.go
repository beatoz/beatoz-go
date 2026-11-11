package evm

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"

	"golang.org/x/crypto/cryptobyte"
	"golang.org/x/crypto/cryptobyte/asn1"
)

func TestP256Verify(t *testing.T) {
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	msgH := sha256.Sum256([]byte("hello world"))

	// Sign the hash
	sigDER, err := privKey.Sign(rand.Reader, msgH[:], nil)
	require.NoError(t, err)

	// Parse DER signature to get R and S
	r, s, err := parseDERSignature(sigDER)
	require.NoError(t, err)

	contractInput := make([]byte, P256VerifyInputLength)
	copy(contractInput[0:32], msgH[:])
	copy(contractInput[32:64], common.LeftPadBytes(r.Bytes(), 32))
	copy(contractInput[64:96], common.LeftPadBytes(s.Bytes(), 32))
	copy(contractInput[96:128], common.LeftPadBytes(privKey.PublicKey.X.Bytes(), 32))
	copy(contractInput[128:160], common.LeftPadBytes(privKey.PublicKey.Y.Bytes(), 32))

	// ecr := &beatoz_p256Verify{}
	ecr := vm.PrecompiledContractsHomestead[common.BytesToAddress([]byte{0x1, 0x00})]

	ret, err := ecr.Run(contractInput)
	require.NoError(t, err)
	require.Equal(t, true32Byte, ret)

	// invalid length
	ret, err = ecr.Run(contractInput[:159])
	require.NotEqual(t, true32Byte, ret)
	require.ErrorContains(t, err, "invalid input length")

	// wrong pubKey
	wrongKey := privKey.PublicKey.X.Bytes()
	wrongKey[0] = 0xff
	copy(contractInput[0:32], msgH[:])
	copy(contractInput[32:64], common.LeftPadBytes(r.Bytes(), 32))
	copy(contractInput[64:96], common.LeftPadBytes(s.Bytes(), 32))
	copy(contractInput[96:128], common.LeftPadBytes(wrongKey, 32))
	copy(contractInput[128:160], common.LeftPadBytes(privKey.PublicKey.Y.Bytes(), 32))
	ret, err = ecr.Run(contractInput)
	require.NotEqual(t, true32Byte, ret)
	require.ErrorContains(t, err, "p256:invalid public key")

	// wrong message
	wrongMsgH := sha256.Sum256([]byte("wrong msg"))
	copy(contractInput[0:32], wrongMsgH[:])
	copy(contractInput[32:64], common.LeftPadBytes(r.Bytes(), 32))
	copy(contractInput[64:96], common.LeftPadBytes(s.Bytes(), 32))
	copy(contractInput[96:128], common.LeftPadBytes(privKey.PublicKey.X.Bytes(), 32))
	copy(contractInput[128:160], common.LeftPadBytes(privKey.PublicKey.Y.Bytes(), 32))

	ret, err = ecr.Run(contractInput)
	require.NotEqual(t, true32Byte, ret)
}

func parseDERSignature(sigDER []byte) (*big.Int, *big.Int, error) {
	var (
		r, s  = new(big.Int), new(big.Int)
		inner cryptobyte.String
	)
	input := cryptobyte.String(sigDER)
	if !input.ReadASN1(&inner, asn1.SEQUENCE) ||
		!input.Empty() ||
		!inner.ReadASN1Integer(r) ||
		!inner.ReadASN1Integer(s) ||
		!inner.Empty() {
		return nil, nil, errors.New("invalid DER signature")
	}
	return r, s, nil
}

var (
	issuer = []byte(`-----BEGIN CERTIFICATE-----
MIICLDCCAdOgAwIBAgIQH6d76oDp1GqKeNDklR8FdjAKBggqhkjOPQQDAjBhMQsw
CQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy
YW5jaXNjbzEQMA4GA1UEChMHb3JnMS5iYzETMBEGA1UEAxMKY2Eub3JnMS5iYzAe
Fw0yNjAxMjkxMDMyMDBaFw0zNjAxMjcxMDMyMDBaMGExCzAJBgNVBAYTAlVTMRMw
EQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQHEw1TYW4gRnJhbmNpc2NvMRAwDgYD
VQQKEwdvcmcxLmJjMRMwEQYDVQQDEwpjYS5vcmcxLmJjMFkwEwYHKoZIzj0CAQYI
KoZIzj0DAQcDQgAEMZCfmyaotsYE4xo8mxtIN+xThVZd9G/R6tANlQUSVFrHWs6y
kiYLSnuVpDSlzDbdKJTUIGJa+2Wan8qyocXpRaNtMGswDgYDVR0PAQH/BAQDAgGm
MB0GA1UdJQQWMBQGCCsGAQUFBwMCBggrBgEFBQcDATAPBgNVHRMBAf8EBTADAQH/
MCkGA1UdDgQiBCDsSsgnzIOgG2p9LH2Vj/ll6KFUOxCrXVcReXDuOc3v0jAKBggq
hkjOPQQDAgNHADBEAiBLnsdw+tZW/y4aZPoKwcvuo9dqb+pZJ9Igs1IBpCKxkwIg
Q7TIYjpz2spaFk0e4KuyxUYZXpFIRhV5ail4HJs7WLc=
-----END CERTIFICATE-----`)
	subject = []byte(`-----BEGIN CERTIFICATE-----
MIICDTCCAbOgAwIBAgIQKOj0TxDuiUdbp1bBS+O7TjAKBggqhkjOPQQDAjBhMQsw
CQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy
YW5jaXNjbzEQMA4GA1UEChMHb3JnMS5iYzETMBEGA1UEAxMKY2Eub3JnMS5iYzAe
Fw0yNjAxMjkxMDMyMDBaFw0zNjAxMjcxMDMyMDBaMGExCzAJBgNVBAYTAlVTMRMw
EQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQHEw1TYW4gRnJhbmNpc2NvMQ0wCwYD
VQQLEwRwZWVyMRYwFAYDVQQDEw1wZWVyMC5vcmcxLmJjMFkwEwYHKoZIzj0CAQYI
KoZIzj0DAQcDQgAEqNVzBDAMK/TaGVTnLkLH9fsKY7y2UbV3SjMmRoY/cc7N3JWK
p6fF8iuVAo9bVEpXqgRqk4l8njzc5TiBHEWoxKNNMEswDgYDVR0PAQH/BAQDAgeA
MAwGA1UdEwEB/wQCMAAwKwYDVR0jBCQwIoAg7ErIJ8yDoBtqfSx9lY/5ZeihVDsQ
q11XEXlw7jnN79IwCgYIKoZIzj0EAwIDSAAwRQIhAKvuftdE5esrXRAs1nJYquhv
uJplZhKQGVm6+e2x0c9wAiAlBZuG0N1b60drYfAMrNG8z/Fgj8Gxs9PZIY2OJMui
PA==
-----END CERTIFICATE-----`)
)

func TestX509Verify(t *testing.T) {
	// 1. Parse issuer certificate and print info
	issuerBlock, _ := pem.Decode(issuer)
	require.NotNil(t, issuerBlock, "failed to decode issuer PEM")
	issuerCert, err := x509.ParseCertificate(issuerBlock.Bytes)
	require.NoError(t, err)

	fmt.Println("=== Issuer Certificate ===")
	fmt.Printf("  Subject:      %s\n", issuerCert.Subject)
	fmt.Printf("  Issuer:       %s\n", issuerCert.Issuer)
	//fmt.Printf("  SerialNumber: %s\n", issuerCert.SerialNumber)
	//fmt.Printf("  NotBefore:    %s\n", issuerCert.NotBefore)
	//fmt.Printf("  NotAfter:     %s\n", issuerCert.NotAfter)
	//fmt.Printf("  IsCA:         %v\n", issuerCert.IsCA)
	//fmt.Printf("  SigAlgorithm: %s\n", issuerCert.SignatureAlgorithm)

	issuerPubKey, ok := issuerCert.PublicKey.(*ecdsa.PublicKey)
	require.True(t, ok, "issuer public key is not ECDSA")
	fmt.Printf("  PubKey.X:     %x\n", issuerPubKey.X)
	fmt.Printf("  PubKey.Y:     %x\n", issuerPubKey.Y)

	// 2. Parse subject certificate and print info
	subjectBlock, _ := pem.Decode(subject)
	require.NotNil(t, subjectBlock, "failed to decode subject PEM")
	subjectCert, err := x509.ParseCertificate(subjectBlock.Bytes)
	require.NoError(t, err)

	fmt.Println("=== Subject Certificate ===")
	fmt.Printf("  Subject:      %s\n", subjectCert.Subject)
	fmt.Printf("  Issuer:       %s\n", subjectCert.Issuer)
	//fmt.Printf("  SerialNumber: %s\n", subjectCert.SerialNumber)
	//fmt.Printf("  OU:           %v\n", subjectCert.Subject.OrganizationalUnit)
	//fmt.Printf("  NotBefore:    %s\n", subjectCert.NotBefore)
	//fmt.Printf("  NotAfter:     %s\n", subjectCert.NotAfter)
	//fmt.Printf("  IsCA:         %v\n", subjectCert.IsCA)
	//fmt.Printf("  SigAlgorithm: %s\n", subjectCert.SignatureAlgorithm)

	subjectPubKey, ok := subjectCert.PublicKey.(*ecdsa.PublicKey)
	require.True(t, ok, "subject public key is not ECDSA")
	fmt.Printf("  PubKey.X:     %x\n", subjectPubKey.X)
	fmt.Printf("  PubKey.Y:     %x\n", subjectPubKey.Y)

	// 3. Verify certificate chain using beatoz_x509Verify
	//    Input: [4B len][issuer DER][4B len][subject DER]
	var contractInput []byte
	// append issuer DER (length-prefixed)
	subjectLen := make([]byte, 4)
	binary.BigEndian.PutUint32(subjectLen, uint32(len(subjectBlock.Bytes)))
	contractInput = append(contractInput, subjectLen...)
	contractInput = append(contractInput, subjectBlock.Bytes...)
	issuerLen := make([]byte, 4)
	binary.BigEndian.PutUint32(issuerLen, uint32(len(issuerBlock.Bytes)))
	contractInput = append(contractInput, issuerLen...)
	contractInput = append(contractInput, issuerBlock.Bytes...)
	// append subject DER (length-prefixed)

	verifier := &beatoz_x509Verify{}
	ret, err := verifier.Run(contractInput)
	require.NoError(t, err)
	require.NotNil(t, ret)

	// Parse return: [32B pubkey.x][32B pubkey.y][32B serialNumber][OU string]
	require.GreaterOrEqual(t, len(ret), 96)
	retX := new(big.Int).SetBytes(ret[0:32])
	retY := new(big.Int).SetBytes(ret[32:64])
	retSerial := new(big.Int).SetBytes(ret[64:96])
	retOU := string(ret[96:])

	fmt.Println("=== beatoz_x509Verify Result ===")
	fmt.Printf("  PubKey.X:     %x\n", retX)
	fmt.Printf("  PubKey.Y:     %x\n", retY)
	fmt.Printf("  SerialNumber: %s\n", retSerial)
	fmt.Printf("  OU:           %s\n", retOU)

	// Verify returned values match the last (subject) certificate
	require.Equal(t, subjectPubKey.X, retX)
	require.Equal(t, subjectPubKey.Y, retY)
	require.Equal(t, subjectCert.SerialNumber, retSerial)
	require.Equal(t, "peer", retOU)
}

var (
	issuerMultiOU = []byte(`-----BEGIN CERTIFICATE-----
MIICATCCAaigAwIBAgIQYLlHnQ9K1CYMp7//tDvKvDAKBggqhkjOPQQDAjBhMQsw
CQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy
YW5jaXNjbzEQMA4GA1UEChMHb3JnMS5iYzETMBEGA1UEAxMKY2Eub3JnMS5iYzAe
Fw0yNjExMDIwMDAwMDBaFw0zNjExMDIwMDAwMDBaMGExCzAJBgNVBAYTAlVTMRMw
EQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQHEw1TYW4gRnJhbmNpc2NvMRAwDgYD
VQQKEwdvcmcxLmJjMRMwEQYDVQQDEwpjYS5vcmcxLmJjMFkwEwYHKoZIzj0CAQYI
KoZIzj0DAQcDQgAEqR0v+zG7nScKNPEBKL8KMysS7vG8zBA461SWPVzrrgNBYiE3
YUcBC6iK0+97eij/0m1RXrZ+j3eaBqdy/UtvSqNCMEAwDgYDVR0PAQH/BAQDAgKE
MA8GA1UdEwEB/wQFMAMBAf8wHQYDVR0OBBYEFKSKD+SEtc9/nyasiDnxjOi855rG
MAoGCCqGSM49BAMCA0cAMEQCID6fg326Q1Dl6qbiIDVjmnkRncUuKKH+qZvjU4L1
ULMLAiBvLgMgLq2fC03stq3DnRVgeONKpR5ZtXVLPhqCiPwjgQ==
-----END CERTIFICATE-----`)
	subjectMultiOU = []byte(`-----BEGIN CERTIFICATE-----
MIICIzCCAcmgAwIBAgIQAxOMQoF3uQYPqI5ms4phITAKBggqhkjOPQQDAjBhMQsw
CQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy
YW5jaXNjbzEQMA4GA1UEChMHb3JnMS5iYzETMBEGA1UEAxMKY2Eub3JnMS5iYzAe
Fw0yNjExMDIwMDAwMDBaFw0zNjExMDIwMDAwMDBaMIGCMQswCQYDVQQGEwJVUzET
MBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZyYW5jaXNjbzEuMAsG
A1UECxMEb3JnMTALBgNVBAsTBHBlZXIwEgYDVQQLEwtkZXBhcnRtZW50MTEWMBQG
A1UEAxMNcGVlcjAub3JnMS5iYzBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABPsM
fxrjssK2qWBaXaXlEWqPnDWZp4TApw2QFkR+RLFzHRj0upqrsw0r2iGpTlzaWgHm
FUZGvslEvqw8rkAQxuijQTA/MA4GA1UdDwEB/wQEAwIHgDAMBgNVHRMBAf8EAjAA
MB8GA1UdIwQYMBaAFKSKD+SEtc9/nyasiDnxjOi855rGMAoGCCqGSM49BAMCA0gA
MEUCIQD6RCFfp9h1/lhYJtgfhV2jVtBDN8eX7DCE7/ufVCdCegIgQwv1gFQOLWkI
NBEdaij+s2Sq/nBlX/OrnUEx+Umo7Sw=
-----END CERTIFICATE-----`)
)

func TestX509VerifyMultipleOUs(t *testing.T) {
	// 1. Parse issuer certificate and print info
	issuerBlock, _ := pem.Decode(issuerMultiOU)
	require.NotNil(t, issuerBlock, "failed to decode issuer PEM")
	issuerCert, err := x509.ParseCertificate(issuerBlock.Bytes)
	require.NoError(t, err)

	fmt.Println("=== Issuer Certificate ===")
	fmt.Printf("  Subject:      %s\n", issuerCert.Subject)
	fmt.Printf("  Issuer:       %s\n", issuerCert.Issuer)

	// 2. Parse subject certificate and print info
	subjectBlock, _ := pem.Decode(subjectMultiOU)
	require.NotNil(t, subjectBlock, "failed to decode subject PEM")
	subjectCert, err := x509.ParseCertificate(subjectBlock.Bytes)
	require.NoError(t, err)

	fmt.Println("=== Subject Certificate ===")
	fmt.Printf("  Subject:      %s\n", subjectCert.Subject)
	fmt.Printf("  Issuer:       %s\n", subjectCert.Issuer)

	subjectPubKey, ok := subjectCert.PublicKey.(*ecdsa.PublicKey)
	require.True(t, ok, "subject public key is not ECDSA")
	fmt.Printf("  PubKey.X:     %x\n", subjectPubKey.X)
	fmt.Printf("  PubKey.Y:     %x\n", subjectPubKey.Y)

	// 3. Verify certificate chain using beatoz_x509Verify
	//    Input: [4B len][subject DER][4B len][issuer DER]
	var contractInput []byte
	subjectLen := make([]byte, 4)
	binary.BigEndian.PutUint32(subjectLen, uint32(len(subjectBlock.Bytes)))
	contractInput = append(contractInput, subjectLen...)
	contractInput = append(contractInput, subjectBlock.Bytes...)
	issuerLen := make([]byte, 4)
	binary.BigEndian.PutUint32(issuerLen, uint32(len(issuerBlock.Bytes)))
	contractInput = append(contractInput, issuerLen...)
	contractInput = append(contractInput, issuerBlock.Bytes...)

	verifier := &beatoz_x509VerifyV2{}
	ret, err := verifier.Run(contractInput)
	require.NoError(t, err)
	require.NotNil(t, ret)

	// Parse return: [32B pubkey.x][32B pubkey.y][32B serialNumber][1B OU count]([1B OU length][OU bytes])...
	require.GreaterOrEqual(t, len(ret), 97)
	retX := new(big.Int).SetBytes(ret[0:32])
	retY := new(big.Int).SetBytes(ret[32:64])
	retSerial := new(big.Int).SetBytes(ret[64:96])
	retOUCnt := int(ret[96])
	offset := 97
	var retOUs []string
	for i := 0; i < retOUCnt; i++ {
		require.Less(t, offset, len(ret))
		retOULen := int(ret[offset])
		offset++
		require.LessOrEqual(t, offset+retOULen, len(ret))
		retOUs = append(retOUs, string(ret[offset:offset+retOULen]))
		offset += retOULen
	}
	require.Len(t, ret, offset)

	fmt.Println("=== beatoz_x509VerifyV2 Result ===")
	fmt.Printf("  PubKey.X:     %x\n", retX)
	fmt.Printf("  PubKey.Y:     %x\n", retY)
	fmt.Printf("  SerialNumber: %s\n", retSerial)
	fmt.Printf("  OU:           %v\n", retOUs)

	// Verify returned values match the subject certificate
	require.Equal(t, subjectPubKey.X, retX)
	require.Equal(t, subjectPubKey.Y, retY)
	require.Equal(t, subjectCert.SerialNumber, retSerial)
	require.Equal(t, 3, retOUCnt)
	require.Equal(t, []string{"org1", "peer", "department1"}, retOUs)
}

func TestX509VerifyMultipleOUsFirstOU(t *testing.T) {
	subjectBlock, _ := pem.Decode(subjectMultiOU)
	require.NotNil(t, subjectBlock)
	issuerBlock, _ := pem.Decode(issuerMultiOU)
	require.NotNil(t, issuerBlock)
	subjectCert, err := x509.ParseCertificate(subjectBlock.Bytes)
	require.NoError(t, err)
	subjectPubKey, ok := subjectCert.PublicKey.(*ecdsa.PublicKey)
	require.True(t, ok)

	var contractInput []byte
	for _, der := range [][]byte{subjectBlock.Bytes, issuerBlock.Bytes} {
		contractInput = binary.BigEndian.AppendUint32(contractInput, uint32(len(der)))
		contractInput = append(contractInput, der...)
	}

	var expected []byte
	expected = append(expected, common.LeftPadBytes(subjectPubKey.X.Bytes(), 32)...)
	expected = append(expected, common.LeftPadBytes(subjectPubKey.Y.Bytes(), 32)...)
	expected = append(expected, common.LeftPadBytes(subjectCert.SerialNumber.Bytes(), 32)...)
	expected = append(expected, []byte("org1")...)

	ret, err := (&beatoz_x509Verify{}).Run(contractInput)
	require.NoError(t, err)
	require.Equal(t, expected, ret)
}

func TestX509PrecompileAddresses(t *testing.T) {
	for _, precompiles := range []map[common.Address]vm.PrecompiledContract{
		vm.PrecompiledContractsHomestead,
		vm.PrecompiledContractsByzantium,
		vm.PrecompiledContractsIstanbul,
		vm.PrecompiledContractsBerlin,
		vm.PrecompiledContractsCancun,
	} {
		require.IsType(t, &beatoz_x509Verify{}, precompiles[common.BytesToAddress([]byte{0xff, 0x00})])
		require.IsType(t, &beatoz_x509VerifyV2{}, precompiles[common.BytesToAddress([]byte{0x2, 0x00})])
	}
}
