package version

import (
	"github.com/stretchr/testify/require"
	"strconv"
	"testing"
)

func TestVersionParsing(t *testing.T) {
	parseVersions("v1.2.3", "abcdef0123")
	require.Equal(t, uint64(1), majorVer)
	require.Equal(t, uint64(2), minorVer)
	require.Equal(t, uint64(3), patchVer)

	n, err := strconv.ParseUint("abcdef0123", 16, 64)
	require.NoError(t, err)
	require.Equal(t, n, commitVer)
}
