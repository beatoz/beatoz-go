package gov

import (
	"fmt"
	cfg "github.com/beatoz/beatoz-go/cmd/config"
	"github.com/beatoz/beatoz-go/ctrlers/gov/proposal"
	"github.com/beatoz/beatoz-go/ctrlers/mocks"
	"github.com/beatoz/beatoz-go/ctrlers/types"
	btztypes "github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/stretchr/testify/require"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/json"
	tmlog "github.com/tendermint/tendermint/libs/log"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

func Test_Punish_By_BlockProcess(t *testing.T) {
	// Use `localGovCtrler` instead of `govCtrler` to avoid interference with other tests.
	rootPath := filepath.Join(os.TempDir(), "gov-punish-test")
	localCfg := cfg.DefaultConfig()
	localCfg.SetRoot(rootPath)
	localCfg.ChainID = "gov-punish-test-chain"

	require.NoError(t, os.RemoveAll(rootPath))

	localGovCtrler, xerr := NewGovCtrler(localCfg, tmlog.NewNopLogger())
	require.NoError(t, xerr)
	localGovCtrler.GovParams = *(types.DefaultGovParams())

	bctx := mocks.InitBlockCtxWith(localCfg.ChainID, 1, localGovCtrler, acctMock, nil, nil, vpowMock)
	bctx.SetChainID(localCfg.ChainID)

	voterAddr := vpowMock.PickAddress(vpowMock.ValCnt - 1)
	expectedvoterPower := vpowMock.TotalPowerOf(voterAddr)

	// proposal
	bzOpt, err := json.Marshal(govParams0)
	require.NoError(t, err)
	tx := web3.NewTrxProposal(
		voterAddr, btztypes.ZeroAddress(), 1, defMinGas, defGasPrice,
		localCfg.ChainID,
		10,
		localGovCtrler.MinVotingPeriodBlocks(),
		10+localGovCtrler.MinVotingPeriodBlocks()+localGovCtrler.LazyApplyingBlocks(),
		proposal.PROPOSAL_GOVPARAMS, bzOpt) // it will be used to test wrong start height
	_ = signTrx(tx, vpowMock.PickAddress(vpowMock.ValCnt-1), localCfg.ChainID)
	txbz, xerr := tx.Encode()
	require.NoError(t, xerr)
	txctx, xerr := types.NewTrxContext(txbz, bctx, true)
	require.NoError(t, xerr)

	require.NoError(t, mocks.DoBeginBlock(localGovCtrler))
	require.NoError(t, mocks.DoRunTrx(localGovCtrler, txctx))
	require.NoError(t, mocks.DoEndBlockAndCommit(localGovCtrler))

	wrongTxHash := bytes.Copy(txctx.TxHash)
	wrongTxHash[0] = ^wrongTxHash[0]
	prop, err := localGovCtrler.ReadProposal(wrongTxHash, false)
	require.Error(t, err)
	require.Nil(t, prop)

	prop, err = localGovCtrler.ReadProposal(txctx.TxHash, false)
	require.NoError(t, err)
	propVoter := prop.Voters[voterAddr.String()]
	require.NotNil(t, propVoter)
	require.Equal(t, expectedvoterPower, propVoter.Power)
	fmt.Println("voter", voterAddr, "voterPower", expectedvoterPower, "prop", prop.TxHash, "totalPower", prop.TotalVotingPower)

	//
	// progress blocks...
	for i := 0; i < 3; i++ {
		require.NoError(t, mocks.DoBeginBlock(localGovCtrler))
		require.NoError(t, mocks.DoEndBlockAndCommit(localGovCtrler))
	}

	//
	// compute exptected values after punishment.
	expectedSlashed := propVoter.Power * int64(localGovCtrler.SlashRate()) / int64(100)
	expectedTotalVotingPower := prop.TotalVotingPower - expectedSlashed
	expectedvoterPower = expectedvoterPower - expectedSlashed

	//
	// occurs offense
	offenseHeight := rand.Int63n(mocks.CurrBlockHeight()-1) + 1
	evidence := abcitypes.Evidence{
		Type: abcitypes.EvidenceType_DUPLICATE_VOTE,
		Validator: abcitypes.Validator{
			Address: voterAddr,
			Power:   0, // don't care
		},
		Height: offenseHeight,
	}
	mocks.CurrBlockCtx().SetByzantine([]abcitypes.Evidence{evidence})

	require.NoError(t, mocks.DoBeginBlock(localGovCtrler))

	{
		// check result
		prop, err = localGovCtrler.ReadProposal(txctx.TxHash, true)
		require.NoError(t, err)
		propVoter = prop.Voters[voterAddr.String()]

		fmt.Println("voter", propVoter.Addr, "voterPower", propVoter.Power, "prop", prop.TxHash, "totalPower", prop.TotalVotingPower)

		require.NotNil(t, propVoter)
		require.Equal(t, expectedvoterPower, propVoter.Power)
		require.Equal(t, expectedTotalVotingPower, prop.TotalVotingPower)
	}

	require.NoError(t, mocks.DoEndBlockAndCommit(localGovCtrler))

	{
		// check result
		prop, err = localGovCtrler.ReadProposal(txctx.TxHash, false)
		require.NoError(t, err)
		propVoter = prop.Voters[voterAddr.String()]
		require.NotNil(t, propVoter)
		require.Equal(t, expectedvoterPower, propVoter.Power)
		require.Equal(t, expectedTotalVotingPower, prop.TotalVotingPower)
	}

}
