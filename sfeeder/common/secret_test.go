package common

import (
	"github.com/beatoz/beatoz-go/types"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestSecret(t *testing.T) {
	addr := types.RandAddress()
	secret0 := []byte("dflasdjflsdjf;lsdjf")
	pwd := []byte("1111")
	dataDir := filepath.Join(os.TempDir(), "sfeeder_test")

	os.MkdirAll(dataDir, 0o700)

	_, err := WriteSecret(addr, secret0, pwd, dataDir)
	require.NoError(t, err)

	secret1, err := ReadPlainSecret(addr, dataDir)
	require.NoError(t, err)
	require.Greater(t, len(secret1), 16)
	require.NotEqual(t, secret0, secret1)

	// wrong pass
	secret1, err = ReadSecret(addr, []byte("11"), dataDir)
	require.Error(t, err)
	require.Nil(t, secret1)

	// success
	secret1, err = ReadSecret(addr, pwd, dataDir)
	require.NoError(t, err)
	require.Equal(t, secret0, secret1)

	require.NoError(t, os.RemoveAll(dataDir))
}
