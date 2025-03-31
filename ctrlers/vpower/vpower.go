package vpower

import (
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	"github.com/shopspring/decimal"
	"sync"
)

var (
	powerRipeningCycle = oneYearSeconds
)

type vpWeight struct {
	power         int64
	bondingHeight int64
	weight        decimal.Decimal
}

type VPower struct {
	From types.Address `json:"from"`
	To   types.Address `json:"to"`

	maturePower   int64
	risingWeights []*vpWeight

	mtx sync.RWMutex
}

func NewVPower(vpow, height int64, from, to types.Address) *VPower {
	ret := &VPower{
		From: from,
		To:   to,
	}
	if vpow > 0 {
		ret.Add(vpow, height)
	}
	return ret
}

func (vp *VPower) Add(pow, height int64) {
	vp.mtx.Lock()
	defer vp.mtx.Unlock()

	vp.risingWeights = append(vp.risingWeights, &vpWeight{
		power:         pow,
		bondingHeight: height,
		weight:        decimal.Zero,
	})
}

// Sub decreases the total power of vp by `pow`.
// If `pow` is greater than the current total power of `vp`,
// Sub returns the actual decreased power that is not equal to `pow`.
func (vp *VPower) Sub(pow int64) int64 {
	vp.mtx.Lock()
	defer vp.mtx.Unlock()

	_rmpow := pow
	_rmpow -= vp.maturePower
	if vp.maturePower > 0 {
		if _rmpow > 0 {
			vp.maturePower = 0
		} else {
			vp.maturePower = -1 * _rmpow
			return pow - _rmpow
		}
	}

	for i := len(vp.risingWeights) - 1; i >= 0; i-- {
		rwt := vp.risingWeights[i]
		_rmpow -= rwt.power
		if _rmpow < 0 {
			rwt.power = -1 * _rmpow
			_rmpow = 0
			break
		} else { // _rmpow >= 0
			// remove rwt
			vp.risingWeights = append(vp.risingWeights[0:i], vp.risingWeights[i+1:]...)
			if _rmpow == 0 {
				break
			}
		}
	}
	return pow - _rmpow
}

func (vp *VPower) TotalPower() int64 {
	vp.mtx.RLock()
	defer vp.mtx.RUnlock()

	return vp.maturePower + vp.RisingPower()
}

func (vp *VPower) MaturePower() int64 {
	vp.mtx.RLock()
	defer vp.mtx.RUnlock()

	return vp.maturePower
}

func (vp *VPower) RisingPower() int64 {
	vp.mtx.RLock()
	defer vp.mtx.RUnlock()

	ret := int64(0)
	for _, rw := range vp.risingWeights {
		ret += rw.power
	}
	return ret
}

func (vp *VPower) Compute(height int64, totalSupply *uint256.Int, tauPermil int) decimal.Decimal {
	_totolSupply := decimal.NewFromBigInt(totalSupply.ToBig(), 0)
	_risingWeight := decimal.Zero
	_risings := vp.risingWeights[:0]

	for _, p := range vp.risingWeights {
		dur := height - p.bondingHeight
		if dur >= powerRipeningCycle {
			vp.maturePower += p.power
		} else {
			p.weight = Wi(p.power, dur, _totolSupply, tauPermil)
			_risingWeight = _risingWeight.Add(p.weight)
			_risings = append(_risings, p)
		}
	}
	vp.risingWeights = _risings

	return Wi(vp.maturePower, powerRipeningCycle, _totolSupply, tauPermil).Add(_risingWeight)
}

func (vp *VPower) Key() v1.LedgerKey {
	vp.mtx.RLock()
	defer vp.mtx.RUnlock()

	// Because the key is `vp.From`, there cannot be multiple `vp.From`.
	// This means that a `vp.From` can only delegate to only one `vp.To`(validator).
	// In contrast, a `vp.To`(validator) can be delegated to by multiple `from`.
	return vp.From
}

func (vp *VPower) Encode() ([]byte, xerrors.XError) {
	//TODO implement me
	panic("implement me")
}

func (vp *VPower) Decode(bytes []byte) xerrors.XError {
	//TODO implement me
	panic("implement me")
}

var _ v1.ILedgerItem = (*VPower)(nil)

// Wa calculates the total voting power weight of all validators and delegators.
// The result may differ from the sum of Wi due to floating-point error.
func Wa(vpows, vpdurs []int64, totalSupply decimal.Decimal, tau int) decimal.Decimal {
	sumPow := decimal.Zero
	sumTmW := decimal.Zero
	for i, vpow := range vpows {
		vpAmt := decimal.New(vpow, int32(types.DECIMAL))
		sumPow = sumPow.Add(vpAmt)

		tmW := vpAmt
		if vpdurs[i] < powerRipeningCycle {
			tmW = decimal.NewFromInt(vpdurs[i]).Mul(vpAmt).Div(decimal.NewFromInt(powerRipeningCycle))
		}
		sumTmW = sumTmW.Add(tmW)
	}

	_tau := decimal.New(int64(tau), -3)
	_keppa := decimalOne.Sub(_tau)

	// Use `QuoRem` instead of `Div`.
	// Because `Div` does round up, the sum of `Wi` can be greater than `1`.
	q, _ := _tau.Mul(sumTmW).Add(_keppa.Mul(sumPow)).QuoRem(totalSupply, 16)
	return q
}

// Wi calculates the voting power weight `W_i` of an validator and delegator like the below.
// `W_i = (tau * min(StakeDurationInSecond/InflationCycle, 1) + keppa) * Stake_i / S_i`
func Wi(vpow, vdur int64, totalSupply decimal.Decimal, tau int) decimal.Decimal {
	decDur := decimalOne
	if vdur < powerRipeningCycle {
		decDur = decimal.NewFromInt(vdur).Div(decimal.NewFromInt(powerRipeningCycle))
	}
	decV := decimal.New(vpow, 18) // todo: make variable for `18`
	decTau := decimal.New(int64(tau), -3)
	decKeppa := decimalOne.Sub(decTau)

	// Use `QuoRem` instead of `Div`.
	// Because `Div` does round up, the sum of `Wi` can be greater than `1`.
	q, _ := decTau.Mul(decDur).Add(decKeppa).Mul(decV).QuoRem(totalSupply, 18)
	return q
}

// H returns the normalized block time corresponding to the given block height.
// It calculates how far along the blockchain is relative to a predefined reference period.
// For example, if the reference period is one year, a return value of 1.0 indicates that
// exactly one reference period has elapsed.
func H(height, blockIntvSec int64) decimal.Decimal {
	return decimal.NewFromInt(height).Mul(decimal.NewFromInt(blockIntvSec)).Div(decimal.NewFromInt(oneYearSeconds))
}
