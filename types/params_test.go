package types

import (
	"testing"

	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func Test_ForkBlocks(t *testing.T) {
	chainId := uint256.MustFromHex("0xbea701")

	require.False(t, IsBTIP27(chainId.Hex(), 0))
	require.False(t, IsBTIP27(chainId.Hex(), 100))
	require.True(t, IsBTIP27(chainId.Hex(), 194_850))
	require.True(t, IsBTIP27(chainId.Hex(), 200_000))
}
