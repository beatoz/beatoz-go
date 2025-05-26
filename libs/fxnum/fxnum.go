package fxnum

import (
	"encoding/binary"
	"fmt"
	"github.com/robaho/fixed"
	"github.com/shopspring/decimal"
)

// fixedScaleDigits represents the default scale (7 decimal places) used by robaho/fixed.
const fixedScaleDigits = 7

// FixedToDecimalByInt converts a robaho/fixed.Fixed value to a shopspring/decimal.Decimal.
// It leverages the internal int64 value via MarshalBinary for optimized performance.
func FixedToDecimalByInt(f fixed.Fixed) (decimal.Decimal, error) {
	// Handle NaN values, as shopspring/decimal does not natively support them.
	if f.IsNaN() {
		return decimal.Decimal{}, fmt.Errorf("cannot convert NaN fixed.Fixed to decimal.Decimal")
	}

	// MarshalBinary writes the internal int64 fp in little-endian format.
	buf, err := f.MarshalBinary()
	if err != nil {
		return decimal.Decimal{}, fmt.Errorf("failed to marshal fixed.Fixed to binary: %w", err)
	}

	// Read the uint64 from the byte slice and cast it back to int64 to get the raw fixed-point value.
	// robaho/fixed's MarshalBinary uses binary.LittleEndian.PutUint64, so this is the correct way to unmarshal.
	raw, _ := binary.Varint(buf)

	// The 'raw' value is effectively (actual_value * 10^fixedScaleDigits).
	// Therefore, decimal.New(raw, -fixedScaleDigits) yields the exact decimal number.
	return decimal.New(raw, -fixedScaleDigits), nil
}

func FixedToDecimalByString(src fixed.Fixed) (decimal.Decimal, error) {
	return decimal.NewFromString(src.String())
}
