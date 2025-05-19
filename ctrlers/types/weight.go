package types

import (
	"github.com/beatoz/beatoz-go/types"
	"github.com/shopspring/decimal"
)

const (
	MinuteSeconds int64 = 60
	HourSeconds         = MinuteSeconds * 60
	DaySeconds          = HourSeconds * 24
	WeekSeconds         = DaySeconds * 7
	YearSeconds         = DaySeconds * 365
)

var (
	DecimalOne  = decimal.NewFromInt(1)
	DecimalZero = decimal.Zero
)

type Weight struct {
	sumWeight     decimal.Decimal
	valsWeight    decimal.Decimal
	beneficiaries []*beneficiary
}

func NewWeight() *Weight {
	return &Weight{sumWeight: decimal.Zero, valsWeight: decimal.Zero}
}

func (w *Weight) SumWeight() decimal.Decimal {
	return w.sumWeight
}

func (w *Weight) ValWeight() decimal.Decimal {
	return w.valsWeight
}

func (w *Weight) Add(addr types.Address, weight, signWeight decimal.Decimal, isVal bool) {
	w.sumWeight = w.sumWeight.Add(weight)
	if isVal {
		w.valsWeight = w.valsWeight.Add(weight)
	}
	w.beneficiaries = append(w.beneficiaries, &beneficiary{addr, weight, signWeight, isVal})
}

func (w *Weight) Beneficiaries() []*beneficiary {
	return w.beneficiaries
}

type beneficiary struct {
	addr        types.Address
	weight      decimal.Decimal
	signingRate decimal.Decimal
	isVal       bool
}

func (b *beneficiary) Address() types.Address {
	return b.addr
}

func (b *beneficiary) Weight() decimal.Decimal {
	return b.weight
}

func (b *beneficiary) SignRate() decimal.Decimal {
	return b.signingRate
}

func (b *beneficiary) IsValidator() bool {
	return b.isVal
}
