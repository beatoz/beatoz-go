package proposal

import (
	"github.com/beatoz/beatoz-go/types"
	abytes "github.com/beatoz/beatoz-go/types/bytes"
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

type Voter struct {
	Addr   types.Address `json:"address"`
	Power  int64         `json:"power"`
	Choice int32         `json:"choice"`
}

type GovProposalHeader struct {
	TxHash            abytes.HexBytes   `json:"txHash"`
	StartVotingHeight int64             `json:"startVotingHeight"`
	EndVotingHeight   int64             `json:"endVotingHeight"`
	ApplyingHeight    int64             `json:"applyingHeight"`
	TotalVotingPower  int64             `json:"totalVotingPower"`
	MajorityPower     int64             `json:"majorityPower"`
	Voters            map[string]*Voter `json:"votes"`
	OptType           int32             `json:"optType"`
}

func (h *GovProposalHeader) GetTxHash() abytes.HexBytes {
	return h.TxHash
}

func (h *GovProposalHeader) GetStartVotingHeight() int64 {
	return h.StartVotingHeight
}

func (h *GovProposalHeader) GetEndVotingHeight() int64 {
	return h.EndVotingHeight
}

func (h *GovProposalHeader) GetApplyingHeight() int64 {
	return h.ApplyingHeight
}

func (h *GovProposalHeader) GetTotalVotingPower() int64 {
	return h.TotalVotingPower
}
func (h *GovProposalHeader) SumVotingPowers() int64 {
	sum := int64(0)
	for _, v := range h.Voters {
		sum += v.Power
	}
	return sum
}

func (h *GovProposalHeader) GetMajorityPower() int64 {
	return h.MajorityPower
}

func (h *GovProposalHeader) IsVoter(addr types.Address) bool {
	_, ok := h.Voters[addr.String()]
	return ok
}

func (h *GovProposalHeader) GetVoter(addr types.Address) *Voter {
	return h.Voters[addr.String()]
}

func (h *GovProposalHeader) GetOptType() int32 {
	return h.OptType
}
