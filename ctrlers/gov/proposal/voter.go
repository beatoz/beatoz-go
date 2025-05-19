package proposal

import "github.com/beatoz/beatoz-go/types"

func NewVoter(addr types.Address, power int64) *VoterProto {
	return &VoterProto{
		Address: addr,
		Power:   power,
		Choice:  0,
	}
}
