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

// A pre-release tag (e.g. injected from the develop branch) must still parse
// major/minor/patch correctly, and the pre-release suffix must be preserved
// in the version string printed by String().
func TestVersionParsingPreRelease(t *testing.T) {
	Version = "v1.3.0-rc.2"
	parseVersions(Version, "abc1234")
	require.Equal(t, uint64(1), majorVer)
	require.Equal(t, uint64(3), minorVer)
	require.Equal(t, uint64(0), patchVer)
	require.Contains(t, String(), "v1.3.0-rc.2")
}
