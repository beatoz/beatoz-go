package types

import (
	"github.com/holiman/uint256"
)

const (
	COIN_DECIMAL int16 = 18
	FONS               = "1"
	KFONS              = "1000"
	MFONS              = "1000000"
	GFONS              = "1000000000"
	UBEATOZ            = "1000000000000"
	MiBEATOZ           = "1000000000000000"
	COIN               = "1000000000000000000"             // 10^18 fons
	KCOIN              = "1000000000000000000000"          // 10^21 fons
	MCOIN              = "1000000000000000000000000"       // 10^24 fons
	GCOIN              = "1000000000000000000000000000"    // 10^27 fons
	TCOIN              = "1000000000000000000000000000000" // 10^30 fons
)

// Simplest Asset Unit (FONS)

var (
	oneCoinInFons = uint256.MustFromDecimal(COIN)
)

// Coin to fons
func ToFons(n uint64) *uint256.Int {
	return new(uint256.Int).Mul(uint256.NewInt(n), oneCoinInFons)
}

// from fons to COIN and Remain
func FromFons(sau *uint256.Int) (uint64, uint64) {
	r := new(uint256.Int)
	q, r := new(uint256.Int).DivMod(sau, oneCoinInFons, r)
	return q.Uint64(), r.Uint64()
}
