package types_test

import (
	"github.com/beatoz/beatoz-go/types"
	"github.com/stretchr/testify/require"
	"math/rand"
	"strconv"
	"testing"
)

func TestConvertAsset(t *testing.T) {
	r := rand.Int63()
	grans := types.ToGrans(r)
	require.Equal(t, strconv.FormatInt(r, 10)+"000000000000000000", grans.Dec())

	xco, rem := types.FromGransRem(grans)
	require.Equal(t, r, xco)
	require.Equal(t, int64(0), rem)
}
