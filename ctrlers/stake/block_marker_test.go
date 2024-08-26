package stake

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestBlockMarker(t *testing.T) {
	marker := &BlockMarker{}

	require.NoError(t, marker.Mark(1))
	require.NoError(t, marker.Mark(100))
	require.Error(t, marker.Mark(100))
	require.Error(t, marker.Mark(99))
	require.NoError(t, marker.Mark(101))
	require.NoError(t, marker.Mark(201))
	require.NoError(t, marker.Mark(332))
	require.NoError(t, marker.Mark(1331))
	require.NoError(t, marker.Mark(2134))

	require.Equal(t, 2, marker.CountInWindow(1, 100, false))
	require.Equal(t, 3, marker.CountInWindow(100, 300, false))
	require.Equal(t, 0, marker.CountInWindow(200, 100, false))
	require.Equal(t, 2, marker.CountInWindow(300, 1331, true))

	require.Equal(t, 0, marker.CountInWindow(1, 100, false))
	require.Equal(t, 0, marker.CountInWindow(100, 300, false))
	require.Equal(t, 2, marker.CountInWindow(300, 1331, true))
	require.Equal(t, 3, marker.CountInWindow(300, 2134, true))
	require.Equal(t, 0, marker.CountInWindow(2135, 2137, true))
}
