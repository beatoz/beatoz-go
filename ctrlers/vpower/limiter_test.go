package vpower

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_limiter(t *testing.T) {
	lastTotal := int64(100)
	allowRate := int32(10)
	limiter := NewVPowerLimiter()
	limiter.Reset(lastTotal, allowRate)

	expectedAdded := int64(0)
	expectedSubed := int64(0)
	expectedNewTotal := lastTotal

	// now    : new.total=100, added=0, subed=0
	// applied: new.total=107, added=7, subed=0
	// expected: no error
	diffPower := int64(7)
	expectedAdded += diffPower
	expectedNewTotal += diffPower
	require.NoError(t, limiter.CheckLimit(nil, nil, diffPower, WHEN_POWER_ADD))
	require.EqualValues(t, lastTotal, limiter.lastTotalPower)
	require.EqualValues(t, expectedNewTotal, limiter.newTotalPower)
	require.EqualValues(t, expectedAdded, limiter.addingPower)
	require.EqualValues(t, expectedSubed, limiter.subingPower)

	// now    : new.total=107, added=7, subed=0
	// applied: new.total=110, added=10, subed=0
	// expected: no error
	diffPower = int64(3)
	expectedAdded += diffPower
	expectedNewTotal += diffPower
	require.NoError(t, limiter.CheckLimit(nil, nil, diffPower, WHEN_POWER_ADD))
	require.EqualValues(t, lastTotal, limiter.lastTotalPower)
	require.EqualValues(t, expectedNewTotal, limiter.newTotalPower)
	require.EqualValues(t, expectedAdded, limiter.addingPower)
	require.EqualValues(t, expectedSubed, limiter.subingPower)

	// now    : new.total=110, added=10, subed=0
	// applied: new.total=111, added=11, subed=0
	// expected: no error
	diffPower = int64(1)
	expectedAdded += diffPower
	expectedNewTotal += diffPower
	require.NoError(t, limiter.CheckLimit(nil, nil, diffPower, WHEN_POWER_ADD))
	require.EqualValues(t, lastTotal, limiter.lastTotalPower)
	require.EqualValues(t, expectedNewTotal, limiter.newTotalPower)
	require.EqualValues(t, expectedAdded, limiter.addingPower)
	require.EqualValues(t, expectedSubed, limiter.subingPower)

	// now    : new.total=111, added=11, subed=0
	// applied: new.total=112, added=12, subed=0
	// expected: error =
	diffPower = int64(1)
	require.Error(t, limiter.CheckLimit(nil, nil, diffPower, WHEN_POWER_ADD))
	require.EqualValues(t, lastTotal, limiter.lastTotalPower)
	require.EqualValues(t, expectedNewTotal, limiter.newTotalPower)
	require.EqualValues(t, expectedAdded, limiter.addingPower)
	require.EqualValues(t, expectedSubed, limiter.subingPower)

	limiter.Reset(lastTotal, allowRate)
	expectedAdded = int64(0)
	expectedSubed = int64(0)
	expectedNewTotal = lastTotal

	// now    : new.total=100, added=0, subed=0
	// applied: new.total=99, added=0, subed=1
	// expected: error =
	diffPower = int64(1)
	expectedSubed += diffPower
	expectedNewTotal -= diffPower
	require.NoError(t, limiter.CheckLimit(nil, nil, diffPower, WHEN_POWER_SUB))
	require.EqualValues(t, lastTotal, limiter.lastTotalPower)
	require.EqualValues(t, expectedNewTotal, limiter.newTotalPower)
	require.EqualValues(t, expectedAdded, limiter.addingPower)
	require.EqualValues(t, expectedSubed, limiter.subingPower)

}
