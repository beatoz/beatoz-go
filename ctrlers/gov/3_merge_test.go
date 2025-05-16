package gov

import (
	"encoding/json"
	"github.com/beatoz/beatoz-go/ctrlers/gov/proposal"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/stretchr/testify/require"
	"math/rand"
	"reflect"
	"testing"
)

var (
	newTrxContext *ctrlertypes.TrxContext
	voteCase      []*Case
)

func init() {
	bzOpt, err := json.Marshal(govParams3)
	if err != nil {
		panic(err)
	}
	newTrx := web3.NewTrxProposal(
		vpowMock.PickAddress(1), types.ZeroAddress(), 1, defMinGas, defGasPrice,
		"test improving governance parameters proposal", 15, 259200, 518400+15, proposal.PROPOSAL_GOVPARAMS, bzOpt)
	_ = signTrx(newTrx, vpowMock.PickAddress(1), "")
	newTrxContext = makeTrxCtx(newTrx, 1, true)
	if xerr := runTrx(newTrxContext); xerr != nil {
		panic(xerr)
	}
	if _, _, xerr := govCtrler.Commit(); xerr != nil {
		panic(xerr)
	}

	var txs []*ctrlertypes.Trx
	for i := 1; i < vpowMock.ValCnt; i++ {
		addr := vpowMock.PickAddress(i)
		choice := int32(0)
		tx := web3.NewTrxVoting(addr, types.ZeroAddress(), 1, defMinGas, defGasPrice,
			newTrxContext.TxHash, choice)
		_ = signTrx(tx, addr, "")
		txs = append(txs, tx)
	}

	for i, tx := range txs {
		voteCase = append(voteCase, &Case{
			txctx: makeTrxCtx(tx, int64(10+i), true),
			err:   nil,
		})
	}
}

func TestMergeGovParams(t *testing.T) {
	oriParams := ctrlertypes.DefaultGovParams()

	newParams := &ctrlertypes.GovParams{}
	ctrlertypes.MergeGovParams(oriParams, newParams)
	if !reflect.DeepEqual(newParams, ctrlertypes.DefaultGovParams()) {
		t.Errorf("unexpected GovParams: %v", newParams)
	}

	newParams = ctrlertypes.DefaultGovParams()
	v0 := rand.Int63()
	v1 := types.RandAddress()

	rawVals := newParams.GetValues()
	rawVals.RipeningBlocks = v0
	rawVals.XBurnAddress = v1

	ctrlertypes.MergeGovParams(oriParams, newParams)
	require.Equal(t, v0, newParams.RipeningBlocks())
	require.Equal(t, v1, newParams.BurnAddress())
	require.False(t, reflect.DeepEqual(newParams, ctrlertypes.DefaultGovParams()))

	rawVals.RipeningBlocks = ctrlertypes.DefaultGovParams().RipeningBlocks()
	rawVals.XBurnAddress = ctrlertypes.DefaultGovParams().BurnAddress()
	require.True(t, reflect.DeepEqual(newParams, ctrlertypes.DefaultGovParams()))
}

func TestApplyMergeProposal(t *testing.T) {
	for _, c := range voteCase {
		runCase(c)
	}
	govCtrler.Commit()

	blockContext := &ctrlertypes.BlockContext{}
	txProposalPayload := newTrxContext.Tx.Payload.(*ctrlertypes.TrxPayloadProposal)
	blockContext.SetHeight(txProposalPayload.StartVotingHeight + txProposalPayload.VotingPeriodBlocks + 1)
	govCtrler.EndBlock(blockContext)
	govCtrler.Commit()

	txProposalPayload, ok := newTrxContext.Tx.Payload.(*ctrlertypes.TrxPayloadProposal)
	require.True(t, ok)

	runHeight := txProposalPayload.StartVotingHeight + txProposalPayload.VotingPeriodBlocks + govCtrler.LazyApplyingBlocks()
	blockContext = &ctrlertypes.BlockContext{}
	blockContext.SetHeight(runHeight)
	_, xerr := govCtrler.EndBlock(blockContext)
	require.NoError(t, xerr)
	_, _, xerr = govCtrler.Commit()
	require.NoError(t, xerr)
}
