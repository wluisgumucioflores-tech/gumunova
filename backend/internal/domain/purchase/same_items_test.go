package purchase_test

import (
	"testing"
	"time"

	"gumunova/backend/internal/domain/product"
	"gumunova/backend/internal/domain/purchase"
	"gumunova/backend/internal/domain/shared/decimal"
)

func sameItemsDraft() purchase.Draft {
	return purchase.Draft{
		SupplierID:      "supplier-1",
		PurchaseDate:    time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC),
		AdditionalCosts: decimal.MustParse("30"),
		Lines: []purchase.LineDraft{
			{ProductID: "pvc", ProductType: product.TypePVC, Rolls: []purchase.RollDraft{
				{LengthM: decimal.MustParse("19.80"), Cost: decimal.MustParse("280")},
			}},
			{ProductID: "cloth", ProductType: product.TypeTablecloth, Quantity: 10, UnitCost: decimal.MustParse("30")},
		},
	}
}

func buildForSameItems(t *testing.T, draft purchase.Draft) purchase.Purchase {
	t.Helper()
	built, err := purchase.New(draft, time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return built
}

func TestSameItemsIgnoresDataThatDoesNotMoveStock(t *testing.T) {
	original := buildForSameItems(t, sameItemsDraft())
	draft := sameItemsDraft()
	draft.SupplierID, draft.Notes, draft.DocumentReference = "", "otra nota", "FAC-2"
	draft.PurchaseDate = draft.PurchaseDate.AddDate(0, 0, -3)
	if !purchase.SameItems(original, buildForSameItems(t, draft)) {
		t.Error("supplier, date, reference and notes do not change the items")
	}
}

func TestSameItemsDetectsEveryChangeThatMovesStock(t *testing.T) {
	original := buildForSameItems(t, sameItemsDraft())
	changes := map[string]func(*purchase.Draft){
		"roll length":  func(d *purchase.Draft) { d.Lines[0].Rolls[0].LengthM = decimal.MustParse("20") },
		"roll cost":    func(d *purchase.Draft) { d.Lines[0].Rolls[0].Cost = decimal.MustParse("281") },
		"extra roll":   func(d *purchase.Draft) { d.Lines[0].Rolls = append(d.Lines[0].Rolls, d.Lines[0].Rolls[0]) },
		"quantity":     func(d *purchase.Draft) { d.Lines[1].Quantity = 11 },
		"unit cost":    func(d *purchase.Draft) { d.Lines[1].UnitCost = decimal.MustParse("31") },
		"product":      func(d *purchase.Draft) { d.Lines[1].ProductID = "other" },
		"freight":      func(d *purchase.Draft) { d.AdditionalCosts = decimal.MustParse("40") },
		"removed line": func(d *purchase.Draft) { d.Lines = d.Lines[:1] },
	}
	for name, change := range changes {
		draft := sameItemsDraft()
		change(&draft)
		if purchase.SameItems(original, buildForSameItems(t, draft)) {
			t.Errorf("%s: should count as different items", name)
		}
	}
}
