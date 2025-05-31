//go:build decimal

package fxnum

import (
	"github.com/robaho/fixed"
	"github.com/shopspring/decimal"
)

var (
	ZERO          = New(0, 0)
	ONE           = New(1, 0)
	TWO           = New(2, 0)
	PercentBase   = New(100, 0)
	PermilBase    = New(1000, 0)
	wantPrecision = int32(decimal.DivisionPrecision)
)

func init() {
	SetDivisionPrecision(16)
}

func SetWantPrecision(precision int32) {
	wantPrecision = precision
	decimal.DivisionPrecision = min(precision*3, 16)
}

func GetWantPrecision() int32 {
	return wantPrecision
}

func GetDivisionPrecision() int32 {
	return int32(decimal.DivisionPrecision)
}

type FxNum struct {
	decimal.Decimal
}

func New(val int64, exp int32) FxNum {
	return FxNum{decimal.New(val, exp)}
}

func FromInt(val int64) FxNum {
	return FxNum{decimal.NewFromInt(val)}
}

func FromFloat(val float64) FxNum {
	return FxNum{decimal.NewFromFloat(val)}
}

func FromString(val string) FxNum {
	return FxNum{decimal.NewFromString(val)}
}

func (x FxNum) Add(o FxNum) FxNum {
	return FxNum{x.Decimal.Add(o.Decimal)}
}

func (x FxNum) Sub(o FxNum) FxNum {
	return FxNum{x.Decimal.Sub(o.Decimal)}
}

func (x FxNum) Mul(o FxNum) FxNum {
	return FxNum{x.Decimal.Mul(o.Decimal)}
}

func (x FxNum) Div(o FxNum) FxNum {
	return FxNum{x.Decimal.Div(o.Decimal)}
}

func (x FxNum) QuoRem(o FxNum, precision int32) (FxNum, FxNum) {
	q, r := x.Decimal.QuoRem(o.Decimal, precision)
	return FxNum{q}, FxNum{r}
}

func (x FxNum) Pow(o FxNum) FxNum {
	return FxNum{x.Decimal.Pow(o.Decimal)}
}

func (x FxNum) Truncate(precision int32) FxNum {
	return FxNum{x.Decimal.Truncate(precision)}
}

func (x FxNum) Equal(o FxNum) bool {
	return x.Decimal.Equal(o.Decimal)
}
func (x FxNum) GreaterThan(o FxNum) bool {
	return x.Decimal.GreaterThan(o.Decimal)
}

func (x FxNum) GreaterThanOrEqual(o FxNum) bool {
	return x.Decimal.GreaterThanOrEqual(o.Decimal)
}

func (x FxNum) LessThan(o FxNum) bool {
	return x.Decimal.LessThan(o.Decimal)
}

func (x FxNum) LessThanOrEqual(o FxNum) bool {
	return x.Decimal.LessThanOrEqual(o.Decimal)
}

func (x FxNum) ToDecimal() (decimal.Decimal, error) {
	return x.Decimal, nil
}

func (x FxNum) ToFixed() (fixed.Fixed, error) {
	panic("implement me")
}
func Percent(n int) FxNum {
	return FromInt(int64(n)).Div(PercentBase)
}
func Permil(n int) FxNum {
	return FromInt(int64(n)).Div(PermilBase)
}
