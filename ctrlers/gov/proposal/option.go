package proposal

import "sort"

func NewVoteOptions(options ...[]byte) []*VoteOptionProto {
	var voteOpts []*VoteOptionProto
	for _, opt := range options {
		voteOpts = append(
			voteOpts,
			&VoteOptionProto{
				Option: opt,
			})
	}
	return voteOpts
}

func (opt *VoteOptionProto) DoVote(power int64) int64 {
	opt.Votes += power
	return opt.Votes
}

func (opt *VoteOptionProto) CancelVote(power int64) int64 {
	opt.Votes -= power
	return opt.Votes
}

type powerOrderVoteOptions []*VoteOptionProto

func (opts powerOrderVoteOptions) Len() int {
	return len(opts)
}

func (opts powerOrderVoteOptions) Less(i, j int) bool {
	return opts[i].Votes > opts[j].Votes
}

func (opts powerOrderVoteOptions) Swap(i, j int) {
	opts[i], opts[j] = opts[j], opts[i]
}

var _ sort.Interface = (powerOrderVoteOptions)(nil)
