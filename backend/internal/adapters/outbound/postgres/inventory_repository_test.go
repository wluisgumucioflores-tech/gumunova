package postgres_test

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"gumunova/backend/internal/adapters/outbound/postgres"
	"gumunova/backend/internal/application/ports"
	"gumunova/backend/internal/domain/inventory"
	"gumunova/backend/internal/domain/product"
)

// registerStock buys 2 PVC rolls (880 of goods + 30 of freight: 289.54 and 310.23 per roll,
// 310.23 for the tablecloth line) so the inventory has something to show.
func registerStock(t *testing.T, f purchaseFixture) {
	t.Helper()
	built := f.build(t, "30",
		f.pvcLine([2]string{"19.80", "280"}, [2]string{"20", "300"}),
		f.clothLine(10, "30"),
	)
	if _, err := f.repository.Register(context.Background(), built, key("stock")); err != nil {
		t.Fatalf("register: %v", err)
	}
}

func newInventoryRepository(f purchaseFixture) *postgres.InventoryRepository {
	return postgres.NewInventoryRepository(f.pool)
}

func stockBySKU(t *testing.T, page ports.StockPage) map[string]ports.StockItem {
	t.Helper()
	result := map[string]ports.StockItem{}
	for _, item := range page.Items {
		result[item.SKU] = item
	}
	return result
}

func TestStockSummarizesPVCAndTableclothsSeparately(t *testing.T) {
	f := newPurchaseFixture(t)
	registerStock(t, f)
	repository := newInventoryRepository(f)

	page, err := repository.ListStock(context.Background(), ports.StockFilter{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	items := stockBySKU(t, page)

	pvc := items["PVC-01"]
	if pvc.Quantity.String() != "39.80" || pvc.RollCount != 2 || pvc.Type != product.TypePVC || pvc.AverageCost != "" {
		t.Errorf("pvc = %+v", pvc)
	}
	// All the cost of two untouched rolls: (280 + 9.54) + (300 + 10.23)... split by cost of 3 items.
	if pvc.StockValue.Hundredths() <= 0 {
		t.Errorf("pvc stock value = %s", pvc.StockValue)
	}
	cloth := items["MAN-01"]
	if cloth.Quantity.String() != "10.00" || cloth.RollCount != 0 || cloth.AverageCost == "" {
		t.Errorf("tablecloth = %+v", cloth)
	}
	if page.Total != 2 {
		t.Errorf("total = %d", page.Total)
	}
}

func TestStockValueAndAverageCostFollowTheCostsOfThePurchase(t *testing.T) {
	f := newPurchaseFixture(t)
	registerStock(t, f)
	page, err := newInventoryRepository(f).ListStock(context.Background(), ports.StockFilter{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	items := stockBySKU(t, page)

	// Goods 880 + 30 of freight split in proportion to cost: rolls 280 and 300, tablecloths 300.
	// The tablecloth line carries 300 + its share, so its average is above its unit cost of 30.
	if items["MAN-01"].StockValue.String() != "310.23" || items["MAN-01"].AverageCost != "31.0230" {
		t.Errorf("tablecloth value %s average %s", items["MAN-01"].StockValue, items["MAN-01"].AverageCost)
	}
	if items["PVC-01"].StockValue.String() != "599.77" {
		t.Errorf("pvc value %s", items["PVC-01"].StockValue)
	}
}

func TestLowStockAlerts(t *testing.T) {
	f := newPurchaseFixture(t)
	registerStock(t, f)
	ctx := context.Background()
	repository := newInventoryRepository(f)

	if _, err := f.pool.Exec(ctx, "UPDATE pvc_product_details SET minimum_stock_m = 50 WHERE product_id = $1", f.pvc.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, "UPDATE tablecloth_product_details SET minimum_units = 10 WHERE product_id = $1", f.cloth.ID); err != nil {
		t.Fatal(err)
	}

	page, err := repository.ListStock(ctx, ports.StockFilter{OnlyLow: true, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	items := stockBySKU(t, page)
	if len(page.Items) != 2 || !items["PVC-01"].IsLow || !items["MAN-01"].IsLow || page.Total != 2 {
		t.Fatalf("low stock = %+v", page)
	}

	// A minimum of zero disables the alert; stock above the minimum is not low.
	f.pool.Exec(ctx, "UPDATE pvc_product_details SET minimum_stock_m = 0 WHERE product_id = $1", f.pvc.ID)
	f.pool.Exec(ctx, "UPDATE tablecloth_product_details SET minimum_units = 9 WHERE product_id = $1", f.cloth.ID)
	page, err = repository.ListStock(ctx, ports.StockFilter{OnlyLow: true, Limit: 10})
	if err != nil || len(page.Items) != 0 {
		t.Errorf("low stock = %+v, err %v", page, err)
	}
}

func TestStockFiltersAndVisibilityOfInactiveVariants(t *testing.T) {
	f := newPurchaseFixture(t)
	registerStock(t, f)
	ctx := context.Background()
	repository := newInventoryRepository(f)

	// A variant that never received stock still appears (with zero) while it is active.
	if _, err := products(t, f).Create(ctx, tableclothProduct("MAN-02", "Mantel azul")); err != nil {
		t.Fatal(err)
	}
	page, _ := repository.ListStock(ctx, ports.StockFilter{Limit: 10})
	if got := stockBySKU(t, page)["MAN-02"]; got.Quantity.String() != "0.00" || got.StockValue.String() != "0.00" || got.AverageCost != "" {
		t.Errorf("empty variant = %+v", got)
	}

	tests := []struct {
		name   string
		filter ports.StockFilter
		want   int
	}{
		{"only pvc", ports.StockFilter{Type: product.TypePVC, Limit: 10}, 1},
		{"only tablecloths", ports.StockFilter{Type: product.TypeTablecloth, Limit: 10}, 2},
		{"search by name", ports.StockFilter{Query: "azul", Limit: 10}, 1},
		{"search by sku", ports.StockFilter{Query: "pvc-0", Limit: 10}, 1},
		{"paging", ports.StockFilter{Limit: 1, Offset: 1}, 1},
	}
	for _, test := range tests {
		page, err := repository.ListStock(ctx, test.filter)
		if err != nil || len(page.Items) != test.want {
			t.Errorf("%s: %d items, err %v", test.name, len(page.Items), err)
		}
	}

	// Deactivating a variant that still has stock keeps it visible; an empty one disappears.
	f.pool.Exec(ctx, "UPDATE products SET is_active = false WHERE sku IN ('MAN-01', 'MAN-02')")
	page, _ = repository.ListStock(ctx, ports.StockFilter{Limit: 10})
	items := stockBySKU(t, page)
	if _, kept := items["MAN-01"]; !kept {
		t.Error("an inactive variant with stock must stay visible")
	}
	if _, kept := items["MAN-02"]; kept {
		t.Error("an inactive variant without stock must be hidden")
	}
}

func TestRollsListingFiltersAndCosts(t *testing.T) {
	f := newPurchaseFixture(t)
	registerStock(t, f)
	ctx := context.Background()
	repository := newInventoryRepository(f)

	page, err := repository.ListRolls(ctx, ports.RollFilter{Limit: 10})
	if err != nil || page.Total != 2 || len(page.Items) != 2 {
		t.Fatalf("rolls = %+v, err %v", page, err)
	}
	var first ports.RollItem
	for _, roll := range page.Items {
		if roll.Code == "ROL-000001" {
			first = roll
		}
	}
	if first.AvailableM.String() != "19.80" || !first.IsIntact() || first.Acquisition.String() != "280.00" {
		t.Errorf("first roll = %+v", first)
	}
	// attributable cost / initial usable length, four decimals; remaining value of an untouched roll is its whole cost.
	if first.CostPerMeter == "" || first.RemainingValue.String() != first.Attributable.String() {
		t.Errorf("cost per meter %q, remaining %s vs %s", first.CostPerMeter, first.RemainingValue, first.Attributable)
	}

	// A cut roll: the value left follows the meters left, and it is no longer intact.
	if _, err := f.pool.Exec(ctx, "UPDATE pvc_rolls SET available_length_m = 9.90 WHERE roll_code = 'ROL-000001'"); err != nil {
		t.Fatal(err)
	}
	cut, err := repository.GetRoll(ctx, first.ID)
	if err != nil || cut.IsIntact() || cut.AvailableM.String() != "9.90" {
		t.Fatalf("cut roll = %+v, err %v", cut, err)
	}
	if cut.RemainingValue.Hundredths() >= cut.Attributable.Hundredths() || cut.RemainingValue.Hundredths() <= 0 {
		t.Errorf("remaining value %s of %s", cut.RemainingValue, cut.Attributable)
	}

	// Depleted rolls are only listed on request.
	f.pool.Exec(ctx, "UPDATE pvc_rolls SET available_length_m = 0 WHERE roll_code = 'ROL-000002'")
	tests := []struct {
		name   string
		filter ports.RollFilter
		codes  []string
	}{
		{"default is available only", ports.RollFilter{Limit: 10}, []string{"ROL-000001"}},
		{"depleted", ports.RollFilter{Status: ports.RollStatusDepleted, Limit: 10}, []string{"ROL-000002"}},
		{"all", ports.RollFilter{Status: ports.RollStatusAll, Limit: 10}, []string{"ROL-000002", "ROL-000001"}},
		{"by product", ports.RollFilter{ProductID: f.pvc.ID, Status: ports.RollStatusAll, Limit: 10}, []string{"ROL-000002", "ROL-000001"}},
		{"by code", ports.RollFilter{Query: "000002", Status: ports.RollStatusAll, Limit: 10}, []string{"ROL-000002"}},
		{"unknown product", ports.RollFilter{ProductID: "00000000-0000-0000-0000-000000000000", Limit: 10}, nil},
		{"malformed product", ports.RollFilter{ProductID: "nope", Limit: 10}, nil},
	}
	for _, test := range tests {
		got, err := repository.ListRolls(ctx, test.filter)
		if err != nil {
			t.Errorf("%s: %v", test.name, err)
			continue
		}
		var codes []string
		for _, roll := range got.Items {
			codes = append(codes, roll.Code)
		}
		if len(codes) != len(test.codes) {
			t.Errorf("%s: codes %v, want %v", test.name, codes, test.codes)
			continue
		}
		for i := range codes {
			if codes[i] != test.codes[i] {
				t.Errorf("%s: codes %v, want %v", test.name, codes, test.codes)
			}
		}
	}
}

func TestGetRollNotFound(t *testing.T) {
	f := newPurchaseFixture(t)
	repository := newInventoryRepository(f)
	for _, id := range []string{"00000000-0000-0000-0000-000000000000", "not-a-uuid", ""} {
		if _, err := repository.GetRoll(context.Background(), id); !errors.Is(err, inventory.ErrRollNotFound) {
			t.Errorf("GetRoll(%q) = %v", id, err)
		}
	}
}

func TestMovementLedger(t *testing.T) {
	f := newPurchaseFixture(t)
	registerStock(t, f)
	ctx := context.Background()
	repository := newInventoryRepository(f)

	all, err := repository.ListMovements(ctx, ports.MovementFilter{Limit: 10})
	if err != nil || all.Total != 3 {
		t.Fatalf("movements = %+v, err %v", all, err)
	}
	var rollMovements, unitMovements int
	for _, movement := range all.Items {
		if movement.Type != inventory.MovementPurchaseIn || movement.ReferenceType != inventory.ReferencePurchase || movement.QuantityDelta.Hundredths() <= 0 {
			t.Errorf("movement = %+v", movement)
		}
		switch movement.Unit {
		case inventory.UnitMeter:
			rollMovements++
			if movement.RollCode == "" || movement.SKU != "PVC-01" {
				t.Errorf("roll movement = %+v", movement)
			}
		case inventory.UnitPiece:
			unitMovements++
			if movement.RollID != "" || movement.QuantityDelta.String() != "10.00" {
				t.Errorf("unit movement = %+v", movement)
			}
		}
	}
	if rollMovements != 2 || unitMovements != 1 {
		t.Errorf("roll movements %d, unit movements %d", rollMovements, unitMovements)
	}

	// Movements of one purchase share a timestamp, so pick a roll movement explicitly.
	var rollID string
	for _, movement := range all.Items {
		if movement.RollID != "" {
			rollID = movement.RollID
		}
	}
	byProduct, _ := repository.ListMovements(ctx, ports.MovementFilter{ProductID: f.cloth.ID, Limit: 10})
	byRoll, _ := repository.ListMovements(ctx, ports.MovementFilter{RollID: rollID, Limit: 10})
	byType, _ := repository.ListMovements(ctx, ports.MovementFilter{Type: inventory.MovementPurchaseIn, Limit: 2})
	malformed, err := repository.ListMovements(ctx, ports.MovementFilter{ProductID: "nope", Limit: 10})
	if byProduct.Total != 1 || byRoll.Total != 1 || byType.Total != 3 || len(byType.Items) != 2 || err != nil || len(malformed.Items) != 0 {
		t.Errorf("by product %d, by roll %d, by type %d/%d, malformed %d", byProduct.Total, byRoll.Total, byType.Total, len(byType.Items), len(malformed.Items))
	}
}

func products(t *testing.T, f purchaseFixture) *postgres.ProductRepository {
	t.Helper()
	return postgres.NewProductRepository(f.pool)
}

func TestEveryRollGetsAUniqueLabelCodeThatCanBeReset(t *testing.T) {
	f := newPurchaseFixture(t)
	registerStock(t, f)
	repository := newInventoryRepository(f)
	ctx := context.Background()

	page, err := repository.ListRolls(ctx, ports.RollFilter{Status: ports.RollStatusAll, Limit: 50})
	if err != nil || len(page.Items) != 2 {
		t.Fatalf("rolls: %v, %d", err, len(page.Items))
	}
	first, second := page.Items[0], page.Items[1]
	pattern := regexp.MustCompile(`^R-[0-9A-F]{12}$`)
	if !pattern.MatchString(first.LabelCode) || first.LabelCode == second.LabelCode {
		t.Fatalf("label codes %q and %q", first.LabelCode, second.LabelCode)
	}

	movementsBefore := f.count(t, "inventory_movements")
	reset, err := repository.ResetRollLabelCode(ctx, first.ID)
	if err != nil {
		t.Fatalf("reset: %v", err)
	}
	if reset.LabelCode == first.LabelCode || reset.Code != first.Code || reset.AvailableM != first.AvailableM {
		t.Errorf("reset = %+v", reset)
	}
	if f.count(t, "inventory_movements") != movementsBefore {
		t.Error("resetting a label code must not write a stock movement")
	}
	if other, _ := repository.GetRoll(ctx, second.ID); other.LabelCode != second.LabelCode {
		t.Error("resetting one roll must not change another")
	}

	for _, id := range []string{"00000000-0000-0000-0000-000000000000", "nope"} {
		if _, err := repository.ResetRollLabelCode(ctx, id); !errors.Is(err, inventory.ErrRollNotFound) {
			t.Errorf("reset %q: %v, want ErrRollNotFound", id, err)
		}
	}
}
