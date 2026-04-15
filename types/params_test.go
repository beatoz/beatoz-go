package types

import (
	"testing"

	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func Test_ForkBlocks(t *testing.T) {
	chainId := uint256.MustFromHex("0xbea701")

	require.False(t, IsBud(chainId.Hex(), 0))
	require.False(t, IsBud(chainId.Hex(), 100))
	require.True(t, IsBud(chainId.Hex(), 100_000))
	require.True(t, IsBud(chainId.Hex(), 1_000_000))
}
