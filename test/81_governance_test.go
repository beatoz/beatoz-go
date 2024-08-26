package test

import (
	"encoding/json"
	"fmt"
	"github.com/beatoz/beatoz-go/ctrlers/gov/proposal"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/libs/web3"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	tmjson "github.com/tendermint/tendermint/libs/json"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
	"sync"
	"testing"
)

var (
	proposalHash      bytes.HexBytes
	targetBlockHeight int64
)

func TestProposalAndVoting(t *testing.T) {
	//get validator wallet
	bzweb3 := randBeatozWeb3()
	validatorWallet := validatorWallets[0]
	require.NoError(t, validatorWallet.SyncAccount(bzweb3))
	require.NoError(t, validatorWallet.Unlock(defaultRpcNode.Pass))

	bzOpt, err := json.Marshal(ctrlertypes.Test3GovParams())
	require.NoError(t, err)

	//asset transfer for unit test
	if validatorWallet.GetBalance().IsZero() {
		sender := randCommonWallet()
		require.NoError(t, sender.Unlock([]byte("1111")))
		require.NoError(t, sender.SyncAccount(bzweb3))

		_amt := new(uint256.Int).Div(sender.GetBalance(), uint256.NewInt(2))
		fmt.Println("wallet amount", sender.GetBalance().Dec())
		fmt.Println("transfer amount", _amt.Dec())

		txRet, err := sender.TransferCommit(validatorWallet.Address(), defGas, defGasPrice, _amt, bzweb3)
		require.NoError(t, err)
		require.Equal(t, xerrors.ErrCodeSuccess, txRet.CheckTx.Code, txRet.CheckTx.Log)
		require.Equal(t, xerrors.ErrCodeSuccess, txRet.DeliverTx.Code, txRet.DeliverTx.Log)
	}

	//subscriber
	subWg := sync.WaitGroup{}
	sub, err := web3.NewSubscriber(defaultRpcNode.WSEnd)
	require.NoError(t, err)

	subWg.Add(2)
	propSubErr := sub.Start("tm.event='NewBlock'", func(sub *web3.Subscriber, result []byte) {
		event := &coretypes.ResultEvent{}
		err := tmjson.Unmarshal(result, event)
		require.NoError(t, err)
		currentBlockHeight := event.Data.(types.EventDataNewBlock).Block.Height
		if currentBlockHeight > 10 {
			targetBlockHeight = currentBlockHeight + 5
			proposalResult, err2 := validatorWallet.ProposalCommit(defGas, defGasPrice, "proposal test", targetBlockHeight, 259200, 518410+targetBlockHeight, proposal.PROPOSAL_GOVPARAMS, bzOpt, bzweb3)
			require.NoError(t, err2)
			require.Equal(t, xerrors.ErrCodeSuccess, proposalResult.CheckTx.Code)
			proposalHash = bytes.HexBytes(proposalResult.Hash)
			sub.Stop()
			subWg.Done()
		}
	})
	require.NoError(t, propSubErr)

	sub2, err := web3.NewSubscriber(defaultRpcNode.WSEnd)
	require.NoError(t, err)

	votingSubErr := sub2.Start("tm.event='NewBlock'", func(sub *web3.Subscriber, result []byte) {
		event := &coretypes.ResultEvent{}
		err := tmjson.Unmarshal(result, event)
		require.NoError(t, err)

		votingBlockHeight := event.Data.(types.EventDataNewBlock).Block.Height
		if votingBlockHeight > targetBlockHeight && proposalHash != nil {
			require.NoError(t, validatorWallet.SyncAccount(bzweb3))
			votingResult, err := validatorWallet.VotingCommit(defGas, defGasPrice, proposalHash, 0, bzweb3)
			require.NoError(t, err)
			require.Equal(t, xerrors.ErrCodeSuccess, votingResult.CheckTx.Code)
			sub2.Stop()
			subWg.Done()
		} else if votingBlockHeight > targetBlockHeight+100 {
			require.NoError(t, fmt.Errorf("no proposal hash for voting"))
			sub2.Stop()
			subWg.Done()
		}
	})
	require.NoError(t, votingSubErr)
	subWg.Wait()
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
		require.NoError(t, sender.SyncAccount(bzweb3))

		_amt := new(uint256.Int).Div(sender.GetBalance(), uint256.NewInt(2))
		fmt.Println("wallet amount", sender.GetBalance().Dec())
		fmt.Println("transfer amount", _amt.Dec())

		txRet, err := sender.TransferCommit(validatorWallet.Address(), defGas, defGasPrice, _amt, bzweb3)
		require.NoError(t, err)
		require.Equal(t, xerrors.ErrCodeSuccess, txRet.CheckTx.Code, txRet.CheckTx.Log)
		require.Equal(t, xerrors.ErrCodeSuccess, txRet.DeliverTx.Code, txRet.DeliverTx.Log)
	}

	bzOpt := []byte(`{"slashRatio": "60""}`)

	//subscriber
	subWg := sync.WaitGroup{}
	sub, err := web3.NewSubscriber(defaultRpcNode.WSEnd)
	require.NoError(t, err)

	subWg.Add(1)
	propSubErr := sub.Start("tm.event='NewBlock'", func(sub *web3.Subscriber, result []byte) {
		event := &coretypes.ResultEvent{}
		err := tmjson.Unmarshal(result, event)
		require.NoError(t, err)
		currentBlockHeight := event.Data.(types.EventDataNewBlock).Block.Height
		if currentBlockHeight > 10 {
			targetBlockHeight = currentBlockHeight + 5
			proposalResult, err2 := validatorWallet.ProposalCommit(defGas, defGasPrice, "proposal test", targetBlockHeight, 259200, 0, proposal.PROPOSAL_GOVPARAMS, bzOpt, bzweb3)
			require.Equal(t, xerrors.ErrCheckTx.Code(), proposalResult.CheckTx.Code)
			require.NoError(t, err2)
			sub.Stop()
			subWg.Done()
		}
	})
	require.NoError(t, propSubErr)
	subWg.Wait()
}
