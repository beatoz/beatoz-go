package supply

import (
	"fmt"
	btzcfg "github.com/beatoz/beatoz-go/cmd/config"
	"github.com/beatoz/beatoz-go/ctrlers/mocks"
	"github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/ctrlers/vpower"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/crypto"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"os"
	"path/filepath"
	"testing"
)

var (
	config    *btzcfg.Config
	govParams *types.GovParams
)

func init() {
	rootDir := filepath.Join(os.TempDir(), "supply-test")
	config = btzcfg.DefaultConfig()
	config.SetRoot(rootDir)

	govParams = types.DefaultGovParams()
}

func Test_InitLedger(t *testing.T) {
	require.NoError(t, os.RemoveAll(config.RootDir))

	intiSupply := types.PowerToAmount(350_000_000)
	ctrler, xerr := initLedger(intiSupply)
	require.NoError(t, xerr)
	require.Equal(t, intiSupply.Dec(), ctrler.lastTotalSupply.Dec())
	require.Equal(t, intiSupply.Dec(), ctrler.lastAdjustedSupply.Dec())
	require.Equal(t, int64(1), ctrler.lastAdjustedHeight)

	_ = mocks.InitBlockCtxWith(1, nil, govParams, nil)
	require.NoError(t, mocks.DoBeginBlock(ctrler))
	require.NoError(t, mocks.DoCommitBlock(ctrler))
	require.NoError(t, ctrler.Close())

	ctrler, xerr = NewSupplyCtrler(config, log.NewNopLogger())
	require.NoError(t, xerr)
	require.Equal(t, intiSupply.Dec(), ctrler.lastTotalSupply.Dec())
	require.Equal(t, intiSupply.Dec(), ctrler.lastAdjustedSupply.Dec())
	require.Equal(t, int64(1), ctrler.lastAdjustedHeight)

	require.NoError(t, ctrler.Close())
	require.NoError(t, os.RemoveAll(config.RootDir))
}

func Test_Mint(t *testing.T) {
	require.NoError(t, os.RemoveAll(config.RootDir))

	initSupply := types.PowerToAmount(350_000_000)
	adjustedSupply := initSupply.Clone()
	ctrler, xerr := initLedger(initSupply)

	vpowCtrler, xerr := vpower.NewVPowerCtrler(config, int(govParams.MaxValidatorCnt()), log.NewNopLogger())
	require.NoError(t, xerr)

	_, pub := crypto.NewKeypairBytes()
	dgtee := vpower.NewDelegatee(pub)
	vpow := vpower.NewVPower(dgtee.Address(), dgtee.PubKey) // self power
	xerr = vpowCtrler.BondPowerChunk(dgtee, vpow, 70_000_000, 1, bytes.RandBytes(32), true)
	require.NoError(t, xerr)

	height0 := govParams.InflationCycleBlocks()
	bctx := types.NewBlockContext(abcitypes.RequestBeginBlock{
		Header: tmproto.Header{Height: height0},
	}, govParams, nil, vpowCtrler, nil)

	// before vpowCtrler.EndBlock. (vpowCtrler.lastValidators is nil)
	// expect 0 minting
	ctrler.mint(bctx)
	result, xerr := ctrler.waitMint()
	require.NoError(t, xerr)
	supplyHeight := result.newSupply.Height
	totalSupply := new(uint256.Int).SetBytes(result.newSupply.XSupply)
	changeSupply := new(uint256.Int).SetBytes(result.newSupply.XChange)

	require.Equal(t, height0, supplyHeight)
	require.Equal(t, initSupply.String(), totalSupply.String())
	require.Equal(t, "0", changeSupply.String())

	_, xerr = vpowCtrler.EndBlock(bctx)
	require.NoError(t, xerr)

	for currHeight := int64(2); currHeight < oneYearSeconds*30; currHeight += govParams.InflationCycleBlocks() {
		// expect x minting

		wa := vpower.WaEx64ByPowerChunk(vpow.PowerChunks, currHeight, govParams.RipeningBlocks(), govParams.BondingBlocksWeightPermil(), totalSupply)
		wa = wa.Truncate(6)

		expectedTotalSupply := Si(currHeight, 1, adjustedSupply, govParams.MaxTotalSupply(), govParams.InflationWeightPermil(), wa)
		expectedChange := new(uint256.Int).Sub(expectedTotalSupply, totalSupply)
		fmt.Println("expected", "height", currHeight, "wa", wa.String(), "adjustedSupply", adjustedSupply, "adjustedHeight", 1, "max", govParams.MaxTotalSupply(), "lamda", govParams.InflationWeightPermil(), "t1", expectedTotalSupply, "t0", totalSupply)

		bctx := types.NewBlockContext(abcitypes.RequestBeginBlock{
			Header: tmproto.Header{Height: currHeight},
		}, govParams, nil, vpowCtrler, nil)
		ctrler.mint(bctx)
		result, xerr = ctrler.waitMint()
		require.NoError(t, xerr)
		supplyHeight = result.newSupply.Height
		totalSupply = new(uint256.Int).SetBytes(result.newSupply.XSupply)
		changeSupply = new(uint256.Int).SetBytes(result.newSupply.XChange)

		require.Equal(t, currHeight, supplyHeight)
		require.NotEqual(t, expectedTotalSupply.Dec(), initSupply.Dec())
		require.Equal(t, expectedTotalSupply.Dec(), totalSupply.Dec(), "height", currHeight)
		require.NotEqual(t, "0", changeSupply.Dec())
		require.Equal(t, expectedChange.Dec(), changeSupply.Dec())
	}

	require.NoError(t, ctrler.Close())
	require.NoError(t, vpowCtrler.Close())
	require.NoError(t, os.RemoveAll(config.RootDir))
}

func Test_Burn(t *testing.T) {

}

func initLedger(initSupply *uint256.Int) (*SupplyCtrler, xerrors.XError) {
	ctrler, xerr := NewSupplyCtrler(config, log.NewNopLogger())
	if xerr != nil {
		return nil, xerr
	}

	if xerr := ctrler.InitLedger(initSupply); xerr != nil {
		return nil, xerr
	}
	return ctrler, nil
}
