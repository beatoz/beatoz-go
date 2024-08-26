package bytes

import (
	"crypto/rand"
	"encoding/hex"
	"github.com/holiman/uint256"
	rand2 "github.com/tendermint/tendermint/libs/rand"
)

func RandBytes(n int) []byte {
	bz := make([]byte, n)
	_, _ = rand.Read(bz)
	return bz
}

func ZeroBytes(n int) []byte {
	return make([]byte, n)
}

func RandHexBytes(n int) HexBytes {
	bz := RandBytes(n)
	return HexBytes(bz)
}

func RandHexString(n int) string {
	bz := RandBytes(n)
	return "0x" + hex.EncodeToString(bz)
}

func RandU256IntN(cap *uint256.Int) *uint256.Int {
	b, _ := rand.Int(rand.Reader, cap.ToBig())
	return uint256.MustFromBig(b)
}

func RandU256Int() *uint256.Int {
	return new(uint256.Int).SetBytes(RandBytes(32))
}

func RandInt64N(cap int64) int64 {
	return rand2.Int63n(cap)
}

func RandUint64N(cap uint64) uint64 {
	return rand2.Uint64() % cap
}

func ClearBytes(b []byte) {
	for i := 0; i < len(b); i++ {
		b[i] = 0x00
	}
}
