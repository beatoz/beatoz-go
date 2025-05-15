package supply

import (
	btzcfg "github.com/beatoz/beatoz-go/cmd/config"
	"github.com/beatoz/beatoz-go/ctrlers/mocks"
	acctmock "github.com/beatoz/beatoz-go/ctrlers/mocks/acct"
	govmock "github.com/beatoz/beatoz-go/ctrlers/mocks/gov"
	"github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"
	"os"
	"path/filepath"
	"testing"
)

var (
	config   *btzcfg.Config
	govMock  *govmock.GovHandlerMock
	acctMock *acctmock.AcctHandlerMock
)

func init() {
	rootDir := filepath.Join(os.TempDir(), "supply-test")
	config = btzcfg.DefaultConfig()
	config.SetRoot(rootDir)

	govMock = govmock.NewGovHandlerMock(types.DefaultGovParams())
	govMock.GetValues().InflationCycleBlocks = 10
	acctMock = acctmock.NewAcctHandlerMock(1000)
	//acctMock.Iterate(func(idx int, w *web3.Wallet) bool {
	//	w.GetAccount().SetBalance(btztypes.ToFons(1_000_000_000))
	//	return true
	//})
}

func Test_InitLedger(t *testing.T) {
	require.NoError(t, os.RemoveAll(config.RootDir))

	intiSupply := types.PowerToAmount(350_000_000)
	ctrler, xerr := initLedger(intiSupply)
	require.NoError(t, xerr)
	require.Equal(t, intiSupply.Dec(), ctrler.lastTotalSupply.GetTotalSupply().Dec())
	require.Equal(t, intiSupply.Dec(), ctrler.lastTotalSupply.GetAdjustSupply().Dec())
	require.Equal(t, int64(1), ctrler.lastTotalSupply.GetHeight())
	require.Equal(t, int64(1), ctrler.lastTotalSupply.GetAdjustHeight())

	_ = mocks.InitBlockCtxWith(1, govMock, nil, nil, nil, nil)
	require.NoError(t, mocks.DoBeginBlock(ctrler))
	require.NoError(t, mocks.DoCommitBlock(ctrler))
	require.NoError(t, ctrler.Close())

	ctrler, xerr = NewSupplyCtrler(config, log.NewNopLogger())
	require.NoError(t, xerr)
	require.Equal(t, intiSupply.Dec(), ctrler.lastTotalSupply.GetTotalSupply().Dec())
	require.Equal(t, intiSupply.Dec(), ctrler.lastTotalSupply.GetAdjustSupply().Dec())
	require.Equal(t, int64(1), ctrler.lastTotalSupply.GetHeight())
	require.Equal(t, int64(1), ctrler.lastTotalSupply.GetAdjustHeight())

	require.NoError(t, ctrler.Close())
	require.NoError(t, os.RemoveAll(config.RootDir))
}

func initLedger(initSupply *uint256.Int) (*SupplyCtrler, xerrors.XError) {
	ctrler, xerr := NewSupplyCtrler(config, log.NewNopLogger() /*log.NewTMLogger(os.Stdout)*/)
	if xerr != nil {
		return nil, xerr
	}

	if xerr := ctrler.InitLedger(initSupply); xerr != nil {
		return nil, xerr
	}
	return ctrler, nil
}
