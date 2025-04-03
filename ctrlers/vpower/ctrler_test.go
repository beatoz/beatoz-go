package vpower

import (
	beatozcfg "github.com/beatoz/beatoz-go/cmd/config"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
)

var (
	config   = beatozcfg.DefaultConfig()
	govParam = ctrlertypes.DefaultGovParams()
)

//func Test_Bond(t *testing.T) {
//	config.DBPath = filepath.Join(os.TempDir(), "validator-ctrler-test")
//	_ = os.RemoveAll(config.DBPath)
//
//	ctrler, xerr := NewVPowerCtrler(config, 0, govParam, log.NewNopLogger())
//	require.NoError(t, xerr)
//
//	_, pubVal := crypto.NewKeypairBytes()
//
//	// not found validator
//	xerr = ctrler.Bond(
//		bytes.RandInt64N(700_000_000),
//		1,
//		types.RandAddress(), pubVal,
//		bytes.RandBytes(32), true)
//	require.Error(t, xerrors.ErrNotFoundDelegatee, xerr)
//
//	expected := struct {
//		totalPower int64
//		selfPower  int64
//	}{0, 0}
//	height := int64(1)
//
//	// self voting power
//	vpow := newVPower(
//		bytes.RandInt64N(700_000_000),
//		height,
//		crypto.PubKeyBytes2Addr(pubVal),
//		pubVal,
//		bytes.RandBytes(32),
//	)
//
//	// self bonding
//	xerr = ctrler.Bond(vpow.Power, vpow.Height, vpow.From, vpow.PubKeyTo, vpow.TxHash, true)
//	require.NoError(t, xerr)
//
//	expected.totalPower += vpow.Power
//	expected.selfPower += vpow.Power
//
//	for ; height < 1000; height++ {
//		from := types.RandAddress()
//		if height%2 == 0 {
//			// self bonding
//			from = crypto.PubKeyBytes2Addr(pubVal)
//		}
//		vpow = newVPower(
//			bytes.RandInt64N(700_000_000),
//			height,
//			from,
//			pubVal,
//			bytes.RandBytes(32),
//		)
//		xerr = ctrler.Bond(vpow.Power, vpow.Height, vpow.From, vpow.PubKeyTo, vpow.TxHash, true)
//		require.NoError(t, xerr)
//
//		_, v, xerr := ctrler.Commit()
//		require.NoError(t, xerr)
//		require.Equal(t, height, v)
//
//		expected.totalPower += vpow.Power
//		if height%2 == 0 {
//			expected.selfPower += vpow.Power
//		}
//	}
//
//	require.NoError(t, ctrler.Close())
//
//	height--
//	ctrler, xerr = NewVPowerCtrler(config, height, govParam, log.NewNopLogger())
//	require.NoError(t, xerr, height)
//
//	fmt.Println(ctrler.allDelegatees[0].TotalPower(), ctrler.allDelegatees[0].SelfPower())
//
//	require.Equal(t, 1, len(ctrler.allDelegatees))
//	require.Equal(t, expected.totalPower, ctrler.allDelegatees[0].TotalPower())
//	require.Equal(t, expected.selfPower, ctrler.allDelegatees[0].SelfPower())
//	require.NotEqual(t, ctrler.allDelegatees[0].SelfPower(), ctrler.allDelegatees[0].TotalPower())
//	require.Equal(t, pubVal, ctrler.allDelegatees[0].pubKey)
//}
