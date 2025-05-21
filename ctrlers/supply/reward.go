package supply

import (
	"fmt"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/libs/jsonx"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	"google.golang.org/protobuf/proto"
)

type Reward struct {
	_proto    RewardProto
	issued    *uint256.Int
	slashed   *uint256.Int
	withdrawn *uint256.Int
	cumulated *uint256.Int
}

func NewReward(addr types.Address) *Reward {
	return &Reward{
		_proto:    RewardProto{Address: addr, Height: 0},
		issued:    uint256.NewInt(0),
		withdrawn: uint256.NewInt(0),
		slashed:   uint256.NewInt(0),
		cumulated: uint256.NewInt(0),
	}
}

func (rwd *Reward) Encode() ([]byte, xerrors.XError) {
	rwd.toProto()
	bz, err := proto.Marshal(&rwd._proto)
	if err != nil {
		return nil, xerrors.From(err)
	}
	return bz, nil
}

func (rwd *Reward) Decode(k, v []byte) xerrors.XError {
	if err := proto.Unmarshal(v, &rwd._proto); err != nil {
		return xerrors.From(err)
	}
	rwd.fromProto()
	return nil
}

func (rwd *Reward) fromProto() {
	rwd.issued = new(uint256.Int).SetBytes(rwd._proto.XIssued)
	rwd.slashed = new(uint256.Int).SetBytes(rwd._proto.XSlashed)
	rwd.withdrawn = new(uint256.Int).SetBytes(rwd._proto.XWithdrawn)
	rwd.cumulated = new(uint256.Int).SetBytes(rwd._proto.XCumulated)
}

func (rwd *Reward) toProto() {
	rwd._proto.XIssued = rwd.issued.Bytes()
	rwd._proto.XSlashed = rwd.slashed.Bytes()
	rwd._proto.XWithdrawn = rwd.withdrawn.Bytes()
	rwd._proto.XCumulated = rwd.cumulated.Bytes()
}

var _ v1.ILedgerItem = (*Reward)(nil)

func (rwd *Reward) MarshalJSON() ([]byte, error) {
	_tmp := &struct {
		Address   types.Address `json:"address,omitempty"`
		Issued    string        `json:"issued,omitempty"`
		Withdrawn string        `json:"withdrawn,omitempty"`
		Slashed   string        `json:"slashed,omitempty"`
		Cumulated string        `json:"cumulated,omitempty"`
		Height    int64         `json:"height,omitempty"`
	}{
		Address:   rwd._proto.Address,
		Issued:    rwd.issued.Dec(),
		Withdrawn: rwd.withdrawn.Dec(),
		Slashed:   rwd.slashed.Dec(),
		Cumulated: rwd.cumulated.Dec(),
		Height:    rwd._proto.Height,
	}
	return jsonx.Marshal(_tmp)
}

func (rwd *Reward) UnmarshalJSON(d []byte) error {
	tmp := &struct {
		Address   types.Address `json:"address,omitempty"`
		Issued    string        `json:"issued,omitempty"`
		Withdrawn string        `json:"withdrawn,omitempty"`
		Slashed   string        `json:"slashed,omitempty"`
		Cumulated string        `json:"cumulated,omitempty"`
		Height    int64         `json:"height,omitempty"`
	}{}

	if err := jsonx.Unmarshal(d, tmp); err != nil {
		return err
	}

	rwd._proto.Address = tmp.Address
	rwd.issued = uint256.MustFromDecimal(tmp.Issued)
	rwd.withdrawn = uint256.MustFromDecimal(tmp.Withdrawn)
	rwd.slashed = uint256.MustFromDecimal(tmp.Slashed)
	rwd.cumulated = uint256.MustFromDecimal(tmp.Cumulated)
	rwd._proto.Height = tmp.Height
	return nil
}

func (rwd *Reward) Issue(r *uint256.Int, h int64) xerrors.XError {
	if rwd._proto.Height < h {
		rwd.issued = new(uint256.Int).Set(r)
		rwd._proto.Height = h
	} else if rwd._proto.Height == h {
		_ = rwd.issued.Add(rwd.issued, r)
	} else {
		panic(fmt.Errorf("the Reward::height(%v) is not same current height(%v)", rwd._proto.Height, h))
	}

	_ = rwd.cumulated.Add(rwd.cumulated, r)

	return nil
}

func (rwd *Reward) Withdraw(r *uint256.Int, h int64) xerrors.XError {

	if rwd._proto.Height < h {
		rwd.withdrawn = new(uint256.Int).Set(r)
		rwd._proto.Height = h
	} else if rwd._proto.Height == h {
		_ = rwd.withdrawn.Add(rwd.withdrawn, r)
	} else {
		panic(fmt.Errorf("the Reward::height(%v) is not same current height(%v)", rwd._proto.Height, h))
	}

	_ = rwd.cumulated.Sub(rwd.cumulated, r)

	return nil
}

func (rwd *Reward) Slash(r *uint256.Int, h int64) xerrors.XError {
	if rwd._proto.Height < h {
		rwd.slashed = new(uint256.Int).Set(r)
		rwd._proto.Height = h
	} else if rwd._proto.Height == h {
		_ = rwd.slashed.Add(rwd.slashed, r)
	} else {
		panic(fmt.Errorf("the Reward::height(%v) is not same current height(%v)", rwd._proto.Height, h))
	}

	_ = rwd.cumulated.Sub(rwd.cumulated, r)

	return nil
}

func (rwd *Reward) String() string {
	bz, _ := jsonx.MarshalIndent(rwd, "", "  ")
	return string(bz)
}

func (rwd *Reward) MintedAmount() *uint256.Int {
	return rwd.issued.Clone()
}

func (rwd *Reward) SlashedAmount() *uint256.Int {
	return rwd.slashed.Clone()
}
func (rwd *Reward) WithdrawnAmount() *uint256.Int {
	return rwd.withdrawn.Clone()
}
func (rwd *Reward) CumulatedAmount() *uint256.Int {
	return rwd.cumulated.Clone()
}
func (rwd *Reward) Address() types.Address {
	return rwd._proto.Address
}
func (rwd *Reward) Height() int64 {
	return rwd._proto.Height
}
