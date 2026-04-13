package types

import (
	"testing"

	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func Test_ForkBlocks(t *testing.T) {
	chainId := uint256.MustFromHex("0xbea701")

	require.False(t, IsForkedBig(chainId, 0, Bud))
	require.False(t, IsForkedBig(chainId, 100, Bud))
	require.True(t, IsForkedBig(chainId, 100_000, Bud))
	require.True(t, IsForkedBig(chainId, 1_000_000, Bud))
}
