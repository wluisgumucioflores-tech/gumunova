package purchase_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"gumunova/backend/internal/domain/product"
	"gumunova/backend/internal/domain/purchase"
	"gumunova/backend/internal/domain/shared/decimal"
	"gumunova/backend/internal/domain/shared/validation"
)

var now = time.Date(2026, 11, 11, 15, 0, 0, 0, time.UTC)

func d(value string) decimal.Decimal { return decimal.MustParse(value) }

func pvcLine(rolls ...purchase.RollDraft) purchase.LineDraft {
	return purchase.LineDraft{ProductID: "pvc-1", ProductType: product.TypePVC, Rolls: rolls}
}

func roll(length, cost string) purchase.RollDraft {
	return purchase.RollDraft{LengthM: d(length), Cost: d(cost)}
}

func clothLine(quantity int, unitCost string) purchase.LineDraft {
	return purchase.LineDraft{ProductID: "cloth-1", ProductType: product.TypeTablecloth, Quantity: quantity, UnitCost: d(unitCost)}
}

func draft(lines ...purchase.LineDraft) purchase.Draft {
	return purchase.Draft{
		SupplierID:   "sup-1",
		PurchaseDate: time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC),
		Lines:        lines,
	}
}

func problems(t *testing.T, err error) map[string]string {
	t.Helper()
	var validationError *validation.ValidationError
	if !errors.As(err, &validationError) {
		t.Fatalf("expected *ValidationError, got %v", err)
	}
	result := map[string]string{}
	for _, field := range validationError.Fields {
		result[field.Field] = field.Code
	}
	return result
}

func TestNewPVCPurchaseWithoutAdditionalCosts(t *testing.T) {
	created, err := purchase.New(draft(pvcLine(roll("19.80", "280"), roll("20", "300.50"))), now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if created.Status != purchase.StatusConfirmed || created.Total.String() != "580.50" {
		t.Errorf("status %q total %s", created.Status, created.Total)
	}
	line := created.Lines[0]
	if line.Number != 1 || line.Quantity != 2 || len(line.Rolls) != 2 || line.Total.String() != "580.50" {
		t.Errorf("unexpected line: %+v", line)
	}
	first := line.Rolls[0]
	if first.LengthM.String() != "19.80" || first.AcquisitionCost.String() != "280.00" ||
		first.AdditionalCost.String() != "0.00" || first.AttributableCost().String() != "280.00" {
		t.Errorf("unexpected roll: %+v", first)
	}
}

func TestNewTableclothPurchase(t *testing.T) {
	created, err := purchase.New(draft(clothLine(10, "45.25")), now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	line := created.Lines[0]
	if line.Quantity != 10 || line.UnitCost.String() != "45.25" || line.Total.String() != "452.50" || created.Total.String() != "452.50" {
		t.Errorf("unexpected line or total: %+v / %s", line, created.Total)
	}
	if len(line.Rolls) != 0 {
		t.Error("tablecloths never create rolls")
	}
}

func TestAdditionalCostsAreSplitInProportionToCost(t *testing.T) {
	input := draft(
		pvcLine(roll("20", "200"), roll("20", "100")),
		clothLine(2, "50"), // 100.00
	)
	input.AdditionalCosts = d("40")

	created, err := purchase.New(input, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rolls := created.Lines[0].Rolls
	// 400.00 of goods carry 40.00: 10% of each item's cost.
	if rolls[0].AdditionalCost.String() != "20.00" || rolls[1].AdditionalCost.String() != "10.00" {
		t.Errorf("roll shares = %s / %s", rolls[0].AdditionalCost, rolls[1].AdditionalCost)
	}
	if rolls[0].AttributableCost().String() != "220.00" {
		t.Errorf("attributable = %s", rolls[0].AttributableCost())
	}
	cloth := created.Lines[1]
	if cloth.AdditionalCost.String() != "10.00" || cloth.Total.String() != "110.00" {
		t.Errorf("cloth line = %+v", cloth)
	}
	if created.Total.String() != "440.00" {
		t.Errorf("total = %s, want goods + additional", created.Total)
	}
}

func TestAdditionalCostsNeverLoseACent(t *testing.T) {
	input := draft(pvcLine(roll("20", "100"), roll("20", "100"), roll("20", "100")))
	input.AdditionalCosts = d("100")

	created, err := purchase.New(input, now)
	if err != nil {
		t.Fatal(err)
	}

	shares := []string{}
	var sum int64
	for _, r := range created.Lines[0].Rolls {
		shares = append(shares, r.AdditionalCost.String())
		sum += r.AdditionalCost.Hundredths()
	}
	if sum != 10_000 {
		t.Errorf("shares %v add up to %d hundredths, want 10000", shares, sum)
	}
	if created.Lines[0].AdditionalCost.String() != "100.00" || created.Total.String() != "400.00" {
		t.Errorf("line additional %s total %s", created.Lines[0].AdditionalCost, created.Total)
	}
}

func TestNewTrimsTextAndNormalizesTheDate(t *testing.T) {
	input := draft(clothLine(1, "10"))
	input.DocumentReference = "  FAC-001 "
	input.Notes = " nota "
	input.PurchaseDate = time.Date(2026, 11, 10, 23, 59, 0, 0, time.FixedZone("x", -4*3600))

	created, err := purchase.New(input, now)
	if err != nil {
		t.Fatal(err)
	}
	if created.DocumentReference != "FAC-001" || created.Notes != "nota" {
		t.Errorf("text not trimmed: %+v", created)
	}
	if created.PurchaseDate.Hour() != 0 || created.PurchaseDate.Day() != 10 {
		t.Errorf("date = %v, want a plain date", created.PurchaseDate)
	}
}

func TestSupplierIsOptional(t *testing.T) {
	input := draft(clothLine(1, "10"))
	input.SupplierID = ""

	created, err := purchase.New(input, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.SupplierID != "" {
		t.Errorf("SupplierID = %q, want empty", created.SupplierID)
	}
}

func TestDateAndTopLevelValidation(t *testing.T) {
	input := draft()
	input.PurchaseDate = time.Time{}
	input.AdditionalCosts = d("-1")
	input.DocumentReference = strings.Repeat("x", 61)
	input.Notes = strings.Repeat("x", 501)

	_, err := purchase.New(input, now)
	got := problems(t, err)

	want := map[string]string{
		"purchase_date":      validation.CodeRequired,
		"additional_costs":   validation.CodeMustNotBeNegative,
		"document_reference": validation.CodeTooLong,
		"notes":              validation.CodeTooLong,
		"lines":              validation.CodeRequired,
	}
	for field, code := range want {
		if got[field] != code {
			t.Errorf("%s = %q, want %q", field, got[field], code)
		}
	}
}

func TestPurchaseDateInTheFuture(t *testing.T) {
	input := draft(clothLine(1, "10"))

	input.PurchaseDate = now.AddDate(0, 0, 1) // tomorrow is tolerated (time zones)
	if _, err := purchase.New(input, now); err != nil {
		t.Errorf("tomorrow should be accepted: %v", err)
	}
	input.PurchaseDate = now.AddDate(0, 0, 2)
	_, err := purchase.New(input, now)
	if got := problems(t, err)["purchase_date"]; got != validation.CodeInFuture {
		t.Errorf("purchase_date = %q, want in_the_future", got)
	}
}

func TestPVCLineValidation(t *testing.T) {
	line := pvcLine(roll("0", "0"), roll("20", "-5"))
	line.Quantity = 3
	line.UnitCost = d("1")

	_, err := purchase.New(draft(line), now)
	got := problems(t, err)

	want := map[string]string{
		"lines[0].quantity":          validation.CodeNotAllowed,
		"lines[0].unit_cost":         validation.CodeNotAllowed,
		"lines[0].rolls[0].length_m": validation.CodeMustBePositive,
		"lines[0].rolls[0].cost":     validation.CodeMustBePositive,
		"lines[0].rolls[1].cost":     validation.CodeMustBePositive,
	}
	for field, code := range want {
		if got[field] != code {
			t.Errorf("%s = %q, want %q", field, got[field], code)
		}
	}
	if _, bad := got["lines[0].rolls[1].length_m"]; bad {
		t.Error("a valid length must not be reported")
	}
}

func TestPVCLineNeedsRollsButNotTooMany(t *testing.T) {
	_, err := purchase.New(draft(pvcLine()), now)
	if got := problems(t, err)["lines[0].rolls"]; got != validation.CodeRequired {
		t.Errorf("empty rolls = %q", got)
	}

	many := make([]purchase.RollDraft, 101)
	for i := range many {
		many[i] = roll("20", "100")
	}
	_, err = purchase.New(draft(pvcLine(many...)), now)
	if got := problems(t, err)["lines[0].rolls"]; got != validation.CodeTooMany {
		t.Errorf("101 rolls = %q", got)
	}
}

func TestTableclothLineValidation(t *testing.T) {
	line := clothLine(0, "0")
	line.Rolls = []purchase.RollDraft{roll("20", "100")}

	_, err := purchase.New(draft(line), now)
	got := problems(t, err)

	if got["lines[0].rolls"] != validation.CodeNotAllowed || got["lines[0].quantity"] != validation.CodeMustBePositive ||
		got["lines[0].unit_cost"] != validation.CodeMustBePositive {
		t.Errorf("problems = %v", got)
	}

	_, err = purchase.New(draft(clothLine(100_001, "1")), now)
	if got := problems(t, err)["lines[0].quantity"]; got != validation.CodeOutOfRange {
		t.Errorf("quantity above the limit = %q", got)
	}
}

func TestLineNeedsAKnownProduct(t *testing.T) {
	unknown := purchase.LineDraft{ProductID: "", ProductType: "other"}
	_, err := purchase.New(draft(unknown), now)
	got := problems(t, err)
	if got["lines[0].product_id"] == "" {
		t.Errorf("problems = %v", got)
	}
}

func TestTooManyLines(t *testing.T) {
	lines := make([]purchase.LineDraft, 101)
	for i := range lines {
		lines[i] = clothLine(1, "1")
	}
	_, err := purchase.New(draft(lines...), now)
	if got := problems(t, err)["lines"]; got != validation.CodeTooMany {
		t.Errorf("lines = %q", got)
	}
}

func TestHugeAmountsAreRejectedNotWrapped(t *testing.T) {
	line := clothLine(100_000, "9999999999.99")
	input := draft(line, line, line, line, line, line, line, line, line, line)
	if _, err := purchase.New(input, now); err != nil {
		// A very large but representable purchase is fine; an overflow must be a validation error.
		var validationError *validation.ValidationError
		if !errors.As(err, &validationError) {
			t.Fatalf("overflow must surface as a validation error, got %v", err)
		}
	}
}

func TestFingerprint(t *testing.T) {
	base := draft(pvcLine(roll("19.8", "280")), clothLine(2, "10"))
	same := draft(pvcLine(roll("19.80", "280.00")), clothLine(2, "10.00"))
	if base.Fingerprint() != same.Fingerprint() {
		t.Error("the same purchase written differently must have the same fingerprint")
	}

	changes := map[string]func(*purchase.Draft){
		"supplier":   func(x *purchase.Draft) { x.SupplierID = "sup-2" },
		"date":       func(x *purchase.Draft) { x.PurchaseDate = x.PurchaseDate.AddDate(0, 0, 1) },
		"reference":  func(x *purchase.Draft) { x.DocumentReference = "F-1" },
		"notes":      func(x *purchase.Draft) { x.Notes = "n" },
		"additional": func(x *purchase.Draft) { x.AdditionalCosts = d("1") },
		"roll cost":  func(x *purchase.Draft) { x.Lines[0].Rolls[0].Cost = d("281") },
		"quantity":   func(x *purchase.Draft) { x.Lines[1].Quantity = 3 },
	}
	for name, change := range changes {
		other := draft(pvcLine(roll("19.8", "280")), clothLine(2, "10"))
		change(&other)
		if other.Fingerprint() == base.Fingerprint() {
			t.Errorf("changing the %s must change the fingerprint", name)
		}
	}
}

func newPVCProductDraft(sku string) *product.Draft {
	return &product.Draft{SKU: sku, Name: "PVC nuevo", Type: product.TypePVC, PVC: &product.PVCDetails{
		WidthM: d("1.40"), NominalLengthM: d("20"), ReferencePricePerMeter: d("15"), ReferencePricePerRoll: d("280"),
	}}
}

func TestALineCanCreateItsProduct(t *testing.T) {
	line := pvcLine(roll("19.80", "280"))
	line.ProductID = ""
	line.ProductType = "" // the type comes from the new product, never from the client
	line.NewProduct = newPVCProductDraft(" pvc-nuevo ")

	created, err := purchase.New(draft(line), now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	built := created.Lines[0]
	if built.NewProduct == nil || built.NewProduct.SKU != "PVC-NUEVO" || built.ProductType != product.TypePVC {
		t.Errorf("line = %+v", built)
	}
	if len(built.Rolls) != 1 || created.Total.String() != "280.00" {
		t.Errorf("rolls = %d, total %s", len(built.Rolls), created.Total)
	}
}

func TestNewProductProblemsPointToTheLine(t *testing.T) {
	invalid := pvcLine(roll("19.80", "280"))
	invalid.ProductID = ""
	invalid.NewProduct = newPVCProductDraft("x")
	invalid.NewProduct.PVC.WidthM = d("0")

	both := pvcLine(roll("10", "100"))
	both.NewProduct = newPVCProductDraft("PVC-OK")

	_, err := purchase.New(draft(invalid, both), now)
	got := problems(t, err)

	want := map[string]string{
		"lines[0].new_product.sku":         validation.CodeInvalidLength,
		"lines[0].new_product.pvc.width_m": validation.CodeMustBePositive,
		"lines[1].product_id":              validation.CodeNotAllowed,
	}
	for field, code := range want {
		if got[field] != code {
			t.Errorf("%s = %q, want %q (all: %v)", field, got[field], code, got)
		}
	}
}

func TestTheSameNewSKUCannotBeCreatedTwice(t *testing.T) {
	first := pvcLine(roll("10", "100"))
	first.ProductID = ""
	first.NewProduct = newPVCProductDraft("PVC-DUP")
	second := pvcLine(roll("10", "100"))
	second.ProductID = ""
	second.NewProduct = newPVCProductDraft("pvc-dup")

	_, err := purchase.New(draft(first, second), now)

	if got := problems(t, err)["lines[1].new_product.sku"]; got != validation.CodeAlreadyExists {
		t.Errorf("sku = %q", got)
	}
}

func TestFingerprintCoversTheNewProduct(t *testing.T) {
	build := func(name string) purchase.Draft {
		line := pvcLine(roll("10", "100"))
		line.ProductID = ""
		line.NewProduct = newPVCProductDraft("PVC-NUEVO")
		line.NewProduct.Name = name
		return draft(line)
	}
	if build("A").Fingerprint() == build("B").Fingerprint() {
		t.Error("a different new product must change the fingerprint")
	}
	if build("A").Fingerprint() != build("A").Fingerprint() {
		t.Error("the fingerprint must be stable")
	}
}
