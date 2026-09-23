package vpower

import (
	"math/rand"
	"os"
	"strconv"
	"testing"

	"github.com/beatoz/beatoz-go/ctrlers/mocks"
	supplymock "github.com/beatoz/beatoz-go/ctrlers/mocks/supply"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/stretchr/testify/require"
	abcitypes "github.com/tendermint/tendermint/abci/types"
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
	delegatorWal := acctMock.RandWallet()
	_, xerr = doDelegate(ctrler, delegatorWal, targetAddr, govMock.MinDelegatorPower(), mocks.CurrBlockHeight())
	require.NoError(t, xerr)

	dgtee, xerr := ctrler.readDelegatee(targetAddr, true)
	require.NoError(t, xerr)
	require.True(t, ctrler.IsValidator(dgtee.Address()))
	require.Len(t, dgtee.Delegators, 2)

	//
	// compute expected result
	expectedByzantine := dgtee.Clone()
	expectedVPowers := make([]*VPower, len(expectedByzantine.Delegators))
	expectedSlashed := int64(0)
	for i, _addr := range expectedByzantine.Delegators {
		vpow, xerr := ctrler.readVPower(_addr, expectedByzantine.Address(), true)
		require.NoError(t, xerr)
		expected := vpow.Clone()

		if expected.IsSelfPower() {
			for _, pc := range expected.PowerChunks {
				slashed := pc.Power * int64(govMock.SlashRate()) / int64(100)
				pc.Power -= slashed
				expected.SumPower -= slashed
				expectedSlashed += slashed
			}
		}
		expectedVPowers[i] = expected
	}

	//
	// doSlash
	refundHeight := mocks.CurrBlockHeight() + govMock.LazyUnbondingBlocks()
	tombstoned, xerr := ctrler.isTombstoned(expectedByzantine.Address(), true)
	require.NoError(t, xerr)
	require.False(t, tombstoned)
	slashed, xerr := ctrler.doSlash(
		expectedByzantine.Address(), govMock.SlashRate(), refundHeight,
	)
	require.NoError(t, xerr)
	require.Equal(t, expectedSlashed, slashed)

	//
	// removed dgtee
	_, xerr = ctrler.readDelegatee(targetAddr, true)
	require.Equal(t, xerrors.ErrNotFoundResult, xerr)
	tombstoned, xerr = ctrler.isTombstoned(targetAddr, true)
	require.NoError(t, xerr)
	require.True(t, tombstoned)
	_, xerr = doDelegate(ctrler, acctMock.RandWallet(), targetAddr, govMock.MinDelegatorPower(), mocks.CurrBlockHeight())
	require.Error(t, xerr)
	require.True(t, xerr.Contains(xerrors.ErrInvalidTrx))
	require.False(t, xerr.Contains(xerrors.ErrNotFoundDelegatee))
	require.Contains(t, xerr.Error(), "tombstoned")

	{
		//
		// check result
		for _, expected := range expectedVPowers {
			_, xerr := ctrler.readVPower(expected.from, expected.to, true)
			require.Equal(t, xerrors.ErrNotFoundResult, xerr)

			requireFrozenVPowerEqual(t, ctrler, refundHeight, expected)
		}
	}
	require.NoError(t, ctrler.Close())
	require.NoError(t, os.RemoveAll(config.DBDir()))
}

func Test_FrozenPowerSlashing(t *testing.T) {
	t.Run("slash twice for two evidences in the same block", func(t *testing.T) {
		ctrler, degtee := setupFrozenSlashTest(t)
		degteeAddr := degtee.Address()
		selfPowerBefore := requireSelfPower(t, ctrler, degteeAddr)

		const blockHeight int64 = 3
		_ = mocks.InitBlockCtxWith(
			config.ChainIdHex(), blockHeight, govMock, acctMock, nil,
			supplymock.NewSupplyHandlerMock(), ctrler,
		)
		tombstoned, xerr := ctrler.isTombstoned(degteeAddr, true)
		require.NoError(t, xerr)
		require.False(t, tombstoned)

		events := beginBlockWithDuplicateVoteEvidence(
			t, ctrler, degteeAddr, blockHeight-2, blockHeight-1,
		)

		firstSlashed := selfPowerBefore * int64(govMock.SlashRate()) / 100
		afterFirst := selfPowerBefore - firstSlashed
		secondSlashed := afterFirst * int64(govMock.SlashRate()) / 100
		afterSecond := afterFirst - secondSlashed

		requireSlashingEvent(t, events[0], firstSlashed)
		requireSlashingEvent(t, events[1], secondSlashed)
		require.Equal(t, selfPowerBefore-afterSecond, firstSlashed+secondSlashed)

		refundHeight := blockHeight + govMock.LazyUnbondingBlocks()
		requireFrozenPower(t, ctrler, refundHeight, degteeAddr, afterSecond)
		requireTombstoned(t, ctrler, degteeAddr)
	})

	t.Run("slash frozen power after self power rate drops below minimum", func(t *testing.T) {
		ctrler, degtee := setupFrozenSlashTest(t)
		degteeAddr := degtee.Address()
		_, lastHeight, xerr := ctrler.Commit()
		require.NoError(t, xerr)
		bctx := mocks.InitBlockCtxWith(
			config.ChainIdHex(), lastHeight+1, govMock, acctMock, nil,
			supplymock.NewSupplyHandlerMock(), ctrler,
		)
		require.NoError(t, mocks.DoBeginBlock(ctrler))
		activeSelfPower := requireSelfPower(t, ctrler, degteeAddr)
		unstakingPower := activeSelfPower
		delegator := acctMock.GetWallet(0)
		decreaseSelfPowerRate(t, ctrler, degtee, delegator, bctx.Height())

		dgteeBelowRate, xerr := ctrler.readDelegatee(degteeAddr, true)
		require.NoError(t, xerr)
		require.Equal(t, activeSelfPower, dgteeBelowRate.SelfPower)
		require.False(t, hasEnoughSelfPower(
			dgteeBelowRate.SelfPower,
			dgteeBelowRate.SumPower,
			govMock.MinSelfPowerRate(),
		))

		unstakingRefundHeight := bctx.Height() + govMock.LazyUnbondingBlocks()
		requireFrozenPower(t, ctrler, unstakingRefundHeight, degteeAddr, unstakingPower)

		require.NoError(t, mocks.DoEndBlockAndCommit(ctrler))
		require.False(t, ctrler.IsValidator(degteeAddr))

		bctx = mocks.CurrBlockCtx()
		slashingRefundHeight := bctx.Height() + govMock.LazyUnbondingBlocks()
		tombstoned, xerr := ctrler.isTombstoned(degteeAddr, true)
		require.NoError(t, xerr)
		require.False(t, tombstoned)

		events := beginBlockWithDuplicateVoteEvidence(
			t, ctrler, degteeAddr, bctx.Height()-1,
		)

		frozenSlashed := unstakingPower * int64(govMock.SlashRate()) / 100
		activeSlashed := activeSelfPower * int64(govMock.SlashRate()) / 100
		requireSlashingEvent(t, events[0], frozenSlashed+activeSlashed)
		requireFrozenPower(
			t, ctrler, unstakingRefundHeight, degteeAddr, unstakingPower-frozenSlashed,
		)
		requireFrozenPower(
			t, ctrler, slashingRefundHeight, degteeAddr, activeSelfPower-activeSlashed,
		)

		requireTombstoned(t, ctrler, degteeAddr)
	})

	t.Run("slash frozen power after forced unbonding for missed blocks", func(t *testing.T) {
		ctrler, degtee := setupFrozenSlashTest(t)
		degteeAddr := degtee.Address()
		selfPowerBefore := requireSelfPower(t, ctrler, degteeAddr)

		forcedUnbondHeight := missBlocksUntilUnbonded(t, ctrler, degteeAddr)

		missingRefundHeight := forcedUnbondHeight + govMock.LazyUnbondingBlocks()
		requireFrozenPower(t, ctrler, missingRefundHeight, degteeAddr, selfPowerBefore)
		require.False(t, ctrler.IsValidator(degteeAddr))

		tombstoned, xerr := ctrler.isTombstoned(degteeAddr, true)
		require.NoError(t, xerr)
		require.False(t, tombstoned)

		events := beginBlockWithDuplicateVoteEvidence(
			t, ctrler, degteeAddr, forcedUnbondHeight-1,
		)

		expectedSlashed := selfPowerBefore * int64(govMock.SlashRate()) / 100
		requireSlashingEvent(t, events[0], expectedSlashed)
		requireFrozenPower(t, ctrler, missingRefundHeight, degteeAddr, selfPowerBefore-expectedSlashed)
		requireTombstoned(t, ctrler, degteeAddr)
	})

	t.Run("skip tombstoned validator with no frozen power", func(t *testing.T) {
		ctrler, _ := setupFrozenSlashTest(t)
		degteeAddr := web3.NewWallet(nil).Address()
		require.NoError(t, ctrler.setTombstone(degteeAddr, true))

		const blockHeight int64 = 3
		bctx := mocks.InitBlockCtxWith(
			config.ChainIdHex(), blockHeight, govMock, acctMock, nil,
			supplymock.NewSupplyHandlerMock(), ctrler,
		)
		bctx.SetByzantine([]abcitypes.Evidence{
			{
				Type: abcitypes.EvidenceType_DUPLICATE_VOTE,
				Validator: abcitypes.Validator{
					Address: degteeAddr,
				},
				Height: blockHeight - 1,
			},
		})

		events, xerr := ctrler.BeginBlock(bctx)
		require.NoError(t, xerr)
		require.Empty(t, events)
	})
}

func Test_Punish_Byzantine_By_BlockProcess(t *testing.T) {
	require.NoError(t, os.RemoveAll(config.RootDir))

	//totalSupply := types.ToGrans(uint64(350_000_000))

	ctrler, lastValUps0, valWallets0, xerr := initLedger(config)
	require.NoError(t, xerr)
	require.Equal(t, len(lastValUps0), len(valWallets0))

	_ = mocks.InitBlockCtxWith(config.ChainIdHex(), 1, govMock, acctMock, nil, supplymock.NewSupplyHandlerMock(), ctrler)
	require.NoError(t, mocks.DoBeginBlock(ctrler))
	require.NoError(t, mocks.DoEndBlockAndCommit(ctrler))

	remainingValWallets := valWallets0
	for h := int64(2); h <= 100; h++ {
		rwalIdx := rand.Intn(len(remainingValWallets))
		rwal := remainingValWallets[rwalIdx]
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

		refundHeight := h + govMock.LazyUnbondingBlocks()
		offensed := len(remainingValWallets) > 1 && rand.Intn(3)%3 == 0
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
				if expected.IsSelfPower() {
					for _, pc := range expected.PowerChunks {
						slashed := pc.Power * int64(govMock.SlashRate()) / int64(100)
						pc.Power -= slashed
						expected.SumPower -= slashed
					}
				}
			}
		}

		// Punish(Slash) byzantine validator when offensed is true
		require.NoError(t, mocks.DoBeginBlock(ctrler))
		requireByzantineBlockProcessState(t, ctrler, offensed, expectedByzantine, expectedVPowers, refundHeight)

		require.NoError(t, mocks.DoEndBlockAndCommit(ctrler))
		requireByzantineBlockProcessState(t, ctrler, offensed, expectedByzantine, expectedVPowers, refundHeight)

		if offensed {
			remainingValWallets = append(remainingValWallets[:rwalIdx], remainingValWallets[rwalIdx+1:]...)
		}
	}

	require.NoError(t, ctrler.Close())
	require.NoError(t, os.RemoveAll(config.DBDir()))
}

func Test_Punish_MissingBlock(t *testing.T) {
	require.NoError(t, os.RemoveAll(config.RootDir))

	ctrler, lastValUps0, valWallets0, xerr := initLedger(config)
	require.NoError(t, xerr)
	require.Equal(t, len(lastValUps0), len(valWallets0))
	t.Cleanup(func() {
		require.NoError(t, ctrler.Close())
	})

	targetValWal := valWallets0[rand.Intn(len(valWallets0))]
	missBlocksUntilUnbonded(t, ctrler, targetValWal.Address())
}

// missBlocksUntilUnbonded commits the forced unbonding and returns its block height.
// The current block context is then ready for evidence in the following block.
func missBlocksUntilUnbonded(t *testing.T, ctrler *VPowerCtrler, targetAddr types.Address) int64 {
	t.Helper()

	allowedDownCnt := govMock.InflationCycleBlocks() - govMock.MinSignedBlocks()
	require.Positive(t, allowedDownCnt)

	_ = mocks.InitBlockCtxWith(config.ChainIdHex(), 1, govMock, acctMock, nil, supplymock.NewSupplyHandlerMock(), ctrler)
	require.NoError(t, mocks.DoAllProcess(ctrler))

	require.True(t, ctrler.IsValidator(targetAddr))
	dgtee0, xerr := ctrler.readDelegatee(targetAddr, true)
	require.NoError(t, xerr)
	require.NotNil(t, dgtee0)

	// It will return an error because the target has not missed any block.
	// And missedCnt is set to 0.
	missedCnt, xerr := ctrler.getMissedBlockCount(targetAddr, true)
	require.Error(t, xerr)

	for {
		bctx := mocks.CurrBlockCtx()
		require.NotNil(t, bctx)

		// make target not sign block
		bi := mocks.CurrBlockCtx().BlockInfo()
		require.NotNil(t, bi)
		bi.LastCommitInfo.Votes = append([]abcitypes.VoteInfo(nil), abcitypes.VoteInfo{
			Validator: abcitypes.Validator{
				Address: targetAddr,
			},
			SignedLastBlock: false,
		})
		mocks.CurrBlockCtx().SetBlockInfo(bi)

		// BeginBlock
		// missedBlock is increased.
		require.NoError(t, mocks.DoBeginBlock(ctrler))

		_missedCnt, xerr := ctrler.getMissedBlockCount(targetAddr, true)
		require.NoError(t, xerr)
		require.Equal(t, missedCnt+1, _missedCnt)
		missedCnt = _missedCnt

		if int64(missedCnt) >= allowedDownCnt {
			// all voting power of target should be unstaked.
			_, xerr := ctrler.readDelegatee(targetAddr, true)
			require.Error(t, xerr)

			for _, addr := range dgtee0.Delegators {
				_, xerr := ctrler.readVPower(addr, dgtee0.Address(), true)
				require.Error(t, xerr)
			}

			// EndBlock and Commit
			// update validators
			require.NoError(t, mocks.DoEndBlockAndCommit(ctrler))

			require.False(t, ctrler.IsValidator(targetAddr))
			return bctx.Height()
		}

		// EndBlock and Commit
		require.NoError(t, mocks.DoEndBlockAndCommit(ctrler))
		require.True(t, ctrler.IsValidator(targetAddr))
	}
}

func requirePowerChunksEqual(t *testing.T, expected, actual []*PowerChunkProto) {
	t.Helper()

	require.Equal(t, len(expected), len(actual))
	for i, actualChunk := range actual {
		require.Equal(t, expected[i].Power, actualChunk.Power)
		require.Equal(t, expected[i].Height, actualChunk.Height)
		require.EqualValues(t, expected[i].TxHash, actualChunk.TxHash)
	}
}

func setupFrozenSlashTest(t *testing.T) (*VPowerCtrler, *web3.Wallet) {
	t.Helper()

	require.NoError(t, os.RemoveAll(config.RootDir))
	ctrler, _, valWallets, xerr := initLedger(config)
	require.NoError(t, xerr)
	require.NotEmpty(t, valWallets)

	t.Cleanup(func() {
		require.NoError(t, ctrler.Close())
		require.NoError(t, os.RemoveAll(config.DBDir()))
	})
	return ctrler, valWallets[0]
}

func beginBlockWithDuplicateVoteEvidence(
	t *testing.T,
	ctrler *VPowerCtrler,
	targetAddr types.Address,
	offenseHeights ...int64,
) []abcitypes.Event {
	t.Helper()

	evidences := make([]abcitypes.Evidence, len(offenseHeights))
	for i, offenseHeight := range offenseHeights {
		evidences[i] = abcitypes.Evidence{
			Type: abcitypes.EvidenceType_DUPLICATE_VOTE,
			Validator: abcitypes.Validator{
				Address: targetAddr,
			},
			Height: offenseHeight,
		}
	}

	bctx := mocks.CurrBlockCtx()
	require.NotNil(t, bctx)
	bctx.SetByzantine(evidences)
	events, xerr := ctrler.BeginBlock(bctx)
	require.NoError(t, xerr)
	require.Len(t, events, len(evidences))
	return events
}

func requireTombstoned(t *testing.T, ctrler *VPowerCtrler, targetAddr types.Address) {
	t.Helper()

	requireDelegateeRemoved(t, ctrler, targetAddr)

	tombstoned, xerr := ctrler.isTombstoned(targetAddr, true)
	require.NoError(t, xerr)
	require.True(t, tombstoned)
}

func requireDelegateeRemoved(t *testing.T, ctrler *VPowerCtrler, targetAddr types.Address) {
	t.Helper()

	dgtee, xerr := ctrler.readDelegatee(targetAddr, true)
	require.Nil(t, dgtee)
	require.Error(t, xerr)
	require.True(t, xerr.Contains(xerrors.ErrNotFoundResult))
}

func requireSelfPower(t *testing.T, ctrler *VPowerCtrler, targetAddr types.Address) int64 {
	t.Helper()

	dgtee, xerr := ctrler.readDelegatee(targetAddr, true)
	require.NoError(t, xerr)
	require.NotNil(t, dgtee)
	return dgtee.SelfPower
}

func requireFrozenPower(
	t *testing.T,
	ctrler *VPowerCtrler,
	refundHeight int64,
	from types.Address,
	expectedPower int64,
) {
	t.Helper()

	frozen, xerr := ctrler.readFrozenVPower(refundHeight, from, true)
	require.NoError(t, xerr)
	require.Equal(t, expectedPower, frozen.RefundPower)

	sumPower := int64(0)
	for _, pc := range frozen.PowerChunks {
		sumPower += pc.Power
	}
	require.Equal(t, frozen.RefundPower, sumPower)
}

func requireSlashingEvent(t *testing.T, event abcitypes.Event, expectedSlashed int64) {
	t.Helper()

	require.Equal(t, "vpower.slashing", event.Type)
	for _, attr := range event.Attributes {
		if string(attr.Key) == "slashed" {
			require.Equal(t, strconv.FormatInt(expectedSlashed, 10), string(attr.Value))
			return
		}
	}
	require.Fail(t, "slashed event attribute not found")
}

func requireFrozenVPowerEqual(t *testing.T, ctrler *VPowerCtrler, refundHeight int64, expected *VPower) {
	t.Helper()

	frozen, xerr := ctrler.readFrozenVPower(refundHeight, expected.from, true)
	require.NoError(t, xerr)
	require.Equal(t, expected.SumPower, frozen.RefundPower)
	requirePowerChunksEqual(t, expected.PowerChunks, frozen.PowerChunks)
}

func requireByzantineBlockProcessState(
	t *testing.T,
	ctrler *VPowerCtrler,
	offensed bool,
	expectedByzantine *Delegatee,
	expectedVPowers []*VPower,
	refundHeight int64,
) {
	t.Helper()

	if offensed {
		_, xerr := ctrler.readDelegatee(expectedByzantine.Address(), true)
		require.Equal(t, xerrors.ErrNotFoundResult, xerr)

		tombstoned, xerr := ctrler.isTombstoned(expectedByzantine.Address(), true)
		require.NoError(t, xerr)
		require.True(t, tombstoned)

		for _, expected := range expectedVPowers {
			_, xerr := ctrler.readVPower(expected.from, expected.to, true)
			require.Equal(t, xerrors.ErrNotFoundResult, xerr)
			requireFrozenVPowerEqual(t, ctrler, refundHeight, expected)
		}
		return
	}

	dgtee, xerr := ctrler.readDelegatee(expectedByzantine.Address(), true)
	require.NoError(t, xerr)
	require.Equal(t, expectedByzantine.SumPower, dgtee.SumPower)
	require.Equal(t, expectedByzantine.SelfPower, dgtee.SelfPower)
	require.Equal(t, len(expectedByzantine.Delegators), len(dgtee.Delegators))

	for i, addr := range dgtee.Delegators {
		vpow, xerr := ctrler.readVPower(addr, dgtee.Address(), true)
		require.NoError(t, xerr)
		require.Equal(t, expectedVPowers[i].SumPower, vpow.SumPower)
		requirePowerChunksEqual(t, expectedVPowers[i].PowerChunks, vpow.PowerChunks)
	}
}
