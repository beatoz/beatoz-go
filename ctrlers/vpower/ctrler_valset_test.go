package vpower

import (
	"fmt"
	"github.com/beatoz/beatoz-go/ctrlers/mocks"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/crypto"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/abci/types"
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"
	"github.com/tendermint/tendermint/libs/rand"
	"math"
	"os"
	"sort"
	"testing"
)

// Test_validatorUpdates tests that the validatorUpdates function correctly returns the difference
// between the previous validator set and the new validator set.
func Test_validatorUpdates(t *testing.T) {
	maxValCnt := 5

	for _i := 0; _i < 1000; _i++ {
		var alls []*Delegatee
		var lastVals []*Delegatee

		topPow := int64(0)
		bottomPow := int64(math.MaxInt64)
		for i := 0; i < maxValCnt*2; i++ {
			pow := int64(i + 1)
			alls = append(alls, makeDelegateeOne(int64(i+1)))

			topPow = max(topPow, pow)
		}

		newVals := selectValidators(alls, maxValCnt)
		for i, v := range newVals {
			require.Equal(t, topPow-int64(i), v.SumPower)
		}

		upVals := validatorUpdates(lastVals, newVals)
		require.Len(t, upVals, len(newVals))

		for _, u := range upVals {
			uPub, err := cryptoenc.PubKeyFromProto(u.PubKey)
			require.NoError(t, err)

			dgtee := findDelegateeByPubKey(uPub.Bytes(), newVals)
			require.NotNil(t, dgtee)
			require.Equal(t, dgtee.SumPower, u.Power)
		}

		//
		// add new validator
		lastVals = copyDelegateeArray(newVals)

		sort.Sort(orderByPowerDelegatee(alls))
		expectedOutDgtee := alls[maxValCnt-1]

		bottomPow = alls[maxValCnt-1].SumPower
		pow := bytes.RandInt64N(topPow-bottomPow) + bottomPow + 1
		expectedNewDgtee := makeDelegateeOne(pow)

		alls = append(alls, expectedNewDgtee)

		newVals = selectValidators(alls, maxValCnt)
		upVals = validatorUpdates(lastVals, newVals)

		require.Equal(t, 2, len(upVals)) // 2 = out(1) + new(1)

		for _, u := range upVals {
			uPub, err := cryptoenc.PubKeyFromProto(u.PubKey)
			require.NoError(t, err)
			if bytes.Equal(uPub.Bytes(), expectedNewDgtee.PubKey) {
				require.Equal(t, expectedNewDgtee.SumPower, u.Power)
			} else if bytes.Equal(uPub.Bytes(), expectedOutDgtee.PubKey) {
				require.Equal(t, int64(0), u.Power)
			} else {
				require.True(t, false, "not reachable")
			}
		}

		//
		// slash
		lastVals = copyDelegateeArray(newVals)

		sort.Sort(orderByPowerDelegatee(alls))
		expectedNewDgtee = alls[maxValCnt]

		// slash the power of one of validators.
		// as a result, the validator is excluded from validator set.
		bottomPow = expectedNewDgtee.SumPower
		expectedOutDgtee = alls[rand.Intn(maxValCnt)]
		expectedOutDgtee.SumPower = bottomPow - 1

		newVals = selectValidators(alls, maxValCnt)
		upVals = validatorUpdates(lastVals, newVals)

		require.Equal(t, 2, len(upVals))
		for _, u := range upVals {
			uPub, err := cryptoenc.PubKeyFromProto(u.PubKey)
			require.NoError(t, err)
			if bytes.Equal(uPub.Bytes(), expectedNewDgtee.PubKey) {
				require.Equal(t, expectedNewDgtee.SumPower, u.Power)
			} else if bytes.Equal(uPub.Bytes(), expectedOutDgtee.PubKey) {
				// it was removed.
				require.Equal(t, int64(0), u.Power)
			} else {
				require.True(t, false, "not reachable")
			}
		}

		//
		// slash partially
		lastVals = copyDelegateeArray(newVals)

		sort.Sort(orderByPowerDelegatee(alls))
		expectedUpdatedVal := alls[0]
		expectedUpdatedVal.SumPower--

		// slash the power of one of validators.
		// as a result, the changed power of validator is included validator update.
		newVals = selectValidators(alls, maxValCnt)
		upVals = validatorUpdates(lastVals, newVals)

		require.Equal(t, 1, len(upVals))
		for _, u := range upVals {
			uPub, err := cryptoenc.PubKeyFromProto(u.PubKey)
			require.NoError(t, err)
			if bytes.Equal(uPub.Bytes(), expectedUpdatedVal.PubKey) {
				require.Equal(t, expectedUpdatedVal.SumPower, u.Power)
			} else {
				require.True(t, false, "not reachable")
			}
		}
	}
}

func makeDelegateeOne(pow int64) *Delegatee {
	_, pub := crypto.NewKeypairBytes()
	dgtee := newDelegatee(pub)
	dgtee.SumPower = pow
	return dgtee
}

func Test_NewValidatorSet(t *testing.T) {
	require.NoError(t, os.RemoveAll(config.RootDir))

	ctrler, lastValUps0, valWallets0, xerr := initLedger(config)
	require.NoError(t, xerr)
	require.Equal(t, len(lastValUps0), len(valWallets0))

	_ = mocks.InitBlockCtxWith(1, acctMock, govParams, ctrler)

	var expectedValUps []types.ValidatorUpdate

	for mocks.LastBlockHeight() < 100 {
		fmt.Println("--------- height:", mocks.CurrBlockHeight())
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
		for i, v := range expectedValUps {
			fmt.Printf("expectedValUps[%d]: %X, power: %v\n", i, v.PubKey.GetSecp256K1(), v.Power)
		}
		for i, v := range bctx.ValUpdates {
			fmt.Printf("bctx.ValUpdates[%d]: %X, power: %v\n", i, v.PubKey.GetSecp256K1(), v.Power)
		}

		require.Equal(t, len(expectedValUps), len(bctx.ValUpdates), "height", bctx.Height())
		for _, vup := range expectedValUps {
			_vup, cnt := findValUp(vup.PubKey.GetSecp256K1(), bctx.ValUpdates)
			require.Equal(t, 1, cnt)
			require.Equal(t, vup.Power, _vup.Power)
		}
		fmt.Printf("%d validators are changed, all delegatees are %v\n", len(bctx.ValUpdates), len(ctrler.allDelegatees))

		require.NoError(t, mocks.DoCommitBlock(ctrler))

		//// check ledger state
		//require.NoError(t, xerr)
		//xerr = ctrler.powersState.Seek(v1.KeyPrefixDelegatee, true, func(key v1.LedgerKey, item v1.ILedgerItem) xerrors.XError {
		//	dgtee := item.(*Delegatee)
		//	require.NotNil(t, findValUp(dgtee.PubKey, expectedValUps))
		//	require.NotNil(t, findValUp(dgtee.PubKey, bctx.ValUpdates))
		//	return nil
		//}, true)
		//require.NoError(t, xerr)

		//for i, v := range ctrler.lastValidators {
		//	fmt.Printf("last validator[%d]: %X, power: %v\n", i, v.PubKey, v.SumPower)
		//}
		fmt.Println("---------")

	}
}

func randBonding(ctrler *VPowerCtrler) ([]types.ValidatorUpdate, xerrors.XError) {
	allDgtees := copyDelegateeArray(ctrler.allDelegatees)
	lastVals := copyDelegateeArray(ctrler.lastValidators)

	// delegating...
	fromW := acctMock.RandWallet()
	fromAddr := fromW.Address()

	toPubKey := fromW.GetPubKey()
	toAddr := crypto.PubKeyBytes2Addr(toPubKey)

	power := rand.Int63n(lastVals[0].SumPower) + govParams.MinValidatorPower()

	if _, xerr := doDelegate(ctrler, fromW, toAddr, power, mocks.CurrBlockHeight()); xerr != nil {
		return nil, xerr
	}

	if dgt := findDelegateeByAddr(toAddr, allDgtees); dgt == nil {
		dgt = newDelegatee(toPubKey)
		dgt.addPower(fromAddr, power)
		dgt.addDelegator(fromAddr)
		allDgtees = append(allDgtees, dgt)
	} else {
		// add power to exist validator
		dgt.addPower(fromAddr, power)
		dgt.addDelegator(fromAddr)
	}
	sort.Sort(orderByPowerDelegatee(allDgtees))
	newLastVals := allDgtees[:govParams.MaxValidatorCnt()]
	sort.Sort(orderByPowerDelegatee(lastVals))
	sort.Sort(orderByPowerDelegatee(newLastVals))

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

func existOnlyOne(pubKey bytes.HexBytes, vals []types.ValidatorUpdate) bool {
	found := false
	for _, v := range vals {
		if found {
			// already exist
			return false
		}
		if bytes.Equal(pubKey, v.PubKey.GetSecp256K1()) {
			found = true
		}
	}
	return found
}
