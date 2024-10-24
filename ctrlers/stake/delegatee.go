package stake

import (
	"bytes"
	"encoding/json"
	"fmt"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types"
	bytes2 "github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"sort"
	"sync"
)

type Delegatee struct {
	Addr   types.Address   `json:"address"`
	PubKey bytes2.HexBytes `json:"pubKey"`

	SelfPower    int64 `json:"selfPower,string"`
	TotalPower   int64 `json:"totalPower,string"`
	SlashedPower int64 `json:"slashedPower,string"`

	Stakes []*Stake `json:"stakes"`

	NotSignedHeights *BlockMarker

	mtx sync.RWMutex
}

func (delegatee *Delegatee) Key() v1.LedgerKey {
	delegatee.mtx.RLock()
	defer delegatee.mtx.RUnlock()

	return delegatee.Addr
}

func (delegatee *Delegatee) Encode() ([]byte, xerrors.XError) {
	delegatee.mtx.RLock()
	defer delegatee.mtx.RUnlock()

	if bz, err := json.Marshal(delegatee); err != nil {
		return nil, xerrors.From(err)
	} else {
		return bz, nil
	}
}

func (delegatee *Delegatee) Decode(d []byte) xerrors.XError {
	delegatee.mtx.Lock()
	defer delegatee.mtx.Unlock()

	if err := json.Unmarshal(d, delegatee); err != nil {
		return xerrors.From(err)
	}
	return nil
}

var _ v1.ILedgerItem = (*Delegatee)(nil)

func NewDelegatee(addr types.Address, pubKey bytes2.HexBytes) *Delegatee {
	return &Delegatee{
		Addr:             addr,
		PubKey:           pubKey,
		SelfPower:        0,
		TotalPower:       0,
		NotSignedHeights: &BlockMarker{},
	}
}

func (delegatee *Delegatee) GetAddress() types.Address {
	delegatee.mtx.RLock()
	defer delegatee.mtx.RUnlock()

	return delegatee.Addr
}

func (delegatee *Delegatee) AddStake(stakes ...*Stake) xerrors.XError {
	delegatee.mtx.Lock()
	defer delegatee.mtx.Unlock()

	return delegatee.addStake(stakes...)
}

func (delegatee *Delegatee) addStake(stakes ...*Stake) xerrors.XError {
	delegatee.Stakes = append(delegatee.Stakes, stakes...)

	for _, s := range stakes {
		if s.IsSelfStake() {
			delegatee.SelfPower += s.Power
		}
		delegatee.TotalPower += s.Power
	}
	return nil
}

func (delegatee *Delegatee) DelStake(txhash bytes2.HexBytes) *Stake {
	delegatee.mtx.Lock()
	defer delegatee.mtx.Unlock()

	if s := delegatee.delStakeByHash(txhash); s != nil {
		if s.IsSelfStake() {
			delegatee.SelfPower -= s.Power
		}
		delegatee.TotalPower -= s.Power
		return s
	}
	return nil
}

func (delegatee *Delegatee) DelStakeByIdx(idx int) *Stake {
	delegatee.mtx.Lock()
	defer delegatee.mtx.Unlock()

	if s := delegatee.delStakeByIdx(idx); s != nil {
		if s.IsSelfStake() {
			delegatee.SelfPower -= s.Power
		}
		delegatee.TotalPower -= s.Power
		return s
	}
	return nil
}

func (delegatee *Delegatee) delStakeByHash(txhash bytes2.HexBytes) *Stake {
	i, s0 := delegatee.findStake(txhash)
	if i < 0 || s0 == nil {
		return nil
	}

	return delegatee.delStakeByIdx(i)
}

func (delegatee *Delegatee) delStakeByIdx(idx int) *Stake {
	if idx >= len(delegatee.Stakes) {
		return nil
	} else {
		s := delegatee.Stakes[idx]
		delegatee.Stakes = append(delegatee.Stakes[:idx], delegatee.Stakes[idx+1:]...)
		return s
	}
}

func (delegatee *Delegatee) DelAllStakes() []*Stake {
	delegatee.mtx.Lock()
	defer delegatee.mtx.Unlock()

	stakes := delegatee.Stakes
	delegatee.Stakes = nil

	for _, s := range stakes {
		delegatee.TotalPower -= s.Power
	}

	return stakes
}

func (delegatee *Delegatee) GetAllStakes() []*Stake {
	delegatee.mtx.RLock()
	defer delegatee.mtx.RUnlock()

	ret := make([]*Stake, len(delegatee.Stakes))
	copy(ret, delegatee.Stakes)
	return ret
}

func (delegatee *Delegatee) GetStake(idx int) *Stake {
	delegatee.mtx.RLock()
	defer delegatee.mtx.RUnlock()

	return delegatee.getStake(idx)
}

func (delegatee *Delegatee) getStake(idx int) *Stake {
	if idx >= len(delegatee.Stakes) {
		return nil
	}
	return delegatee.Stakes[idx]
}

func (delegatee *Delegatee) FindStake(txhash bytes2.HexBytes) (int, *Stake) {
	delegatee.mtx.RLock()
	defer delegatee.mtx.RUnlock()

	return delegatee.findStake(txhash)
}

func (delegatee *Delegatee) findStake(txhash bytes2.HexBytes) (int, *Stake) {
	for i, s := range delegatee.Stakes {
		if bytes.Compare(txhash, s.TxHash) == 0 {
			return i, s
		}
	}
	return -1, nil
}

func (delegatee *Delegatee) StakesOf(addr types.Address) []*Stake {
	delegatee.mtx.RLock()
	defer delegatee.mtx.RUnlock()

	var ret []*Stake
	for _, s := range delegatee.Stakes {
		if bytes.Compare(addr, s.From) == 0 {
			ret = append(ret, s)
		}
	}
	return ret
}

func (delegatee *Delegatee) StakesLen() int {
	delegatee.mtx.RLock()
	defer delegatee.mtx.RUnlock()

	return len(delegatee.Stakes)
}

func (delegatee *Delegatee) GetSelfPower() int64 {
	delegatee.mtx.RLock()
	defer delegatee.mtx.RUnlock()

	return delegatee.SelfPower
}

func (delegatee *Delegatee) GetTotalPower() int64 {
	delegatee.mtx.RLock()
	defer delegatee.mtx.RUnlock()

	return delegatee.TotalPower
}

func (delegatee *Delegatee) SelfStakeRatio(added int64) int64 {
	delegatee.mtx.RLock()
	defer delegatee.mtx.RUnlock()

	return (delegatee.SelfPower * int64(100)) / (delegatee.TotalPower + added)
}

func (delegatee *Delegatee) ProcessNotSignedBlock(height int64) xerrors.XError {
	return delegatee.NotSignedHeights.Mark(height)
}

// GetNotSignedBlockCount() returns the number of marked height in [h0, h1].
func (delegatee *Delegatee) GetNotSignedBlockCount(h0, h1 int64) int {
	return delegatee.NotSignedHeights.CountInWindow(h0, h1, true)
}

func (delegatee *Delegatee) DoSlash(ratio int64) int64 {
	delegatee.mtx.Lock()
	defer delegatee.mtx.Unlock()

	// to slash delegators too. issue #49
	return delegatee.doSlashAll(ratio)
}

func (delegatee *Delegatee) doSlashAll(ratio int64) int64 {
	sumSlashedPower := int64(0)

	var removingStakes []*Stake
	for _, s0 := range delegatee.Stakes {
		slashedPower := (s0.Power * ratio) / int64(100)
		if slashedPower < 1 {
			removingStakes = append(removingStakes, s0)
			slashedPower = s0.Power
			continue
		}

		s0.Power -= slashedPower
		sumSlashedPower += slashedPower
	}

	if removingStakes != nil {
		for _, s1 := range removingStakes {
			_ = delegatee.delStakeByHash(s1.TxHash)
		}
	}

	delegatee.SelfPower = delegatee.sumPowerOf(delegatee.Addr)
	delegatee.TotalPower = delegatee.sumPowerOf(nil)

	return sumSlashedPower
}

func (delegatee *Delegatee) String() string {
	bz, err := json.MarshalIndent(delegatee, "", "  ")
	if err != nil {
		return fmt.Sprintf("{error: %v}", err)
	}
	return string(bz)
}

func (delegatee *Delegatee) SumPower() int64 {
	delegatee.mtx.RLock()
	defer delegatee.mtx.RUnlock()

	return delegatee.sumPowerOf(nil)
}

func (delegatee *Delegatee) SumPowerOf(addr types.Address) int64 {
	delegatee.mtx.RLock()
	defer delegatee.mtx.RUnlock()

	return delegatee.sumPowerOf(addr)
}

func (delegatee *Delegatee) sumPowerOf(addr types.Address) int64 {
	power := int64(0)
	for _, s := range delegatee.Stakes {
		if addr == nil || bytes.Compare(addr, s.From) == 0 {
			power += s.Power
		}
	}
	return power
}

//
// DelegateeArray

type DelegateeArray []*Delegatee

func (delgs DelegateeArray) SumTotalPower() int64 {
	power := int64(0)
	for _, d := range delgs {
		power += d.TotalPower
	}
	return power
}

func (delgs DelegateeArray) Find(addr types.Address) (int, *Delegatee) {
	for i, d := range delgs {
		if bytes.Compare(addr, d.Addr) == 0 {
			return i, d
		}
	}
	return -1, nil
}

type PowerOrderDelegatees []*Delegatee

func (vs PowerOrderDelegatees) Len() int {
	return len(vs)
}

// descending order by TotalPower
func (vs PowerOrderDelegatees) Less(i, j int) bool {
	if vs[i].TotalPower != vs[j].TotalPower {
		return vs[i].TotalPower > vs[j].TotalPower
	}
	if len(vs[i].Stakes) != len(vs[j].Stakes) {
		return len(vs[i].Stakes) > len(vs[j].Stakes)
	}
	if bytes.Compare(vs[i].Addr, vs[j].Addr) > 0 {
		return true
	}
	return false
}

func (vs PowerOrderDelegatees) Swap(i, j int) {
	vs[i], vs[j] = vs[j], vs[i]
}

var _ sort.Interface = (PowerOrderDelegatees)(nil)

type AddressOrderDelegatees []*Delegatee

func (vs AddressOrderDelegatees) Len() int {
	return len(vs)
}

// ascending order by address
func (vs AddressOrderDelegatees) Less(i, j int) bool {
	return bytes.Compare(vs[i].Addr, vs[j].Addr) < 0
}

func (vs AddressOrderDelegatees) Swap(i, j int) {
	vs[i], vs[j] = vs[j], vs[i]
}

var _ sort.Interface = (AddressOrderDelegatees)(nil)
