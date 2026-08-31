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
			alls = append(alls, makeDelegateeOne(0, int64(i+1)))

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

		sort.Sort(OrderByPowerDelegatees(alls))
		expectedOutDgtee := alls[maxValCnt-1]

		bottomPow = alls[maxValCnt-1].SumPower
		pow := bytes.RandInt64N(topPow-bottomPow) + bottomPow + 1
		expectedNewDgtee := makeDelegateeOne(0, pow)

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

		sort.Sort(OrderByPowerDelegatees(alls))
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

		sort.Sort(OrderByPowerDelegatees(alls))
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

func makeDelegateeOne(selfPower, sumPower int64) *Delegatee {
	_, pub := crypto.NewKeypairBytes()
	dgtee := NewDelegatee(pub)
	dgtee.SelfPower = selfPower
	dgtee.SumPower = sumPower
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

func Test_hasEnoughSelfPower(t *testing.T) {
	require.True(t, hasEnoughSelfPower(0, 100, 0))
	require.True(t, hasEnoughSelfPower(50, 100, 50))
	require.False(t, hasEnoughSelfPower(49, 100, 50))
	// 30% of 251 is 75.3, so the minimum integral self power is 76.
	require.True(t, hasEnoughSelfPower(76, 251, 30))
	require.False(t, hasEnoughSelfPower(75, 251, 30))
}

func Test_selectEligibleValidators(t *testing.T) {
	// Case 1: No delegatees are available.
	got, xerr := selectEligibleValidators(nil, 3, 100, 50)
	require.Error(t, xerr)
	require.Empty(t, got)

	// Case 2: Delegatees below either eligibility threshold are filtered out.
	eligible := makeDelegateeOne(100, 100)
	belowMinPower := makeDelegateeOne(99, 99)
	belowMinRate := makeDelegateeOne(100, 1_000)
	got, xerr = selectEligibleValidators(
		[]*Delegatee{belowMinPower, belowMinRate, eligible},
		3,
		100,
		50,
	)
	require.NoError(t, xerr)
	require.Equal(t, []*Delegatee{eligible}, got)

	// Case 3: Existing top-N selection is preserved when all delegatees are eligible.
	var eligibles []*Delegatee
	for i := 0; i < 5; i++ {
		eligibles = append(eligibles, makeDelegateeOne(int64(1000*(i+1)), int64(1000*(i+1))))
	}
	expected := selectValidators(copyDelegateeArray(eligibles), 3)

	got, xerr = selectEligibleValidators(eligibles, 3, 100, 50)
	require.NoError(t, xerr)
	require.Len(t, got, len(expected))
	for i := range expected {
		require.EqualValues(t, expected[i].addr, got[i].addr)
		require.Equal(t, expected[i].SumPower, got[i].SumPower)
	}

	// Case 4: Eligibility filtering happens before the validator-count limit is applied.
	var strong []*Delegatee
	for i := 0; i < 4; i++ {
		strong = append(strong, makeDelegateeOne(int64(1000*(i+2)), int64(1000*(i+2))))
	}
	ineligible := makeDelegateeOne(1, 1_000_000)
	promoted := makeDelegateeOne(500, 500)
	all := append(copyDelegateeArray(strong), ineligible, promoted)

	naive := selectValidators(copyDelegateeArray(all), 5)
	for _, d := range naive {
		require.NotEqualValues(t, promoted.addr, d.addr, "sanity check failed: `promoted` should not fit without filtering")
	}

	got, xerr = selectEligibleValidators(all, 5, 100, 50)
	require.NoError(t, xerr)
	require.Len(t, got, 5)

	foundPromoted := false
	for _, d := range got {
		require.NotEqualValues(t, ineligible.addr, d.addr)
		if bytes.Equal(d.addr, promoted.addr) {
			foundPromoted = true
		}
	}
	require.True(t, foundPromoted, "the next-ranked eligible delegatee should be promoted")

	// Case 5: An error is returned when every delegatee is ineligible.
	allIneligible := makeDelegateeOne(1, 1_000_000)
	_, xerr = selectEligibleValidators([]*Delegatee{allIneligible}, 5, 100, 50)
	require.Error(t, xerr)
}
