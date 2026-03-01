package evm

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

func init() {
	// P256(secp256r1) signature verification
	vm.PrecompiledContractsHomestead[common.BytesToAddress([]byte{0x1, 0x00})] = &beatoz_p256Verify{}
	vm.PrecompiledContractsByzantium[common.BytesToAddress([]byte{0x1, 0x00})] = &beatoz_p256Verify{}
	vm.PrecompiledContractsIstanbul[common.BytesToAddress([]byte{0x1, 0x00})] = &beatoz_p256Verify{}
	vm.PrecompiledContractsBerlin[common.BytesToAddress([]byte{0x1, 0x00})] = &beatoz_p256Verify{}
	vm.PrecompiledContractsCancun[common.BytesToAddress([]byte{0x1, 0x00})] = &beatoz_p256Verify{}

	// x509 certificate verification
	vm.PrecompiledContractsHomestead[common.BytesToAddress([]byte{0xff, 0x00})] = &beatoz_x509Verify{}
	vm.PrecompiledContractsByzantium[common.BytesToAddress([]byte{0xff, 0x00})] = &beatoz_x509Verify{}
	vm.PrecompiledContractsIstanbul[common.BytesToAddress([]byte{0xff, 0x00})] = &beatoz_x509Verify{}
	vm.PrecompiledContractsBerlin[common.BytesToAddress([]byte{0xff, 0x00})] = &beatoz_x509Verify{}
	vm.PrecompiledContractsCancun[common.BytesToAddress([]byte{0xff, 0x00})] = &beatoz_x509Verify{}
}

const (
	P256VerifyGas         = 6900
	P256VerifyInputLength = 160

	X509VerifyBaseGas    = 50000
	X509VerifyPerCertGas = 50000
)

var (
	true32Byte = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
)

// P256VERIFY (secp256r1 signature verification)
// implemented as a native contract
type beatoz_p256Verify struct{}

// RequiredGas returns the gas required to execute the precompiled contract
func (c *beatoz_p256Verify) RequiredGas(input []byte) uint64 {
	return P256VerifyGas
}

// Run executes the precompiled contract with given 160 bytes of param, returning the output and the used gas
func (c *beatoz_p256Verify) Run(input []byte) ([]byte, error) {
	if len(input) != P256VerifyInputLength {
		return nil, errors.New("p256:invalid input length")
	}

	// Extract hash, r, s, x, y from the input.
	hash := input[0:32]
	r, s := new(big.Int).SetBytes(input[32:64]), new(big.Int).SetBytes(input[64:96])
	x, y := new(big.Int).SetBytes(input[96:128]), new(big.Int).SetBytes(input[128:160])

	// Verify the signature.
	if x == nil || y == nil || !elliptic.P256().IsOnCurve(x, y) {
		return nil, errors.New("p256:invalid public key")
	}
	pk := &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}
	if ecdsa.Verify(pk, hash, r, s) {
		return true32Byte, nil
	}

	return nil, errors.New("p256: failed verification")
}

type beatoz_x509Verify struct{}

func (c *beatoz_x509Verify) RequiredGas(input []byte) uint64 {
	n := countDERCerts(input)
	if n == 0 {
		return X509VerifyBaseGas
	}
	return X509VerifyBaseGas + uint64(n)*X509VerifyPerCertGas
}

// Run verifies an X.509 certificate chain and returns the last certificate's
// serial number, OU, and public key.
//
// Input encoding: [4-byte big-endian length][DER cert][4-byte big-endian length][DER cert]...
// Output encoding (ABI): abi.encode(bytes serialNumber, bytes ou, bytes publicKey)
func (c *beatoz_x509Verify) Run(input []byte) ([]byte, error) {
	// 1. Decode input into [][]byte (length-prefixed DER certificates)
	derCerts, err := decodeDERCertArray(input)
	if err != nil {
		return nil, err
	}
	if len(derCerts) == 0 {
		return nil, errors.New("x509: no certificates provided")
	}

	// 2. Parse all DER certificates
	certs := make([]*x509.Certificate, len(derCerts))
	for i, der := range derCerts {
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, fmt.Errorf("x509: failed to parse certificate %d: %v", i, err)
		}
		certs[i] = cert
	}

	// 3. Verify chain: certs[i] (issuer) verifies certs[i+1] (subject)
	for i := 0; i < len(certs)-1; i++ {
		issuer := certs[i]
		subject := certs[i+1]
		if err := subject.CheckSignatureFrom(issuer); err != nil {
			return nil, fmt.Errorf("x509: cert[%d] failed verification by cert[%d]: %v", i+1, i, err)
		}
	}

	// 4. Extract public key (x, y), serial number, and OU from the last certificate
	lastCert := certs[len(certs)-1]

	ecdsaKey, ok := lastCert.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("x509: unsupported public key type %T", lastCert.PublicKey)
	}

	// Return format: [32B pubkey.x][32B pubkey.y][32B serialNumber][OU string bytes]
	var result []byte
	result = append(result, common.LeftPadBytes(ecdsaKey.X.Bytes(), 32)...)
	result = append(result, common.LeftPadBytes(ecdsaKey.Y.Bytes(), 32)...)
	result = append(result, common.LeftPadBytes(lastCert.SerialNumber.Bytes(), 32)...)
	if len(lastCert.Subject.OrganizationalUnit) > 0 {
		result = append(result, []byte(lastCert.Subject.OrganizationalUnit[0])...)
	}

	return result, nil
}

// countDERCerts counts the number of length-prefixed DER certificates in data.
func countDERCerts(data []byte) int {
	count := 0
	offset := 0
	for offset+4 <= len(data) {
		certLen := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		offset += 4
		if certLen <= 0 || offset+certLen > len(data) {
			break
		}
		offset += certLen
		count++
	}
	return count
}

// decodeDERCertArray decodes length-prefixed DER certificates from input.
// Format: [4-byte big-endian length][DER data][4-byte big-endian length][DER data]...
func decodeDERCertArray(input []byte) ([][]byte, error) {
	var certs [][]byte
	offset := 0
	for offset < len(input) {
		if offset+4 > len(input) {
			return nil, errors.New("x509: truncated certificate length")
		}
		certLen := int(binary.BigEndian.Uint32(input[offset : offset+4]))
		offset += 4
		if certLen <= 0 {
			return nil, errors.New("x509: invalid certificate length")
		}
		if offset+certLen > len(input) {
			return nil, errors.New("x509: truncated certificate data")
		}
		certs = append(certs, input[offset:offset+certLen])
		offset += certLen
	}
	return certs, nil
}
