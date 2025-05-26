package vpower

import (
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/libs/fxnum"
	"github.com/beatoz/beatoz-go/types"
	"github.com/shopspring/decimal"
)

var (
	DecimalOne = decimal.NewFromInt(1)
)

type fxnumWeight struct {
	sumWeight     fxnum.FxNum
	valsWeight    fxnum.FxNum
	beneficiaries []*ctrlertypes.Beneficiary
}

func NewWeight() *fxnumWeight {
	return &fxnumWeight{
		sumWeight:  fxnum.New(0, 0),
		valsWeight: fxnum.New(0, 0),
	}
}

func (w *fxnumWeight) SumWeight() fxnum.FxNum {
	return w.sumWeight
}

func (w *fxnumWeight) ValWeight() fxnum.FxNum {
	return w.valsWeight
}

func (w *fxnumWeight) Add(addr types.Address, weight, signWeight fxnum.FxNum, isVal bool) {
	w.sumWeight = w.sumWeight.Add(weight)
	if isVal {
		w.valsWeight = w.valsWeight.Add(weight)
	}
	w.beneficiaries = append(w.beneficiaries, ctrlertypes.NewBeneficiary(addr, weight, signWeight, isVal))
}

func (w *fxnumWeight) Beneficiaries() []*ctrlertypes.Beneficiary {
	return w.beneficiaries
}
