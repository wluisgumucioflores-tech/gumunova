package sale_test

import (
	"testing"

	"gumunova/backend/internal/domain/sale"
	"gumunova/backend/internal/domain/shared/validation"
)

// A roll of 19.80 m that cost 289.54 (280 plus 9.54 of freight).
func inventoryWithRoll() *sale.Inventory {
	return &sale.Inventory{
		Rolls: map[string]*sale.RollStock{
			"roll-1": {
				ID: "roll-1", Code: "ROL-000001", ProductID: "pvc-1", SKU: "PVC-01", Name: "PVC floral", ProductActive: true,
				InitialM: d("19.80"), AvailableM: d("19.80"), Attributable: d("289.54"),
				ReferencePricePerMeter: d("15.50"), ReferencePricePerRoll: d("280"),
			},
		},
		Tablecloths: map[string]*sale.TableclothStock{
			"cloth-1": {ProductID: "cloth-1", SKU: "MAN-01", Name: "Mantel", ProductActive: true, Units: 10, Value: d("310.23"), ReferencePrice: d("60")},
		},
		NotTablecloth: map[string]bool{"pvc-1": true},
	}
}

func newSale(t *testing.T, lines ...sale.LineDraft) *sale.Sale {
	t.Helper()
	built, err := sale.New(draft(lines...), now)
	if err != nil {
		t.Fatalf("new sale: %v", err)
	}
	return &built
}

func codes(found []validation.FieldError) map[string]string {
	result := map[string]string{}
	for _, problem := range found {
		result[problem.Field] = problem.Code
	}
	return result
}

func TestSellingMetersFromARoll(t *testing.T) {
	inventory := inventoryWithRoll()
	built := newSale(t, sale.LineDraft{Kind: sale.KindRollMeters, RollID: "roll-1", Meters: d("2.50"), UnitPrice: d("18")})

	if found := built.Allocate(inventory); len(found) != 0 {
		t.Fatalf("problems: %v", found)
	}

	line := built.Lines[0]
	if line.ProductID != "pvc-1" || line.RollCode != "ROL-000001" || line.ReferencePrice.String() != "15.50" {
		t.Errorf("line = %+v", line)
	}
	if line.LineTotal.String() != "45.00" { // 2.5 m x 18
		t.Errorf("total = %s", line.LineTotal)
	}
	// 289.54 x 2.50 / 19.80 = 36.5580...
	if line.CostOfSale.String() != "36.56" || line.GrossProfit().String() != "8.44" {
		t.Errorf("cost %s profit %s", line.CostOfSale, line.GrossProfit())
	}
	if inventory.Rolls["roll-1"].AvailableM.String() != "17.30" || built.Total.String() != "45.00" || built.TotalCost.String() != "36.56" {
		t.Errorf("available %s, totals %s / %s", inventory.Rolls["roll-1"].AvailableM, built.Total, built.TotalCost)
	}
}

func TestSellingMoreMetersThanTheRollHasIsRefused(t *testing.T) {
	built := newSale(t, sale.LineDraft{Kind: sale.KindRollMeters, RollID: "roll-1", Meters: d("19.81"), UnitPrice: d("18")})

	found := built.Allocate(inventoryWithRoll())

	if codes(found)["lines[0].meters"] != validation.CodeInsufficientStock {
		t.Errorf("problems = %v", found)
	}
}

func TestSellingEveryMeterIsAllowedAndTakesTheWholeCost(t *testing.T) {
	inventory := inventoryWithRoll()
	built := newSale(t, sale.LineDraft{Kind: sale.KindRollMeters, RollID: "roll-1", Meters: d("19.80"), UnitPrice: d("15")})

	if found := built.Allocate(inventory); len(found) != 0 {
		t.Fatalf("problems: %v", found)
	}

	if built.Lines[0].CostOfSale.String() != "289.54" || !inventory.Rolls["roll-1"].AvailableM.IsZero() {
		t.Errorf("cost %s available %s", built.Lines[0].CostOfSale, inventory.Rolls["roll-1"].AvailableM)
	}
}

func TestTwoLinesOnTheSameRollSeeWhatTheFirstTook(t *testing.T) {
	inventory := inventoryWithRoll()
	built := newSale(t,
		sale.LineDraft{Kind: sale.KindRollMeters, RollID: "roll-1", Meters: d("15"), UnitPrice: d("18")},
		sale.LineDraft{Kind: sale.KindRollMeters, RollID: "roll-1", Meters: d("5"), UnitPrice: d("18")},
	)

	found := built.Allocate(inventory)

	if codes(found)["lines[1].meters"] != validation.CodeInsufficientStock || codes(found)["lines[0].meters"] != "" {
		t.Errorf("problems = %v", found)
	}
}

func TestTheCostsOfSuccessiveSalesOfARollAddUpToItsCost(t *testing.T) {
	inventory := inventoryWithRoll()
	var totalCost int64
	for _, meters := range []string{"7.33", "5.01", "7.46"} { // 19.80 in three cuts
		built := newSale(t, sale.LineDraft{Kind: sale.KindRollMeters, RollID: "roll-1", Meters: d(meters), UnitPrice: d("10")})
		if found := built.Allocate(inventory); len(found) != 0 {
			t.Fatalf("problems: %v", found)
		}
		totalCost += built.TotalCost.Hundredths()
	}

	if totalCost != 28954 {
		t.Errorf("costs add up to %d hundredths, want 28954", totalCost)
	}
}

func TestSellingAWholeIntactRoll(t *testing.T) {
	inventory := inventoryWithRoll()
	built := newSale(t, sale.LineDraft{Kind: sale.KindFullRoll, RollID: "roll-1", UnitPrice: d("350")})

	if found := built.Allocate(inventory); len(found) != 0 {
		t.Fatalf("problems: %v", found)
	}

	line := built.Lines[0]
	if line.LineTotal.String() != "350.00" || line.CostOfSale.String() != "289.54" || line.ReferencePrice.String() != "280.00" {
		t.Errorf("line = %+v", line)
	}
	if !inventory.Rolls["roll-1"].AvailableM.IsZero() {
		t.Errorf("available = %s", inventory.Rolls["roll-1"].AvailableM)
	}
}

func TestACutRollCannotBeSoldAsWhole(t *testing.T) {
	inventory := inventoryWithRoll()
	inventory.Rolls["roll-1"].AvailableM = d("19.79") // one centimeter already left

	built := newSale(t, sale.LineDraft{Kind: sale.KindFullRoll, RollID: "roll-1", UnitPrice: d("350")})
	found := built.Allocate(inventory)

	if codes(found)["lines[0].roll_id"] != validation.CodeRollNotIntact {
		t.Errorf("problems = %v", found)
	}
}

func TestARollCannotBeSoldWholeAfterAnEarlierLineCutIt(t *testing.T) {
	built := newSale(t,
		sale.LineDraft{Kind: sale.KindRollMeters, RollID: "roll-1", Meters: d("1"), UnitPrice: d("18")},
		sale.LineDraft{Kind: sale.KindFullRoll, RollID: "roll-1", UnitPrice: d("350")},
	)

	found := built.Allocate(inventoryWithRoll())

	if codes(found)["lines[1].roll_id"] != validation.CodeRollNotIntact {
		t.Errorf("problems = %v", found)
	}
}

func TestSellingTheSameWholeRollTwiceInOneSaleIsRefused(t *testing.T) {
	built := newSale(t,
		sale.LineDraft{Kind: sale.KindFullRoll, RollID: "roll-1", UnitPrice: d("350")},
		sale.LineDraft{Kind: sale.KindFullRoll, RollID: "roll-1", UnitPrice: d("350")},
	)

	found := built.Allocate(inventoryWithRoll())

	if codes(found)["lines[1].roll_id"] != validation.CodeRollNotIntact {
		t.Errorf("problems = %v", found)
	}
}

func TestSellingTablecloths(t *testing.T) {
	inventory := inventoryWithRoll()
	built := newSale(t, sale.LineDraft{Kind: sale.KindTablecloth, ProductID: "cloth-1", Quantity: 4, UnitPrice: d("65")})

	if found := built.Allocate(inventory); len(found) != 0 {
		t.Fatalf("problems: %v", found)
	}

	line := built.Lines[0]
	if line.LineTotal.String() != "260.00" || line.ReferencePrice.String() != "60.00" || line.ProductSKU != "MAN-01" {
		t.Errorf("line = %+v", line)
	}
	// 310.23 x 4 / 10 = 124.092
	if line.CostOfSale.String() != "124.09" {
		t.Errorf("cost = %s", line.CostOfSale)
	}
	stock := inventory.Tablecloths["cloth-1"]
	if stock.Units != 6 || stock.Value.String() != "186.14" {
		t.Errorf("stock left %d units worth %s", stock.Units, stock.Value)
	}
}

func TestSellingAllTheTableclothsLeavesNoValueBehind(t *testing.T) {
	inventory := inventoryWithRoll()
	for _, quantity := range []int{3, 3, 4} {
		built := newSale(t, sale.LineDraft{Kind: sale.KindTablecloth, ProductID: "cloth-1", Quantity: quantity, UnitPrice: d("65")})
		if found := built.Allocate(inventory); len(found) != 0 {
			t.Fatalf("problems: %v", found)
		}
	}

	stock := inventory.Tablecloths["cloth-1"]
	if stock.Units != 0 || !stock.Value.IsZero() {
		t.Errorf("stock left %d units worth %s: no value may remain once the units are gone", stock.Units, stock.Value)
	}
}

func TestNotEnoughTableclothsIsRefused(t *testing.T) {
	built := newSale(t, sale.LineDraft{Kind: sale.KindTablecloth, ProductID: "cloth-1", Quantity: 11, UnitPrice: d("65")})

	found := built.Allocate(inventoryWithRoll())

	if codes(found)["lines[0].quantity"] != validation.CodeInsufficientStock {
		t.Errorf("problems = %v", found)
	}
}

func TestUnknownWrongTypeAndInactiveReferences(t *testing.T) {
	inventory := inventoryWithRoll()
	inventory.Rolls["roll-off"] = &sale.RollStock{ID: "roll-off", ProductActive: false, InitialM: d("5"), AvailableM: d("5")}
	inventory.Tablecloths["cloth-off"] = &sale.TableclothStock{ProductID: "cloth-off", ProductActive: false, Units: 5, Value: d("50")}

	built := newSale(t,
		sale.LineDraft{Kind: sale.KindRollMeters, RollID: "nope", Meters: d("1"), UnitPrice: d("1")},
		sale.LineDraft{Kind: sale.KindRollMeters, RollID: "roll-off", Meters: d("1"), UnitPrice: d("1")},
		sale.LineDraft{Kind: sale.KindTablecloth, ProductID: "nope", Quantity: 1, UnitPrice: d("1")},
		sale.LineDraft{Kind: sale.KindTablecloth, ProductID: "pvc-1", Quantity: 1, UnitPrice: d("1")},
		sale.LineDraft{Kind: sale.KindTablecloth, ProductID: "cloth-off", Quantity: 1, UnitPrice: d("1")},
	)

	got := codes(built.Allocate(inventory))

	want := map[string]string{
		"lines[0].roll_id":    validation.CodeNotFound,
		"lines[1].roll_id":    validation.CodeInactive,
		"lines[2].product_id": validation.CodeNotFound,
		"lines[3].product_id": validation.CodeWrongType,
		"lines[4].product_id": validation.CodeInactive,
	}
	for field, code := range want {
		if got[field] != code {
			t.Errorf("%s = %q, want %q (all: %v)", field, got[field], code, got)
		}
	}
}

func TestATableclothWithNoStockRowIsSimplyOutOfStock(t *testing.T) {
	inventory := inventoryWithRoll()
	inventory.Tablecloths["cloth-new"] = &sale.TableclothStock{ProductID: "cloth-new", ProductActive: true, Units: 0}
	built := newSale(t, sale.LineDraft{Kind: sale.KindTablecloth, ProductID: "cloth-new", Quantity: 1, UnitPrice: d("65")})

	found := built.Allocate(inventory)

	if codes(found)["lines[0].quantity"] != validation.CodeInsufficientStock {
		t.Errorf("problems = %v", found)
	}
}

func TestTotalsOfAMixedSale(t *testing.T) {
	built := newSale(t,
		sale.LineDraft{Kind: sale.KindRollMeters, RollID: "roll-1", Meters: d("2"), UnitPrice: d("20")},
		sale.LineDraft{Kind: sale.KindTablecloth, ProductID: "cloth-1", Quantity: 2, UnitPrice: d("70")},
	)

	if found := built.Allocate(inventoryWithRoll()); len(found) != 0 {
		t.Fatalf("problems: %v", found)
	}

	if built.Total.String() != "180.00" { // 40 + 140
		t.Errorf("total = %s", built.Total)
	}
	if built.GrossProfit().Hundredths() != built.Total.Hundredths()-built.TotalCost.Hundredths() {
		t.Error("gross profit must be total minus cost")
	}
}
