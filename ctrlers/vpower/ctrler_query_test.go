package vpower

import (
	"context"
	"encoding/binary"
	"strconv"
	"testing"
	"time"

	beatozcfg "github.com/beatoz/beatoz-go/cmd/config"
	"github.com/beatoz/beatoz-go/libs/jsonx"
	btztypes "github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/stretchr/testify/require"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

func Test_Query(t *testing.T) {
	testConfig := beatozcfg.DefaultConfig("1")
	testConfig.App.QueryTimeout = time.Second
	testConfig.SetRoot(t.TempDir())

	ctrler, validators, wallets, xerr := initLedger(testConfig)
	require.NoError(t, xerr)
	t.Cleanup(func() { require.NoError(t, ctrler.Close()) })

	_, height, xerr := ctrler.Commit()
	require.NoError(t, xerr)

	address := wallets[0].Address()
	expectedPower := validators[0].Power
	expectedTotalPower := int64(0)
	for _, validator := range validators {
		expectedTotalPower += validator.Power
	}

	// Query stakes.
	value, xerr := ctrler.Query(abcitypes.RequestQuery{Path: "stakes", Data: address, Height: height})
	require.NoError(t, xerr)
	var stakes []struct {
		Owner btztypes.Address `json:"owner"`
		To    btztypes.Address `json:"to"`
		Power int64            `json:"power,string"`
	}
	require.NoError(t, jsonx.Unmarshal(value, &stakes))
	require.Len(t, stakes, 1)
	require.Equal(t, address, stakes[0].Owner)
	require.Equal(t, address, stakes[0].To)
	require.Equal(t, expectedPower, stakes[0].Power)

	// Query the delegatee.
	value, xerr = ctrler.Query(abcitypes.RequestQuery{Path: "delegatee", Data: address, Height: height})
	require.NoError(t, xerr)
	var delegatee struct {
		Address    btztypes.Address   `json:"address"`
		SelfPower  int64              `json:"selfPower,string"`
		TotalPower int64              `json:"totalPower,string"`
		Delegators []btztypes.Address `json:"delegators"`
	}
	require.NoError(t, jsonx.Unmarshal(value, &delegatee))
	require.Equal(t, address, delegatee.Address)
	require.Equal(t, expectedPower, delegatee.SelfPower)
	require.Equal(t, expectedPower, delegatee.TotalPower)
	require.Equal(t, []btztypes.Address{address}, delegatee.Delegators)

	// Query total power.
	value, xerr = ctrler.Query(abcitypes.RequestQuery{Path: "stakes/total_power", Height: height})
	require.NoError(t, xerr)
	var totalPower string
	require.NoError(t, jsonx.Unmarshal(value, &totalPower))
	require.Equal(t, strconv.FormatInt(expectedTotalPower, 10), totalPower)

	// Query voting power.
	value, xerr = ctrler.Query(
		abcitypes.RequestQuery{Path: "stakes/voting_power", Height: height},
		func() interface{} { return int32(len(validators)) },
		func() interface{} { return int64(0) },
	)
	require.NoError(t, xerr)
	var votingPower string
	require.NoError(t, jsonx.Unmarshal(value, &votingPower))
	require.Equal(t, strconv.FormatInt(expectedTotalPower, 10), votingPower)

	// Query an unknown path.
	value, xerr = ctrler.Query(abcitypes.RequestQuery{Path: "unknown", Height: height})
	require.Nil(t, value)
	require.Error(t, xerr)
	require.Equal(t, xerrors.ErrCodeQuery, xerr.Code())
}

func Test_Query_Timeout(t *testing.T) {
	// Exercise timeout during actual ledger decoding and iteration instead of
	// starting the query with an effectively expired nanosecond deadline.
	const (
		timeoutTestDuration        = 5 * time.Millisecond
		timeoutTestPowerChunkCount = 100_000
	)

	testConfig := beatozcfg.DefaultConfig("1")
	testConfig.App.QueryTimeout = timeoutTestDuration
	testConfig.SetRoot(t.TempDir())

	ctrler, _, wallets, xerr := initLedger(testConfig)
	require.NoError(t, xerr)
	t.Cleanup(func() { require.NoError(t, ctrler.Close()) })

	address := wallets[0].Address()
	addPowerChunksForQueryTimeout(t, ctrler, address, timeoutTestPowerChunkCount)

	_, height, xerr := ctrler.Commit()
	require.NoError(t, xerr)

	for _, path := range []string{"stakes", "delegatee"} {
		value, xerr := ctrler.Query(abcitypes.RequestQuery{Path: path, Data: address, Height: height})

		require.Nil(t, value, "path=%s", path)
		require.Error(t, xerr, "path=%s", path)
		require.Equal(t, xerrors.ErrCodeQuery, xerr.Code(), "path=%s", path)
		require.ErrorContains(t, xerr, context.DeadlineExceeded.Error(), "path=%s", path)
		require.ErrorIs(t, xerr.Cause(), context.DeadlineExceeded, "path=%s", path)
		require.True(t, ctrler.mtx.TryLock(), "path=%s", path)
		ctrler.mtx.Unlock()
	}
}

func addPowerChunksForQueryTimeout(t *testing.T, ctrler *VPowerCtrler, addr btztypes.Address, count int) {
	t.Helper()

	vpow, xerr := ctrler.readVPower(addr, addr, true)
	require.NoError(t, xerr)
	require.NotNil(t, vpow)

	const powerPerChunk int64 = 1
	for i := 0; i < count; i++ {
		txHash := make([]byte, 32)
		binary.BigEndian.PutUint64(txHash[24:], uint64(i+1))
		vpow.addPowerWithTxHash(powerPerChunk, 1, txHash)
	}
	require.NoError(t, ctrler.writeVPower(vpow, true))

	dgtee, xerr := ctrler.readDelegatee(addr, true)
	require.NoError(t, xerr)
	require.NotNil(t, dgtee)
	addedPower := int64(count) * powerPerChunk
	dgtee.addPower(addr, addedPower)
	require.NoError(t, ctrler.writeDelegatee(dgtee, true))
}
