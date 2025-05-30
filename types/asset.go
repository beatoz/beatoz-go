package types

import (
	"fmt"
	"github.com/holiman/uint256"
)

const (
	DECIMAL int16 = 18
	FONS          = "1"
	KFONS         = "1000"
	MFONS         = "1000000"
	GFONS         = "1000000000"
	UBTOZ         = "1000000000000"
	MiBTOZ        = "1000000000000000"
	BTOZ          = "1000000000000000000"             // 10^18 fons
	KBTOZ         = "1000000000000000000000"          // 10^21 fons
	MBTOZ         = "1000000000000000000000000"       // 10^24 fons
	GBTOZ         = "1000000000000000000000000000"    // 10^27 fons
	TBTOZ         = "1000000000000000000000000000000" // 10^30 fons
)

// Simplest Asset Unit (FONS)

var (
	oneCoinInFons = uint256.MustFromDecimal(BTOZ)
)

// Coin to fons
func ToFons(n uint64) *uint256.Int {
	return new(uint256.Int).Mul(uint256.NewInt(n), oneCoinInFons)
}

// from fons to BTOZ and Remain
func FromFons(sau *uint256.Int) (uint64, uint64) {
	r := new(uint256.Int)
	q, r := new(uint256.Int).DivMod(sau, oneCoinInFons, r)
	return q.Uint64(), r.Uint64()
}

func ToBTOZ(sau *uint256.Int) uint64 {
	r := new(uint256.Int)
	q, _ := new(uint256.Int).DivMod(sau, oneCoinInFons, r)
	return q.Uint64()
}

func FormattedString(sau *uint256.Int) string {
	q, r := FromFons(sau)
	return fmt.Sprintf("%d.%018d", q, r)
}
