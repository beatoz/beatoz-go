package gov

import (
	"github.com/beatoz/beatoz-go/ctrlers/gov/proposal"
	"github.com/stretchr/testify/require"
	"math/rand"
	"testing"
	"time"
)

// At the package level, all proposals are typically applied or removed by preceding tests,
// meaning that no proposals remain when TestPunish is executed.
func TestPunish(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	byzantineAddr := vpowMock.PickAddress(rand.Intn(vpowMock.ValCnt))

	// original proposals and voter's power
	props, err := govCtrler.ReadAllProposals(false)
	require.NoError(t, err)

	type propSnapshot struct {
		proposal      *proposal.GovProposal
		punishedVoter *proposal.Voter
	}
	var props0 []propSnapshot
	for _, prop := range props {
		v, ok := prop.Voters[byzantineAddr.String()]
		if ok {
			props0 = append(props0,
				propSnapshot{
					proposal:      prop,
					punishedVoter: v,
				})
		}
	}

	slashed, err := govCtrler.doSlash(byzantineAddr)
	require.NoError(t, err)

	// commit
	_, _, err = govCtrler.Commit()
	require.NoError(t, err)

	// proposals and voter's power after punishing
	props, err = govCtrler.ReadAllProposals(false)
	require.NoError(t, err)

	var props1 []propSnapshot
	for _, prop := range props {
		v, ok := prop.Voters[byzantineAddr.String()]
		if ok {
			props1 = append(props1,
				propSnapshot{
					proposal:      prop,
					punishedVoter: v,
				})
		}
	}

	// check punishment result
	require.Equal(t, len(props0), len(props1))
	power0, power1 := int64(0), int64(0)
	for i, prop0 := range props0 {
		require.Equal(t, prop0.proposal.SumVotingPowers(), prop0.proposal.TotalVotingPower)
		require.Equal(t, props1[i].proposal.SumVotingPowers(), props1[i].proposal.TotalVotingPower)
		power0 += prop0.proposal.TotalVotingPower
		power1 += props1[i].proposal.TotalVotingPower
	}

	require.Equal(t, power0-slashed, power1)
}
