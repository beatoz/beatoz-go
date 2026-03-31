package vpower

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_limiter(t *testing.T) {
	lastTotal := int64(100)
	allowRate := int32(10)
	limiter := NewVPowerLimiter()
	limiter.Reset(lastTotal, allowRate)

	expectedAdded := int64(0)
	expectedSubed := int64(0)
	expectedNewTotal := lastTotal

	// applied: added=11, subed=0, remain.total=100, new.total=111
	// remain.total / new.total = 0.90
	// expected: no error
	diffPower := int64(11)
	expectedAdded += diffPower
	expectedNewTotal += diffPower
	require.NoError(t, limiter.CheckLimit(diffPower, ADD_POWER))
	require.EqualValues(t, lastTotal, limiter.lastTotalPower)
	require.EqualValues(t, expectedNewTotal, limiter.estimatedTotalPower)
	require.EqualValues(t, expectedAdded, limiter.addingPower)
	require.EqualValues(t, expectedSubed, limiter.subingPower)

	// applied: added=12, subed=0, remain.total=100, new.total=112
	// remain.total / new.total = 0.89
	// expected: error
	diffPower = int64(1)
	require.Error(t, limiter.CheckLimit(diffPower, ADD_POWER))
	require.EqualValues(t, lastTotal, limiter.lastTotalPower)
	require.EqualValues(t, expectedNewTotal, limiter.estimatedTotalPower)
	require.EqualValues(t, expectedAdded, limiter.addingPower)
	require.EqualValues(t, expectedSubed, limiter.subingPower)

	limiter.Reset(lastTotal, allowRate)
	expectedAdded = int64(0)
	expectedSubed = int64(0)
	expectedNewTotal = lastTotal

	// applied: added=0, subed=10, remain.total=90, new.total=90
	// expected: no error
	diffPower = int64(10)
	expectedSubed += diffPower
	expectedNewTotal -= diffPower
	require.NoError(t, limiter.CheckLimit(diffPower, SUB_POWER))
	require.EqualValues(t, lastTotal, limiter.lastTotalPower)
	require.EqualValues(t, expectedNewTotal, limiter.estimatedTotalPower)
	require.EqualValues(t, expectedAdded, limiter.addingPower)
	require.EqualValues(t, expectedSubed, limiter.subingPower)

	// applied: added=10, subed=10, remain.total=90, new.total=100
	// expected: no error
	// remain.total/new.total = 0.9
	diffPower = int64(10)
	expectedAdded += diffPower
	expectedNewTotal += diffPower
	require.NoError(t, limiter.CheckLimit(diffPower, ADD_POWER))
	require.EqualValues(t, lastTotal, limiter.lastTotalPower)
	require.EqualValues(t, expectedNewTotal, limiter.estimatedTotalPower)
	require.EqualValues(t, expectedAdded, limiter.addingPower)
	require.EqualValues(t, expectedSubed, limiter.subingPower)

	// applied: added=11, subed=10, remain.total=90, new.total=101
	// expected: no error
	// remain.total/new.total = 0.8910
	diffPower = int64(1)
	require.Error(t, limiter.CheckLimit(diffPower, ADD_POWER))
	require.EqualValues(t, lastTotal, limiter.lastTotalPower)
	require.EqualValues(t, expectedNewTotal, limiter.estimatedTotalPower)
	require.EqualValues(t, expectedAdded, limiter.addingPower)
	require.EqualValues(t, expectedSubed, limiter.subingPower)
}
