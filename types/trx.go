package types

import (
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/holiman/uint256"
)

type ITrx interface {
	GetVersion()
	GetType()
	FromAddr() Address
	ToAddr() Address
	GetNonce() int64
	GetAmount() *uint256.Int
	GetGas() int64
	GetGasPrice() *uint256.Int
	GetSignature() bytes.HexBytes
	GetSigParts() (bytes.HexBytes, bytes.HexBytes, byte) // R, S, V
	RawSignatureValue() bytes.HexBytes
}
