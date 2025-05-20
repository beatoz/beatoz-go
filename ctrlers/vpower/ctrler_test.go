package vpower

import (
	beatozcfg "github.com/beatoz/beatoz-go/cmd/config"
	"github.com/beatoz/beatoz-go/ctrlers/mocks"
	"github.com/beatoz/beatoz-go/ctrlers/mocks/acct"
	"github.com/beatoz/beatoz-go/ctrlers/mocks/gov"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	btztypes "github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/crypto"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/rand"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

var (
	config   *beatozcfg.Config
	govMock  *gov.GovHandlerMock
	acctMock *acct.AcctHandlerMock
)

func init() {
	rootDir := filepath.Join(os.TempDir(), "test-vpowctrler")
	config = beatozcfg.DefaultConfig()
	config.SetRoot(rootDir)
	acctMock = acct.NewAcctHandlerMock(1000)
	acctMock.Iterate(func(idx int, w *web3.Wallet) bool {
		w.GetAccount().SetBalance(btztypes.ToFons(1_000_000_000))
		return true
	})

	govMock = gov.NewGovHandlerMock(ctrlertypes.DefaultGovParams())
	govMock.GetValues().LazyUnbondingBlocks = 500
	govMock.GetValues().InflationCycleBlocks = 10
	govMock.GetValues().MinSignedBlocks = 5
	govMock.GetValues().RipeningBlocks = 10 * govMock.InflationCycleBlocks()
}

func Test_NewValidatorSet(t *testing.T) {
	require.NoError(t, os.RemoveAll(config.RootDir))

	ctrler, lastValUps0, valWallets0, xerr := initLedger(config)
	require.NoError(t, xerr)
	require.Equal(t, len(lastValUps0), len(valWallets0))

	_ = mocks.InitBlockCtxWith(config.ChainID, 1, govMock, acctMock, nil, nil, ctrler)

	var expectedValUps []types.ValidatorUpdate

	for mocks.LastBlockHeight() < 100 {
		//fmt.Println("--------- height:", mocks.CurrBlockHeight())
		require.NoError(t, mocks.DoBeginBlock(ctrler))
		require.NotNil(t, ctrler.allDelegatees)

		if mocks.CurrBlockHeight() >= 2 {
			expectedValUps, xerr = randBonding(ctrler)
			require.NoError(t, xerr)
		}

		require.NoError(t, mocks.DoEndBlock(ctrler))
		require.NotNil(t, ctrler.allDelegatees)
		require.NotNil(t, ctrler.lastValidators)

		// bctx.ValUpdates should equal to expectedValUps
		bctx := mocks.CurrBlockCtx()
		//for i, v := range expectedValUps {
		//	fmt.Printf("expectedValUps[%d]: %X, power: %v\n", i, v.PubKey.GetSecp256K1(), v.Power)
		//}
		//for i, v := range bctx.ValUpdates {
		//	fmt.Printf("bctx.ValUpdates[%d]: %X, power: %v\n", i, v.PubKey.GetSecp256K1(), v.Power)
		//}

		require.Equal(t, len(expectedValUps), len(bctx.ValUpdates), "height", bctx.Height())
		for _, vup := range expectedValUps {
			_vup, cnt := findValUp(vup.PubKey.GetSecp256K1(), bctx.ValUpdates)
			require.Equal(t, 1, cnt)
			require.Equal(t, vup.Power, _vup.Power)
		}
		//fmt.Printf("%d validators are changed, all delegatees are %v\n", len(bctx.ValUpdates), len(ctrler.allDelegatees))

		require.NoError(t, mocks.DoCommit(ctrler))

		//// check ledger state
		//require.NoError(t, xerr)
		//xerr = ctrler.vpowerState.Seek(v1.KeyPrefixDelegatee, true, func(key v1.LedgerKey, item v1.ILedgerItem) xerrors.XError {
		//	dgtee := item.(*Delegatee)
		//	require.NotNil(t, findValUp(dgtee.PubKey, expectedValUps))
		//	require.NotNil(t, findValUp(dgtee.PubKey, bctx.ValUpdates))
		//	return nil
		//}, true)
		//require.NoError(t, xerr)

		//for i, v := range ctrler.lastValidators {
		//	fmt.Printf("last validator[%d]: %X, power: %v\n", i, v.PubKey, v.SumPower)
		//}

		//fmt.Println("---------")

	}
	require.NoError(t, ctrler.Close())
	require.NoError(t, os.RemoveAll(config.DBDir()))
}

func randBonding(ctrler *VPowerCtrler) ([]types.ValidatorUpdate, xerrors.XError) {
	allDgtees := copyDelegateeArray(ctrler.allDelegatees)
	lastVals := copyDelegateeArray(ctrler.lastValidators)

	// delegating...
	fromW := acctMock.RandWallet()
	fromAddr := fromW.Address()

	toPubKey := fromW.GetPubKey()
	toAddr := crypto.PubKeyBytes2Addr(toPubKey)

	power := rand.Int63n(lastVals[0].SumPower) + govMock.MinValidatorPower()

	if _, xerr := doDelegate(ctrler, fromW, toAddr, power, mocks.CurrBlockHeight()); xerr != nil {
		return nil, xerr
	}

	if dgt := findDelegateeByAddr(toAddr, allDgtees); dgt == nil {
		dgt = NewDelegatee(toPubKey)
		dgt.addPower(fromAddr, power)
		dgt.addDelegator(fromAddr)
		allDgtees = append(allDgtees, dgt)
	} else {
		// add power to exist validator
		dgt.addPower(fromAddr, power)
		dgt.addDelegator(fromAddr)
	}
	sort.Sort(OrderByPowerDelegatees(allDgtees))
	newLastVals := allDgtees[:govMock.MaxValidatorCnt()]
	sort.Sort(OrderByPowerDelegatees(lastVals))
	sort.Sort(OrderByPowerDelegatees(newLastVals))

	//
	// Compute expected result
	var expected []types.ValidatorUpdate
	for _, newDgt := range newLastVals {
		exist := findDelegateeByAddr(newDgt.addr, lastVals)
		if exist == nil {
			// new validator
			expected = append(expected, types.UpdateValidator(newDgt.PubKey, newDgt.SumPower, "secp256k1"))
		}
	}

	for _, exist := range lastVals {
		newDgt := findDelegateeByAddr(exist.addr, newLastVals)
		if newDgt == nil {
			// out
			expected = append(expected, types.UpdateValidator(exist.PubKey, 0, "secp256k1"))
		} else if exist.SumPower != newDgt.SumPower {
			// update
			expected = append(expected, types.UpdateValidator(newDgt.PubKey, newDgt.SumPower, "secp256k1"))
		}
	}
	return expected, nil
}

func findValUp(pubKey bytes.HexBytes, vals []types.ValidatorUpdate) (*types.ValidatorUpdate, int) {
	var ret *types.ValidatorUpdate
	cnt := 0
	for _, v := range vals {
		if bytes.Equal(pubKey, v.PubKey.GetSecp256K1()) {
			cnt++
			ret = &v
		}
	}
	return ret, cnt
}
