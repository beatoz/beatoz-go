package proposal

import (
	"encoding/json"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	"sort"
	"sync"
)

type GovProposal struct {
	GovProposalHeader `json:"header"`
	Options           []*voteOption `json:"options"`
	MajorOption       *voteOption   `json:"majorOption"`

	mtx sync.RWMutex
}

func NewGovProposal(txhash bytes.HexBytes, optType int32, startHeight, votingBlocks, totalVotingPower, applyingHeight int64, voters map[string]*Voter, options ...[]byte) (*GovProposal, xerrors.XError) {
	endVotingHeight := startHeight + votingBlocks
	return &GovProposal{
		GovProposalHeader: GovProposalHeader{
			TxHash:            txhash,
			StartVotingHeight: startHeight,
			EndVotingHeight:   endVotingHeight,
			ApplyingHeight:    applyingHeight,
			TotalVotingPower:  totalVotingPower,
			MajorityPower:     (totalVotingPower * 2) / 3,
			Voters:            voters,
			OptType:           optType,
		},
		Options:     NewVoteOptions(options...),
		MajorOption: nil,
	}, nil
}

func (prop *GovProposal) Key() v1.LedgerKey {
	prop.mtx.RLock()
	defer prop.mtx.RUnlock()

	_key := make([]byte, len(prop.TxHash))
	copy(_key, prop.TxHash)
	return _key
}

func (prop *GovProposal) Encode() ([]byte, xerrors.XError) {
	prop.mtx.RLock()
	defer prop.mtx.RUnlock()

	if bz, err := json.Marshal(prop); err != nil {
		return bz, xerrors.From(err)
	} else {
		return bz, nil
	}
}

func (prop *GovProposal) Decode(bz []byte) xerrors.XError {
	prop.mtx.Lock()
	defer prop.mtx.Unlock()

	if err := json.Unmarshal(bz, prop); err != nil {
		return xerrors.From(err)
	}
	return nil
}

var _ v1.ILedgerItem = (*GovProposal)(nil)

func (prop *GovProposal) DoVote(addr types.Address, choice int32) xerrors.XError {
	prop.mtx.Lock()
	defer prop.mtx.Unlock()

	// cancel previous vote
	voter := prop.Voters[addr.String()]
	if voter == nil {
		return xerrors.NewOrdinary("not found voter")
	}

	if voter.Choice == choice {
		// same option is already selected.
		return nil
	}

	prop.cancelVote(voter)
	prop.doVote(voter, choice)

	return nil
}

func (prop *GovProposal) cancelVote(voter *Voter) {
	if voter.Choice >= 0 {
		opt := prop.Options[voter.Choice]
		opt.CancelVote(voter.Power)
		voter.Choice = -1
	}
}

func (prop *GovProposal) doVote(voter *Voter, choice int32) {
	if choice >= 0 {
		opt := prop.Options[choice]
		if opt == nil {
			return //xerrors.NewOrdinary("not found option")
		}

		opt.DoVote(voter.Power)
		voter.Choice = choice
	}
}

func (prop *GovProposal) DoPunish(addr types.Address, ratio int64) (int64, xerrors.XError) {
	prop.mtx.Lock()
	defer prop.mtx.Unlock()

	voter, ok := prop.Voters[addr.String()]
	if !ok {
		return 0, xerrors.ErrNotFoundVoter
	}

	choice := voter.Choice
	if choice >= 0 {
		// if voter already finishes selection, cancel it.
		prop.cancelVote(voter)
	}

	_p0 := uint256.NewInt(uint64(voter.Power))
	_ = _p0.Mul(_p0, uint256.NewInt(uint64(ratio)))
	_ = _p0.Div(_p0, uint256.NewInt(uint64(100)))
	slashingPower := int64(_p0.Uint64())

	voter.Power -= slashingPower

	if voter.Power <= 0 {
		delete(prop.Voters, addr.String())
	} else if choice >= 0 {
		// vote again with slashed power
		prop.doVote(voter, choice)
	}
	prop.TotalVotingPower -= slashingPower
	prop.MajorityPower = (prop.TotalVotingPower * 2) / 3

	return slashingPower, nil
}

func (prop *GovProposal) UpdateMajorOption() *voteOption {
	prop.mtx.Lock()
	defer prop.mtx.Unlock()

	return prop.updateMajorOption()
}

func (prop *GovProposal) updateMajorOption() *voteOption {
	sort.Sort(powerOrderVoteOptions(prop.Options))
	if prop.Options[0].Votes() >= prop.MajorityPower {
		prop.MajorOption = prop.Options[0]
	}
	return prop.MajorOption
}

func (prop *GovProposal) isMajor(opt *voteOption) bool {
	return opt.Votes() >= prop.MajorityPower
}

func (prop *GovProposal) IsVoter(addr types.Address) bool {
	_, ok := prop.Voters[addr.String()]
	return ok
}

type powerOrderVoteOptions []*voteOption

func (opts powerOrderVoteOptions) Len() int {
	return len(opts)
}

func (opts powerOrderVoteOptions) Less(i, j int) bool {
	return opts[i].votes > opts[j].votes
}

func (opts powerOrderVoteOptions) Swap(i, j int) {
	opts[i], opts[j] = opts[j], opts[i]
}

var _ sort.Interface = (powerOrderVoteOptions)(nil)
