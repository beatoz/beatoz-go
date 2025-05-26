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

type WeightResult struct {
	sumWeight     decimal.Decimal
	valsWeight    decimal.Decimal
	beneficiaries []*beneficiary
}

func NewWeight() *WeightResult {
	return &WeightResult{sumWeight: decimal.Zero, valsWeight: decimal.Zero}
}

func (w *WeightResult) SumWeight() decimal.Decimal {
	return w.sumWeight
}

func (w *WeightResult) ValWeight() decimal.Decimal {
	return w.valsWeight
}

func (w *WeightResult) Add(addr types.Address, weight, signWeight decimal.Decimal, isVal bool) {
	w.sumWeight = w.sumWeight.Add(weight)
	if isVal {
		w.valsWeight = w.valsWeight.Add(weight)
	}
	w.beneficiaries = append(w.beneficiaries, &beneficiary{addr, weight, signWeight, isVal})
}

func (w *WeightResult) Beneficiaries() []*beneficiary {
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
