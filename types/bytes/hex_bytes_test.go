package bytes

import (
	"encoding/base64"
	"encoding/hex"
	"github.com/beatoz/beatoz-go/libs/jsonx"
	"github.com/stretchr/testify/require"
	"testing"
)

var (
	bz20 = RandBytes(32)
)

// Test_UnmarshalJSON_HexString test to unmarshal hex string("AABB...").
func Test_UnmarshalJSON_HexString(t *testing.T) {
	hexStr := hex.EncodeToString(bz20)
	require.Equal(t, len(bz20)*2, len(hexStr))
	data := []byte("\"" + hexStr + "\"")

	hexBytes := HexBytes{}
	require.NoError(t, jsonx.Unmarshal(data, &hexBytes))
	require.Equal(t, HexBytes(bz20), hexBytes)
}

// Test_UnmarshalJSON_0xHexString test to unmarshal "0xAABB...".
func Test_UnmarshalJSON_0xHexString(t *testing.T) {
	hexStr := hex.EncodeToString(bz20)
	require.Equal(t, len(bz20)*2, len(hexStr))
	data := []byte("\"0x" + hexStr + "\"")

	hexBytes := HexBytes{}
	require.NoError(t, jsonx.Unmarshal(data, &hexBytes))
	require.Equal(t, HexBytes(bz20), hexBytes)
}

// Test_UnmarshalJSON_Base64 test to unmarshal base64 string.
func Test_UnmarshalJSON_Base64(t *testing.T) {
	b64 := base64.StdEncoding.EncodeToString(bz20)
	require.True(t, len(b64) > 0)
	data := []byte("\"" + b64 + "\"")

	hexBytes := HexBytes{}
	require.NoError(t, jsonx.Unmarshal(data, &hexBytes))
	require.Equal(t, HexBytes(bz20), hexBytes)
}
