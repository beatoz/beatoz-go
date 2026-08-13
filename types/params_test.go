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

	require.False(t, IsBTIP45(chainId.Hex(), 499_999))
	require.True(t, IsBTIP45(chainId.Hex(), 500_000))
	require.True(t, IsBTIP45(chainId.Hex(), 500_001))
	require.True(t, IsBTIP45("0xbea700", 0))
	require.True(t, IsBTIP45("0xbea702", 0))
	require.True(t, IsBTIP45("0xabc", 0))
}
