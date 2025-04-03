package vpower

import (
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/crypto"
	"github.com/stretchr/testify/require"
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"
	"math"
	"math/rand"
	"sort"
	"testing"
)

// Test_validatorUpdates tests that the validatorUpdates function correctly returns the difference
// between the previous validator set and the new validator set.
func Test_validatorUpdates(t *testing.T) {
	maxValCnt := 5

	for _i := 0; _i < 1000; _i++ {
		var alls DelegateeArray
		var lastVals DelegateeArray

		topPow := int64(0)
		bottomPow := int64(math.MaxInt64)
		for i := 0; i < maxValCnt*2; i++ {
			pow := int64(i + 1)
			alls = append(alls, makeDelegateeOne(int64(i+1)))

			topPow = max(topPow, pow)
		}

		newVals := selectValidators(alls, maxValCnt)
		for i, v := range newVals {
			require.Equal(t, topPow-int64(i), v.totalPower)
		}

		upVals := validatorUpdates(lastVals, newVals)
		require.Len(t, upVals, len(newVals))

		for _, u := range upVals {
			uPub, err := cryptoenc.PubKeyFromProto(u.PubKey)
			require.NoError(t, err)

			dgtee := findByPubKey(uPub.Bytes(), newVals)
			require.NotNil(t, dgtee)
			require.Equal(t, dgtee.totalPower, u.Power)
		}

		//
		// add new validator
		lastVals = copyDelegateeArray(newVals)

		sort.Sort(orderByPowerDelegatees(alls))
		expectedOutDgtee := alls[maxValCnt-1]

		bottomPow = alls[maxValCnt-1].totalPower
		pow := bytes.RandInt64N(topPow-bottomPow) + bottomPow + 1
		expectedNewDgtee := makeDelegateeOne(pow)

		alls = append(alls, expectedNewDgtee)

		newVals = selectValidators(alls, maxValCnt)
		upVals = validatorUpdates(lastVals, newVals)

		require.Equal(t, 2, len(upVals)) // 2 = out(1) + new(1)

		for _, u := range upVals {
			uPub, err := cryptoenc.PubKeyFromProto(u.PubKey)
			require.NoError(t, err)
			if bytes.Equal(uPub.Bytes(), expectedNewDgtee.pubKey) {
				require.Equal(t, expectedNewDgtee.totalPower, u.Power)
			} else if bytes.Equal(uPub.Bytes(), expectedOutDgtee.pubKey) {
				require.Equal(t, int64(0), u.Power)
			} else {
				require.True(t, false, "not reachable")
			}
		}

		//
		// slash
		lastVals = copyDelegateeArray(newVals)

		sort.Sort(orderByPowerDelegatees(alls))
		expectedNewDgtee = alls[maxValCnt]

		// slash the power of one of validators.
		// as a result, the validator is excluded from validator set.
		bottomPow = expectedNewDgtee.totalPower
		expectedOutDgtee = alls[rand.Intn(maxValCnt)]
		expectedOutDgtee.totalPower = bottomPow - 1

		newVals = selectValidators(alls, maxValCnt)
		upVals = validatorUpdates(lastVals, newVals)

		require.Equal(t, 2, len(upVals))
		for _, u := range upVals {
			uPub, err := cryptoenc.PubKeyFromProto(u.PubKey)
			require.NoError(t, err)
			if bytes.Equal(uPub.Bytes(), expectedNewDgtee.pubKey) {
				require.Equal(t, expectedNewDgtee.totalPower, u.Power)
			} else if bytes.Equal(uPub.Bytes(), expectedOutDgtee.pubKey) {
				// it was removed.
				require.Equal(t, int64(0), u.Power)
			} else {
				require.True(t, false, "not reachable")
			}
		}

		//
		// slash partially
		lastVals = copyDelegateeArray(newVals)

		sort.Sort(orderByPowerDelegatees(alls))
		expectedUpdatedVal := alls[0]
		expectedUpdatedVal.totalPower--

		// slash the power of one of validators.
		// as a result, the changed power of validator is included validator update.
		newVals = selectValidators(alls, maxValCnt)
		upVals = validatorUpdates(lastVals, newVals)

		require.Equal(t, 1, len(upVals))
		for _, u := range upVals {
			uPub, err := cryptoenc.PubKeyFromProto(u.PubKey)
			require.NoError(t, err)
			if bytes.Equal(uPub.Bytes(), expectedUpdatedVal.pubKey) {
				require.Equal(t, expectedUpdatedVal.totalPower, u.Power)
			} else {
				require.True(t, false, "not reachable")
			}
		}
	}
}

func makeDelegateeListRandPower(c int) []*Delegatee {
	var ret []*Delegatee
	for i := 0; i < c; i++ {
		ret = append(ret, makeDelegateeOne(bytes.RandInt64N(700_000_000)))
	}
	return ret
}
func makeDelegateeList(c int, pow int64) []*Delegatee {
	var ret []*Delegatee
	for i := 0; i < c; i++ {
		ret = append(ret, makeDelegateeOne(pow))
	}
	return ret
}

func makeDelegateeOne(pow int64) *Delegatee {
	_, pub := crypto.NewKeypairBytes()
	dgtee := NewDelegatee(pub)
	dgtee.totalPower = pow
	return dgtee
}
