package evm

import (
	"github.com/beatoz/beatoz-go/types/bytes"
	beatoz_crypto "github.com/beatoz/beatoz-go/types/crypto"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"testing"
)

var (
	prvKeyHex       = "83b8749ffd3b90bb26bdfa430f8df21d881df9962eb96b4ee68b3f60c57c5ccb"
	expectedBTCAddr = "7612536BD0991DB67E60DA9ECA1E3E276889B8DC"
)

func TestEcRecover(t *testing.T) {
	// create and check signature
	prvKey, err := beatoz_crypto.ImportPrvKeyHex(prvKeyHex)
	require.NoError(t, err)

	pubKey := prvKey.PublicKey

	randBytes := bytes.RandBytes(1024)
	sig, err := beatoz_crypto.Sign(randBytes, prvKey)
	require.NoError(t, err)
	require.True(t, beatoz_crypto.VerifySig(beatoz_crypto.CompressPubkey(&pubKey), randBytes, sig))

	addr0, _, err := beatoz_crypto.Sig2Addr(randBytes, sig)
	require.NoError(t, err)
	require.Equal(t, expectedBTCAddr, addr0.String())

	// test for beatoz_ecrecover
	ecr_input := make([]byte, 128)
	copy(ecr_input, beatoz_crypto.DefaultHash(randBytes))
	ecr_input[63] = sig[64]
	copy(ecr_input[64:], sig)

	ecr := &beatoz_ecrecover{}
	addr1, err := ecr.Run(ecr_input)
	require.NoError(t, err)
	require.Equal(t, common.LeftPadBytes(addr0, 32), addr1)
	require.Equal(t, expectedBTCAddr, bytes.HexBytes(common.TrimLeftZeroes(addr1)).String())
}
