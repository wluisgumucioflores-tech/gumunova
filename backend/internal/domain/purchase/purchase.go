// Package purchase holds the purchase domain. A confirmed purchase creates one physical roll
// per PVC roll received and adds units of finished tablecloths to stock.
package purchase

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"gumunova/backend/internal/domain/product"
	"gumunova/backend/internal/domain/shared/decimal"
	"gumunova/backend/internal/domain/shared/validation"
)

var (
	ErrNotFound = errors.New("purchase not found")
	// ErrItemsLocked means the lines or rolls of the purchase can no longer be changed because
	// some of what it brought in has already been sold. Its data (supplier, date, notes) can still be edited.
	ErrItemsLocked = errors.New("purchase items cannot be changed: stock from this purchase was already sold")
	// ErrIdempotencyKeyReused means the key was already used for a different purchase.
	ErrIdempotencyKeyReused = errors.New("idempotency key already used for a different purchase")
)

type Status string

const StatusConfirmed Status = "confirmed"

const (
	maxLines             = 100
	maxRollsPerLine      = 100
	maxQuantity          = 100_000
	maxDocumentReference = 60
	maxNotes             = 500
)

// RollDraft is a PVC roll as it was actually received: real usable length and its cost.
type RollDraft struct {
	LengthM decimal.Decimal
	Cost    decimal.Decimal
}

// SKUConflictError means the SKU of a product created by a purchase line is already taken.
type SKUConflictError struct {
	Line int // zero-based index of the line
}

func (e *SKUConflictError) Error() string {
	return fmt.Sprintf("sku of the new product in line %d already exists", e.Line)
}

// LineDraft is one purchased item. A PVC line lists its rolls; a tablecloth line has a
// quantity and a unit cost. ProductType is filled in from the catalog by the use case.
// Instead of ProductID, a line may carry NewProduct: the product is then created together
// with the purchase, so a product bought for the first time needs no prior catalog step.
type LineDraft struct {
	ProductID   string
	NewProduct  *product.Draft
	ProductType product.Type
	Rolls       []RollDraft
	Quantity    int
	UnitCost    decimal.Decimal
}

// Draft is the data entered to register a purchase.
type Draft struct {
	// SupplierID is empty for a purchase without a supplier (for example, a cash purchase).
	SupplierID        string
	PurchaseDate      time.Time // calendar date; the time of day is ignored
	DocumentReference string
	Notes             string
	// AdditionalCosts (freight and similar) are split across the items in proportion to cost.
	AdditionalCosts decimal.Decimal
	Lines           []LineDraft
}

// Roll is a physical PVC roll created by a purchase.
type Roll struct {
	ID              string
	Code            string // assigned by persistence, for example ROL-000125
	ProductID       string
	LengthM         decimal.Decimal // initial usable length
	AcquisitionCost decimal.Decimal
	AdditionalCost  decimal.Decimal // share of the purchase's additional costs
}

// AttributableCost is the roll's own cost plus its share of additional costs. The cost per
// meter derives from it divided by the initial usable length.
func (r Roll) AttributableCost() decimal.Decimal {
	total, _ := r.AcquisitionCost.Add(r.AdditionalCost) // both bounded at creation
	return total
}

type Line struct {
	ID        string
	Number    int
	ProductID string
	// NewProduct is set only while registering a line that creates its product; persistence
	// stores it in the same transaction and fills in ProductID.
	NewProduct  *product.Product
	ProductType product.Type
	Rolls       []Roll          // PVC lines
	Quantity    int             // tablecloth units, or the number of rolls for PVC
	UnitCost    decimal.Decimal // tablecloth lines
	// AdditionalCost is the line's share of the purchase's additional costs.
	AdditionalCost decimal.Decimal
	// Total is the acquisition cost plus the additional cost share.
	Total decimal.Decimal
}

// Purchase is a confirmed purchase. Its supplier, date, reference and notes can always be
// corrected. Its items (lines, rolls, costs) can be replaced only while nothing they brought
// in has been sold; the replacement is recorded with compensating movements.
type Purchase struct {
	ID                string
	SupplierID        string
	PurchaseDate      time.Time
	DocumentReference string
	Notes             string
	AdditionalCosts   decimal.Decimal
	Total             decimal.Decimal
	Status            Status
	Lines             []Line
	// Revision starts at 1 and grows each time the items are replaced.
	Revision  int
	CreatedAt time.Time
}

// New validates a draft and builds the purchase, splitting the additional costs. now is the
// current instant, used only to reject dates in the future.
func New(draft Draft, now time.Time) (Purchase, error) {
	draft.DocumentReference = strings.TrimSpace(draft.DocumentReference)
	draft.Notes = strings.TrimSpace(draft.Notes)

	newProducts, problems := buildNewProducts(&draft)
	problems = append(problems, validate(draft, now)...)
	if len(problems) > 0 {
		return Purchase{}, &validation.ValidationError{Fields: problems}
	}

	purchase := Purchase{
		SupplierID:        draft.SupplierID,
		PurchaseDate:      dateOnly(draft.PurchaseDate),
		DocumentReference: draft.DocumentReference,
		Notes:             draft.Notes,
		AdditionalCosts:   draft.AdditionalCosts,
		Status:            StatusConfirmed,
	}

	// Cost of every item, in order: each PVC roll, and each tablecloth line as a whole.
	var weights []int64
	for _, line := range draft.Lines {
		if line.ProductType == product.TypePVC {
			for _, roll := range line.Rolls {
				weights = append(weights, roll.Cost.Hundredths())
			}
			continue
		}
		total, err := line.UnitCost.MulInt(int64(line.Quantity))
		if err != nil {
			return Purchase{}, outOfRange("lines")
		}
		weights = append(weights, total.Hundredths())
	}
	shares := Allocate(draft.AdditionalCosts.Hundredths(), weights)

	next := 0
	acquisitionTotal := decimal.Decimal{}
	for index, lineDraft := range draft.Lines {
		line := Line{
			Number:      index + 1,
			ProductID:   lineDraft.ProductID,
			NewProduct:  newProducts[index],
			ProductType: lineDraft.ProductType,
		}
		acquisition := decimal.Decimal{}
		additional := decimal.Decimal{}
		var err error

		if lineDraft.ProductType == product.TypePVC {
			line.Quantity = len(lineDraft.Rolls)
			for _, rollDraft := range lineDraft.Rolls {
				share := decimal.FromHundredths(shares[next])
				next++
				line.Rolls = append(line.Rolls, Roll{
					ProductID:       lineDraft.ProductID,
					LengthM:         rollDraft.LengthM,
					AcquisitionCost: rollDraft.Cost,
					AdditionalCost:  share,
				})
				if acquisition, err = acquisition.Add(rollDraft.Cost); err != nil {
					return Purchase{}, outOfRange("lines")
				}
				if additional, err = additional.Add(share); err != nil {
					return Purchase{}, outOfRange("lines")
				}
			}
		} else {
			line.Quantity = lineDraft.Quantity
			line.UnitCost = lineDraft.UnitCost
			if acquisition, err = lineDraft.UnitCost.MulInt(int64(lineDraft.Quantity)); err != nil {
				return Purchase{}, outOfRange("lines")
			}
			additional = decimal.FromHundredths(shares[next])
			next++
		}

		line.AdditionalCost = additional
		if line.Total, err = acquisition.Add(additional); err != nil {
			return Purchase{}, outOfRange("lines")
		}
		if acquisitionTotal, err = acquisitionTotal.Add(acquisition); err != nil {
			return Purchase{}, outOfRange("lines")
		}
		purchase.Lines = append(purchase.Lines, line)
	}

	total, err := acquisitionTotal.Add(draft.AdditionalCosts)
	if err != nil {
		return Purchase{}, outOfRange("lines")
	}
	purchase.Total = total
	return purchase, nil
}

func outOfRange(field string) error {
	return &validation.ValidationError{Fields: []validation.FieldError{{Field: field, Code: validation.CodeOutOfRange}}}
}

func dateOnly(value time.Time) time.Time {
	year, month, day := value.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func validate(draft Draft, now time.Time) []validation.FieldError {
	var problems []validation.FieldError
	add := func(field, code string) { problems = append(problems, validation.FieldError{Field: field, Code: code}) }

	switch {
	case draft.PurchaseDate.IsZero():
		add("purchase_date", validation.CodeRequired)
	case dateOnly(draft.PurchaseDate).After(dateOnly(now).AddDate(0, 0, 1)):
		// One day of tolerance, so the business time zone never rejects "today".
		add("purchase_date", validation.CodeInFuture)
	}

	if utf8.RuneCountInString(draft.DocumentReference) > maxDocumentReference {
		add("document_reference", validation.CodeTooLong)
	}
	if utf8.RuneCountInString(draft.Notes) > maxNotes {
		add("notes", validation.CodeTooLong)
	}
	if draft.AdditionalCosts.IsNegative() {
		add("additional_costs", validation.CodeMustNotBeNegative)
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
		switch {
		case line.NewProduct != nil && strings.TrimSpace(line.ProductID) != "":
			add(prefix+".product_id", validation.CodeNotAllowed)
		case line.NewProduct == nil && strings.TrimSpace(line.ProductID) == "":
			add(prefix+".product_id", validation.CodeRequired)
		}
		switch line.ProductType {
		case product.TypePVC:
			validatePVCLine(prefix, line, add)
		case product.TypeTablecloth:
			validateTableclothLine(prefix, line, add)
		default:
			add(prefix+".product_id", validation.CodeInvalidValue)
		}
	}
	return problems
}

func validatePVCLine(prefix string, line LineDraft, add func(field, code string)) {
	if line.Quantity != 0 {
		add(prefix+".quantity", validation.CodeNotAllowed)
	}
	if !line.UnitCost.IsZero() {
		add(prefix+".unit_cost", validation.CodeNotAllowed)
	}
	switch {
	case len(line.Rolls) == 0:
		add(prefix+".rolls", validation.CodeRequired)
		return
	case len(line.Rolls) > maxRollsPerLine:
		add(prefix+".rolls", validation.CodeTooMany)
		return
	}
	for index, roll := range line.Rolls {
		rollPrefix := fmt.Sprintf("%s.rolls[%d]", prefix, index)
		if !roll.LengthM.IsPositive() {
			add(rollPrefix+".length_m", validation.CodeMustBePositive)
		}
		if !roll.Cost.IsPositive() {
			add(rollPrefix+".cost", validation.CodeMustBePositive)
		}
	}
}

func validateTableclothLine(prefix string, line LineDraft, add func(field, code string)) {
	if len(line.Rolls) != 0 {
		add(prefix+".rolls", validation.CodeNotAllowed)
	}
	switch {
	case line.Quantity < 1:
		add(prefix+".quantity", validation.CodeMustBePositive)
	case line.Quantity > maxQuantity:
		add(prefix+".quantity", validation.CodeOutOfRange)
	}
	if !line.UnitCost.IsPositive() {
		add(prefix+".unit_cost", validation.CodeMustBePositive)
	}
}

// buildNewProducts validates the products that lines ask to create and fixes each such line's
// type from its product (the type is never taken from the client). The result is indexed by
// line. The same SKU may not be created twice in one purchase.
func buildNewProducts(draft *Draft) (map[int]*product.Product, []validation.FieldError) {
	var problems []validation.FieldError
	built := make(map[int]*product.Product)
	seenSKU := make(map[string]bool)

	lines := make([]LineDraft, len(draft.Lines))
	copy(lines, draft.Lines)
	draft.Lines = lines

	for index := range draft.Lines {
		line := &draft.Lines[index]
		if line.NewProduct == nil {
			continue
		}
		prefix := fmt.Sprintf("lines[%d].new_product.", index)
		created, err := product.New(*line.NewProduct)
		var invalid *validation.ValidationError
		if errors.As(err, &invalid) {
			for _, field := range invalid.Fields {
				problems = append(problems, validation.FieldError{Field: prefix + field.Field, Code: field.Code})
			}
			line.ProductType = line.NewProduct.Type
			continue
		}
		if seenSKU[created.SKU] {
			problems = append(problems, validation.FieldError{Field: prefix + "sku", Code: validation.CodeAlreadyExists})
		}
		seenSKU[created.SKU] = true
		line.ProductType = created.Type
		built[index] = &created
	}
	return built, problems
}
