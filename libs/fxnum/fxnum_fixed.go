//go:build !decimal

package fxnum

import (
	"github.com/robaho/fixed"
	"github.com/shopspring/decimal"
	"math"
)

var (
	ZERO          = New(0, 0)
	ONE           = New(1, 0)
	TWO           = New(2, 0)
	PercentBase   = New(100, 0)
	PermilBase    = New(1000, 0)
	wantPrecision = int32(3) // no meaning; it's for decimal.Decimal
)

func SetWantPrecision(precision int32) {
	wantPrecision = min(precision, 7)
}

func GetWantPrecision() int32 {
	return wantPrecision
}

func GetDivisionPrecision() int32 {
	return int32(7)
}

type FxNum struct {
	fixed.Fixed
}

func New(val int64, nexp int32) FxNum {
	return FxNum{fixed.NewI(val, uint(nexp))}
}

func FromInt(val int64) FxNum {
	return New(val, 0)
}

func FromFloat(val float64) FxNum {
	return FxNum{fixed.NewF(val)}
}

func FromString(val string) FxNum {
	return FxNum{fixed.NewS(val)}
}

func (x FxNum) Add(o FxNum) FxNum {
	return FxNum{x.Fixed.Add(o.Fixed)}
}

func (x FxNum) Sub(o FxNum) FxNum {
	return FxNum{x.Fixed.Sub(o.Fixed)}
}

func (x FxNum) Mul(o FxNum) FxNum {
	return FxNum{x.Fixed.Mul(o.Fixed)}
}

func (x FxNum) Div(o FxNum) FxNum {
	return FxNum{x.Fixed.Div(o.Fixed)}
}

func (x FxNum) QuoRem(o FxNum, precision int32) (FxNum, FxNum) {
	return FxNum{x.Fixed.Div(o.Fixed)}, FxNum{zero}
}

func (x FxNum) Pow(o FxNum) FxNum {
	ret, _ := FixedPow(x.Fixed, o.Fixed)
	return FxNum{ret}
}

func (x FxNum) Equal(o FxNum) bool {
	return x.Fixed.Equal(o.Fixed)
}
func (x FxNum) Truncate(precision int32) FxNum {
	fxPrec := FromInt(int64(math.Pow10(int(precision))))
	_x := x.Div(fxPrec)
	_y := _x.Mul(fxPrec)
	return _y
}

func (x FxNum) GreaterThan(o FxNum) bool {
	return x.Fixed.GreaterThan(o.Fixed)
}

func (x FxNum) GreaterThanOrEqual(o FxNum) bool {
	return x.Fixed.GreaterThanOrEqual(o.Fixed)
}

func (x FxNum) LessThan(o FxNum) bool {
	return x.Fixed.LessThan(o.Fixed)
}

func (x FxNum) LessThanOrEqual(o FxNum) bool {
	return x.Fixed.LessThanOrEqual(o.Fixed)
}
func (x FxNum) ToDecimal() (decimal.Decimal, error) {
	return FixedToDecimalByInt(x.Fixed)
}

func (x FxNum) ToFixed() (fixed.Fixed, error) {
	return x.Fixed, nil
}

func Percent(n int) FxNum {
	return FromInt(int64(n)).Div(PercentBase)
}
func Permil(n int) FxNum {
	return FromInt(int64(n)).Div(PermilBase)
}
