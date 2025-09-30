package gov

import (
	"testing"

	"github.com/beatoz/beatoz-go/ctrlers/gov/proposal"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/libs/jsonx"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/stretchr/testify/require"
)

var (
	trxCtxProposal        *ctrlertypes.TrxContext
	voteTestCases1        []*Case
	voteTestCases2        []*Case
	testFlagAlreadyFrozen = false
)

func init() {
	bzOpt, err := jsonx.Marshal(govParams1)
	if err != nil {
		panic(err)
	}
	txProposal := web3.NewTrxProposal(
		vpowMock.PickAddress(1), types.ZeroAddress(), 1, defMinGas, defGasPrice,
		"test govparams proposal", 10, govCtrler.MinVotingPeriodBlocks(), 10+govCtrler.MinVotingPeriodBlocks()+govCtrler.LazyApplyingBlocks(), proposal.PROPOSAL_GOVPARAMS, bzOpt)
	_ = signTrx(txProposal, vpowMock.PickAddress(1), govTestChainId)
	trxCtxProposal = makeTrxCtx(txProposal, 1, true)
	if xerr := runTrx(trxCtxProposal); xerr != nil {
		panic(xerr)
	}
	if _, _, xerr := govCtrler.Commit(); xerr != nil {
		panic(xerr)
	}

	// no error
	tx0 := web3.NewTrxVoting(vpowMock.PickAddress(0), types.ZeroAddress(), 1, defMinGas, defGasPrice,
		trxCtxProposal.TxHash, 0)
	_ = signTrx(tx0, vpowMock.PickAddress(0), govTestChainId)
	// no right
	tx1 := web3.NewTrxVoting(vpowMock.PickAddress(vpowMock.ValCnt), types.ZeroAddress(), 1, defMinGas, defGasPrice,
		trxCtxProposal.TxHash, 0)
	_ = signTrx(tx1, vpowMock.PickAddress(vpowMock.ValCnt), govTestChainId)

	// invalid payload params : wrong choice
	tx2 := web3.NewTrxVoting(vpowMock.PickAddress(0), types.ZeroAddress(), 1, defMinGas, defGasPrice,
		trxCtxProposal.TxHash, 1)
	_ = signTrx(tx2, vpowMock.PickAddress(0), govTestChainId)
	// invalid payload params : wrong choice
	tx3 := web3.NewTrxVoting(vpowMock.PickAddress(0), types.ZeroAddress(), 1, defMinGas, defGasPrice,
		trxCtxProposal.TxHash, -1)
	_ = signTrx(tx3, vpowMock.PickAddress(0), govTestChainId)
	// not found result
	tx4 := web3.NewTrxVoting(vpowMock.PickAddress(0), types.ZeroAddress(), 1, defMinGas, defGasPrice,
		bytes.RandBytes(32), 0)
	_ = signTrx(tx4, vpowMock.PickAddress(0), govTestChainId)

	// test cases #1
	voteTestCases1 = []*Case{
		{txctx: makeTrxCtx(tx0, 1, true), err: xerrors.ErrNotVotingPeriod},                                      // not voting period
		{txctx: makeTrxCtx(tx0, 10+govCtrler.MinVotingPeriodBlocks()+1, true), err: xerrors.ErrNotVotingPeriod}, // not voting period
		{txctx: makeTrxCtx(tx1, 10, true), err: xerrors.ErrNoRight},                                             // no right
		{txctx: makeTrxCtx(tx2, 10, true), err: xerrors.ErrInvalidTrxPayloadParams},                             // not found result
		{txctx: makeTrxCtx(tx3, 10, true), err: xerrors.ErrInvalidTrxPayloadParams},                             // not found result
		{txctx: makeTrxCtx(tx4, 10, true), err: xerrors.ErrNotFoundResult},                                      // not found result
		{txctx: makeTrxCtx(tx0, 10, true), err: nil},                                                            // success
	}

	// txs of validators except vpowMock.delegatees[0]
	var txs []*ctrlertypes.Trx
	for i := 1; i < vpowMock.ValCnt; i++ {
		addr := vpowMock.PickAddress(i)
		choice := int32(0)
		//rn := int(bytes.RandInt63n(int64(len(vpowMock.delegatees))))
		//if rn%3 == 0 {
		//	choice = 1
		//}
		tx := web3.NewTrxVoting(addr, types.ZeroAddress(), 1, defMinGas, defGasPrice,
			trxCtxProposal.TxHash, choice)
		_ = signTrx(tx, addr, govTestChainId)
		txs = append(txs, tx)
	}

	// test cases #2 - all success case
	for i, tx := range txs {
		voteTestCases2 = append(voteTestCases2, &Case{
			txctx: makeTrxCtx(tx, int64(10+i), true),
			err:   nil,
		})
	}
}

func TestVoting(t *testing.T) {
	votedPowers := int64(0)
	for i, c := range voteTestCases1 {
		xerr := runCase(c)
		require.Equal(t, c.err, xerr, "index", i)

		if xerr == nil {
			votedPowers += vpowMock.TotalPowerOf(c.txctx.Tx.From)
		}
	}

	_, _, xerr := govCtrler.Commit()
	require.NoError(t, xerr)

	prop, xerr := govCtrler.ReadProposal(trxCtxProposal.TxHash, false)
	require.NoError(t, xerr)

	sumVotedPowers := int64(0)
	for i, c := range voteTestCases1 {
		if c.err == nil {
			power := vpowMock.TotalPowerOf(c.txctx.Tx.From)
			require.Equal(t, power, prop.Option(0).Votes, "index", i)
			sumVotedPowers += prop.Option(0).Votes
		}
	}

	require.Equal(t, votedPowers, sumVotedPowers)
}

func TestMajority(t *testing.T) {
	prop, xerr := govCtrler.ReadProposal(trxCtxProposal.TxHash, false)
	require.NoError(t, xerr)
	require.NotNil(t, prop)

	opt := prop.UpdateMajorOption()
	require.Nil(t, opt)

	votedPowers := prop.Option(0).Votes
	for i, c := range voteTestCases2 {
		xerr := runCase(c)
		require.Equal(t, c.err, xerr, "index", i)

		_, _, xerr = govCtrler.Commit()
		require.NoError(t, xerr)

		prop, xerr := govCtrler.ReadProposal(trxCtxProposal.TxHash, false)
		require.NoError(t, xerr)
		require.NotNil(t, prop)

		votedPowers += vpowMock.TotalPowerOf(c.txctx.Tx.From)
		if votedPowers >= prop.Header().MajorityPower {
			opt := prop.UpdateMajorOption()
			require.NotNil(t, opt, votedPowers, prop.Header().MajorityPower)
			require.Equal(t, votedPowers, opt.Votes)
		} else {
			opt := prop.UpdateMajorOption()
			require.Nil(t, opt)
		}
	}

	//
	// duplicated voting
	// its votes MUST not changed
	for i, c := range voteTestCases2 {
		xerr := runCase(c)
		require.Equal(t, c.err, xerr, "index", i)

		_, _, xerr = govCtrler.Commit()
		require.NoError(t, xerr)

		prop, xerr := govCtrler.ReadProposal(trxCtxProposal.TxHash, false)
		require.NoError(t, xerr)
		require.NotNil(t, prop)

		opt := prop.UpdateMajorOption()
		require.NotNil(t, opt)
		require.Equal(t, votedPowers, opt.Votes)
	}
}

func TestFreezingProposal(t *testing.T) {
	// make the proposal majority
	for i, c := range voteTestCases2 {
		xerr := runCase(c)
		require.Equal(t, c.err, xerr, "index", i)
	}
	_, _, xerr := govCtrler.Commit()
	require.NoError(t, xerr)

	prop, xerr := govCtrler.ReadProposal(trxCtxProposal.TxHash, false)
	require.NoError(t, xerr)

	//
	// not changed
	bctx := &ctrlertypes.BlockContext{}
	bctx.SetHeight(prop.Header().EndVotingHeight)
	_, xerr = govCtrler.EndBlock(bctx)
	require.NoError(t, xerr)

	_, _, xerr = govCtrler.Commit()
	require.NoError(t, xerr)
	prop, xerr = govCtrler.ReadProposal(trxCtxProposal.TxHash, false)
	require.NoError(t, xerr)

	//
	// freezing the proposal
	bctx = &ctrlertypes.BlockContext{}
	bctx.SetHeight(prop.Header().EndVotingHeight + 1)
	_, xerr = govCtrler.EndBlock(bctx)
	require.NoError(t, xerr)

	_, _, xerr = govCtrler.Commit()
	require.NoError(t, xerr)

	// the proposal is frozen.
	_, xerr = govCtrler.ReadProposal(trxCtxProposal.TxHash, false)
	require.Equal(t, xerrors.ErrNotFoundProposal, xerr)
	item, xerr := govCtrler.govState.Get(v1.LedgerKeyFrozenProp(trxCtxProposal.TxHash), false)
	require.NoError(t, xerr)
	frozenProp, _ := item.(*proposal.GovProposal)
	require.NotNil(t, frozenProp.MajorOption())

	//// prop.MajorOption is nil, so...
	//prop.MajorOption = frozenProp.MajorOption
	//require.Equal(t, prop, frozenProp)

	testFlagAlreadyFrozen = true
}

func TestApplyingProposal(t *testing.T) {
	oriParams := govCtrler.GovParams
	require.True(t, ctrlertypes.DefaultGovParams().Equal(&oriParams))

	txProposalPayload, ok := trxCtxProposal.Tx.Payload.(*ctrlertypes.TrxPayloadProposal)
	require.True(t, ok)

	if testFlagAlreadyFrozen == false {
		// make the proposal majority
		for i, c := range voteTestCases2 {
			xerr := runCase(c)
			require.Equal(t, c.err, xerr, "index", i)
		}
		_, _, xerr := govCtrler.Commit()
		require.NoError(t, xerr)

		// freezing the proposal
		bctx := &ctrlertypes.BlockContext{}
		bctx.SetHeight(txProposalPayload.StartVotingHeight + txProposalPayload.VotingPeriodBlocks + 1)
		_, xerr = govCtrler.EndBlock(bctx)
		require.NoError(t, xerr)
		_, _, xerr = govCtrler.Commit()
		require.NoError(t, xerr)
	}

	//
	// not changed
	runHeight := txProposalPayload.StartVotingHeight + txProposalPayload.VotingPeriodBlocks + govCtrler.LazyApplyingBlocks() - 1
	bctx := &ctrlertypes.BlockContext{}
	bctx.SetHeight(runHeight)
	_, xerr := govCtrler.EndBlock(bctx)
	require.NoError(t, xerr)
	_, _, xerr = govCtrler.Commit()
	require.NoError(t, xerr)
	frozenProp, xerr := govCtrler.govState.Get(v1.LedgerKeyFrozenProp(trxCtxProposal.TxHash), false)
	require.NoError(t, xerr)
	require.NotNil(t, frozenProp)

	//
	// apply new gov rule
	runHeight = txProposalPayload.StartVotingHeight + txProposalPayload.VotingPeriodBlocks + govCtrler.LazyApplyingBlocks()
	bctx = &ctrlertypes.BlockContext{}
	bctx.SetHeight(runHeight)
	_, xerr = govCtrler.EndBlock(bctx)
	require.NoError(t, xerr)
	require.NotNil(t, govCtrler.newGovParams)

	_, _, xerr = govCtrler.Commit()
	require.NoError(t, xerr)
	frozenProp, xerr = govCtrler.govState.Get(v1.LedgerKeyFrozenProp(trxCtxProposal.TxHash), false)
	require.Equal(t, xerrors.ErrNotFoundResult, xerr)
	require.Nil(t, frozenProp)

	require.NotEqual(t, oriParams, govCtrler.GovParams)
	require.True(t, govParams1.Equal(&govCtrler.GovParams))
}
