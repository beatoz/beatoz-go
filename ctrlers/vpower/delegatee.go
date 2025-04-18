package vpower

import (
	"fmt"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/crypto"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	"github.com/shopspring/decimal"
	"sort"
	"sync"
)

// Delegatee contains who delegated, how much voting power was delegated,
// and when it was delegated to `addr`.
type Delegatee struct {
	addr   types.Address
	pubKey bytes.HexBytes

	mapPowers  map[string]*VPowerProto
	totalPower int64

	// the following fields are used when calculating the weight of voting power.
	sumMaturePower int64
	sumOfBlocks    int64
	lastHeight     int64

	mtx sync.RWMutex
}

func NewDelegatee(pubKey bytes.HexBytes) *Delegatee {
	addr, _ := crypto.PubBytes2Addr(pubKey)
	ret := &Delegatee{
		addr:      addr,
		pubKey:    pubKey,
		mapPowers: make(map[string]*VPowerProto),
	}
	return ret
}

func LoadAllDelegateeProtos(dgteesLedger v1.IStateLedger[*DelegateeProto]) ([]*DelegateeProto, xerrors.XError) {
	var dgteeProtos []*DelegateeProto
	if xerr := dgteesLedger.Seek([]byte(prefixDelegateeProto), true, func(elem *DelegateeProto) xerrors.XError {
		dgteeProtos = append(dgteeProtos, elem)
		return nil
	}, true); xerr != nil {
		return nil, xerr
	}
	return dgteeProtos, nil
}

func LoadAllVPowerProtos(vpowLedger v1.IStateLedger[*VPowerProto], dgteeProtos []*DelegateeProto, currHeight, ripeningBlocks int64) (DelegateeArray, xerrors.XError) {
	var dgtees DelegateeArray

	for _, dgt := range dgteeProtos {
		dgtees = append(dgtees, NewDelegatee(dgt.PubKey))
	}

	xerr := vpowLedger.Seek([]byte(prefixVPowerProto), true, func(vpow *VPowerProto) xerrors.XError {
		stratHeight := vpow.PowerChunks[0].Height
		lastHeight := vpow.PowerChunks[len(vpow.PowerChunks)-1].Height
		if stratHeight <= 0 {
			return xerrors.NewOrdinary("the height when the voting power was bonded is less than 1")
		}
		if currHeight < lastHeight {
			return xerrors.NewOrdinary("the height when the voting power was bonded is greater than the current height")
		}

		dgtee := findByAddr(vpow.To, dgtees)
		if dgtee == nil {
			return xerrors.ErrNotFoundDelegatee
		}

		if _, ok := dgtee.mapPowers[types.Address(vpow.From).String()]; ok {
			return xerrors.From(fmt.Errorf("VPower[validaotr(%v):delegator(%v)] pair is duplicated", dgtee.addr, vpow.From))
		}
		dgtee.mapPowers[types.Address(vpow.From).String()] = vpow
		dgtee.totalPower += vpow.SumPower

		for _, c := range vpow.PowerChunks {
			dur := currHeight - c.Height
			if dur >= ripeningBlocks {
				dgtee.sumMaturePower += c.Power
			}
			dgtee.sumOfBlocks += dur
		}

		return nil
	}, true)

	return dgtees, xerr
}

// AddPower increases the power of `from` by `pow`.
// It means that the `from` delegates the `pow` to `dgtee`.
// NOTE: After calling AddPower, the `dgtee.maturePower` MUST be recomputed.
func (dgtee *Delegatee) AddPower(from types.Address, pow, height int64) *VPowerProto {
	dgtee.mtx.Lock()
	defer dgtee.mtx.Unlock()

	mapKey := from.String()
	vpow, ok := dgtee.mapPowers[mapKey]
	if ok {
		vpow.addPowerChunk(pow, height)
	} else {
		vpow = newVPower(from, dgtee.addr, pow, height)
		dgtee.mapPowers[mapKey] = vpow
	}

	dgtee.totalPower += pow

	return vpow
}

func (dgtee *Delegatee) AddPowerWithTxHash(from types.Address, pow, height int64, txhash []byte) *VPowerProto {
	dgtee.mtx.Lock()
	defer dgtee.mtx.Unlock()

	mapKey := from.String()
	vpow, ok := dgtee.mapPowers[mapKey]
	if ok {
		vpow.addPowerWithTxHash(pow, height, txhash)
	} else {
		vpow = newVPowerWithTxHash(from, dgtee.addr, pow, height, txhash)
		dgtee.mapPowers[mapKey] = vpow
	}
	dgtee.totalPower += pow

	return vpow
}

// DelPower decreases the power of `from` by `pow` and
// returns `*VPowerProto` that has the removed power information.
// NOTE: After calling DelPower, the dgtee.maturePower` MUST be recomputed.
func (dgtee *Delegatee) DelPower(from types.Address, pow int64) (*VPowerProto, *VPowerProto) {
	dgtee.mtx.Lock()
	defer dgtee.mtx.Unlock()

	mapKey := from.String()
	vpow, ok := dgtee.mapPowers[mapKey]
	if !ok {
		return nil, nil
	}

	removed := &VPowerProto{
		From: vpow.From,
		To:   vpow.To,
	}
	for i := len(vpow.PowerChunks) - 1; i >= 0; i-- {
		pc := vpow.PowerChunks[i]

		if pc.Power < pow {
			vpow.delPowerChunk(i) // remove pc
			dgtee.totalPower -= pc.Power
			removed.addPowerChunk(pc.Power, pc.Height)
		} else {
			// pc.Power >= pow
			pc.Power -= pow
			if pc.Power == 0 {
				vpow.delPowerChunk(i)
			}
			dgtee.totalPower -= pow
			removed.addPowerChunk(pow, pc.Height)
			break
		}

		pow -= pc.Power
	}

	if len(vpow.PowerChunks) == 0 {
		delete(dgtee.mapPowers, mapKey)
	}
	return removed, vpow
}

func (dgtee *Delegatee) DelPowerWithTxHash(from types.Address, txhash []byte) (*VPowerProto, *VPowerProto) {
	dgtee.mtx.Lock()
	defer dgtee.mtx.Unlock()

	mapKey := from.String()
	vpow, ok := dgtee.mapPowers[mapKey]
	if !ok {
		return nil, nil
	}

	removed := &VPowerProto{
		From: vpow.From,
		To:   vpow.To,
	}
	for i, pc := range vpow.PowerChunks {
		if bytes.Equal(txhash, pc.TxHash) {
			vpow.delPowerChunk(i)
			dgtee.totalPower -= pc.Power
			removed.addPowerChunk(pc.Power, pc.Height)
			break
		}
	}

	if len(vpow.PowerChunks) == 0 {
		delete(dgtee.mapPowers, mapKey)
	}
	return removed, vpow
}

func (dgtee *Delegatee) FindPowerChunk(from types.Address, txhash bytes.HexBytes) *PowerChunk {
	dgtee.mtx.RLock()
	defer dgtee.mtx.RUnlock()

	mapKey := from.String()
	if vpow, ok := dgtee.mapPowers[mapKey]; !ok {
		return nil
	} else {
		return vpow.findPowerChunk(txhash)
	}
}

func (dgtee *Delegatee) TotalPower() int64 {
	dgtee.mtx.RLock()
	defer dgtee.mtx.RUnlock()

	return dgtee.totalPower
}

func (dgtee *Delegatee) TotalPowerOf(from types.Address) int64 {
	dgtee.mtx.RLock()
	defer dgtee.mtx.RUnlock()

	if vpow, ok := dgtee.mapPowers[from.String()]; ok {
		return vpow.SumPower
	}
	return 0
}

func (dgtee *Delegatee) SelfPower() int64 {
	dgtee.mtx.RLock()
	defer dgtee.mtx.RUnlock()

	return dgtee.TotalPowerOf(dgtee.addr)
}

func (dgtee *Delegatee) MaturePower() int64 {
	dgtee.mtx.RLock()
	defer dgtee.mtx.RUnlock()

	return dgtee.sumMaturePower
}

func (dgtee *Delegatee) RisingPower() int64 {
	dgtee.mtx.RLock()
	defer dgtee.mtx.RUnlock()

	return dgtee.totalPower - dgtee.sumMaturePower
}

func (dgtee *Delegatee) ExpectedSelfStakeRatio(added int64) int64 {
	dgtee.mtx.RLock()
	defer dgtee.mtx.RUnlock()

	return (dgtee.SelfPower() * int64(100)) / (dgtee.totalPower + added)
}

func (dgtee *Delegatee) Compute(height, ripeningCycle int64, totalSupply *uint256.Int, tauPermil int) decimal.Decimal {
	dgtee.mtx.Lock()
	defer dgtee.mtx.Unlock()

	_totolSupply := decimal.NewFromBigInt(totalSupply.ToBig(), 0)
	_risingWeight := decimal.Zero
	dgtee.sumMaturePower = 0
	dgtee.sumOfBlocks = 0

	for _, vpow := range dgtee.mapPowers {
		for _, pc := range vpow.PowerChunks {
			dur := height - pc.Height

			if dur >= ripeningCycle {
				dgtee.sumMaturePower += pc.Power
				dgtee.sumOfBlocks += dur
			} else if dur > 0 {
				pow := pc.Power
				wt := Wi(pow, dur, ripeningCycle, _totolSupply, tauPermil)
				_risingWeight = _risingWeight.Add(wt)
				dgtee.sumOfBlocks += dur
			} else {
				// todo: handle the case where dur <= 0
				// continue ???
			}
		}
	}
	dgtee.lastHeight = height
	return Wi(dgtee.sumMaturePower, ripeningCycle, ripeningCycle, _totolSupply, tauPermil).Add(_risingWeight)
}

func (dgtee *Delegatee) ComputeEx(height, ripeningCycle int64, totalSupply *uint256.Int, tauPermil int) decimal.Decimal {
	dgtee.mtx.Lock()
	defer dgtee.mtx.Unlock()

	dgtee.sumMaturePower = 0
	dgtee.sumOfBlocks = 0
	dgtee.lastHeight = height

	var powChunks []*PowerChunk
	for _, vpow := range dgtee.mapPowers {
		powChunks = append(powChunks, vpow.PowerChunks...)
	}

	_totolSupply := decimal.NewFromBigInt(totalSupply.ToBig(), 0)
	return WaWeightedEx(powChunks, height, ripeningCycle, _totolSupply, tauPermil)
}

func (dgtee *Delegatee) Clone() *Delegatee {
	dgtee.mtx.RLock()
	defer dgtee.mtx.RUnlock()

	mapPowersClone := make(map[string]*VPowerProto)
	for k, vpow := range dgtee.mapPowers {
		mapPowersClone[k] = vpow.Clone()
	}

	return &Delegatee{
		addr:           bytes.Copy(dgtee.addr),
		pubKey:         bytes.Copy(dgtee.pubKey),
		mapPowers:      mapPowersClone,
		totalPower:     dgtee.totalPower,
		sumMaturePower: dgtee.sumMaturePower,
		sumOfBlocks:    dgtee.sumOfBlocks,
		lastHeight:     dgtee.lastHeight,
	}
}

func (dgtee *Delegatee) GetDelegateeProto() *DelegateeProto {
	dgtee.mtx.RLock()
	defer dgtee.mtx.RUnlock()

	proto := newDelegateeProto(dgtee.pubKey)
	for _, v := range dgtee.mapPowers {
		proto.AddPower(v.From, v.SumPower)
	}
	return proto
}

type DelegateeArray []*Delegatee

func copyDelegateeArray(src DelegateeArray) DelegateeArray {
	dst := make(DelegateeArray, len(src))
	for i, d := range src {
		dst[i] = d.Clone()
	}
	return dst
}
func findByAddr(addr types.Address, dgtees []*Delegatee) *Delegatee {
	for _, v := range dgtees {
		if bytes.Equal(v.addr, addr) {
			return v
		}
	}
	return nil
}

func findByPubKey(pubKey bytes.HexBytes, dgtees []*Delegatee) *Delegatee {
	for _, v := range dgtees {
		if bytes.Equal(v.pubKey, pubKey) {
			return v
		}
	}
	return nil
}

type orderByPowerDelegatees []*DelegateeProto

func (dgtees orderByPowerDelegatees) Len() int {
	return len(dgtees)
}

// descending order by TotalPower
func (dgtees orderByPowerDelegatees) Less(i, j int) bool {
	if dgtees[i].TotalPower != dgtees[j].TotalPower {
		return dgtees[i].TotalPower > dgtees[j].TotalPower
	}
	if dgtees[i].SelfPower != dgtees[j].SelfPower {
		return dgtees[i].SelfPower > dgtees[j].SelfPower
	}
	if dgtees[i].MaturePower != dgtees[j].MaturePower {
		return dgtees[i].MaturePower > dgtees[j].MaturePower
	}
	if bytes.Compare(dgtees[i].PubKey, dgtees[j].PubKey) > 0 {
		return true
	}
	return false
}

func (dgtees orderByPowerDelegatees) Swap(i, j int) {
	dgtees[i], dgtees[j] = dgtees[j], dgtees[i]
}

var _ sort.Interface = (orderByPowerDelegatees)(nil)

type orderByAddrDelegatees []*DelegateeProto

func (dgtees orderByAddrDelegatees) Len() int {
	return len(dgtees)
}

// ascending order by address
func (dgtees orderByAddrDelegatees) Less(i, j int) bool {
	return bytes.Compare(dgtees[i].PubKey, dgtees[j].PubKey) < 0
}

func (dgtees orderByAddrDelegatees) Swap(i, j int) {
	dgtees[i], dgtees[j] = dgtees[j], dgtees[i]
}

var _ sort.Interface = (orderByAddrDelegatees)(nil)
