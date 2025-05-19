package proposal

import (
	"bytes"
	"github.com/beatoz/beatoz-go/types"
)

const (
	PROPOSAL_ONCHAIN   = 0x0100
	PROPOSAL_OFFCHAIN  = 0x0200
	PROPOSAL_GOVPARAMS = PROPOSAL_ONCHAIN | 0x01
	PROPOSAL_COMMON    = PROPOSAL_OFFCHAIN | 0x00
)

const (
	NOT_CHOICE int32 = -1
)

func (x *GovProposalHeaderProto) SumVotingPowers() int64 {
	sum := int64(0)
	for _, v := range x.Voters {
		sum += v.Power
	}
	return sum
}

func (x *GovProposalHeaderProto) IsVoter(addr types.Address) bool {
	for _, v := range x.Voters {
		if bytes.Equal(v.Address, addr) {
			return true
		}
	}
	return false
}

func (x *GovProposalHeaderProto) addVoter(addr types.Address, power int64) {
	v := x.findVoter(addr)
	if v != nil {
		return // do nothing
	}

	v = &VoterProto{
		Address: addr,
		Power:   power,
		Choice:  NOT_CHOICE,
	}
	x.Voters = append(x.Voters, v)
}
func (x *GovProposalHeaderProto) findVoter(addr types.Address) *VoterProto {
	for _, v := range x.Voters {
		if bytes.Equal(v.Address, addr) {
			return v
		}
	}
	return nil
}

func (x *GovProposalHeaderProto) removeVoter(addr types.Address) *VoterProto {
	for i := len(x.Voters) - 1; i >= 0; i-- {
		v := x.Voters[i]
		if bytes.Equal(v.Address, addr) {
			x.Voters = append(x.Voters[:i], x.Voters[i+1:]...)
			return v
		}
	}
	return nil
}
