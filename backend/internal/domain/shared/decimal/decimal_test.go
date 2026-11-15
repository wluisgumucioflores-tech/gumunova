package decimal_test

import (
	"errors"
	"testing"

	"gumunova/backend/internal/domain/shared/decimal"
)

func TestParseAndString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"0", "0.00"},
		{"15", "15.00"},
		{"15.5", "15.50"},
		{"15.50", "15.50"},
		{"0.05", "0.05"},
		{"007.10", "7.10"},
		{"-1.25", "-1.25"},
		{"9999999999.99", "9999999999.99"},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got, err := decimal.Parse(test.input)
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", test.input, err)
			}
			if got.String() != test.want {
				t.Errorf("Parse(%q).String() = %q, want %q", test.input, got.String(), test.want)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		input string
		want  error
	}{
		{"", decimal.ErrInvalidFormat},
		{"abc", decimal.ErrInvalidFormat},
		{"1,5", decimal.ErrInvalidFormat},
		{".5", decimal.ErrInvalidFormat},
		{"5.", decimal.ErrInvalidFormat},
		{"-", decimal.ErrInvalidFormat},
		{"+5", decimal.ErrInvalidFormat},
		{"1e3", decimal.ErrInvalidFormat},
		{"1.234", decimal.ErrTooManyDecimals},
		{"10000000000", decimal.ErrOutOfRange},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			_, err := decimal.Parse(test.input)
			if !errors.Is(err, test.want) {
				t.Errorf("Parse(%q) error = %v, want %v", test.input, err, test.want)
			}
		})
	}
}

func TestSignPredicates(t *testing.T) {
	if !decimal.MustParse("0").IsZero() {
		t.Error("0 should be zero")
	}
	if !decimal.MustParse("0.01").IsPositive() {
		t.Error("0.01 should be positive")
	}
	if !decimal.MustParse("-0.01").IsNegative() {
		t.Error("-0.01 should be negative")
	}
	if decimal.MustParse("0").IsPositive() || decimal.MustParse("0").IsNegative() {
		t.Error("0 is neither positive nor negative")
	}
}

func TestExactArithmeticFriendlyValues(t *testing.T) {
	// 0.1 and 0.2 are not exactly representable in binary floating point;
	// here they round-trip exactly.
	if got := decimal.MustParse("0.10").String(); got != "0.10" {
		t.Errorf("got %q", got)
	}
	if got := decimal.MustParse("0.20").String(); got != "0.20" {
		t.Errorf("got %q", got)
	}
}

func TestAdd(t *testing.T) {
	sum, err := decimal.MustParse("0.10").Add(decimal.MustParse("0.20"))
	if err != nil || sum.String() != "0.30" {
		t.Errorf("0.10 + 0.20 = %v (err %v), want exactly 0.30", sum, err)
	}
	negative, _ := decimal.MustParse("1.00").Add(decimal.MustParse("-2.50"))
	if negative.String() != "-1.50" {
		t.Errorf("got %v", negative)
	}
	huge := decimal.FromHundredths(9_000_000_000_000_000_000)
	if _, err := huge.Add(huge); !errors.Is(err, decimal.ErrOverflow) {
		t.Errorf("overflow error = %v, want ErrOverflow", err)
	}
}

func TestMulInt(t *testing.T) {
	product, err := decimal.MustParse("45.25").MulInt(10)
	if err != nil || product.String() != "452.50" {
		t.Errorf("45.25 * 10 = %v (err %v)", product, err)
	}
	if zero, _ := decimal.MustParse("45.25").MulInt(0); !zero.IsZero() {
		t.Error("anything times zero is zero")
	}
	if _, err := decimal.FromHundredths(5_000_000_000_000_000_000).MulInt(3); !errors.Is(err, decimal.ErrOverflow) {
		t.Errorf("overflow error = %v, want ErrOverflow", err)
	}
}

func TestHundredthsRoundTrip(t *testing.T) {
	value := decimal.MustParse("123.45")
	if value.Hundredths() != 12345 || decimal.FromHundredths(12345).String() != "123.45" {
		t.Errorf("round trip failed: %d", value.Hundredths())
	}
}
