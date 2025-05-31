package fxnum

import (
	"fmt"
	"github.com/robaho/fixed"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"math"
	"testing"
)

func Test_Order_Float64_NonDet(t *testing.T) {
	a := 1e16
	b := -1e16
	c := 1.0

	r1 := (a + b) + c
	r2 := a + (b + c)

	fmt.Printf("(a + b) + c = %.17f\n", r1)
	fmt.Printf("a + (b + c) = %.17f\n", r2)
}

func Test_Order_Decimal(t *testing.T) {
	da := decimal.NewFromFloat(1e16)
	db := decimal.NewFromFloat(-1e16)
	dc := decimal.NewFromFloat(1.0)

	dr1 := da.Add(db).Add(dc)
	dr2 := db.Add(dc).Add(da)
	require.True(t, dr1.Equal(dr2))
}

func Test_CalOrder_Fixed(t *testing.T) {
	fa := fixed.NewF(1e7)
	fb := fixed.NewF(-1e7)
	fc := fixed.NewF(1.0)

	fr1 := fa.Add(fb).Add(fc)
	fr2 := fb.Add(fc).Add(fa)
	require.True(t, fr1.Equal(fr2))
}

func Test_Subnormal_Float64_NonDet(t *testing.T) {
	x := 1e-310 // subnormal
	y := 1.0
	z := x + y

	fmt.Printf("x = %.17g\n", x)
	fmt.Printf("z = x + 1 = %.17g\n", z)
	fmt.Printf("z - 1 = %.17g\n", z-1)
}

func Test_Subnormal_Decimal(t *testing.T) {
	x := decimal.NewFromFloat(1e-310) // subnormal
	y := decimal.NewFromFloat(1.0)
	z := x.Add(y)

	require.True(t, x.Equal(decimal.New(1, -310)))
	require.True(t, y.Equal(decimal.NewFromInt(1)))
	require.True(t, z.Equal(decimal.NewFromInt(1).Add(decimal.New(1, -310))))
	require.False(t, z.Equal(decimal.Zero))
	//fmt.Printf("x = %v\n", x)
	//fmt.Printf("z = x + 1 = %v\n", z)
	//fmt.Printf("z - 1 = %v\n", z.Sub(decimal.New(1, 0)))
}

func Test_FMA_Float64_NonDet(t *testing.T) {
	a := 1e16
	b := 1.000000000000001
	c := -1e16
	//    10000000000000010
	//	- 10000000000000000
	//  ---------------------
	//	= 10
	normal := a*b + c
	fma := math.FMA(a, b, c)

	fmt.Printf("normal: %.17f\n", normal)
	fmt.Printf("fma:    %.17f\n", fma)
}

func Test_FMA_Decimal(t *testing.T) {
	a := decimal.NewFromFloat(1e16)
	b := decimal.NewFromFloat(1.000000000000001)
	c := decimal.NewFromFloat(-1e16)

	normal := a.Mul(b).Add(c)
	fmt.Printf("normal: %v\n", normal)
}
