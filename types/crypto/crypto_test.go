package crypto

import (
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/stretchr/testify/require"
	"testing"
)

var (
	prvKeyHex       = "83b8749ffd3b90bb26bdfa430f8df21d881df9962eb96b4ee68b3f60c57c5ccb"
	expectedBTCAddr = "7612536BD0991DB67E60DA9ECA1E3E276889B8DC"
)

func TestPub2Addr(t *testing.T) {
	prvKey, err := ImportPrvKeyHex(prvKeyHex)
	require.NoError(t, err)

	addr := Pub2Addr(&prvKey.PublicKey)
	require.Equal(t, expectedBTCAddr, addr.String())
}

func BenchmarkPub2Addr(b *testing.B) {
	prvKey, err := ImportPrvKeyHex(prvKeyHex)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Pub2Addr(&prvKey.PublicKey)
	}
}

func TestSig2Addr(t *testing.T) {
	prvKey, err := ImportPrvKeyHex(prvKeyHex)
	require.NoError(t, err)

	pubKey := prvKey.PublicKey

	randBytes := bytes.RandBytes(1024)
	sig, err := Sign(randBytes, prvKey)
	require.NoError(t, err)
	require.True(t, VerifySig(CompressPubkey(&pubKey), randBytes, sig))

	addr, _, err := Sig2Addr(randBytes, sig)
	require.NoError(t, err)
	require.Equal(t, expectedBTCAddr, addr.String())
}

func BenchmarkSig2Addr(b *testing.B) {
	prvKey, err := ImportPrvKeyHex(prvKeyHex)
	require.NoError(b, err)

	pubKey := prvKey.PublicKey

	randBytes := bytes.RandBytes(1024)
	sig, err := Sign(randBytes, prvKey)
	require.NoError(b, err)
	require.True(b, VerifySig(CompressPubkey(&pubKey), randBytes, sig))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err = Sig2Addr(randBytes, sig)
		require.NoError(b, err)
	}
}
