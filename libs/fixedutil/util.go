// Package fixedutil provides deterministic, fixed-point implementations of
// exp, ln, and pow for robaho/fixed.Fixed (7-decimal precision), optimized for performance and precision.
package fixedutil

import (
	"errors"

	"github.com/robaho/fixed"
)

const (
	// Increased terms for improved precision; trade-off with performance
	lnTerms  = 32
	expTerms = 32
)

var (
	ZERO        = fixed.ZERO       // 0.0000000
	ONE         = fixed.NewI(1, 0) // 1.0000000
	TWO         = fixed.NewI(2, 0) // 2.0000000
	PercentBase = fixed.NewI(100, 0)
	PermilBase  = fixed.NewI(1000, 0)

	// Precompute denominators for ln series: 1, 3, 5, ...
	lnDenoms = func() []fixed.Fixed {
		d := make([]fixed.Fixed, lnTerms)
		for k := 0; k < lnTerms; k++ {
			d[k] = fixed.NewI(int64(2*k+1), 0)
		}
		return d
	}()

	// Precompute denominators for exp Horner method: 1, 2, ..., expTerms
	expDenoms = func() []fixed.Fixed {
		d := make([]fixed.Fixed, expTerms+1)
		for n := 1; n <= expTerms; n++ {
			d[n] = fixed.NewI(int64(n), 0)
		}
		return d
	}()
)

// FixedLn returns ln(x) for x > 0 using the identity:
// ln(x) = 2 * ( t + t^3/3 + t^5/5 + ... ), where t = (x-1)/(x+1).
// Deterministic and integer-only operations.
func FixedLn(x fixed.Fixed) (fixed.Fixed, error) {
	if x.Cmp(ZERO) <= 0 {
		return ZERO, errors.New("ln: input must be > 0")
	}
	t := x.Sub(ONE).Div(x.Add(ONE))
	t2 := t.Mul(t)

	sum := t
	term := t
	for k := 1; k < lnTerms; k++ {
		term = term.Mul(t2)
		sum = sum.Add(term.Div(lnDenoms[k]))
	}
	return sum.Mul(TWO), nil
}

// FixedExp returns e^x using Horner's method to minimize intermediate allocations:
// exp(x) ≈ 1 + x/1*(1 + x/2*(1 + ... (1 + x/N))).
func FixedExp(x fixed.Fixed) fixed.Fixed {
	s := ONE
	for n := expTerms; n > 0; n-- {
		s = ONE.Add(x.Div(expDenoms[n]).Mul(s))
	}
	return s
}

// FixedPow computes x^y = exp(y * ln(x)) deterministically.
func FixedPow(base, exponent fixed.Fixed) (fixed.Fixed, error) {
	lnVal, err := FixedLn(base)
	if err != nil {
		return ZERO, err
	}
	return FixedExp(lnVal.Mul(exponent)), nil
}

func Percent(n int) fixed.Fixed {
	return fixed.NewI(int64(n), 0).Div(PercentBase)
}

func Permil(n int) fixed.Fixed {
	return fixed.NewI(int64(n), 0).Div(PermilBase)
}

//// Package fixedutil provides deterministic, fixed-point implementations of
//// exp, ln, and pow for robaho/fixed.Fixed (7-decimal precision), optimized for performance.
//package _tmp
//
//import (
//	"errors"
//
//	"github.com/robaho/fixed"
//)
//
//const (
//	lnTerms  = 16 // precision vs. performance
//	expTerms = 24
//)
//
//// Pre-allocated constants to avoid repeated allocations
//var (
//	ZERO = fixed.NewI(0, 0) // 0.0000000
//	ONE  = fixed.NewI(1, 0) // 1.0000000
//	TWO  = fixed.NewI(2, 0) // 2.0000000
//)
//
//// FixedLn returns ln(x) for x > 0 using an optimized series with minimal allocations.
//func FixedLn(x fixed.Fixed) (fixed.Fixed, error) {
//	if x.Cmp(ZERO) <= 0 {
//		return ZERO, errors.New("ln: input must be > 0")
//	}
//	// t = (x-1)/(x+1)
//	t := x.Sub(ONE).Div(x.Add(ONE))
//	t2 := t.Mul(t)
//
//	// series: 2 * (t + t^3/3 + t^5/5 + ...)
//	// compute iteratively
//	s := t // first term t^(2*0+1)/(2*0+1)
//	for k := 1; k < lnTerms; k++ {
//		tExp := t2
//		// raise t2 to k-th power without allocation: reuse t2Exp
//		for i := 1; i < k; i++ {
//			tExp = tExp.Mul(t2)
//		}
//		d := fixed.NewI(int64(2*k+1), 0)
//		s = s.Add(tExp.Div(d))
//	}
//	return s.Mul(TWO), nil
//}
//
//// FixedExp returns e^x using Horner's method to reduce allocations:
//// exp(x) ≈ 1 + x/1*(1 + x/2*(1 + ... (1 + x/n)))
//func FixedExp(x fixed.Fixed) fixed.Fixed {
//	// Horner-style expansion
//	s := fixed.NewI(1, 0)
//	for n := expTerms; n > 0; n-- {
//		// s = 1 + x/n * s
//		s = ONE.Add(x.Div(fixed.NewI(int64(n), 0)).Mul(s))
//	}
//	return s
//}
//
//// FixedPow computes x^y = exp(y * ln(x)) deterministically.
//func FixedPow(base, exponent fixed.Fixed) (fixed.Fixed, error) {
//	lnb, err := FixedLn(base)
//	if err != nil {
//		return ZERO, err
//	}
//	// exponent * ln(base)
//	arg := lnb.Mul(exponent)
//	return FixedExp(arg), nil
//}
