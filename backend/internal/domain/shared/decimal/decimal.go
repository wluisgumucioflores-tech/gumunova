// Package decimal provides an exact fixed-point number with two fractional digits.
//
// It is used for money and for measures (meters, widths), which the business
// records with hundredths precision. Floating point types are never used.
package decimal

import (
	"errors"
	"fmt"
	"strings"
)

// maxHundredths matches NUMERIC(12,2): ten integer digits and two fractional digits.
const maxHundredths int64 = 999_999_999_999

var (
	ErrInvalidFormat   = errors.New("decimal: invalid format")
	ErrTooManyDecimals = errors.New("decimal: more than two decimal places")
	ErrOutOfRange      = errors.New("decimal: value out of range")
)

// Decimal is an exact number stored as hundredths. The zero value is 0.00.
type Decimal struct {
	hundredths int64
}

// Parse reads a plain decimal string such as "15", "15.5" or "15.50".
// Only a dot is accepted as separator; no thousands separators or exponents.
func Parse(input string) (Decimal, error) {
	if input == "" {
		return Decimal{}, ErrInvalidFormat
	}

	negative := false
	digits := input
	if digits[0] == '-' {
		negative = true
		digits = digits[1:]
	}

	wholePart, fractionPart, hasFraction := strings.Cut(digits, ".")
	if wholePart == "" || !onlyDigits(wholePart) {
		return Decimal{}, ErrInvalidFormat
	}
	if hasFraction {
		if fractionPart == "" || !onlyDigits(fractionPart) {
			return Decimal{}, ErrInvalidFormat
		}
		if len(fractionPart) > 2 {
			return Decimal{}, ErrTooManyDecimals
		}
	}
	fractionPart += strings.Repeat("0", 2-len(fractionPart))

	wholePart = strings.TrimLeft(wholePart, "0")
	if len(wholePart) > 10 {
		return Decimal{}, ErrOutOfRange
	}

	var value int64
	for _, digit := range wholePart + fractionPart {
		value = value*10 + int64(digit-'0')
	}
	if value > maxHundredths {
		return Decimal{}, ErrOutOfRange
	}
	if negative {
		value = -value
	}
	return Decimal{hundredths: value}, nil
}

// MustParse is like Parse but panics on error. Intended for tests and constants.
func MustParse(input string) Decimal {
	value, err := Parse(input)
	if err != nil {
		panic(err)
	}
	return value
}

// String returns the canonical representation with exactly two decimals.
func (d Decimal) String() string {
	value := d.hundredths
	sign := ""
	if value < 0 {
		sign = "-"
		value = -value
	}
	return fmt.Sprintf("%s%d.%02d", sign, value/100, value%100)
}

// ErrOverflow is returned by arithmetic whose result does not fit the underlying integer.
var ErrOverflow = errors.New("decimal: arithmetic overflow")

// Hundredths returns the value as an integer count of hundredths (1.25 -> 125).
func (d Decimal) Hundredths() int64 { return d.hundredths }

// FromHundredths builds a decimal from an integer count of hundredths.
func FromHundredths(hundredths int64) Decimal { return Decimal{hundredths: hundredths} }

// Add returns d + other. Sums may exceed the range accepted by Parse (which mirrors a
// single NUMERIC(12,2) column); totals are stored in wider columns.
func (d Decimal) Add(other Decimal) (Decimal, error) {
	sum := d.hundredths + other.hundredths
	if (other.hundredths > 0 && sum < d.hundredths) || (other.hundredths < 0 && sum > d.hundredths) {
		return Decimal{}, ErrOverflow
	}
	return Decimal{hundredths: sum}, nil
}

// MulInt returns d * factor for a whole number such as a quantity of units.
func (d Decimal) MulInt(factor int64) (Decimal, error) {
	if factor == 0 || d.hundredths == 0 {
		return Decimal{}, nil
	}
	product := d.hundredths * factor
	if product/factor != d.hundredths {
		return Decimal{}, ErrOverflow
	}
	return Decimal{hundredths: product}, nil
}

func (d Decimal) IsZero() bool     { return d.hundredths == 0 }
func (d Decimal) IsPositive() bool { return d.hundredths > 0 }
func (d Decimal) IsNegative() bool { return d.hundredths < 0 }

func onlyDigits(value string) bool {
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}
