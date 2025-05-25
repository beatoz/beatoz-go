package fixedutil

import (
	"fmt"
	"github.com/robaho/fixed"
	"math"
	"testing"
	"testing/quick"
)

func TestFixedOpsDeterministic(t *testing.T) {
	cases := []struct {
		base, exp string
	}{
		{"2.0000000", "3.5000000"},
		{"1.2345678", "0.8765432"},
		{"10.0000000", "1.0000000"},
		// …원하는 모든 경계/샘플 케이스…
	}
	for _, c := range cases {
		b, _ := fixed.Parse(c.base)
		e, _ := fixed.Parse(c.exp)
		v1, err1 := FixedPow(b, e)
		v2, err2 := FixedPow(b, e)
		if err1 != nil || err2 != nil {
			t.Fatalf("err on %v^%v: %v %v", c.base, c.exp, err1, err2)
		}
		if !v1.Equal(v2) {
			t.Errorf("non-deterministic: %s^%s: %s != %s", c.base, c.exp, v1.String(), v2.String())
		}
	}
}

func TestFixedPowProp(t *testing.T) {
	const scale = 7
	maxVal := float64(math.MaxInt64) / math.Pow10(scale) // ≈ 9.22e14

	f := func(b, e float64) bool {
		// check b>0, e>=0
		// check out-of-range cases
		if b <= 0 || e < 0 || b > maxVal || e > maxVal {
			return true // skip out-of-range cases
		}
		base := fixed.NewI(int64(b*1e7), scale)
		exp := fixed.NewI(int64(e*1e7), scale)
		v1, _ := FixedPow(base, exp)
		v2, _ := FixedPow(base, exp)
		return v1.Equal(v2)
	}

	if err := quick.Check(f, nil); err != nil {
		t.Fatal(err)
	}
}

func TestGolden(t *testing.T) {
	data := []struct {
		base, exp, out string
	}{
		{"2.0000000", "3.5000000", "11.3137085"},
	}
	for _, rec := range data {
		b, _ := fixed.Parse(rec.base)
		e, _ := fixed.Parse(rec.exp)
		got, _ := FixedPow(b, e)
		if got.String() != rec.out {
			t.Errorf("mismatch on %v^%v: got %v want %v", rec.base, rec.exp, got, rec.out)
		}
	}
}

func NonDetPow(x, y float64) float64 {
	// x^y = exp(y * ln(x)) using float64
	return math.Exp(y * math.Log(x))
}

func TestNonDetPow(t *testing.T) {
	inputs := []struct {
		x, y float64
	}{
		{1.2345678, 0.8765432},
		{2.7182818, 1.6180339},
		{10.0000000, 0.1234567},
		{5.0000000, 2.5000000},
	}

	for _, in := range inputs {
		r := NonDetPow(in.x, in.y)
		fmt.Printf("NonDetPow(%.7f, %.7f) = %.15f\n", in.x, in.y, r)
	}
}
