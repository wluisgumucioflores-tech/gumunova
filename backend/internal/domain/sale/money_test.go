package sale_test

import (
	"testing"

	"gumunova/backend/internal/domain/sale"
	"gumunova/backend/internal/domain/shared/decimal"
)

func d(value string) decimal.Decimal { return decimal.MustParse(value) }

func TestLineTotalRoundsHalfUpOnce(t *testing.T) {
	tests := []struct{ price, quantity, want string }{
		{"15.50", "1.25", "19.38"}, // 19.375 -> 19.38
		{"15.50", "2", "31.00"},
		{"0.01", "0.50", "0.01"}, // 0.005 -> 0.01
		{"0.01", "0.49", "0.00"}, // 0.0049 -> 0.00
		{"280", "1", "280.00"},
		{"60", "10", "600.00"},
		{"9999999.99", "100", "999999999.00"},
	}
	for _, test := range tests {
		got, err := sale.LineTotal(d(test.price), d(test.quantity))
		if err != nil || got.String() != test.want {
			t.Errorf("%s x %s = %s (err %v), want %s", test.price, test.quantity, got, err, test.want)
		}
	}
}

func TestRollCostSoldToDate(t *testing.T) {
	attributable, initial := d("289.54"), d("19.80")

	if got, _ := sale.RollCostSoldToDate(attributable, initial, d("0")); got.String() != "0.00" {
		t.Errorf("nothing sold costs %s", got)
	}
	if got, _ := sale.RollCostSoldToDate(attributable, initial, initial); got.String() != attributable.String() {
		t.Errorf("selling the whole roll costs %s, want %s", got, attributable)
	}
	// Half of the roll: 144.77 exactly.
	if got, _ := sale.RollCostSoldToDate(attributable, initial, d("9.90")); got.String() != "144.77" {
		t.Errorf("half costs %s", got)
	}
}

func TestSuccessiveSalesOfARollAddUpExactlyToItsCost(t *testing.T) {
	attributable, initial := d("100.00"), d("3.00") // one third of a roll costs 33.333...
	sold := []string{"1.00", "1.00", "1.00"}

	var previous, total int64
	var soldSoFar int64
	for _, meters := range sold {
		soldSoFar += d(meters).Hundredths()
		cumulative, err := sale.RollCostSoldToDate(attributable, initial, decimal.FromHundredths(soldSoFar))
		if err != nil {
			t.Fatal(err)
		}
		total += cumulative.Hundredths() - previous
		previous = cumulative.Hundredths()
	}

	if total != attributable.Hundredths() {
		t.Errorf("costs add up to %d hundredths, want %d: no cent may be lost or invented", total, attributable.Hundredths())
	}
}

func TestTableclothCost(t *testing.T) {
	// 10 units worth 310.23: one unit is 31.023.
	if got, _ := sale.TableclothCost(d("310.23"), 10, 1); got.String() != "31.02" {
		t.Errorf("one unit costs %s", got)
	}
	if got, _ := sale.TableclothCost(d("310.23"), 10, 10); got.String() != "310.23" {
		t.Errorf("selling everything costs %s, want the whole value", got)
	}
	if got, _ := sale.TableclothCost(d("310.23"), 10, 5); got.String() != "155.12" { // 155.115 -> 155.12
		t.Errorf("half costs %s", got)
	}
}
