package vpower

import (
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"
	"math/rand"
	"os"
	"testing"
)

func Test_BlockCount(t *testing.T) {
	a0 := BlockCount(1234)
	bz, xerr := a0.Encode()
	require.NoError(t, xerr)

	//fmt.Printf("%x\n", bz)

	var a1 BlockCount
	xerr = a1.Decode(nil, bz)
	require.NoError(t, xerr)
	//fmt.Println(a0, a1)
}

func Test_SignBlock(t *testing.T) {
	require.NoError(t, os.RemoveAll(config.RootDir))

	ctrler, xerr := NewVPowerCtrler(config, int(govMock.MaxValidatorCnt()), log.NewNopLogger())
	require.NoError(t, xerr)

	signerAddr := types.RandAddress()
	count := rand.Intn(100)

	for i := 0; i < count; i++ {
		_, xerr := ctrler.addMissedBlockCount(signerAddr, true)
		require.NoError(t, xerr)
	}

	n, xerr := ctrler.getMissedBlockCount(signerAddr, true)
	require.NoError(t, xerr)
	require.EqualValues(t, count, n)

	require.NoError(t, ctrler.Close())
	require.NoError(t, os.RemoveAll(config.DBDir()))
}

func Test_SignBlock_Reset(t *testing.T) {
	require.NoError(t, os.RemoveAll(config.RootDir))

	ctrler, xerr := NewVPowerCtrler(config, int(govMock.MaxValidatorCnt()), log.NewNopLogger())
	require.NoError(t, xerr)

	signerAddr := types.RandAddress()
	count := rand.Intn(100)

	for i := 0; i < count; i++ {
		_, xerr := ctrler.addMissedBlockCount(signerAddr, true)
		require.NoError(t, xerr)
	}
	c, xerr := ctrler.getMissedBlockCount(signerAddr, true)
	require.NoError(t, xerr)
	require.EqualValues(t, count, c)

	// delete all missed block count
	require.NoError(t, ctrler.resetAllMissedBlockCount(true))

	_, xerr = ctrler.getMissedBlockCount(signerAddr, true)
	require.Error(t, xerr)
	require.True(t, xerr.Contains(xerrors.ErrNotFoundResult))

	require.NoError(t, ctrler.Close())
	require.NoError(t, os.RemoveAll(config.DBDir()))
}
