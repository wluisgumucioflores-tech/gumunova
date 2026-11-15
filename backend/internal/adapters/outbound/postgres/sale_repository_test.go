package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"gumunova/backend/internal/adapters/outbound/postgres"
	"gumunova/backend/internal/application/ports"
	"gumunova/backend/internal/domain/inventory"
	"gumunova/backend/internal/domain/sale"
	"gumunova/backend/internal/domain/shared/decimal"
	"gumunova/backend/internal/domain/shared/validation"
)

type saleFixture struct {
	purchaseFixture
	sales   *postgres.SaleRepository
	rollIDs map[string]string // roll code -> id
}

// newSaleFixture buys two PVC rolls (19.80 m costing 289.54, and 20 m costing 310.23 once the
// freight is shared) and 10 tablecloths worth 310.23, so there is stock to sell.
func newSaleFixture(t *testing.T) saleFixture {
	t.Helper()
	base := newPurchaseFixture(t)
	registerStock(t, base)

	f := saleFixture{purchaseFixture: base, sales: postgres.NewSaleRepository(base.pool), rollIDs: map[string]string{}}
	rows, err := base.pool.Query(context.Background(), `SELECT roll_code, id::text FROM pvc_rolls`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var code, id string
		if err := rows.Scan(&code, &id); err != nil {
			t.Fatal(err)
		}
		f.rollIDs[code] = id
	}
	return f
}

var saleDay = time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)

func (f saleFixture) build(t *testing.T, lines ...sale.LineDraft) sale.Sale {
	t.Helper()
	built, err := sale.New(sale.Draft{SaleDate: saleDay, Lines: lines}, time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("build sale: %v", err)
	}
	return built
}

func (f saleFixture) meters(code, meters, price string) sale.LineDraft {
	return sale.LineDraft{Kind: sale.KindRollMeters, RollID: f.rollIDs[code], Meters: decimal.MustParse(meters), UnitPrice: decimal.MustParse(price)}
}

func (f saleFixture) wholeRoll(code, price string) sale.LineDraft {
	return sale.LineDraft{Kind: sale.KindFullRoll, RollID: f.rollIDs[code], UnitPrice: decimal.MustParse(price)}
}

func (f saleFixture) cloths(quantity int, price string) sale.LineDraft {
	return sale.LineDraft{Kind: sale.KindTablecloth, ProductID: f.cloth.ID, Quantity: quantity, UnitPrice: decimal.MustParse(price)}
}

func saleKey(name string) ports.IdempotencyKey {
	return ports.IdempotencyKey{Key: "sale-" + name, Fingerprint: "fp-" + name}
}

func (f saleFixture) available(t *testing.T, code string) string {
	t.Helper()
	var text string
	if err := f.pool.QueryRow(context.Background(), `SELECT available_length_m::text FROM pvc_rolls WHERE roll_code = $1`, code).Scan(&text); err != nil {
		t.Fatal(err)
	}
	return text
}

func (f saleFixture) clothStock(t *testing.T) (int, string) {
	t.Helper()
	var units int
	var value string
	if err := f.pool.QueryRow(context.Background(), `SELECT quantity, stock_value::text FROM tablecloth_stock WHERE product_id = $1`, f.cloth.ID).Scan(&units, &value); err != nil {
		t.Fatal(err)
	}
	return units, value
}

func fieldCodesOf(t *testing.T, err error) map[string]string {
	t.Helper()
	var invalid *validation.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("expected a validation error, got %v", err)
	}
	codes := map[string]string{}
	for _, field := range invalid.Fields {
		codes[field.Field] = field.Code
	}
	return codes
}

func TestSellingMetersReducesTheRollAndWritesTheMovement(t *testing.T) {
	f := newSaleFixture(t)
	ctx := context.Background()

	result, err := f.sales.Register(ctx, f.build(t, f.meters("ROL-000001", "2.50", "18")), saleKey("a"))
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	stored := result.Sale
	line := stored.Lines[0]
	if stored.ID == "" || stored.Total.String() != "45.00" || line.LineTotal.String() != "45.00" || line.ReferencePrice.String() != "15.50" {
		t.Errorf("sale = %+v", stored)
	}
	if line.RollCode != "ROL-000001" || line.ProductSKU != "PVC-01" || line.Meters.String() != "2.50" || line.CostOfSale.Hundredths() <= 0 {
		t.Errorf("line = %+v", line)
	}
	if got := f.available(t, "ROL-000001"); got != "17.30" {
		t.Errorf("available = %s, want 17.30", got)
	}

	movements, err := postgres.NewInventoryRepository(f.pool).ListMovements(ctx, ports.MovementFilter{Type: inventory.MovementSaleOut, Limit: 10})
	if err != nil || movements.Total != 1 {
		t.Fatalf("movements = %+v, err %v", movements, err)
	}
	movement := movements.Items[0]
	if movement.QuantityDelta.String() != "-2.50" || movement.Unit != inventory.UnitMeter || movement.RollCode != "ROL-000001" ||
		movement.ReferenceType != inventory.ReferenceSale || movement.ReferenceID != stored.ID {
		t.Errorf("movement = %+v", movement)
	}

	loaded, err := f.sales.GetByID(ctx, stored.ID)
	if err != nil || loaded.Total.String() != "45.00" || loaded.TotalCost.String() != stored.TotalCost.String() || len(loaded.Lines) != 1 {
		t.Errorf("loaded = %+v, err %v", loaded, err)
	}
}

func TestTheCostOfRollSalesAddsUpExactlyToTheCostOfTheRoll(t *testing.T) {
	f := newSaleFixture(t)
	ctx := context.Background()
	var attributable string
	f.pool.QueryRow(ctx, `SELECT (acquisition_cost + additional_cost)::text FROM pvc_rolls WHERE roll_code = 'ROL-000001'`).Scan(&attributable)

	var sold int64
	for index, meters := range []string{"7.33", "5.01", "7.46"} { // the 19.80 m in three cuts
		result, err := f.sales.Register(ctx, f.build(t, f.meters("ROL-000001", meters, "10")), saleKey(fmt.Sprint(index)))
		if err != nil {
			t.Fatalf("cut %d: %v", index+1, err)
		}
		sold += result.Sale.TotalCost.Hundredths()
	}

	if got := decimal.FromHundredths(sold).String(); got != attributable || f.available(t, "ROL-000001") != "0.00" {
		t.Errorf("costs add up to %s, roll cost %s, available %s", got, attributable, f.available(t, "ROL-000001"))
	}
}

func TestSellingAWholeRoll(t *testing.T) {
	f := newSaleFixture(t)
	ctx := context.Background()

	result, err := f.sales.Register(ctx, f.build(t, f.wholeRoll("ROL-000001", "350")), saleKey("a"))
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	line := result.Sale.Lines[0]
	if line.LineTotal.String() != "350.00" || line.ReferencePrice.String() != "280.00" || line.Sold().String() != "1.00" {
		t.Errorf("line = %+v", line)
	}
	if f.available(t, "ROL-000001") != "0.00" {
		t.Errorf("available = %s", f.available(t, "ROL-000001"))
	}
	movements, _ := postgres.NewInventoryRepository(f.pool).ListMovements(ctx, ports.MovementFilter{Type: inventory.MovementSaleOut, Limit: 10})
	if movements.Items[0].QuantityDelta.String() != "-19.80" {
		t.Errorf("a whole roll leaves with its whole length: %s", movements.Items[0].QuantityDelta)
	}
}

func TestACutRollCannotBeSoldWholeAndNothingIsSaved(t *testing.T) {
	f := newSaleFixture(t)
	ctx := context.Background()
	if _, err := f.sales.Register(ctx, f.build(t, f.meters("ROL-000001", "1", "18")), saleKey("a")); err != nil {
		t.Fatal(err)
	}
	salesBefore := f.count(t, "sales")

	_, err := f.sales.Register(ctx, f.build(t, f.wholeRoll("ROL-000001", "350")), saleKey("b"))

	if codes := fieldCodesOf(t, err); codes["lines[0].roll_id"] != validation.CodeRollNotIntact {
		t.Errorf("codes = %v", codes)
	}
	if f.count(t, "sales") != salesBefore || f.available(t, "ROL-000001") != "18.80" {
		t.Errorf("sales %d, available %s: a refused sale must leave no trace", f.count(t, "sales"), f.available(t, "ROL-000001"))
	}
}

func TestSellingTablecloths(t *testing.T) {
	f := newSaleFixture(t)
	ctx := context.Background()

	result, err := f.sales.Register(ctx, f.build(t, f.cloths(4, "65")), saleKey("a"))
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	line := result.Sale.Lines[0]
	if line.LineTotal.String() != "260.00" || line.CostOfSale.String() != "124.09" || line.ReferencePrice.String() != "60.00" || line.Quantity != 4 {
		t.Errorf("line = %+v", line)
	}
	if units, value := f.clothStock(t); units != 6 || value != "186.14" {
		t.Errorf("stock = %d units worth %s", units, value)
	}

	// Selling the rest leaves no value behind.
	if _, err := f.sales.Register(ctx, f.build(t, f.cloths(6, "65")), saleKey("b")); err != nil {
		t.Fatal(err)
	}
	if units, value := f.clothStock(t); units != 0 || value != "0.00" {
		t.Errorf("stock = %d units worth %s", units, value)
	}
	movements, _ := postgres.NewInventoryRepository(f.pool).ListMovements(ctx, ports.MovementFilter{ProductID: f.cloth.ID, Type: inventory.MovementSaleOut, Limit: 10})
	if movements.Total != 2 || movements.Items[0].Unit != inventory.UnitPiece || movements.Items[0].RollID != "" {
		t.Errorf("movements = %+v", movements)
	}
}

func TestOneBadLineSavesNothingFromTheOthers(t *testing.T) {
	f := newSaleFixture(t)
	ctx := context.Background()
	movementsBefore := f.count(t, "inventory_movements")

	_, err := f.sales.Register(ctx, f.build(t,
		f.meters("ROL-000001", "2", "18"),
		f.cloths(3, "65"),
		f.cloths(8, "65"), // only 7 are left after the previous line
	), saleKey("a"))

	if codes := fieldCodesOf(t, err); codes["lines[2].quantity"] != validation.CodeInsufficientStock {
		t.Errorf("codes = %v", codes)
	}
	for table, want := range map[string]int{"sales": 0, "sale_lines": 0, "inventory_movements": movementsBefore} {
		if got := f.count(t, table); got != want {
			t.Errorf("%s has %d rows, want %d", table, got, want)
		}
	}
	if units, value := f.clothStock(t); units != 10 || value != "310.23" || f.available(t, "ROL-000001") != "19.80" {
		t.Errorf("stock changed: %d units worth %s, roll %s", units, value, f.available(t, "ROL-000001"))
	}
}

func TestReferencesAreChecked(t *testing.T) {
	f := newSaleFixture(t)
	ctx := context.Background()
	missing := "00000000-0000-0000-0000-000000000000"

	_, err := f.sales.Register(ctx, f.build(t,
		sale.LineDraft{Kind: sale.KindRollMeters, RollID: missing, Meters: decimal.MustParse("1"), UnitPrice: decimal.MustParse("1")},
		sale.LineDraft{Kind: sale.KindRollMeters, RollID: "not-a-uuid", Meters: decimal.MustParse("1"), UnitPrice: decimal.MustParse("1")},
		sale.LineDraft{Kind: sale.KindTablecloth, ProductID: missing, Quantity: 1, UnitPrice: decimal.MustParse("1")},
		sale.LineDraft{Kind: sale.KindTablecloth, ProductID: f.pvc.ID, Quantity: 1, UnitPrice: decimal.MustParse("1")},
	), saleKey("a"))

	codes := fieldCodesOf(t, err)
	want := map[string]string{
		"lines[0].roll_id": validation.CodeNotFound, "lines[1].roll_id": validation.CodeNotFound,
		"lines[2].product_id": validation.CodeNotFound, "lines[3].product_id": validation.CodeWrongType,
	}
	for field, code := range want {
		if codes[field] != code {
			t.Errorf("%s = %q, want %q (all %v)", field, codes[field], code, codes)
		}
	}
}

func TestADeactivatedProductCannotBeSold(t *testing.T) {
	f := newSaleFixture(t)
	f.pool.Exec(context.Background(), `UPDATE products SET is_active = false`)

	_, err := f.sales.Register(context.Background(), f.build(t, f.meters("ROL-000001", "1", "18"), f.cloths(1, "65")), saleKey("a"))

	codes := fieldCodesOf(t, err)
	if codes["lines[0].roll_id"] != validation.CodeInactive || codes["lines[1].product_id"] != validation.CodeInactive {
		t.Errorf("codes = %v", codes)
	}
}

func TestRetryWithTheSameKeyReturnsTheSameSaleWithoutSellingAgain(t *testing.T) {
	f := newSaleFixture(t)
	ctx := context.Background()
	built := f.build(t, f.meters("ROL-000001", "2.50", "18"), f.cloths(2, "65"))
	first, err := f.sales.Register(ctx, built, saleKey("a"))
	if err != nil {
		t.Fatal(err)
	}

	retry, err := f.sales.Register(ctx, built, saleKey("a"))

	if err != nil || !retry.Replayed || retry.Sale.ID != first.Sale.ID || retry.Sale.Total.String() != first.Sale.Total.String() {
		t.Fatalf("retry = %+v, err %v", retry, err)
	}
	if f.count(t, "sales") != 1 || f.count(t, "sale_lines") != 2 || f.available(t, "ROL-000001") != "17.30" {
		t.Error("a retry must not sell anything again")
	}
	if units, _ := f.clothStock(t); units != 8 {
		t.Errorf("units = %d", units)
	}
}

func TestReusingAKeyForADifferentSaleIsRejected(t *testing.T) {
	f := newSaleFixture(t)
	ctx := context.Background()
	if _, err := f.sales.Register(ctx, f.build(t, f.cloths(1, "65")), saleKey("a")); err != nil {
		t.Fatal(err)
	}

	_, err := f.sales.Register(ctx, f.build(t, f.cloths(2, "65")), ports.IdempotencyKey{Key: "sale-a", Fingerprint: "another"})

	if !errors.Is(err, sale.ErrIdempotencyKeyReused) {
		t.Errorf("error = %v", err)
	}
}

func TestTwoSalesCannotTakeTheSameMeters(t *testing.T) {
	f := newSaleFixture(t)
	ctx := context.Background()

	// 15 m twice from a 19.80 m roll: only one of them fits.
	outcomes := runConcurrently(2, func(index int) error {
		_, err := f.sales.Register(ctx, f.build(t, f.meters("ROL-000001", "15", "18")), saleKey(fmt.Sprint("race", index)))
		return err
	})

	succeeded, refused := 0, 0
	for _, err := range outcomes {
		var invalid *validation.ValidationError
		switch {
		case err == nil:
			succeeded++
		case errors.As(err, &invalid) && fieldCodesOf(t, err)["lines[0].meters"] == validation.CodeInsufficientStock:
			refused++
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}
	if succeeded != 1 || refused != 1 || f.available(t, "ROL-000001") != "4.80" {
		t.Errorf("succeeded %d, refused %d, available %s", succeeded, refused, f.available(t, "ROL-000001"))
	}
}

func TestTwoSalesCannotTakeTheSameWholeRoll(t *testing.T) {
	f := newSaleFixture(t)
	ctx := context.Background()

	outcomes := runConcurrently(6, func(index int) error {
		_, err := f.sales.Register(ctx, f.build(t, f.wholeRoll("ROL-000001", "350")), saleKey(fmt.Sprint("whole", index)))
		return err
	})

	succeeded := 0
	for _, err := range outcomes {
		if err == nil {
			succeeded++
		} else if fieldCodesOf(t, err)["lines[0].roll_id"] != validation.CodeRollNotIntact {
			t.Errorf("unexpected error: %v", err)
		}
	}
	if succeeded != 1 || f.count(t, "sales") != 1 {
		t.Errorf("succeeded %d, sales %d: a roll can be sold whole only once", succeeded, f.count(t, "sales"))
	}
}

func TestManySalesCannotOversellTablecloths(t *testing.T) {
	f := newSaleFixture(t)
	ctx := context.Background()

	// 14 buyers of one unit each, 10 units in stock.
	outcomes := runConcurrently(14, func(index int) error {
		_, err := f.sales.Register(ctx, f.build(t, f.cloths(1, "65")), saleKey(fmt.Sprint("cloth", index)))
		return err
	})

	succeeded := 0
	for _, err := range outcomes {
		if err == nil {
			succeeded++
		} else if fieldCodesOf(t, err)["lines[0].quantity"] != validation.CodeInsufficientStock {
			t.Errorf("unexpected error: %v", err)
		}
	}
	if units, value := f.clothStock(t); succeeded != 10 || units != 0 || value != "0.00" {
		t.Errorf("succeeded %d, left %d units worth %s", succeeded, units, value)
	}
}

func TestSalesThatLockTheSameStockInOppositeOrderDoNotDeadlock(t *testing.T) {
	f := newSaleFixture(t)
	ctx := context.Background()

	// 10 sales, because 10 tablecloths are in stock: each needs one.
	outcomes := runConcurrently(10, func(index int) error {
		first, second := "ROL-000001", "ROL-000002"
		if index%2 == 1 {
			first, second = second, first
		}
		_, err := f.sales.Register(ctx, f.build(t, f.meters(first, "0.50", "18"), f.meters(second, "0.50", "18"), f.cloths(1, "65")),
			saleKey(fmt.Sprint("order", index)))
		return err
	})

	for _, err := range outcomes {
		if err != nil {
			t.Errorf("a sale failed: %v", err)
		}
	}
	if f.available(t, "ROL-000001") != "14.80" || f.available(t, "ROL-000002") != "15.00" {
		t.Errorf("rolls = %s / %s", f.available(t, "ROL-000001"), f.available(t, "ROL-000002"))
	}
}

func TestTheDatabaseRefusesNegativeBalances(t *testing.T) {
	f := newSaleFixture(t)
	ctx := context.Background()

	if _, err := f.pool.Exec(ctx, `UPDATE pvc_rolls SET available_length_m = -1 WHERE roll_code = 'ROL-000001'`); err == nil {
		t.Error("a roll balance below zero must be refused by the database")
	}
	if _, err := f.pool.Exec(ctx, `UPDATE tablecloth_stock SET quantity = -1`); err == nil {
		t.Error("tablecloth stock below zero must be refused by the database")
	}
	if _, err := f.pool.Exec(ctx,
		`INSERT INTO sales (sale_date, idempotency_key, request_fingerprint) VALUES ('2026-11-10', 'k-manual', 'f')`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx,
		`INSERT INTO sale_lines (sale_id, line_number, sale_kind, product_id, quantity, unit_price, reference_price, line_total, cost_of_sale)
		 SELECT id, 1, 'roll_meters', $1, 1, 1, 1, 1, 0 FROM sales WHERE idempotency_key = 'k-manual'`, f.pvc.ID); err == nil {
		t.Error("a roll line without its roll must be refused by the database")
	}
}

func TestListAndFilterSales(t *testing.T) {
	f := newSaleFixture(t)
	ctx := context.Background()
	earlier := saleDay.AddDate(0, 0, -5)
	for name, line := range map[string]struct {
		line sale.LineDraft
		day  time.Time
	}{
		"a": {f.meters("ROL-000001", "1", "18"), saleDay},
		"b": {f.cloths(1, "65"), earlier},
		"c": {f.wholeRoll("ROL-000002", "350"), saleDay.AddDate(0, 0, -1)},
	} {
		built := f.build(t, line.line)
		built.SaleDate = line.day
		if _, err := f.sales.Register(ctx, built, saleKey(name)); err != nil {
			t.Fatal(err)
		}
	}

	date := func(value time.Time) *time.Time { return &value }
	tests := []struct {
		name   string
		filter ports.SaleFilter
		want   int
	}{
		{"all", ports.SaleFilter{Limit: 10}, 3},
		{"from", ports.SaleFilter{From: date(saleDay.AddDate(0, 0, -1)), Limit: 10}, 2},
		{"to", ports.SaleFilter{To: date(saleDay.AddDate(0, 0, -2)), Limit: 10}, 1},
		{"roll code", ports.SaleFilter{Query: "rol-000002", Limit: 10}, 1},
		{"product sku", ports.SaleFilter{Query: "MAN-01", Limit: 10}, 1},
		{"product name", ports.SaleFilter{Query: "floral", Limit: 10}, 2},
		{"nothing", ports.SaleFilter{Query: "zzz", Limit: 10}, 0},
		{"paging", ports.SaleFilter{Limit: 1, Offset: 2}, 3},
	}
	for _, test := range tests {
		page, err := f.sales.List(ctx, test.filter)
		got := page.Total
		if test.name == "paging" {
			got = page.Total
			if len(page.Items) != 1 {
				t.Errorf("paging returned %d items", len(page.Items))
			}
		}
		if err != nil || got != test.want {
			t.Errorf("%s: total %d (err %v), want %d", test.name, got, err, test.want)
		}
	}

	page, _ := f.sales.List(ctx, ports.SaleFilter{Limit: 10})
	if page.Items[0].SaleDate.Before(page.Items[2].SaleDate) {
		t.Error("the most recent sale comes first")
	}
	if page.Items[0].Total.Hundredths() <= 0 || page.Items[0].LineCount != 1 {
		t.Errorf("summary = %+v", page.Items[0])
	}
}

func TestGetSaleNotFound(t *testing.T) {
	f := newSaleFixture(t)
	for _, id := range []string{"00000000-0000-0000-0000-000000000000", "not-a-uuid", ""} {
		if _, err := f.sales.GetByID(context.Background(), id); !errors.Is(err, sale.ErrNotFound) {
			t.Errorf("GetByID(%q) = %v", id, err)
		}
	}
}

// runConcurrently runs work in n goroutines at the same moment and returns each one's error.
func runConcurrently(n int, work func(index int) error) []error {
	var wait, start sync.WaitGroup
	start.Add(1)
	errs := make([]error, n)
	for index := 0; index < n; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			start.Wait()
			errs[index] = work(index)
		}()
	}
	start.Done()
	wait.Wait()
	return errs
}
