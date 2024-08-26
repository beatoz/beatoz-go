package proposal

import (
	"encoding/json"
	"github.com/beatoz/beatoz-go/types/xerrors"
)

type voteOption struct {
	option []byte
	votes  int64
}

func NewVoteOptions(opts ...[]byte) []*voteOption {
	var voteOpts []*voteOption
	for _, opt := range opts {
		voteOpts = append(voteOpts, &voteOption{
			option: opt,
		})
	}
	return voteOpts
}

func (opt *voteOption) Encode() ([]byte, xerrors.XError) {
	if bz, err := json.Marshal(opt); err != nil {
		return nil, xerrors.From(err)
	} else {
		return bz, nil
	}
}

func (opt *voteOption) Decode(bz []byte) xerrors.XError {
	if err := json.Unmarshal(bz, opt); err != nil {
		return xerrors.From(err)
	}
	return nil
}

func (opt *voteOption) DoVote(power int64) int64 {
	opt.votes += power
	return opt.votes
}

func (opt *voteOption) CancelVote(power int64) int64 {
	opt.votes -= power
	return opt.votes
}

func (opt *voteOption) Votes() int64 {
	return opt.votes
}

func (opt *voteOption) Option() []byte {
	return opt.option
}

func (opt *voteOption) MarshalJSON() ([]byte, error) {
	_tmp := &struct {
		Option []byte `json:"option"`
		Votes  int64  `json:"votes"`
	}{
		Option: opt.option,
		Votes:  opt.votes,
	}
	return json.Marshal(_tmp)
}

func (opt *voteOption) UnmarshalJSON(bz []byte) error {
	_tmp := &struct {
		Option []byte `json:"option"`
		Votes  int64  `json:"votes"`
	}{}
	if err := json.Unmarshal(bz, _tmp); err != nil {
		return err
	}
	opt.option = _tmp.Option
	opt.votes = _tmp.Votes
	return nil
}
