package types

import (
	"github.com/holiman/uint256"
)

const (
	COIN_DECIMAL int16 = 18
	MOTE               = "1"
	KMOTE              = "1000"
	MMOTE              = "1000000"
	GMOTE              = "1000000000"
	UBEATOZ            = "1000000000000"
	MiBEATOZ           = "1000000000000000"
	COIN               = "1000000000000000000"             // 10^18 mote
	KCOIN              = "1000000000000000000000"          // 10^21 mote
	MCOIN              = "1000000000000000000000000"       // 10^24 mote
	GCOIN              = "1000000000000000000000000000"    // 10^27 mote
	TCOIN              = "1000000000000000000000000000000" // 10^30 mote
)

// Simplest Asset Unit (MOTE)

var (
	oneCoinInMote = uint256.MustFromDecimal(COIN)
)

// Coin to mote
func ToMote(n uint64) *uint256.Int {
	return new(uint256.Int).Mul(uint256.NewInt(n), oneCoinInMote)
}

// from mote to COIN and Remain
func FromMote(sau *uint256.Int) (uint64, uint64) {
	r := new(uint256.Int)
	q, r := new(uint256.Int).DivMod(sau, oneCoinInMote, r)
	return q.Uint64(), r.Uint64()
}
