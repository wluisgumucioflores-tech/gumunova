package sale_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"gumunova/backend/internal/domain/sale"
	"gumunova/backend/internal/domain/shared/validation"
)

var now = time.Date(2026, 11, 11, 15, 0, 0, 0, time.UTC)

func metersLine() sale.LineDraft {
	return sale.LineDraft{Kind: sale.KindRollMeters, RollID: "roll-1", Meters: d("2.50"), UnitPrice: d("15.50")}
}

func fullRollLine() sale.LineDraft {
	return sale.LineDraft{Kind: sale.KindFullRoll, RollID: "roll-2", UnitPrice: d("280")}
}

func clothLine() sale.LineDraft {
	return sale.LineDraft{Kind: sale.KindTablecloth, ProductID: "cloth-1", Quantity: 3, UnitPrice: d("60")}
}

func draft(lines ...sale.LineDraft) sale.Draft {
	return sale.Draft{SaleDate: time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC), Lines: lines}
}

func problems(t *testing.T, err error) map[string]string {
	t.Helper()
	var invalid *validation.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("expected *ValidationError, got %v", err)
	}
	result := map[string]string{}
	for _, field := range invalid.Fields {
		result[field.Field] = field.Code
	}
	return result
}

func TestNewAcceptsEachKindOfLine(t *testing.T) {
	built, err := sale.New(draft(metersLine(), fullRollLine(), clothLine()), now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if built.Status != sale.StatusConfirmed || len(built.Lines) != 3 || built.Lines[2].Number != 3 {
		t.Errorf("sale = %+v", built)
	}
	if built.SaleDate.Hour() != 0 || built.SaleDate.Day() != 10 {
		t.Errorf("date = %v, want a plain date", built.SaleDate)
	}
}

func TestSoldIsExpressedInTheUnitOfTheLine(t *testing.T) {
	built, _ := sale.New(draft(metersLine(), fullRollLine(), clothLine()), now)
	want := []string{"2.50", "1.00", "3.00"}
	for index, line := range built.Lines {
		if got := line.Sold().String(); got != want[index] {
			t.Errorf("line %d sold %s, want %s", index+1, got, want[index])
		}
	}
}

func TestNewRejectsAnEmptySaleAFutureDateAndLongNotes(t *testing.T) {
	input := draft()
	input.SaleDate = now.AddDate(0, 0, 5)
	input.Notes = strings.Repeat("x", 501)

	_, err := sale.New(input, now)

	got := problems(t, err)
	want := map[string]string{"sale_date": "in_the_future", "notes": "too_long", "lines": "required"}
	for field, code := range want {
		if got[field] != code {
			t.Errorf("%s = %q, want %q (all: %v)", field, got[field], code, got)
		}
	}
}

func TestEveryLineNeedsAPositivePrice(t *testing.T) {
	line := clothLine()
	line.UnitPrice = d("0")

	_, err := sale.New(draft(line), now)

	if got := problems(t, err)["lines[0].unit_price"]; got != validation.CodeMustBePositive {
		t.Errorf("unit_price = %q", got)
	}
}

func TestRollLinesAreCheckedForWhatTheyMustAndMustNotCarry(t *testing.T) {
	noRoll := metersLine()
	noRoll.RollID = ""
	noMeters := metersLine()
	noMeters.Meters = d("0")
	wrongProduct := metersLine()
	wrongProduct.ProductID = "pvc-1" // the roll decides the product, not the client
	meterOnFullRoll := fullRollLine()
	meterOnFullRoll.Meters = d("5")
	unitsOnRoll := metersLine()
	unitsOnRoll.Quantity = 2

	_, err := sale.New(draft(noRoll, noMeters, wrongProduct, meterOnFullRoll, unitsOnRoll), now)

	got := problems(t, err)
	want := map[string]string{
		"lines[0].roll_id":    validation.CodeRequired,
		"lines[1].meters":     validation.CodeMustBePositive,
		"lines[2].product_id": validation.CodeNotAllowed,
		"lines[3].meters":     validation.CodeNotAllowed,
		"lines[4].quantity":   validation.CodeNotAllowed,
	}
	for field, code := range want {
		if got[field] != code {
			t.Errorf("%s = %q, want %q (all: %v)", field, got[field], code, got)
		}
	}
}

func TestTableclothLinesAreChecked(t *testing.T) {
	noProduct := clothLine()
	noProduct.ProductID = ""
	noUnits := clothLine()
	noUnits.Quantity = 0
	tooMany := clothLine()
	tooMany.Quantity = 100_001
	withRoll := clothLine()
	withRoll.RollID = "roll-1"
	withMeters := clothLine()
	withMeters.Meters = d("1")

	_, err := sale.New(draft(noProduct, noUnits, tooMany, withRoll, withMeters), now)

	got := problems(t, err)
	want := map[string]string{
		"lines[0].product_id": validation.CodeRequired,
		"lines[1].quantity":   validation.CodeMustBePositive,
		"lines[2].quantity":   validation.CodeOutOfRange,
		"lines[3].roll_id":    validation.CodeNotAllowed,
		"lines[4].meters":     validation.CodeNotAllowed,
	}
	for field, code := range want {
		if got[field] != code {
			t.Errorf("%s = %q, want %q (all: %v)", field, got[field], code, got)
		}
	}
}

func TestAnUnknownKindIsRejected(t *testing.T) {
	line := clothLine()
	line.Kind = "gift"

	_, err := sale.New(draft(line), now)

	if got := problems(t, err)["lines[0].kind"]; got != validation.CodeInvalidValue {
		t.Errorf("kind = %q", got)
	}
}

func TestFingerprintChangesWithAnyEnteredValue(t *testing.T) {
	base := draft(metersLine(), clothLine()).Fingerprint()
	changes := map[string]func(*sale.Draft){
		"price":    func(x *sale.Draft) { x.Lines[0].UnitPrice = d("16") },
		"meters":   func(x *sale.Draft) { x.Lines[0].Meters = d("2.51") },
		"roll":     func(x *sale.Draft) { x.Lines[0].RollID = "roll-9" },
		"quantity": func(x *sale.Draft) { x.Lines[1].Quantity = 4 },
		"kind":     func(x *sale.Draft) { x.Lines[0].Kind = sale.KindFullRoll },
		"notes":    func(x *sale.Draft) { x.Notes = "otra" },
		"date":     func(x *sale.Draft) { x.SaleDate = x.SaleDate.AddDate(0, 0, -1) },
	}
	for name, change := range changes {
		changed := draft(metersLine(), clothLine())
		change(&changed)
		if changed.Fingerprint() == base {
			t.Errorf("changing the %s must change the fingerprint", name)
		}
	}
	if draft(metersLine(), clothLine()).Fingerprint() != base {
		t.Error("the fingerprint must be stable")
	}
}

func TestGrossProfit(t *testing.T) {
	line := sale.Line{LineTotal: d("100.00"), CostOfSale: d("62.50")}
	if line.GrossProfit().String() != "37.50" {
		t.Errorf("profit = %s", line.GrossProfit())
	}
	total := sale.Sale{Total: d("300"), TotalCost: d("120.25")}
	if total.GrossProfit().String() != "179.75" {
		t.Errorf("profit = %s", total.GrossProfit())
	}
}
