package test

import (
	"encoding/json"
	"fmt"
	"github.com/beatoz/beatoz-go/ctrlers/gov/proposal"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	types2 "github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/stretchr/testify/require"
	"testing"
)

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

	//
	// new proposal
	bzOpt, err := json.Marshal(ctrlertypes.Test3GovParams())
	require.NoError(t, err)

	lastBlockHeight, err := waitBlock(10)
	require.NoError(t, err)

	startVoteBlockHeight := lastBlockHeight + 5
	proposalResult, err := validatorWallet.ProposalCommit(defGas, defGasPrice, "proposal test", startVoteBlockHeight, 259200, 518410+startVoteBlockHeight, proposal.PROPOSAL_GOVPARAMS, bzOpt, bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, proposalResult.CheckTx.Code)
	require.Equal(t, xerrors.ErrCodeSuccess, proposalResult.DeliverTx.Code)
	require.NotNil(t, proposalResult.Hash)

	proposalHash := bytes.HexBytes(proposalResult.Hash)

	prop, err := bzweb3.QueryProposal(proposalHash, 0)
	require.NoError(t, err)
	fmt.Println("proposal", prop)

	//
	// voting to new proposal
	lastBlockHeight, err = waitBlock(startVoteBlockHeight)
	require.NoError(t, err)

	prop, err = bzweb3.QueryProposal(proposalHash, 0)
	require.NoError(t, err)
	fmt.Println("proposal", prop)

	require.NoError(t, validatorWallet.SyncAccount(bzweb3))
	votingResult, err := validatorWallet.VotingCommit(defGas, defGasPrice, proposalHash, 0, bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCodeSuccess, votingResult.CheckTx.Code)
	require.Equal(t, xerrors.ErrCodeSuccess, votingResult.DeliverTx.Code)

	prop, err = bzweb3.QueryProposal(proposalHash, 0)
	require.NoError(t, err)
	fmt.Println("proposal", prop)
}

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

	// the following has wrong json format.
	bzOpt := []byte(`{"slashRatio": "60""}`)

	lastBlockHeight, err := waitBlock(10)
	require.NoError(t, err)

	startVoteBlockHeight := lastBlockHeight + 5
	proposalResult, err := validatorWallet.ProposalCommit(defGas, defGasPrice, "proposal test", startVoteBlockHeight, 259200, 518410+startVoteBlockHeight, proposal.PROPOSAL_GOVPARAMS, bzOpt, bzweb3)
	require.NoError(t, err)
	require.Equal(t, xerrors.ErrCheckTx.Code(), proposalResult.CheckTx.Code)
}
