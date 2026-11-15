// Package sale holds the sales domain. A confirmed sale reduces stock: meters of one physical
// roll, a whole intact roll, or units of a finished tablecloth.
package sale

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"gumunova/backend/internal/domain/shared/decimal"
	"gumunova/backend/internal/domain/shared/validation"
)

var (
	ErrNotFound = errors.New("sale not found")
	// ErrIdempotencyKeyReused means the key was already used for a different sale.
	ErrIdempotencyKeyReused = errors.New("idempotency key already used for a different sale")
)

type Status string

const StatusConfirmed Status = "confirmed"

// Kind says what a line sells.
type Kind string

const (
	// KindRollMeters sells meters cut from one physical roll.
	KindRollMeters Kind = "roll_meters"
	// KindFullRoll sells an intact roll whole.
	KindFullRoll Kind = "full_roll"
	// KindTablecloth sells units of a finished tablecloth.
	KindTablecloth Kind = "tablecloth_units"
)

const (
	maxLines    = 100
	maxUnits    = 100_000
	maxNotes    = 500
	futureGrace = 24 * time.Hour
)

// LineDraft is one item to sell, as entered. Which fields apply depends on the kind:
//   - roll_meters: RollID, Meters and UnitPrice (price per meter)
//   - full_roll: RollID and UnitPrice (price of the whole roll)
//   - tablecloth_units: ProductID, Quantity and UnitPrice (price per unit)
//
// The price is what is charged; it may differ from the catalog reference and is kept as a snapshot.
type LineDraft struct {
	Kind      Kind
	RollID    string
	ProductID string
	Meters    decimal.Decimal
	Quantity  int
	UnitPrice decimal.Decimal
}

type Draft struct {
	SaleDate time.Time // calendar date; the time of day is ignored
	Notes    string
	Lines    []LineDraft
}

// Line is a sold item with the figures fixed when the sale was confirmed.
type Line struct {
	ID     string
	Number int
	Kind   Kind
	// Entered by the user.
	RollID    string
	ProductID string
	Meters    decimal.Decimal
	Quantity  int
	UnitPrice decimal.Decimal

	// Filled in when the sale is registered or read back.
	ProductSKU     string
	ProductName    string
	RollCode       string
	ReferencePrice decimal.Decimal // catalog price at the time of the sale
	LineTotal      decimal.Decimal
	CostOfSale     decimal.Decimal
}

// Gross profit of the line: what was charged minus what the sold stock cost.
func (l Line) GrossProfit() decimal.Decimal {
	return decimal.FromHundredths(l.LineTotal.Hundredths() - l.CostOfSale.Hundredths())
}

// Sold is the amount sold in the line's own unit: meters, one roll, or units.
func (l Line) Sold() decimal.Decimal {
	switch l.Kind {
	case KindRollMeters:
		return l.Meters
	case KindFullRoll:
		return decimal.FromHundredths(100)
	default:
		return decimal.FromHundredths(int64(l.Quantity) * 100)
	}
}

// Sale is a confirmed sale. Confirmed sales are never edited.
type Sale struct {
	ID        string
	SaleDate  time.Time
	Notes     string
	Status    Status
	Lines     []Line
	Total     decimal.Decimal
	TotalCost decimal.Decimal
	CreatedAt time.Time
}

func (s Sale) GrossProfit() decimal.Decimal {
	return decimal.FromHundredths(s.Total.Hundredths() - s.TotalCost.Hundredths())
}

// New validates a draft and builds the sale with the entered data. Prices, totals and costs are
// completed by the persistence step, which reads the stock it locks (a sale must be priced and
// costed against the stock it actually takes).
func New(draft Draft, now time.Time) (Sale, error) {
	draft.Notes = strings.TrimSpace(draft.Notes)
	for index := range draft.Lines {
		draft.Lines[index].RollID = strings.TrimSpace(draft.Lines[index].RollID)
		draft.Lines[index].ProductID = strings.TrimSpace(draft.Lines[index].ProductID)
	}
	if problems := validate(draft, now); len(problems) > 0 {
		return Sale{}, &validation.ValidationError{Fields: problems}
	}

	built := Sale{SaleDate: dateOnly(draft.SaleDate), Notes: draft.Notes, Status: StatusConfirmed}
	for index, line := range draft.Lines {
		built.Lines = append(built.Lines, Line{
			Number: index + 1, Kind: line.Kind, RollID: line.RollID, ProductID: line.ProductID,
			Meters: line.Meters, Quantity: line.Quantity, UnitPrice: line.UnitPrice,
		})
	}
	return built, nil
}

func dateOnly(value time.Time) time.Time {
	year, month, day := value.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func validate(draft Draft, now time.Time) []validation.FieldError {
	var problems []validation.FieldError
	add := func(field, code string) { problems = append(problems, validation.FieldError{Field: field, Code: code}) }

	switch {
	case draft.SaleDate.IsZero():
		add("sale_date", validation.CodeRequired)
	case dateOnly(draft.SaleDate).After(dateOnly(now).Add(futureGrace)):
		add("sale_date", validation.CodeInFuture)
	}
	if utf8.RuneCountInString(draft.Notes) > maxNotes {
		add("notes", validation.CodeTooLong)
	}

	switch {
	case len(draft.Lines) == 0:
		add("lines", validation.CodeRequired)
	case len(draft.Lines) > maxLines:
		add("lines", validation.CodeTooMany)
	}

	for index, line := range draft.Lines {
		if index >= maxLines {
			break
		}
		prefix := fmt.Sprintf("lines[%d]", index)
		if !line.UnitPrice.IsPositive() {
			add(prefix+".unit_price", validation.CodeMustBePositive)
		}
		switch line.Kind {
		case KindRollMeters:
			validateRollLine(prefix, line, add)
			if !line.Meters.IsPositive() {
				add(prefix+".meters", validation.CodeMustBePositive)
			}
			if line.Quantity != 0 {
				add(prefix+".quantity", validation.CodeNotAllowed)
			}
		case KindFullRoll:
			validateRollLine(prefix, line, add)
			if !line.Meters.IsZero() {
				add(prefix+".meters", validation.CodeNotAllowed)
			}
			if line.Quantity != 0 {
				add(prefix+".quantity", validation.CodeNotAllowed)
			}
		case KindTablecloth:
			if line.ProductID == "" {
				add(prefix+".product_id", validation.CodeRequired)
			}
			if line.RollID != "" {
				add(prefix+".roll_id", validation.CodeNotAllowed)
			}
			if !line.Meters.IsZero() {
				add(prefix+".meters", validation.CodeNotAllowed)
			}
			switch {
			case line.Quantity < 1:
				add(prefix+".quantity", validation.CodeMustBePositive)
			case line.Quantity > maxUnits:
				add(prefix+".quantity", validation.CodeOutOfRange)
			}
		default:
			add(prefix+".kind", validation.CodeInvalidValue)
		}
	}
	return problems
}

// validateRollLine: a roll line names its roll; the product is the roll's, never the client's.
func validateRollLine(prefix string, line LineDraft, add func(field, code string)) {
	if line.RollID == "" {
		add(prefix+".roll_id", validation.CodeRequired)
	}
	if line.ProductID != "" {
		add(prefix+".product_id", validation.CodeNotAllowed)
	}
}
