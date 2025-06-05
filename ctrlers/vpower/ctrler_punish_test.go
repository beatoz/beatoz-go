package vpower

import (
	"github.com/beatoz/beatoz-go/ctrlers/mocks"
	supplymock "github.com/beatoz/beatoz-go/ctrlers/mocks/supply"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/stretchr/testify/require"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"math/rand"
	"os"
	"testing"
)

func Test_Slash_Byzantine(t *testing.T) {
	require.NoError(t, os.RemoveAll(config.RootDir))

	ctrler, lastValUps0, valWallets0, xerr := initLedger(config)
	require.NoError(t, xerr)
	require.Equal(t, len(lastValUps0), len(valWallets0))

	_ = mocks.InitBlockCtxWith("", 1, govMock, acctMock, nil, supplymock.NewSupplyHandlerMock(), ctrler)
	require.NoError(t, mocks.DoBeginBlock(ctrler))
	require.NoError(t, mocks.DoEndBlockAndCommit(ctrler))

	targetAddr := valWallets0[rand.Intn(len(valWallets0))].Address()
	dgtee, xerr := ctrler.readDelegatee(targetAddr, true)
	require.NoError(t, xerr)
	require.True(t, ctrler.IsValidator(dgtee.Address()))

	//
	// compute expected result
	expectedByzantine := dgtee.Clone()
	expectedVPowers := make([]*VPower, len(expectedByzantine.Delegators))
	expectedSlahsed := int64(0)
	for i, _addr := range expectedByzantine.Delegators {
		vpow, xerr := ctrler.readVPower(_addr, expectedByzantine.Address(), true)
		require.NoError(t, xerr)
		expected := vpow.Clone()

		for _, pc := range expected.PowerChunks {
			slashed := pc.Power * int64(govMock.SlashRate()) / int64(100)
			pc.Power -= slashed
			expected.SumPower -= slashed
			expectedByzantine.SumPower -= slashed
			if bytes.Equal(expected.from, expected.from) {
				expectedByzantine.SelfPower -= slashed
			}

			expectedSlahsed += slashed
		}
		expectedVPowers[i] = expected
	}

	//
	// doSlash
	slashed, xerr := ctrler.doSlash(expectedByzantine.Address(), govMock.SlashRate())
	require.NoError(t, xerr)
	require.Equal(t, expectedSlahsed, slashed)

	//
	// updated dgtee
	dgtee, xerr = ctrler.readDelegatee(targetAddr, true)
	require.NoError(t, xerr)

	{
		//
		// check result
		require.Equal(t, expectedByzantine.SumPower, dgtee.SumPower)
		require.Equal(t, expectedByzantine.SelfPower, dgtee.SelfPower)
		require.Equal(t, len(expectedVPowers), len(dgtee.Delegators))

		for i, addr := range dgtee.Delegators {
			vpow, xerr := ctrler.readVPower(addr, dgtee.Address(), true)
			require.NoError(t, xerr)
			require.Equal(t, expectedVPowers[i].SumPower, vpow.SumPower)
			require.Equal(t, len(expectedVPowers[i].PowerChunks), len(vpow.PowerChunks))
			for j, pc := range vpow.PowerChunks {
				require.Equal(t, expectedVPowers[i].PowerChunks[j].Power, pc.Power)
				require.Equal(t, expectedVPowers[i].PowerChunks[j].Height, pc.Height)
				require.EqualValues(t, expectedVPowers[i].PowerChunks[j].TxHash, pc.TxHash)
			}
		}
	}
	require.NoError(t, ctrler.Close())
	require.NoError(t, os.RemoveAll(config.DBDir()))
}

func Test_Punish_Byzantine_By_BlockProcess(t *testing.T) {
	require.NoError(t, os.RemoveAll(config.RootDir))

	//totalSupply := types.ToGrans(uint64(350_000_000))

	ctrler, lastValUps0, valWallets0, xerr := initLedger(config)
	require.NoError(t, xerr)
	require.Equal(t, len(lastValUps0), len(valWallets0))

	_ = mocks.InitBlockCtxWith(config.ChainID, 1, govMock, acctMock, nil, supplymock.NewSupplyHandlerMock(), ctrler)
	require.NoError(t, mocks.DoBeginBlock(ctrler))
	require.NoError(t, mocks.DoEndBlockAndCommit(ctrler))

	for h := int64(2); h <= 100; h++ {
		rwal := valWallets0[rand.Intn(len(valWallets0))]
		rvalidator, xerr := ctrler.readDelegatee(rwal.Address(), true)
		require.NoError(t, xerr)

		expectedByzantine := rvalidator.Clone()
		require.True(t, ctrler.IsValidator(rvalidator.Address()))
		require.Equal(t, rwal.Address(), rvalidator.Address())

		expectedVPowers := make([]*VPower, len(expectedByzantine.Delegators))
		for i, _addr := range expectedByzantine.Delegators {
			vpow, xerr := ctrler.readVPower(_addr, expectedByzantine.Address(), true)
			require.NoError(t, xerr)
			expectedVPowers[i] = vpow.Clone()
		}

		offensed := rand.Intn(3)%3 == 0
		if offensed {

			// offense occurs or not

			offenseHeight := rand.Int63n(h-1) + 1
			evidence := abcitypes.Evidence{
				Type: abcitypes.EvidenceType_DUPLICATE_VOTE,
				Validator: abcitypes.Validator{
					Address: expectedByzantine.Address(),
					Power:   0, // don't care
				},
				Height: offenseHeight,
			}
			mocks.CurrBlockCtx().SetByzantine([]abcitypes.Evidence{evidence})

			for _, expected := range expectedVPowers {
				for _, pc := range expected.PowerChunks {
					slashed := pc.Power * int64(govMock.SlashRate()) / int64(100)
					pc.Power -= slashed
					expected.SumPower -= slashed
					expectedByzantine.SumPower -= slashed
					if bytes.Equal(expected.from, expected.from) {
						expectedByzantine.SelfPower -= slashed
					}
				}
			}
		}

		// Punish(Slash) byzantine validator when offensed is true
		require.NoError(t, mocks.DoBeginBlock(ctrler))

		{
			// check after beginblock
			dgtee, xerr := ctrler.readDelegatee(rwal.Address(), true)
			require.NoError(t, xerr)
			require.Equal(t, expectedByzantine.SumPower, dgtee.SumPower)
			require.Equal(t, expectedByzantine.SelfPower, dgtee.SelfPower)
			require.Equal(t, len(expectedByzantine.Delegators), len(dgtee.Delegators))

			for i, addr := range dgtee.Delegators {
				vpow, xerr := ctrler.readVPower(addr, dgtee.Address(), true)
				require.NoError(t, xerr)
				require.Equal(t, expectedVPowers[i].SumPower, vpow.SumPower)
				require.Equal(t, len(expectedVPowers[i].PowerChunks), len(vpow.PowerChunks))
				for j, pc := range vpow.PowerChunks {
					require.Equal(t, expectedVPowers[i].PowerChunks[j].Power, pc.Power)
					require.Equal(t, expectedVPowers[i].PowerChunks[j].Height, pc.Height)
					require.EqualValues(t, expectedVPowers[i].PowerChunks[j].TxHash, pc.TxHash)
				}
			}
		}

		require.NoError(t, mocks.DoEndBlockAndCommit(ctrler))

		{
			// check after commit
			dgtee, xerr := ctrler.readDelegatee(rwal.Address(), true)
			require.NoError(t, xerr)
			require.Equal(t, expectedByzantine.SumPower, dgtee.SumPower)
			require.Equal(t, expectedByzantine.SelfPower, dgtee.SelfPower)
			require.Equal(t, len(expectedByzantine.Delegators), len(dgtee.Delegators))

			for i, addr := range dgtee.Delegators {
				vpow, xerr := ctrler.readVPower(addr, dgtee.Address(), true)
				require.NoError(t, xerr)
				require.Equal(t, expectedVPowers[i].SumPower, vpow.SumPower)
				require.Equal(t, len(expectedVPowers[i].PowerChunks), len(vpow.PowerChunks))
				for j, pc := range vpow.PowerChunks {
					require.Equal(t, expectedVPowers[i].PowerChunks[j].Power, pc.Power)
					require.Equal(t, expectedVPowers[i].PowerChunks[j].Height, pc.Height)
					require.EqualValues(t, expectedVPowers[i].PowerChunks[j].TxHash, pc.TxHash)
				}
			}
		}
	}

	require.NoError(t, ctrler.Close())
	require.NoError(t, os.RemoveAll(config.DBDir()))
}

func Test_Punish_MissingBlock(t *testing.T) {
	require.NoError(t, os.RemoveAll(config.RootDir))

	allowedDownCnt := govMock.InflationCycleBlocks() - govMock.MinSignedBlocks()
	require.True(t, allowedDownCnt > 0)

	ctrler, lastValUps0, valWallets0, xerr := initLedger(config)
	require.NoError(t, xerr)
	require.Equal(t, len(lastValUps0), len(valWallets0))

	_ = mocks.InitBlockCtxWith(config.ChainID, 1, govMock, acctMock, nil, supplymock.NewSupplyHandlerMock(), ctrler)
	require.NoError(t, mocks.DoAllProcess(ctrler))

	targetValWal := valWallets0[rand.Intn(len(valWallets0))]
	require.True(t, ctrler.IsValidator(targetValWal.Address()))
	dgtee0, xerr := ctrler.readDelegatee(targetValWal.Address(), true)
	require.NoError(t, xerr)
	require.NotNil(t, dgtee0)

	// It will return an error because the targetValWal has not missed any block.
	// And missedCnt is set to 0.
	missedCnt, xerr := ctrler.getMissedBlockCount(targetValWal.Address(), true)
	require.Error(t, xerr)

	for {
		bctx := mocks.CurrBlockCtx()
		require.NotNil(t, bctx)

		// make targetVal not sign block
		bi := mocks.CurrBlockCtx().BlockInfo()
		require.NotNil(t, bi)
		bi.LastCommitInfo.Votes = append([]abcitypes.VoteInfo(nil), abcitypes.VoteInfo{
			Validator: abcitypes.Validator{
				Address: targetValWal.Address(),
			},
			SignedLastBlock: false,
		})
		mocks.CurrBlockCtx().SetBlockInfo(bi)

		// BeginBlock
		// missedBlock is increased.
		require.NoError(t, mocks.DoBeginBlock(ctrler))

		_missedCnt, xerr := ctrler.getMissedBlockCount(targetValWal.Address(), true)
		require.NoError(t, xerr)
		require.Equal(t, missedCnt+1, _missedCnt)
		missedCnt = _missedCnt

		if int64(missedCnt) >= allowedDownCnt {
			// all voting power of targetValWal should be unstaked.
			_, xerr := ctrler.readDelegatee(targetValWal.Address(), true)
			require.Error(t, xerr)

			for _, addr := range dgtee0.Delegators {
				_, xerr := ctrler.readVPower(addr, dgtee0.Address(), true)
				require.Error(t, xerr)
			}

			// EndBlock and Commit
			// update validators
			require.NoError(t, mocks.DoEndBlockAndCommit(ctrler))

			require.False(t, ctrler.IsValidator(targetValWal.Address()))
			break
		}

		// EndBlock and Commit
		require.NoError(t, mocks.DoEndBlockAndCommit(ctrler))
		require.True(t, ctrler.IsValidator(targetValWal.Address()))
	}
}
