package proposal

import (
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"google.golang.org/protobuf/proto"
	"sort"
	"sync"
)

type GovProposal struct {
	v GovProposalProto

	mtx sync.RWMutex
}

func NewGovProposal(propType int32, txhash bytes.HexBytes, startHeight, votingBlocks, totalVotingPower, applyingHeight int64) (*GovProposal, xerrors.XError) {
	endVotingHeight := startHeight + votingBlocks
	return &GovProposal{
		v: GovProposalProto{
			Header: &GovProposalHeaderProto{
				PropType:          propType,
				TxHash:            txhash,
				StartVotingHeight: startHeight,
				EndVotingHeight:   endVotingHeight,
				ApplyHeight:       applyingHeight,
				TotalVotingPower:  totalVotingPower,
				MajorityPower:     (totalVotingPower * 2) / 3,
			},
			Options:     nil,
			MajorOption: nil,
		},
	}, nil
}

func (prop *GovProposal) AddOption(opt []byte) {
	prop.mtx.Lock()
	defer prop.mtx.Unlock()

	prop.v.Options = append(prop.v.Options, &VoteOptionProto{
		Option: opt,
		Votes:  0,
	})
}

func (prop *GovProposal) AddVoter(addr types.Address, power int64) {
	prop.mtx.Lock()
	defer prop.mtx.Unlock()

	prop.v.Header.addVoter(addr, power)
}

func (prop *GovProposal) Encode() ([]byte, xerrors.XError) {
	prop.mtx.RLock()
	defer prop.mtx.RUnlock()

	if bz, err := proto.Marshal(&prop.v); err != nil {
		return bz, xerrors.From(err)
	} else {
		return bz, nil
	}
}

func (prop *GovProposal) Decode(k, v []byte) xerrors.XError {
	prop.mtx.Lock()
	defer prop.mtx.Unlock()

	if err := proto.Unmarshal(v, &prop.v); err != nil {
		return xerrors.From(err)
	}
	return nil
}

var _ v1.ILedgerItem = (*GovProposal)(nil)

func (prop *GovProposal) Header() *GovProposalHeaderProto {
	prop.mtx.RLock()
	defer prop.mtx.RUnlock()
	return prop.v.GetHeader()
}

func (prop *GovProposal) Option(idx int) *VoteOptionProto {
	prop.mtx.RLock()
	defer prop.mtx.RUnlock()

	return prop.v.Options[idx]
}
func (prop *GovProposal) Options() []*VoteOptionProto {
	prop.mtx.RLock()
	defer prop.mtx.RUnlock()

	return prop.v.Options
}

func (prop *GovProposal) MajorOption() *VoteOptionProto {
	prop.mtx.RLock()
	defer prop.mtx.RUnlock()
	return prop.v.MajorOption
}

func (prop *GovProposal) DoVote(addr types.Address, choice int32) xerrors.XError {
	prop.mtx.Lock()
	defer prop.mtx.Unlock()

	// cancel previous vote
	voter := prop.v.Header.findVoter(addr)
	if voter == nil {
		return xerrors.ErrNotFoundVoter
	}

	if voter.Choice == choice {
		// same option is already selected.
		return nil
	}

	prop.cancelVote(voter)
	prop.doVote(voter, choice)

	return nil
}

func (prop *GovProposal) cancelVote(voter *VoterProto) {
	if voter.Choice >= 0 {
		opt := prop.v.Options[voter.Choice]
		opt.CancelVote(voter.Power)
		voter.Choice = -1
	}
}

func (prop *GovProposal) doVote(voter *VoterProto, choice int32) {
	if choice >= 0 {
		opt := prop.v.Options[choice]
		if opt == nil {
			return //xerrors.NewOrdinary("not found option")
		}

		opt.DoVote(voter.Power)
		voter.Choice = choice
	}
}

func (prop *GovProposal) DoSlash(addr types.Address, rate int32) (int64, xerrors.XError) {
	prop.mtx.Lock()
	defer prop.mtx.Unlock()

	voter := prop.v.Header.findVoter(addr)
	if voter == nil {
		return 0, xerrors.ErrNotFoundVoter
	}

	choice := voter.Choice
	if choice >= 0 {
		//  cancel it, if the voter already finishes selection.
		prop.cancelVote(voter)
	}

	slash := voter.Power * int64(rate) / 100
	if slash <= 0 {
		slash = voter.Power
	}
	voter.Power -= slash

	if voter.Power <= 0 {
		_ = prop.v.Header.removeVoter(addr)
	} else if choice >= 0 {
		// do vote again with slashed power
		prop.doVote(voter, choice)
	}
	prop.v.Header.TotalVotingPower -= slash
	prop.v.Header.MajorityPower = (prop.v.Header.TotalVotingPower * 2) / 3

	return slash, nil
}

func (prop *GovProposal) UpdateMajorOption() *VoteOptionProto {
	prop.mtx.Lock()
	defer prop.mtx.Unlock()

	return prop.updateMajorOption()
}

func (prop *GovProposal) updateMajorOption() *VoteOptionProto {
	sort.Sort(powerOrderVoteOptions(prop.v.Options))
	if prop.v.Options[0].Votes >= prop.v.Header.MajorityPower {
		prop.v.MajorOption = prop.v.Options[0]
	}
	return prop.v.MajorOption
}

func (prop *GovProposal) IsVoter(addr types.Address) bool {
	prop.mtx.RLock()
	defer prop.mtx.RUnlock()

	return prop.v.Header.IsVoter(addr)
}

func (prop *GovProposal) FindVoter(addr types.Address) *VoterProto {
	prop.mtx.RLock()
	defer prop.mtx.RUnlock()

	return prop.v.Header.findVoter(addr)
}
