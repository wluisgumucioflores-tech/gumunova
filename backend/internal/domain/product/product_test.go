package product_test

import (
	"errors"
	"strings"
	"testing"

	"gumunova/backend/internal/domain/product"
	"gumunova/backend/internal/domain/shared/decimal"
)

func validPVCDraft() product.Draft {
	return product.Draft{
		SKU:  "pvc-floral-20",
		Name: "PVC floral 20 m",
		Type: product.TypePVC,
		PVC: &product.PVCDetails{
			Design:                 "Floral",
			WidthM:                 decimal.MustParse("1.40"),
			NominalLengthM:         decimal.MustParse("20"),
			ReferencePricePerMeter: decimal.MustParse("15"),
			ReferencePricePerRoll:  decimal.MustParse("280"),
		},
	}
}

func validTableclothDraft() product.Draft {
	return product.Draft{
		SKU:  "MAN-ROJO-01",
		Name: "Mantel rojo",
		Type: product.TypeTablecloth,
		Tablecloth: &product.TableclothDetails{
			Dimensions: "1.40 x 2.00 m",
			UnitPrice:  decimal.MustParse("60"),
		},
	}
}

func fieldCodes(t *testing.T, err error) map[string]string {
	t.Helper()
	var validation *product.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("expected *ValidationError, got %v", err)
	}
	codes := make(map[string]string)
	for _, field := range validation.Fields {
		codes[field.Field] = field.Code
	}
	return codes
}

func TestNewPVCNormalizesAndActivates(t *testing.T) {
	draft := validPVCDraft()
	draft.SKU = "  pvc-floral-20  "
	draft.Name = "  PVC floral 20 m "

	created, err := product.New(draft)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.SKU != "PVC-FLORAL-20" {
		t.Errorf("SKU = %q, want upper-cased and trimmed", created.SKU)
	}
	if created.Name != "PVC floral 20 m" {
		t.Errorf("Name = %q, want trimmed", created.Name)
	}
	if !created.IsActive {
		t.Error("new products must be active")
	}
}

func TestNewTablecloth(t *testing.T) {
	created, err := product.New(validTableclothDraft())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.Type != product.TypeTablecloth || created.Tablecloth == nil || created.PVC != nil {
		t.Errorf("unexpected product shape: %+v", created)
	}
}

func TestNewRejectsInvalidSKU(t *testing.T) {
	tests := []struct {
		name string
		sku  string
		code string
	}{
		{"empty", "   ", product.CodeRequired},
		{"too short", "AB", product.CodeInvalidLength},
		{"too long", strings.Repeat("A", 41), product.CodeInvalidLength},
		{"spaces inside", "PVC FLORAL", product.CodeInvalidFormat},
		{"accents", "PVC-ÑANDÚ", product.CodeInvalidFormat},
		{"starts with hyphen", "-PVC-01", product.CodeInvalidFormat},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			draft := validPVCDraft()
			draft.SKU = test.sku
			_, err := product.New(draft)
			if got := fieldCodes(t, err)["sku"]; got != test.code {
				t.Errorf("sku code = %q, want %q", got, test.code)
			}
		})
	}
}

func TestNewRejectsInvalidPVC(t *testing.T) {
	draft := validPVCDraft()
	draft.Name = ""
	draft.PVC.WidthM = decimal.MustParse("0")
	draft.PVC.NominalLengthM = decimal.MustParse("-1")
	zero := decimal.MustParse("0")
	draft.PVC.ThicknessMM = &zero
	draft.PVC.ReferencePricePerMeter = decimal.MustParse("0")
	draft.PVC.ReferencePricePerRoll = decimal.MustParse("0")
	draft.PVC.Color = strings.Repeat("x", 81)

	_, err := product.New(draft)
	codes := fieldCodes(t, err)

	want := map[string]string{
		"name":                          product.CodeRequired,
		"pvc.width_m":                   product.CodeMustBePositive,
		"pvc.nominal_length_m":          product.CodeMustBePositive,
		"pvc.thickness_mm":              product.CodeMustBePositive,
		"pvc.reference_price_per_meter": product.CodeMustBePositive,
		"pvc.reference_price_per_roll":  product.CodeMustBePositive,
		"pvc.color":                     product.CodeTooLong,
	}
	for field, code := range want {
		if codes[field] != code {
			t.Errorf("%s = %q, want %q", field, codes[field], code)
		}
	}
}

func TestNewRejectsInvalidTablecloth(t *testing.T) {
	draft := validTableclothDraft()
	draft.Tablecloth.UnitPrice = decimal.MustParse("0")
	negative := decimal.MustParse("-1")
	draft.Tablecloth.ReferenceCost = &negative

	_, err := product.New(draft)
	codes := fieldCodes(t, err)
	if codes["tablecloth.unit_price"] != product.CodeMustBePositive {
		t.Errorf("unit_price = %q", codes["tablecloth.unit_price"])
	}
	if codes["tablecloth.reference_cost"] != product.CodeMustNotBeNegative {
		t.Errorf("reference_cost = %q", codes["tablecloth.reference_cost"])
	}
}

func TestNewRejectsDetailsThatDoNotMatchType(t *testing.T) {
	draft := validPVCDraft()
	draft.Tablecloth = validTableclothDraft().Tablecloth
	if got := fieldCodes(t, mustFail(t, draft))["tablecloth"]; got != product.CodeNotAllowed {
		t.Errorf("tablecloth = %q, want not_allowed", got)
	}

	draft = validPVCDraft()
	draft.PVC = nil
	if got := fieldCodes(t, mustFail(t, draft))["pvc"]; got != product.CodeRequired {
		t.Errorf("pvc = %q, want required", got)
	}

	draft = validPVCDraft()
	draft.Type = "other"
	if got := fieldCodes(t, mustFail(t, draft))["product_type"]; got != product.CodeInvalidValue {
		t.Errorf("product_type = %q, want invalid_value", got)
	}
}

func mustFail(t *testing.T, draft product.Draft) error {
	t.Helper()
	_, err := product.New(draft)
	if err == nil {
		t.Fatal("expected an error")
	}
	return err
}

func TestReviseKeepsSKUAndType(t *testing.T) {
	created, err := product.New(validPVCDraft())
	if err != nil {
		t.Fatal(err)
	}

	revision := validTableclothDraft() // wrong SKU and type must be ignored
	revision.SKU = "OTHER-SKU"
	revision.Type = product.TypeTablecloth
	revision.PVC = validPVCDraft().PVC
	revision.Tablecloth = nil
	revision.Name = "  Renamed  "

	if err := created.Revise(revision); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.SKU != "PVC-FLORAL-20" || created.Type != product.TypePVC {
		t.Errorf("SKU/type changed: %s/%s", created.SKU, created.Type)
	}
	if created.Name != "Renamed" {
		t.Errorf("Name = %q", created.Name)
	}
}

func TestReviseRejectsInvalidData(t *testing.T) {
	created, _ := product.New(validPVCDraft())
	revision := validPVCDraft()
	revision.PVC.WidthM = decimal.MustParse("0")
	if err := created.Revise(revision); err == nil {
		t.Fatal("expected validation error")
	}
	if created.PVC.WidthM.String() != "1.40" {
		t.Error("a failed revision must not modify the product")
	}
}

func TestActivateAndDeactivate(t *testing.T) {
	created, _ := product.New(validPVCDraft())
	created.Deactivate()
	created.Deactivate()
	if created.IsActive {
		t.Error("expected inactive")
	}
	created.Activate()
	if !created.IsActive {
		t.Error("expected active")
	}
}

func TestMinimumStockDefaultsToNoAlert(t *testing.T) {
	pvc, err := product.New(validPVCDraft())
	if err != nil || !pvc.PVC.MinimumStockM.IsZero() {
		t.Errorf("pvc minimum = %v, err %v", pvc.PVC, err)
	}
	cloth, err := product.New(validTableclothDraft())
	if err != nil || cloth.Tablecloth.MinimumUnits != 0 {
		t.Errorf("tablecloth minimum = %v, err %v", cloth.Tablecloth, err)
	}
}

func TestMinimumStockRules(t *testing.T) {
	pvc := validPVCDraft()
	pvc.PVC.MinimumStockM = decimal.MustParse("-1")
	_, err := product.New(pvc)
	if got := fieldCodes(t, err)["pvc.minimum_stock_m"]; got != "must_not_be_negative" {
		t.Errorf("pvc minimum code = %q", got)
	}

	for minimum, want := range map[int]string{-1: "must_not_be_negative", 100_001: "out_of_range"} {
		cloth := validTableclothDraft()
		cloth.Tablecloth.MinimumUnits = minimum
		_, err := product.New(cloth)
		if got := fieldCodes(t, err)["tablecloth.minimum_units"]; got != want {
			t.Errorf("minimum %d code = %q, want %q", minimum, got, want)
		}
	}

	ok := validTableclothDraft()
	ok.Tablecloth.MinimumUnits = 5
	if created, err := product.New(ok); err != nil || created.Tablecloth.MinimumUnits != 5 {
		t.Errorf("valid minimum rejected: %v", err)
	}
}
