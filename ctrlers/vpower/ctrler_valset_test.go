package vpower

import (
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/crypto"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/abci/types"
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"
	"github.com/tendermint/tendermint/libs/rand"
	"math"
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
	dgtee := NewDelegatee(pub)
	dgtee.SumPower = pow
	return dgtee
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
