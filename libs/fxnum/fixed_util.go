// Package fxnum provides deterministic, fixed-point implementations of
// exp, ln, and pow for robaho/fixed.Fixed (7-decimal precision), optimized for performance and precision.
package fxnum

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
	zero = fixed.ZERO       // 0.0000000
	one  = fixed.NewI(1, 0) // 1.0000000
	two  = fixed.NewI(2, 0) // 2.0000000

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
	if x.Cmp(zero) <= 0 {
		return zero, errors.New("ln: input must be > 0")
	}
	t := x.Sub(one).Div(x.Add(one))
	t2 := t.Mul(t)

	sum := t
	term := t
	for k := 1; k < lnTerms; k++ {
		term = term.Mul(t2)
		sum = sum.Add(term.Div(lnDenoms[k]))
	}
	return sum.Mul(two), nil
}

// FixedExp returns e^x using Horner's method to minimize intermediate allocations:
// exp(x) ≈ 1 + x/1*(1 + x/2*(1 + ... (1 + x/N))).
func FixedExp(x fixed.Fixed) fixed.Fixed {
	s := one
	for n := expTerms; n > 0; n-- {
		s = one.Add(x.Div(expDenoms[n]).Mul(s))
	}
	return s
}

// FixedPow computes x^y = exp(y * ln(x)) deterministically.
func FixedPow(base, exponent fixed.Fixed) (fixed.Fixed, error) {
	lnVal, err := FixedLn(base)
	if err != nil {
		return zero, err
	}
	return FixedExp(lnVal.Mul(exponent)), nil
}
