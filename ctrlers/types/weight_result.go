package types

import (
	"github.com/beatoz/beatoz-go/libs/fxnum"
	"github.com/beatoz/beatoz-go/types"
)

const (
	MinuteSeconds int64 = 60
	HourSeconds         = MinuteSeconds * 60
	DaySeconds          = HourSeconds * 24
	WeekSeconds         = DaySeconds * 7
	YearSeconds         = DaySeconds * 365
)

type IWeightResult interface {
	SumWeight() fxnum.FxNum
	ValWeight() fxnum.FxNum
	Add(addr types.Address, weight, signWeight fxnum.FxNum, isVal bool)
	Beneficiaries() []*Beneficiary
}

type Beneficiary struct {
	addr        types.Address
	weight      fxnum.FxNum
	signingRate fxnum.FxNum
	isVal       bool
}

func NewBeneficiary(addr types.Address, weight, signWeight fxnum.FxNum, isVal bool) *Beneficiary {
	return &Beneficiary{addr, weight, signWeight, isVal}
}

func (b *Beneficiary) Address() types.Address {
	return b.addr
}

func (b *Beneficiary) Weight() fxnum.FxNum {
	return b.weight
}

func (b *Beneficiary) SignRate() fxnum.FxNum {
	return b.signingRate
}

func (b *Beneficiary) IsValidator() bool {
	return b.isVal
}
