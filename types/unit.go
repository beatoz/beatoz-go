package types

import (
	"fmt"
	"strings"

	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
)

const (
	DECIMAL int16 = 18
)

var (
	amountPerPower = uint256.NewInt(1_000_000_000_000_000_000) // 1BEATOZ == 1Power
	oneCoinGrans   = uint256.NewInt(1_000_000_000_000_000_000)
)

func ToGrans(n int64) *uint256.Int {
	return new(uint256.Int).Mul(uint256.NewInt(uint64(n)), oneCoinGrans)
}

// from grans to coin and Remain
func FromGransRem(grans *uint256.Int) (int64, int64) {
	r := new(uint256.Int)
	q, r := new(uint256.Int).DivMod(grans, oneCoinGrans, r)
	return int64(q.Uint64()), int64(r.Uint64())
}

func FromGrans(grans *uint256.Int) int64 {
	r := new(uint256.Int)
	q, _ := new(uint256.Int).DivMod(grans, oneCoinGrans, r)
	return int64(q.Uint64())
}

func FormattedString(grans *uint256.Int) string {
	q, r := FromGransRem(grans)
	return fmt.Sprintf("%d.%018d", q, r)
}

func AmountToPower(amt *uint256.Int) (int64, xerrors.XError) {
	// 1 VotingPower == 1 BEATOZ
	_vp := new(uint256.Int).Div(amt, amountPerPower)
	vp := int64(_vp.Uint64())
	if vp < 0 {
		return -1, xerrors.ErrOverFlow.Wrapf("voting power is converted as negative(%v) from amount(%v)", vp, amt.Dec())
	}
	return vp, nil
}

func PowerToAmount(power int64) *uint256.Int {
	// 1 VotingPower == 1 BEATOZ = 10^18 amount
	return new(uint256.Int).Mul(uint256.NewInt(uint64(power)), amountPerPower)
}

func AmountPerPower() *uint256.Int {
	return amountPerPower.Clone()
}

func GasToFee(gas int64, price *uint256.Int) *uint256.Int {
	return new(uint256.Int).Mul(uint256.NewInt(uint64(gas)), price)
}

func FeeToGas(fee, price *uint256.Int) int64 {
	gas := new(uint256.Int).Div(fee, price)
	return int64(gas.Uint64())
}

var (
	digitTab [256]byte
	hexTab   [256]byte
)

func init() {
	// 0-9
	for c := byte('0'); c <= '9'; c++ {
		digitTab[c] = 1
		hexTab[c] = 1
	}
	// a-f, A-F
	for c := byte('a'); c <= 'f'; c++ {
		hexTab[c] = 1
	}
	for c := byte('A'); c <= 'F'; c++ {
		hexTab[c] = 1
	}
}

// IsHexByteString returns true if the string is a hexadecimal string (satisfying the conditions above)
// and its length is even (i.e., represents bytes).
func IsHexByteString(s string) bool {
	if len(s) < 2 || !strings.HasPrefix(s, "0x") {
		return false
	}

	s = s[2:]

	// check even length
	if (len(s) & 1) != 0 {
		return false
	}
	if len(s) == 0 { // empty is not allowed
		return false
	}
	for i := 0; i < len(s); i++ {
		if hexTab[s[i]] == 0 {
			return false
		}
	}
	return true
}

// IsNumericString returns true if s contains only digits [0-9].
// An empty string returns false.
func IsNumericString(s string) bool {
	if len(s) == 0 { // empty is not allowed
		return false
	}
	for i := 0; i < len(s); i++ {
		if digitTab[s[i]] == 0 {
			return false
		}
	}
	return true
}
