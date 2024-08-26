package version

import (
	"github.com/stretchr/testify/require"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"testing"
)

func TestVersionMasking(t *testing.T) {

	for i := 0; i < 100; i++ {
		rand := tmrand.NewRand()
		majorVer = uint64(rand.Intn(0xff))
		minorVer = uint64(rand.Intn(0xff))
		patchVer = uint64(rand.Intn(0xffff))
		commitVer = uint64(rand.Intn(0xffffffff))

		maskedVer := Uint64()
		require.Equal(t, majorVer<<56+minorVer<<48+patchVer<<32+commitVer, maskedVer)

		maskedVer = Uint64(MASK_MAJOR_VER, MASK_MINOR_VER, MASK_PATCH_VER, MASK_COMMIT_VER)
		require.Equal(t, majorVer<<56+minorVer<<48+patchVer<<32+commitVer, maskedVer)

		maskedVer = Uint64(MASK_MAJOR_VER)
		require.Equal(t, majorVer<<56, maskedVer)

		maskedVer = Uint64(MASK_MAJOR_VER, MASK_MINOR_VER)
		require.Equal(t, majorVer<<56+minorVer<<48, maskedVer)

		maskedVer = Uint64(MASK_MAJOR_VER, MASK_MINOR_VER, MASK_PATCH_VER)
		require.Equal(t, majorVer<<56+minorVer<<48+patchVer<<32, maskedVer)

		maskedVer = Uint64(MASK_MAJOR_VER, MASK_PATCH_VER)
		require.Equal(t, majorVer<<56+patchVer<<32, maskedVer)

		maskedVer = Uint64(MASK_MINOR_VER, MASK_COMMIT_VER)
		require.Equal(t, minorVer<<48+commitVer, maskedVer)
	}
}
