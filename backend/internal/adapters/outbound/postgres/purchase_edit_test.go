package postgres_test

import (
	"context"
	"errors"
	"testing"

	"gumunova/backend/internal/adapters/outbound/postgres"
	"gumunova/backend/internal/application/ports"
	"gumunova/backend/internal/domain/inventory"
	"gumunova/backend/internal/domain/product"
	"gumunova/backend/internal/domain/purchase"
)

func (f purchaseFixture) text(t *testing.T, query string, args ...any) string {
	t.Helper()
	var value string
	if err := f.pool.QueryRow(context.Background(), query, args...).Scan(&value); err != nil {
		t.Fatalf("query %q: %v", query, err)
	}
	return value
}

// register buys 2 PVC rolls (19.80 m/280 and 20 m/300) and 10 tablecloths at 30, with 30 of freight.
func (f purchaseFixture) register(t *testing.T) purchase.Purchase {
	t.Helper()
	result, err := f.repository.Register(context.Background(),
		f.build(t, "30", f.pvcLine([2]string{"19.80", "280"}, [2]string{"20", "300"}), f.clothLine(10, "30")), key("edit"))
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	return result.Purchase
}

func TestEditingOnlyTheDataKeepsRollsAndWritesNoMovements(t *testing.T) {
	f := newPurchaseFixture(t)
	original := f.register(t)
	movementsBefore := f.count(t, "inventory_movements")

	edited := f.build(t, "30", f.pvcLine([2]string{"19.80", "280"}, [2]string{"20", "300"}), f.clothLine(10, "30"))
	edited.Notes = "corregida"
	if purchase.SameItems(original, edited) == false {
		t.Fatal("the items should be the same")
	}
	stored, err := f.repository.Update(context.Background(), original.ID, edited, false)
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	if stored.Notes != "corregida" || stored.Revision != 1 {
		t.Errorf("notes %q, revision %d", stored.Notes, stored.Revision)
	}
	if stored.Lines[0].Rolls[0].Code != original.Lines[0].Rolls[0].Code {
		t.Error("the roll codes must not change when only the data is edited")
	}
	if f.count(t, "inventory_movements") != movementsBefore {
		t.Error("editing only the data must not write movements")
	}
}

func TestReplacingTheItemsReversesTheOldStockAndAddsTheNew(t *testing.T) {
	f := newPurchaseFixture(t)
	original := f.register(t)

	edited := f.build(t, "0", f.pvcLine([2]string{"25", "400"}), f.clothLine(4, "50"))
	stored, err := f.repository.Update(context.Background(), original.ID, edited, true)
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	if stored.Revision != 2 || len(stored.Lines) != 2 || len(stored.Lines[0].Rolls) != 1 {
		t.Fatalf("revision %d, %d lines", stored.Revision, len(stored.Lines))
	}
	if stored.Total.String() != "600.00" {
		t.Errorf("total = %s, want 600.00", stored.Total)
	}
	if got := f.text(t, `SELECT coalesce(sum(available_length_m), 0)::text FROM pvc_rolls`); got != "25.00" {
		t.Errorf("meters in stock = %s, want 25.00 (the replaced rolls must be empty)", got)
	}
	if got := f.text(t, `SELECT quantity::text || '/' || stock_value::text FROM tablecloth_stock`); got != "4/200.00" {
		t.Errorf("tablecloth stock = %s, want 4/200.00", got)
	}
	// History is kept: nothing was deleted, the old rolls are superseded and the ledger nets to zero.
	if got := f.count(t, "pvc_rolls"); got != 3 {
		t.Errorf("rolls on record = %d, want 3", got)
	}
	if got := f.text(t, `SELECT count(*)::text FROM purchase_lines`); got != "4" {
		t.Errorf("lines on record = %s, want 4", got)
	}
	if got := f.text(t, `SELECT sum(quantity_delta)::text FROM inventory_movements WHERE unit = 'm'`); got != "25.00" {
		t.Errorf("net meters in the ledger = %s, want 25.00", got)
	}
	if got := f.text(t, `SELECT sum(quantity_delta)::text FROM inventory_movements WHERE unit = 'unit'`); got != "4.00" {
		t.Errorf("net units in the ledger = %s, want 4.00", got)
	}
	if got := f.text(t, `SELECT count(*)::text FROM inventory_movements WHERE movement_type = 'purchase_reversal'`); got != "3" {
		t.Errorf("reversal movements = %s, want 3 (two rolls and the tablecloths)", got)
	}

	rolls, err := newInventoryRepository(f).ListRolls(context.Background(), ports.RollFilter{Status: ports.RollStatusAll, Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if rolls.Total != 1 {
		t.Errorf("the roll listing shows %d rolls, want only the new one", rolls.Total)
	}
	again, err := f.repository.GetByID(context.Background(), original.ID)
	if err != nil || len(again.Lines) != 2 {
		t.Errorf("reading it back: %v, %d lines", err, len(again.Lines))
	}
}

func TestItemsCannotBeReplacedOnceARollWasSold(t *testing.T) {
	f := newSaleFixture(t)
	purchases, err := f.repository.GetByID(context.Background(), f.text(t, `SELECT id::text FROM purchases`))
	if err != nil {
		t.Fatal(err)
	}
	built := f.build(t, f.meters("ROL-000001", "1", "20"))
	if _, err := f.sales.Register(context.Background(), built, saleKey("one")); err != nil {
		t.Fatalf("sell: %v", err)
	}

	editable, err := f.repository.ItemsEditable(context.Background(), purchases.ID)
	if err != nil || editable {
		t.Fatalf("ItemsEditable = %v, %v; want false", editable, err)
	}
	before := f.count(t, "inventory_movements")
	edited := f.purchaseFixture.build(t, "0", f.pvcLine([2]string{"25", "400"}))
	if _, err := f.repository.Update(context.Background(), purchases.ID, edited, true); !errors.Is(err, purchase.ErrItemsLocked) {
		t.Fatalf("err = %v, want ErrItemsLocked", err)
	}
	if f.count(t, "inventory_movements") != before || f.available(t, "ROL-000001") != "18.80" {
		t.Error("a refused edit must change nothing")
	}

	// The data can still be corrected.
	same := f.purchaseFixture.build(t, "30", f.pvcLine([2]string{"19.80", "280"}, [2]string{"20", "300"}), f.clothLine(10, "30"))
	same.Notes = "solo notas"
	stored, err := f.repository.Update(context.Background(), purchases.ID, same, false)
	if err != nil || stored.Notes != "solo notas" {
		t.Fatalf("data edit: %v", err)
	}
}

func TestTableclothItemsLockOnceTheProductWasSold(t *testing.T) {
	f := newSaleFixture(t)
	id := f.text(t, `SELECT id::text FROM purchases`)
	if _, err := f.sales.Register(context.Background(), f.build(t, f.cloths(1, "50")), saleKey("cloth")); err != nil {
		t.Fatalf("sell: %v", err)
	}
	edited := f.purchaseFixture.build(t, "0", f.clothLine(3, "30"))
	if _, err := f.repository.Update(context.Background(), id, edited, true); !errors.Is(err, purchase.ErrItemsLocked) {
		t.Fatalf("err = %v, want ErrItemsLocked", err)
	}
}

func TestAReplacedRollCannotBeSold(t *testing.T) {
	f := newSaleFixture(t)
	id := f.text(t, `SELECT id::text FROM purchases`)
	if _, err := f.repository.Update(context.Background(), id, f.purchaseFixture.build(t, "0", f.pvcLine([2]string{"25", "400"})), true); err != nil {
		t.Fatalf("update: %v", err)
	}
	_, err := f.sales.Register(context.Background(), f.build(t, f.meters("ROL-000001", "1", "20")), saleKey("old"))
	if err == nil {
		t.Fatal("selling from a replaced roll must fail")
	}
}

func TestUpdatingAMissingPurchase(t *testing.T) {
	f := newPurchaseFixture(t)
	edited := f.build(t, "0", f.clothLine(1, "10"))
	for _, id := range []string{"00000000-0000-0000-0000-000000000000", "not-a-uuid"} {
		if _, err := f.repository.Update(context.Background(), id, edited, true); !errors.Is(err, purchase.ErrNotFound) {
			t.Errorf("Update(%q) = %v, want ErrNotFound", id, err)
		}
	}
}

func TestLabelCodesFindProductsAndRollsButNotReplacedRolls(t *testing.T) {
	f := newPurchaseFixture(t)
	original := f.register(t)
	inventoryRepository := newInventoryRepository(f)
	ctx := context.Background()

	byCode, err := postgres.NewProductRepository(f.pool).GetByLabelCode(ctx, f.pvc.LabelCode)
	if err != nil || byCode.ID != f.pvc.ID {
		t.Fatalf("product by code: %v, %+v", err, byCode)
	}
	if _, err := postgres.NewProductRepository(f.pool).GetByLabelCode(ctx, "P-FFFFFFFFFFFF"); !errors.Is(err, product.ErrNotFound) {
		t.Errorf("unknown product code: %v", err)
	}

	rollCode := f.text(t, `SELECT label_code FROM pvc_rolls WHERE roll_code = 'ROL-000001'`)
	roll, err := inventoryRepository.GetRollByLabelCode(ctx, rollCode)
	if err != nil || roll.Code != "ROL-000001" {
		t.Fatalf("roll by code: %v, %+v", err, roll)
	}

	// Replacing the purchase items retires the old rolls: their labels stop working.
	if _, err := f.repository.Update(ctx, original.ID, f.build(t, "0", f.pvcLine([2]string{"25", "400"})), true); err != nil {
		t.Fatalf("update: %v", err)
	}
	if _, err := inventoryRepository.GetRollByLabelCode(ctx, rollCode); !errors.Is(err, inventory.ErrRollNotFound) {
		t.Errorf("a replaced roll must not resolve: %v", err)
	}
}
