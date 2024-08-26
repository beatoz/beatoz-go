package stake

import (
	"encoding/json"
	"fmt"
	"github.com/beatoz/beatoz-go/ledger/v0"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	"google.golang.org/protobuf/proto"
	"sync"
)

type Reward struct {
	address   types.Address
	issued    *uint256.Int
	withdrawn *uint256.Int
	slashed   *uint256.Int
	cumulated *uint256.Int
	height    int64

	mtx sync.RWMutex
}

func NewReward(addr types.Address) *Reward {
	return &Reward{
		address:   addr,
		issued:    uint256.NewInt(0),
		withdrawn: uint256.NewInt(0),
		slashed:   uint256.NewInt(0),
		cumulated: uint256.NewInt(0),
		height:    0,
	}
}

func (rwd *Reward) Issue(r *uint256.Int, h int64) xerrors.XError {
	rwd.mtx.Lock()
	defer rwd.mtx.Unlock()

	if rwd.height < h {
		rwd.issued = new(uint256.Int).Set(r)
		rwd.height = h
	} else if rwd.height == h {
		_ = rwd.issued.Add(rwd.issued, r)
	} else {
		panic(fmt.Errorf("the Reward::height(%v) is not same current height(%v)", rwd.height, h))
	}

	_ = rwd.cumulated.Add(rwd.cumulated, r)

	return nil
}

func (rwd *Reward) Withdraw(r *uint256.Int, h int64) xerrors.XError {
	rwd.mtx.Lock()
	defer rwd.mtx.Unlock()
	if rwd.height < h {
		rwd.withdrawn = new(uint256.Int).Set(r)
		rwd.height = h
	} else if rwd.height == h {
		_ = rwd.withdrawn.Add(rwd.withdrawn, r)
	} else {
		panic(fmt.Errorf("the Reward::height(%v) is not same current height(%v)", rwd.height, h))
	}

	_ = rwd.cumulated.Sub(rwd.cumulated, r)

	return nil
}

func (rwd *Reward) Slash(r *uint256.Int, h int64) xerrors.XError {
	rwd.mtx.Lock()
	defer rwd.mtx.Unlock()

	if rwd.height < h {
		rwd.slashed = new(uint256.Int).Set(r)
		rwd.height = h
	} else if rwd.height == h {
		_ = rwd.slashed.Add(rwd.slashed, r)
	} else {
		panic(fmt.Errorf("the Reward::height(%v) is not same current height(%v)", rwd.height, h))
	}

	_ = rwd.cumulated.Sub(rwd.cumulated, r)

	return nil
}

func (rwd *Reward) Key() v0.LedgerKey {
	return v0.ToLedgerKey(rwd.address)
}

func (rwd *Reward) Encode() ([]byte, xerrors.XError) {
	rwd.mtx.RLock()
	defer rwd.mtx.RUnlock()

	m := &RewardProto{
		Address:    rwd.address,
		XIssued:    rwd.issued.Bytes(),
		XWithdrawn: rwd.withdrawn.Bytes(),
		XSlashed:   rwd.slashed.Bytes(),
		XCumulated: rwd.cumulated.Bytes(),
		Height:     rwd.height,
	}

	if bz, err := proto.Marshal(m); err != nil {
		return nil, xerrors.From(err)
	} else {
		return bz, nil
	}
}

func (rwd *Reward) Decode(d []byte) xerrors.XError {
	m := RewardProto{}
	if err := proto.Unmarshal(d, &m); err != nil {
		return xerrors.From(err)
	}

	rwd.mtx.Lock()
	defer rwd.mtx.Unlock()

	rwd.address = m.Address
	rwd.issued = new(uint256.Int).SetBytes(m.XIssued)
	rwd.withdrawn = new(uint256.Int).SetBytes(m.XWithdrawn)
	rwd.slashed = new(uint256.Int).SetBytes(m.XSlashed)
	rwd.cumulated = new(uint256.Int).SetBytes(m.XCumulated)
	rwd.height = m.Height
	return nil
}

func (rwd *Reward) MarshalJSON() ([]byte, error) {
	rwd.mtx.RLock()
	defer rwd.mtx.RUnlock()

	_tmp := &struct {
		Address   types.Address `json:"address,omitempty"`
		Issued    string        `json:"issued,omitempty"`
		Withdrawn string        `json:"withdrawn,omitempty"`
		Slashed   string        `json:"slashed,omitempty"`
		Cumulated string        `json:"cumulated,omitempty"`
		Height    int64         `json:"height,omitempty"`
	}{
		Address:   rwd.address,
		Issued:    rwd.issued.Dec(),
		Withdrawn: rwd.withdrawn.Dec(),
		Slashed:   rwd.slashed.Dec(),
		Cumulated: rwd.cumulated.Dec(),
		Height:    rwd.height,
	}
	return json.Marshal(_tmp)
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

	if err := json.Unmarshal(d, tmp); err != nil {
		return err
	}

	rwd.mtx.Lock()
	defer rwd.mtx.Unlock()

	rwd.address = tmp.Address
	rwd.issued = uint256.MustFromDecimal(tmp.Issued)
	rwd.withdrawn = uint256.MustFromDecimal(tmp.Withdrawn)
	rwd.slashed = uint256.MustFromDecimal(tmp.Slashed)
	rwd.cumulated = uint256.MustFromDecimal(tmp.Cumulated)
	rwd.height = tmp.Height
	return nil
}

func (rwd *Reward) Address() types.Address {
	rwd.mtx.RLock()
	defer rwd.mtx.RUnlock()

	return rwd.address
}

func (rwd *Reward) GetIssued() *uint256.Int {
	rwd.mtx.RLock()
	defer rwd.mtx.RUnlock()

	return new(uint256.Int).Set(rwd.issued)
}

func (rwd *Reward) GetWithdrawn() *uint256.Int {
	rwd.mtx.RLock()
	defer rwd.mtx.RUnlock()

	return new(uint256.Int).Set(rwd.withdrawn)
}

func (rwd *Reward) GetSlashed() *uint256.Int {
	rwd.mtx.RLock()
	defer rwd.mtx.RUnlock()

	return new(uint256.Int).Set(rwd.slashed)
}

func (rwd *Reward) GetCumulated() *uint256.Int {
	rwd.mtx.RLock()
	defer rwd.mtx.RUnlock()

	return new(uint256.Int).Set(rwd.cumulated)
}

func (rwd *Reward) Height() int64 {
	rwd.mtx.RLock()
	defer rwd.mtx.RUnlock()

	return rwd.height
}

func (rwd *Reward) String() string {
	rwd.mtx.RLock()
	defer rwd.mtx.RUnlock()

	bz, _ := json.MarshalIndent(rwd, "", "  ")
	return string(bz)
}

var _ v0.ILedgerItem = (*Reward)(nil)
