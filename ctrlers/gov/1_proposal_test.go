package gov

import (
	"encoding/json"
	"github.com/beatoz/beatoz-go/ctrlers/gov/proposal"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/stretchr/testify/require"
	"math"
	"testing"
)

type Case struct {
	txctx *ctrlertypes.TrxContext
	err   xerrors.XError
}

var (
	cases1 []*Case
	cases2 []*Case
)

func init() {
	bzOpt, err := json.Marshal(govParams0)
	if err != nil {
		panic(err)
	}

	//tx0 := web3.NewTrxProposal(
	//	vpowMock.PickAddress(vpowMock.ValCnt-1), types.ZeroAddress(), 1, defMinGas, defGasPrice, // insufficient fee
	//	"test govparams proposal",
	//	10,
	//	govCtrler.MinVotingPeriodBlocks(),
	//	10-1+govCtrler.MinVotingPeriodBlocks()+govCtrler.LazyApplyingBlocks(), // too small applying height
	//	proposal.PROPOSAL_GOVPARAMS, bzOpt)
	//_ = signTrx(tx0, vpowMock.PickAddress(vpowMock.ValCnt-1), "")

	tx1 := web3.NewTrxProposal(
		vpowMock.PickAddress(vpowMock.ValCnt+1), types.ZeroAddress(), 1, defMinGas, defGasPrice,
		"test govparams proposal",
		10,
		govCtrler.MinVotingPeriodBlocks(),
		10+govCtrler.MinVotingPeriodBlocks()+govCtrler.LazyApplyingBlocks(),
		proposal.PROPOSAL_GOVPARAMS, bzOpt)
	_ = signTrx(tx1, vpowMock.PickAddress(vpowMock.ValCnt+1), "") // not validator

	tx3 := web3.NewTrxProposal(
		vpowMock.PickAddress(vpowMock.ValCnt-1), types.ZeroAddress(), 1, defMinGas, defGasPrice,
		"test govparams proposal",
		10,
		govCtrler.MinVotingPeriodBlocks()-1, // too small period
		10+govCtrler.MinVotingPeriodBlocks()+govCtrler.LazyApplyingBlocks(),
		proposal.PROPOSAL_GOVPARAMS, bzOpt) // too small period
	_ = signTrx(tx3, vpowMock.PickAddress(vpowMock.ValCnt-1), "")

	tx4 := web3.NewTrxProposal(
		vpowMock.PickAddress(vpowMock.ValCnt-1), types.ZeroAddress(), 1, defMinGas, defGasPrice,
		"test govparams proposal",
		10,
		govCtrler.MinVotingPeriodBlocks(),
		10+govCtrler.MinVotingPeriodBlocks()+govCtrler.LazyApplyingBlocks(),
		proposal.PROPOSAL_GOVPARAMS, bzOpt) // it will be used to test wrong start height
	_ = signTrx(tx4, vpowMock.PickAddress(vpowMock.ValCnt-1), "")
	tx5 := web3.NewTrxProposal(
		vpowMock.PickAddress(vpowMock.ValCnt-1), types.ZeroAddress(), 1, defMinGas, defGasPrice,
		"test govparams proposal", 10, govCtrler.MinVotingPeriodBlocks(), 10+govCtrler.MinVotingPeriodBlocks()+govCtrler.LazyApplyingBlocks(), proposal.PROPOSAL_GOVPARAMS, bzOpt) // all right
	_ = signTrx(tx5, vpowMock.PickAddress(vpowMock.ValCnt-1), "")

	cases1 = []*Case{
		//{txctx: makeTrxCtx(tx0, 1, true), err: xerrors.ErrInvalidTrxPayloadParams}, // too small applying height
		{txctx: makeTrxCtx(tx1, 1, true), err: xerrors.ErrNoRight},
		{txctx: makeTrxCtx(tx3, 1, true), err: xerrors.ErrInvalidTrxPayloadParams},  // wrong period
		{txctx: makeTrxCtx(tx4, 20, true), err: xerrors.ErrInvalidTrxPayloadParams}, // wrong start height
		{txctx: makeTrxCtx(tx5, 1, true), err: nil},                                 // success

	}

	tx6 := web3.NewTrxProposal(
		vpowMock.PickAddress(vpowMock.ValCnt-1), types.ZeroAddress(), 1, defMinGas, defGasPrice,
		"test govparams proposal2", 11, 259200, 518400+11, proposal.PROPOSAL_GOVPARAMS, bzOpt)
	_ = signTrx(tx6, vpowMock.PickAddress(vpowMock.ValCnt-1), "")

	cases2 = []*Case{
		// the tx6 will be submitted two times.
		// the first must success but the second must fail.
		{txctx: makeTrxCtx(tx6, 1, true), err: nil},
	}
}

func TestAddProposal(t *testing.T) {
	props0, _ := govCtrler.ReadAllProposals()
	require.NotNil(t, props0)

	for i, c := range cases1 {
		xerr := runCase(c)
		require.Equal(t, c.err, xerr, "index", i)
	}

	props1, _ := govCtrler.ReadAllProposals()
	require.NotNil(t, props1)
}

func TestProposalDuplicate(t *testing.T) {
	for i, c := range cases2 {
		require.NoError(t, runCase(c), "index", i)
	}

	_, _, err := govCtrler.Commit()
	require.NoError(t, err)

	for _, c := range cases2 {
		key := v1.LedgerKeyProposal(c.txctx.TxHash)
		item, xerr := govCtrler.proposalState.Get(key, false)
		require.NoError(t, xerr)
		require.NotNil(t, item)
		prop, _ := item.(*proposal.GovProposal)
		require.EqualValues(t, key, v1.LedgerKeyProposal(prop.TxHash))
	}
	for i, c := range cases2 {
		require.Error(t, xerrors.ErrDuplicatedKey, runCase(c), "index", i)
	}
}

func TestOverflowBlockHeight(t *testing.T) {
	bzOpt, err := json.Marshal(govParams0)
	require.NoError(t, err)

	tx := web3.NewTrxProposal(
		vpowMock.PickAddress(vpowMock.ValCnt-1), types.ZeroAddress(), 1, defMinGas, defGasPrice,
		"test govparams proposal", math.MaxInt64, 259200, 518400+10, proposal.PROPOSAL_GOVPARAMS, bzOpt)
	require.NoError(t, signTrx(tx, vpowMock.PickAddress(vpowMock.ValCnt-1), ""))
	xerr := runTrx(makeTrxCtx(tx, 1, true))
	require.Error(t, xerr)
	require.Contains(t, xerr.Error(), "overflow occurs")
}

func TestApplyingHeight(t *testing.T) {
	bzOpt, err := json.Marshal(govParams0)
	require.NoError(t, err)

	tx0 := web3.NewTrxProposal( // applyingHeight : 518410
		vpowMock.PickAddress(vpowMock.ValCnt-1), types.ZeroAddress(), 1, defMinGas, defGasPrice,
		"test govparams proposal", 10, 259200, 518400+10, proposal.PROPOSAL_GOVPARAMS, bzOpt)
	require.NoError(t, signTrx(tx0, vpowMock.PickAddress(vpowMock.ValCnt-1), ""))
	xerr := runTrx(makeTrxCtx(tx0, 1, true))
	require.NoError(t, xerr)

	tx1 := web3.NewTrxProposal( // applyingHeight : start + period + lazyApplyingBlocks
		vpowMock.PickAddress(vpowMock.ValCnt-1), types.ZeroAddress(), 1, defMinGas, defGasPrice,
		"test govparams proposal", 10, 259200, ctrlertypes.DefaultGovParams().LazyApplyingBlocks()+259200+10, proposal.PROPOSAL_GOVPARAMS, bzOpt)
	require.NoError(t, signTrx(tx1, vpowMock.PickAddress(vpowMock.ValCnt-1), ""))
	xerr = runTrx(makeTrxCtx(tx1, 1, true))
	require.NoError(t, xerr)

	tx2 := web3.NewTrxProposal( // wrong applyingHeight
		vpowMock.PickAddress(vpowMock.ValCnt-1), types.ZeroAddress(), 1, defMinGas, defGasPrice,
		"test govparams proposal", 10, 259200, 1, proposal.PROPOSAL_GOVPARAMS, bzOpt)
	require.NoError(t, signTrx(tx2, vpowMock.PickAddress(vpowMock.ValCnt-1), ""))
	xerr = runTrx(makeTrxCtx(tx2, 1, true))
	require.Error(t, xerr)
	require.Contains(t, xerr.Error(), "wrong applyingHeight")

	tx3 := web3.NewTrxProposal( // applyingHeight : start + period + lazyApplyingBlocks - 1
		vpowMock.PickAddress(vpowMock.ValCnt-1), types.ZeroAddress(), 1, defMinGas, defGasPrice,
		"test govparams proposal", 10, 259200, ctrlertypes.DefaultGovParams().LazyApplyingBlocks()+259200+10-1, proposal.PROPOSAL_GOVPARAMS, bzOpt)
	require.NoError(t, signTrx(tx3, vpowMock.PickAddress(vpowMock.ValCnt-1), ""))
	xerr = runTrx(makeTrxCtx(tx3, 1, true))
	require.Error(t, xerr)
	require.Contains(t, xerr.Error(), "wrong applyingHeight")

	tx4 := web3.NewTrxProposal( // applyingHeight : -518410
		vpowMock.PickAddress(vpowMock.ValCnt-1), types.ZeroAddress(), 1, defMinGas, defGasPrice,
		"test govparams proposal", 10, 259200, -518410, proposal.PROPOSAL_GOVPARAMS, bzOpt)
	require.NoError(t, signTrx(tx4, vpowMock.PickAddress(vpowMock.ValCnt-1), ""))
	xerr = runTrx(makeTrxCtx(tx4, 1, true))
	require.Error(t, xerr)
	require.Contains(t, xerr.Error(), "wrong applyingHeight")

}
