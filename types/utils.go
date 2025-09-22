package types

import (
	"fmt"
	"math/big"
	"strings"
)

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

func ChainIdFrom(chainIdStr string) (*big.Int, error) {
	if IsHexByteString(chainIdStr) {
		chainId, ret := new(big.Int).SetString(chainIdStr[2:], 16)
		if ret {
			return chainId, nil
		}
	} else if IsNumericString(chainIdStr) {
		chainId, ret := new(big.Int).SetString(chainIdStr, 10)
		if ret {
			return chainId, nil
		}
	}
	return nil, fmt.Errorf("invalid chain id: %v", chainIdStr)
}
