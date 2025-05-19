package stake

import (
	"bytes"
	"encoding/json"
	"fmt"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types"
	abytes "github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	"sort"
	"sync"
)

// DEPRECATED
type Stake struct {
	From types.Address `json:"owner"`
	To   types.Address `json:"to"`

	TxHash       abytes.HexBytes `json:"txhash"`
	StartHeight  int64           `json:"startHeight,string"`
	RefundHeight int64           `json:"refundHeight,string"`

	Power int64 `json:"power,string"`

	mtx sync.RWMutex
}

var _ v1.ILedgerItem = (*Stake)(nil)

func (s *Stake) Key() v1.LedgerKey {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	return s.TxHash
}

func (s *Stake) Encode() ([]byte, xerrors.XError) {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	if bz, err := json.Marshal(s); err != nil {
		return nil, xerrors.From(err)
	} else {
		return bz, nil
	}
}

func (s *Stake) Decode(k, v []byte) xerrors.XError {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	if err := json.Unmarshal(v, s); err != nil {
		return xerrors.From(err)
	}
	return nil
}

func NewStakeWithAmount(owner, to types.Address, amt *uint256.Int, startHeight int64, txhash abytes.HexBytes) (*Stake, xerrors.XError) {
	power, xerr := ctrlertypes.AmountToPower(amt)
	if xerr != nil {
		return nil, xerr
	}
	return NewStakeWithPower(owner, to, power, startHeight, txhash), nil
}

func NewStakeWithPower(owner, to types.Address, power int64, startHeight int64, txhash abytes.HexBytes) *Stake {
	return &Stake{
		From:         owner,
		To:           to,
		TxHash:       txhash,
		StartHeight:  startHeight,
		RefundHeight: 0,
		Power:        power,
	}
}

func (s *Stake) Equal(o *Stake) bool {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	return bytes.Compare(s.From, o.From) == 0 &&
		bytes.Compare(s.To, o.To) == 0 &&
		bytes.Compare(s.TxHash, o.TxHash) == 0 &&
		s.StartHeight == o.StartHeight &&
		s.Power == o.Power
}

func (s *Stake) Clone() *Stake {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	return &Stake{
		From:         append(s.From, nil...),
		To:           append(s.To, nil...),
		TxHash:       append(s.TxHash, nil...),
		StartHeight:  s.StartHeight,
		RefundHeight: s.RefundHeight,
		Power:        s.Power,
	}
}

func (s *Stake) IsSelfStake() bool {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	return bytes.Compare(s.From, s.To) == 0
}

func (s *Stake) String() string {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	bz, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Sprintf("{error: %v}", err)
	}
	return string(bz)
}

type startHeightOrder []*Stake

func (slst startHeightOrder) Len() int {
	return len(slst)
}

// ascending order
func (slst startHeightOrder) Less(i, j int) bool {
	return slst[i].StartHeight < slst[j].StartHeight
}

func (slst startHeightOrder) Swap(i, j int) {
	slst[i], slst[j] = slst[j], slst[i]
}

var _ sort.Interface = (startHeightOrder)(nil)

type refundHeightOrder []*Stake

func (slst refundHeightOrder) Len() int {
	return len(slst)
}

// ascending order
func (slst refundHeightOrder) Less(i, j int) bool {
	return slst[i].RefundHeight < slst[j].RefundHeight
}

func (slst refundHeightOrder) Swap(i, j int) {
	slst[i], slst[j] = slst[j], slst[i]
}

var _ sort.Interface = (refundHeightOrder)(nil)
