package test

import (
	"encoding/json"
	"fmt"
	"github.com/beatoz/beatoz-go/ctrlers/gov/proposal"
	"github.com/beatoz/beatoz-go/ctrlers/types"
	types2 "github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/rand"
	"testing"
)

func TestIncorrectProposal(t *testing.T) {
	//get validator wallet
	bzweb3 := randBeatozWeb3()
	validatorWallet := validatorWallets[0]
	require.NoError(t, validatorWallet.SyncAccount(bzweb3))
	require.NoError(t, validatorWallet.Unlock(defaultRpcNode.Pass))

	//asset transfer for unit test
	if validatorWallet.GetBalance().IsZero() {
		sender := randCommonWallet()
		require.NoError(t, sender.Unlock([]byte("1111")))
		require.NoError(t, transferFrom(sender, validatorWallet.Address(), types2.ToFons(1000), bzweb3))
	}

	//
	// query current parameters
	currGovParams, xerr := bzweb3.GetGovParams()
	require.NoError(t, xerr)
	require.NotNil(t, currGovParams)

	// the following has wrong json format.
	bzOpt := []byte(`{"slashRatio": "60""}`)

	lastBlockHeight, err := waitBlock(10)
	require.NoError(t, err)

	startHeight := lastBlockHeight + 5
	votePeriod := currGovParams.MinVotingPeriodBlocks()
	applyHeight := startHeight + votePeriod + currGovParams.LazyApplyingBlocks()
	proposalResult, err := validatorWallet.ProposalCommit(defGas, defGasPrice, "proposal test", startHeight, votePeriod, applyHeight, proposal.PROPOSAL_GOVPARAMS, bzOpt, bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCheckTx.Code(), proposalResult.CheckTx.Code)

	//
	// new proposal
	newGovParams := &types.GovParams{}
	newGovParams.SetValue(func(v *types.GovParamsProto) {
		v.Version = rand.Int32()
		v.SlashRate = rand.Int32()
		v.XRewardPoolAddress = bytes.RandBytes(20)
	})
	bzOpt, err = json.Marshal(newGovParams)
	require.NoError(t, err)

	// wrong voting period (less than min)
	votePeriod = currGovParams.MinVotingPeriodBlocks() - 1
	applyHeight = startHeight + votePeriod + currGovParams.LazyApplyingBlocks()
	proposalResult, err = validatorWallet.ProposalCommit(defGas, defGasPrice, "proposal test", startHeight, votePeriod, applyHeight, proposal.PROPOSAL_GOVPARAMS, bzOpt, bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCheckTx.Code(), proposalResult.CheckTx.Code)

	// wrong voting period (over than max)
	votePeriod = currGovParams.MaxVotingPeriodBlocks() + 1
	proposalResult, err = validatorWallet.ProposalCommit(defGas, defGasPrice, "proposal test", startHeight, votePeriod, applyHeight, proposal.PROPOSAL_GOVPARAMS, bzOpt, bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCheckTx.Code(), proposalResult.CheckTx.Code)

	// wrong voting period (negative)
	votePeriod = -1 * currGovParams.MaxVotingPeriodBlocks()
	applyHeight = startHeight + votePeriod + currGovParams.LazyApplyingBlocks() - 1
	proposalResult, err = validatorWallet.ProposalCommit(defGas, defGasPrice, "proposal test", startHeight, votePeriod, applyHeight, proposal.PROPOSAL_GOVPARAMS, bzOpt, bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCheckTx.Code(), proposalResult.CheckTx.Code)

	// wrong apply height
	votePeriod = currGovParams.MinVotingPeriodBlocks()
	applyHeight = startHeight + votePeriod + currGovParams.LazyApplyingBlocks() - 1
	proposalResult, err = validatorWallet.ProposalCommit(defGas, defGasPrice, "proposal test", startHeight, votePeriod, applyHeight, proposal.PROPOSAL_GOVPARAMS, bzOpt, bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCheckTx.Code(), proposalResult.CheckTx.Code)

}

func TestProposalAndVoting(t *testing.T) {
	//get validator wallet
	bzweb3 := randBeatozWeb3()
	validatorWallet := validatorWallets[0]
	require.NoError(t, validatorWallet.SyncAccount(bzweb3))
	require.NoError(t, validatorWallet.Unlock(defaultRpcNode.Pass))

	//asset transfer for unit test
	if validatorWallet.GetBalance().IsZero() {
		sender := randCommonWallet()
		require.NoError(t, sender.Unlock([]byte("1111")))
		require.NoError(t, transferFrom(sender, validatorWallet.Address(), types2.ToFons(1000), bzweb3))
	}

	currGovParams, xerr := bzweb3.GetGovParams()
	require.NoError(t, xerr)
	require.NotNil(t, currGovParams)

	//
	// new proposal
	newGovParams := &types.GovParams{}
	newGovParams.SetValue(func(v *types.GovParamsProto) {
		v.Version = rand.Int32()
		v.SlashRate = rand.Int32()
		v.XRewardPoolAddress = bytes.RandBytes(20)
	})
	bzOpt, err := json.Marshal(newGovParams)
	require.NoError(t, err)

	lastBlockHeight, err := waitBlock(10)
	require.NoError(t, err)

	startHeight := lastBlockHeight + 5
	votePeriod := currGovParams.MinVotingPeriodBlocks()
	applyHeight := startHeight + votePeriod + currGovParams.LazyApplyingBlocks()

	proposalResult, err := validatorWallet.ProposalCommit(defGas, defGasPrice, "proposal test", startHeight, votePeriod, applyHeight, proposal.PROPOSAL_GOVPARAMS, bzOpt, bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, proposalResult.CheckTx.Code)
	require.Equal(t, xerrors.ErrCodeSuccess, proposalResult.DeliverTx.Code)
	require.NotNil(t, proposalResult.Hash)

	proposalHash := bytes.HexBytes(proposalResult.Hash)

	prop, err := bzweb3.QueryProposal(proposalHash, 0)
	require.NoError(t, err)
	require.EqualValues(t, proposalResult.Hash, prop.Proposal.Header().TxHash)
	require.Equal(t, 1, len(prop.Proposal.Options()))

	option := prop.Proposal.Option(0)
	require.NotNil(t, option)
	_propGovParams := &types.GovParams{}
	err = json.Unmarshal(option.Option, _propGovParams)
	require.NoError(t, err)
	require.Equal(t, newGovParams.Version(), _propGovParams.Version())
	require.Equal(t, newGovParams.SlashRate(), _propGovParams.SlashRate())
	require.EqualValues(t, newGovParams.RewardPoolAddress(), _propGovParams.RewardPoolAddress())

	//
	// voting to new proposal
	lastBlockHeight, err = waitBlock(startHeight)
	require.NoError(t, err)

	prop, err = bzweb3.QueryProposal(proposalHash, 0)
	require.NoError(t, err)
	fmt.Println("proposal", prop)

	// not validator: error expected
	nonValWal := randCommonWallet()
	require.NoError(t, nonValWal.SyncAccount(bzweb3))
	require.NoError(t, nonValWal.Unlock([]byte("1111")))
	votingResult, err := nonValWal.VotingCommit(defGas, defGasPrice, proposalHash, 0, bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCheckTx.Code(), votingResult.CheckTx.Code)

	// validator
	require.NoError(t, validatorWallet.SyncAccount(bzweb3))
	votingResult, err = validatorWallet.VotingCommit(defGas, defGasPrice, proposalHash, 0, bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, votingResult.CheckTx.Code)
	require.Equal(t, xerrors.ErrCodeSuccess, votingResult.DeliverTx.Code)

	prop, err = bzweb3.QueryProposal(proposalHash, 0)
	require.NoError(t, err)
	fmt.Println("proposal", prop)
}
