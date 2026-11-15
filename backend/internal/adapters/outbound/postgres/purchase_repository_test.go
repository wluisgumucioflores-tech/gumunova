package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"gumunova/backend/internal/adapters/outbound/postgres"
	"gumunova/backend/internal/application/ports"
	"gumunova/backend/internal/domain/product"
	"gumunova/backend/internal/domain/purchase"
	"gumunova/backend/internal/domain/shared/decimal"
	"gumunova/backend/internal/domain/supplier"
)

type purchaseFixture struct {
	repository *postgres.PurchaseRepository
	pool       *pgxpool.Pool
	supplier   supplier.Supplier
	pvc        product.Product
	cloth      product.Product
}

func newPurchaseFixture(t *testing.T) purchaseFixture {
	t.Helper()
	products, pool := newRepository(t) // connects, migrates and empties every table
	ctx := context.Background()

	owner, err := postgres.NewSupplierRepository(pool).Create(ctx, newSupplier("Plásticos del Sur", "", ""))
	if err != nil {
		t.Fatalf("create supplier: %v", err)
	}
	pvc, err := products.Create(ctx, pvcProduct("PVC-01", "PVC floral"))
	if err != nil {
		t.Fatalf("create pvc: %v", err)
	}
	cloth, err := products.Create(ctx, tableclothProduct("MAN-01", "Mantel"))
	if err != nil {
		t.Fatalf("create tablecloth: %v", err)
	}
	return purchaseFixture{
		repository: postgres.NewPurchaseRepository(pool),
		pool:       pool, supplier: owner, pvc: pvc, cloth: cloth,
	}
}

func (f purchaseFixture) build(t *testing.T, additional string, lines ...purchase.LineDraft) purchase.Purchase {
	t.Helper()
	built, err := purchase.New(purchase.Draft{
		SupplierID:      f.supplier.ID,
		PurchaseDate:    time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC),
		AdditionalCosts: decimal.MustParse(additional),
		Lines:           lines,
	}, time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("build purchase: %v", err)
	}
	return built
}

func (f purchaseFixture) pvcLine(rolls ...[2]string) purchase.LineDraft {
	line := purchase.LineDraft{ProductID: f.pvc.ID, ProductType: product.TypePVC}
	for _, r := range rolls {
		line.Rolls = append(line.Rolls, purchase.RollDraft{LengthM: decimal.MustParse(r[0]), Cost: decimal.MustParse(r[1])})
	}
	return line
}

func (f purchaseFixture) clothLine(quantity int, unitCost string) purchase.LineDraft {
	return purchase.LineDraft{ProductID: f.cloth.ID, ProductType: product.TypeTablecloth, Quantity: quantity, UnitCost: decimal.MustParse(unitCost)}
}

func (f purchaseFixture) count(t *testing.T, table string) int {
	t.Helper()
	var total int
	if err := f.pool.QueryRow(context.Background(), "SELECT count(*) FROM "+table).Scan(&total); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return total
}

func key(name string) ports.IdempotencyKey {
	return ports.IdempotencyKey{Key: "key-" + name, Fingerprint: "fp-" + name}
}

func TestRegisterSavesPurchaseRollsStockAndMovements(t *testing.T) {
	f := newPurchaseFixture(t)
	built := f.build(t, "30",
		f.pvcLine([2]string{"19.80", "280"}, [2]string{"20", "320"}), // 600 of rolls
		f.clothLine(10, "30"), // 300 of tablecloths
	)

	result, err := f.repository.Register(context.Background(), built, key("a"))
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if result.Replayed {
		t.Error("a first registration is not a replay")
	}

	saved := result.Purchase
	if saved.ID == "" || saved.CreatedAt.IsZero() || saved.Status != purchase.StatusConfirmed {
		t.Errorf("generated fields missing: %+v", saved)
	}
	if saved.Total.String() != "930.00" || saved.AdditionalCosts.String() != "30.00" || saved.PurchaseDate.Format("2006-01-02") != "2026-11-10" {
		t.Errorf("total %s additional %s date %s", saved.Total, saved.AdditionalCosts, saved.PurchaseDate)
	}
	if len(saved.Lines) != 2 || saved.Lines[0].Number != 1 || saved.Lines[1].Number != 2 {
		t.Fatalf("lines = %+v", saved.Lines)
	}

	rolls := saved.Lines[0].Rolls
	if len(rolls) != 2 || rolls[0].Code != "ROL-000001" || rolls[1].Code != "ROL-000002" {
		t.Fatalf("rolls = %+v", rolls)
	}
	// 900 of goods carry 30 of freight: 600/900 to the rolls in proportion to cost.
	if rolls[0].LengthM.String() != "19.80" || rolls[0].AcquisitionCost.String() != "280.00" || rolls[0].AdditionalCost.String() != "9.33" ||
		rolls[1].AdditionalCost.String() != "10.67" {
		t.Errorf("roll costs = %+v / %+v", rolls[0], rolls[1])
	}
	cloth := saved.Lines[1]
	if cloth.Quantity != 10 || cloth.UnitCost.String() != "30.00" || cloth.AdditionalCost.String() != "10.00" || cloth.Total.String() != "310.00" {
		t.Errorf("tablecloth line = %+v", cloth)
	}

	// Physical rolls start full.
	var available, initial string
	if err := f.pool.QueryRow(context.Background(),
		`SELECT available_length_m::text, initial_usable_length_m::text FROM pvc_rolls WHERE roll_code = 'ROL-000001'`,
	).Scan(&available, &initial); err != nil || available != "19.80" || initial != "19.80" {
		t.Errorf("available %s initial %s (err %v)", available, initial, err)
	}

	// Stock only moves together with its movements.
	var quantity int
	var value string
	if err := f.pool.QueryRow(context.Background(),
		`SELECT quantity, stock_value::text FROM tablecloth_stock WHERE product_id = $1`, f.cloth.ID,
	).Scan(&quantity, &value); err != nil || quantity != 10 || value != "310.00" {
		t.Errorf("stock %d value %s (err %v)", quantity, value, err)
	}
	if movements := f.count(t, "inventory_movements"); movements != 3 {
		t.Errorf("movements = %d, want 2 rolls + 1 tablecloth line", movements)
	}
	var meters, pieces string
	f.pool.QueryRow(context.Background(), `SELECT coalesce(sum(quantity_delta),0)::text FROM inventory_movements WHERE unit = 'm'`).Scan(&meters)
	f.pool.QueryRow(context.Background(), `SELECT coalesce(sum(quantity_delta),0)::text FROM inventory_movements WHERE unit = 'unit'`).Scan(&pieces)
	if meters != "39.80" || pieces != "10.00" {
		t.Errorf("movement totals: %s m, %s units", meters, pieces)
	}
}

func TestRollCodesAreUniqueAndSequentialAcrossPurchases(t *testing.T) {
	f := newPurchaseFixture(t)
	first, _ := f.repository.Register(context.Background(), f.build(t, "0", f.pvcLine([2]string{"20", "100"}, [2]string{"20", "100"})), key("a"))
	second, err := f.repository.Register(context.Background(), f.build(t, "0", f.pvcLine([2]string{"30", "150"})), key("b"))
	if err != nil {
		t.Fatal(err)
	}

	codes := []string{
		first.Purchase.Lines[0].Rolls[0].Code, first.Purchase.Lines[0].Rolls[1].Code, second.Purchase.Lines[0].Rolls[0].Code,
	}
	want := []string{"ROL-000001", "ROL-000002", "ROL-000003"}
	for i := range codes {
		if codes[i] != want[i] {
			t.Errorf("codes = %v, want %v", codes, want)
			break
		}
	}
}

func TestTableclothStockAndValueAccumulateAcrossPurchases(t *testing.T) {
	f := newPurchaseFixture(t)
	ctx := context.Background()
	f.repository.Register(ctx, f.build(t, "0", f.clothLine(10, "30")), key("a"))                           // value 300
	if _, err := f.repository.Register(ctx, f.build(t, "0", f.clothLine(5, "40")), key("b")); err != nil { // value 200
		t.Fatal(err)
	}

	var quantity int
	var value string
	if err := f.pool.QueryRow(ctx, `SELECT quantity, stock_value::text FROM tablecloth_stock WHERE product_id = $1`, f.cloth.ID).Scan(&quantity, &value); err != nil {
		t.Fatal(err)
	}
	// Weighted average cost = 500 / 15 = 33.33…, derived from the exact value, never rounded.
	if quantity != 15 || value != "500.00" {
		t.Errorf("stock %d value %s, want 15 and 500.00", quantity, value)
	}
}

func TestAFailureHalfwayLeavesNothingSaved(t *testing.T) {
	f := newPurchaseFixture(t)
	ctx := context.Background()

	built := f.build(t, "10",
		f.pvcLine([2]string{"20", "100"}),
		f.clothLine(3, "20"),
		f.clothLine(1, "5"),
	)
	// The last line points at a product that does not exist: its insert fails after the
	// earlier lines, rolls, stock and movements were already written inside the transaction.
	built.Lines[2].ProductID = "00000000-0000-0000-0000-000000000000"

	_, err := f.repository.Register(ctx, built, key("a"))
	if err == nil {
		t.Fatal("expected the registration to fail")
	}

	for _, table := range []string{"purchases", "purchase_lines", "pvc_rolls", "tablecloth_stock", "inventory_movements"} {
		if rows := f.count(t, table); rows != 0 {
			t.Errorf("%s has %d rows, want 0: a failed purchase must not leave partial data", table, rows)
		}
	}

	// The same key can be used again for the corrected purchase.
	built.Lines = built.Lines[:2]
	if _, err := f.repository.Register(ctx, built, key("a")); err != nil {
		t.Errorf("a failed attempt must not burn the idempotency key: %v", err)
	}
}

// newProductLine is a PVC line that creates its product together with the purchase.
func newProductLine(sku string) purchase.LineDraft {
	return purchase.LineDraft{
		NewProduct: &product.Draft{SKU: sku, Name: "PVC nuevo", Type: product.TypePVC, PVC: &product.PVCDetails{
			WidthM: decimal.MustParse("1.40"), NominalLengthM: decimal.MustParse("20"),
			ReferencePricePerMeter: decimal.MustParse("15"), ReferencePricePerRoll: decimal.MustParse("280"),
		}},
		Rolls: []purchase.RollDraft{{LengthM: decimal.MustParse("19.80"), Cost: decimal.MustParse("280")}},
	}
}

func TestPurchaseCreatesItsNewProductWithItsRolls(t *testing.T) {
	f := newPurchaseFixture(t)
	ctx := context.Background()
	before := f.count(t, "products")
	built := f.build(t, "0", newProductLine("pvc-nuevo"))

	result, err := f.repository.Register(ctx, built, key("a"))
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	line := result.Purchase.Lines[0]
	if f.count(t, "products") != before+1 || line.ProductID == "" || len(line.Rolls) != 1 {
		t.Fatalf("products %d (before %d), line %+v", f.count(t, "products"), before, line)
	}
	var sku string
	var active bool
	if err := f.pool.QueryRow(ctx, "SELECT sku, is_active FROM products WHERE id = $1", line.ProductID).Scan(&sku, &active); err != nil {
		t.Fatal(err)
	}
	if sku != "PVC-NUEVO" || !active || f.count(t, "pvc_product_details") != 2 {
		t.Errorf("sku %q active %v", sku, active)
	}
	if f.count(t, "inventory_movements") != 1 {
		t.Error("the new roll must have its movement")
	}

	// A retry returns the same purchase and does not create the product again.
	if _, err := f.repository.Register(ctx, built, key("a")); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if f.count(t, "products") != before+1 {
		t.Error("a retry must not create the product twice")
	}
}

func TestATakenSKUOfANewProductSavesNothing(t *testing.T) {
	f := newPurchaseFixture(t)
	ctx := context.Background()
	before := f.count(t, "products")
	built := f.build(t, "0", f.clothLine(2, "10"), newProductLine("pvc-otro"), newProductLine(f.pvc.SKU))

	_, err := f.repository.Register(ctx, built, key("a"))

	var conflict *purchase.SKUConflictError
	if !errors.As(err, &conflict) || conflict.Line != 2 {
		t.Fatalf("error = %v, want a SKU conflict on line 2", err)
	}
	// The product of the earlier line was created inside the same transaction: it is gone too.
	for table, want := range map[string]int{"products": before, "purchases": 0, "purchase_lines": 0, "pvc_rolls": 0, "tablecloth_stock": 0} {
		if rows := f.count(t, table); rows != want {
			t.Errorf("%s has %d rows, want %d", table, rows, want)
		}
	}
}

func TestRetryWithTheSameKeyReturnsTheSamePurchase(t *testing.T) {
	f := newPurchaseFixture(t)
	ctx := context.Background()
	built := f.build(t, "0", f.pvcLine([2]string{"20", "100"}), f.clothLine(2, "10"))
	first, err := f.repository.Register(ctx, built, key("a"))
	if err != nil {
		t.Fatal(err)
	}

	retry, err := f.repository.Register(ctx, built, key("a"))
	if err != nil {
		t.Fatalf("retry: %v", err)
	}

	if !retry.Replayed || retry.Purchase.ID != first.Purchase.ID {
		t.Errorf("replayed=%v ids %s/%s", retry.Replayed, first.Purchase.ID, retry.Purchase.ID)
	}
	if retry.Purchase.Lines[0].Rolls[0].Code != first.Purchase.Lines[0].Rolls[0].Code {
		t.Error("a retry must return the original rolls")
	}
	if f.count(t, "purchases") != 1 || f.count(t, "pvc_rolls") != 1 || f.count(t, "inventory_movements") != 2 {
		t.Error("a retry must not create anything new")
	}
	var quantity int
	f.pool.QueryRow(ctx, `SELECT quantity FROM tablecloth_stock WHERE product_id = $1`, f.cloth.ID).Scan(&quantity)
	if quantity != 2 {
		t.Errorf("stock = %d, a retry must not add stock again", quantity)
	}
}

func TestReusingAKeyForADifferentPurchaseIsRejected(t *testing.T) {
	f := newPurchaseFixture(t)
	ctx := context.Background()
	f.repository.Register(ctx, f.build(t, "0", f.clothLine(2, "10")), ports.IdempotencyKey{Key: "key-a", Fingerprint: "one"})

	_, err := f.repository.Register(ctx, f.build(t, "0", f.clothLine(9, "10")), ports.IdempotencyKey{Key: "key-a", Fingerprint: "two"})

	if !errors.Is(err, purchase.ErrIdempotencyKeyReused) {
		t.Errorf("error = %v, want ErrIdempotencyKeyReused", err)
	}
	if f.count(t, "purchases") != 1 {
		t.Error("the second purchase must not be saved")
	}
}

func TestConcurrentRetriesRegisterExactlyOnePurchase(t *testing.T) {
	f := newPurchaseFixture(t)
	ctx := context.Background()
	built := f.build(t, "0", f.pvcLine([2]string{"20", "100"}), f.clothLine(4, "10"))

	const attempts = 12
	results := make([]ports.RegisterResult, attempts)
	failures := make([]error, attempts)
	var group sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < attempts; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			results[i], failures[i] = f.repository.Register(ctx, built, key("double-tap"))
		}()
	}
	close(start)
	group.Wait()

	replays := 0
	for i := range results {
		if failures[i] != nil {
			t.Fatalf("attempt %d failed: %v", i, failures[i])
		}
		if results[i].Purchase.ID != results[0].Purchase.ID {
			t.Errorf("attempt %d returned a different purchase", i)
		}
		if results[i].Replayed {
			replays++
		}
	}
	if replays != attempts-1 {
		t.Errorf("replays = %d, want %d: exactly one attempt creates the purchase", replays, attempts-1)
	}
	if f.count(t, "purchases") != 1 || f.count(t, "pvc_rolls") != 1 || f.count(t, "inventory_movements") != 2 {
		t.Errorf("rows: purchases %d rolls %d movements %d", f.count(t, "purchases"), f.count(t, "pvc_rolls"), f.count(t, "inventory_movements"))
	}
	var quantity int
	f.pool.QueryRow(ctx, `SELECT quantity FROM tablecloth_stock WHERE product_id = $1`, f.cloth.ID).Scan(&quantity)
	if quantity != 4 {
		t.Errorf("stock = %d, want 4: concurrent retries must not add stock more than once", quantity)
	}
}

func TestDatabaseRefusesInvalidStock(t *testing.T) {
	f := newPurchaseFixture(t)
	ctx := context.Background()
	result, err := f.repository.Register(ctx, f.build(t, "0", f.pvcLine([2]string{"20", "100"}), f.clothLine(3, "10")), key("a"))
	if err != nil {
		t.Fatal(err)
	}
	rollID := result.Purchase.Lines[0].Rolls[0].ID

	if _, err := f.pool.Exec(ctx, `UPDATE pvc_rolls SET available_length_m = -1 WHERE id = $1`, rollID); err == nil {
		t.Error("a negative balance must be refused by the database")
	}
	if _, err := f.pool.Exec(ctx, `UPDATE pvc_rolls SET available_length_m = 25 WHERE id = $1`, rollID); err == nil {
		t.Error("a balance above the initial length must be refused by the database")
	}
	if tag, err := f.pool.Exec(ctx, `UPDATE tablecloth_stock SET quantity = -1`); err == nil {
		t.Errorf("negative tablecloth stock must be refused by the database (updated %d rows)", tag.RowsAffected())
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO pvc_rolls (roll_code, product_id, purchase_id, purchase_line_id, line_position,
		initial_usable_length_m, available_length_m, acquisition_cost, received_at)
		SELECT 'ROL-000001', product_id, purchase_id, purchase_line_id, 9, 20, 20, 100, received_at FROM pvc_rolls LIMIT 1`); err == nil {
		t.Error("a duplicated roll code must be refused by the database")
	}
}

func TestGetByID(t *testing.T) {
	f := newPurchaseFixture(t)
	ctx := context.Background()
	built := f.build(t, "12.34", f.pvcLine([2]string{"19.80", "280"}), f.clothLine(3, "20"))
	built.DocumentReference = "FAC-001"
	built.Notes = "Entrega parcial"
	registered, err := f.repository.Register(ctx, built, key("a"))
	if err != nil {
		t.Fatal(err)
	}

	loaded, err := f.repository.GetByID(ctx, registered.Purchase.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if loaded.DocumentReference != "FAC-001" || loaded.Notes != "Entrega parcial" || loaded.SupplierID != f.supplier.ID {
		t.Errorf("unexpected purchase: %+v", loaded)
	}
	if loaded.Lines[0].ProductType != product.TypePVC || loaded.Lines[1].ProductType != product.TypeTablecloth {
		t.Errorf("product types = %s / %s", loaded.Lines[0].ProductType, loaded.Lines[1].ProductType)
	}
	if loaded.Total.String() != registered.Purchase.Total.String() {
		t.Errorf("total %s vs %s", loaded.Total, registered.Purchase.Total)
	}

	for _, id := range []string{"00000000-0000-0000-0000-000000000000", "not-a-uuid", ""} {
		if _, err := f.repository.GetByID(ctx, id); !errors.Is(err, purchase.ErrNotFound) {
			t.Errorf("GetByID(%q) error = %v, want ErrNotFound", id, err)
		}
	}
}

func TestPurchaseWithoutSupplier(t *testing.T) {
	f := newPurchaseFixture(t)
	ctx := context.Background()
	built := f.build(t, "0", f.clothLine(2, "15"))
	built.SupplierID = ""
	built.DocumentReference = "SIN-PROV"
	registered, err := f.repository.Register(ctx, built, key("a"))
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	loaded, err := f.repository.GetByID(ctx, registered.Purchase.ID)
	if err != nil || loaded.SupplierID != "" {
		t.Fatalf("GetByID supplier = %q, err = %v", loaded.SupplierID, err)
	}

	page, err := f.repository.List(ctx, ports.PurchaseFilter{Limit: 10})
	if err != nil || len(page.Items) != 1 || page.Total != 1 {
		t.Fatalf("List = %+v, err = %v", page, err)
	}
	if item := page.Items[0]; item.SupplierID != "" || item.SupplierName != "" {
		t.Errorf("summary supplier = %q / %q, want empty", item.SupplierID, item.SupplierName)
	}

	// Filtering by a supplier must not return purchases that have none.
	page, err = f.repository.List(ctx, ports.PurchaseFilter{SupplierID: f.supplier.ID, Limit: 10})
	if err != nil || len(page.Items) != 0 {
		t.Errorf("filtered by supplier = %+v, err = %v", page, err)
	}
}

func TestListFiltersOrderAndPaging(t *testing.T) {
	f := newPurchaseFixture(t)
	ctx := context.Background()
	other, err := postgres.NewSupplierRepository(f.pool).Create(ctx, newSupplier("Textiles Norte", "", ""))
	if err != nil {
		t.Fatal(err)
	}

	register := func(name, supplierID, date, reference string) {
		built := f.build(t, "0", f.clothLine(1, "10"))
		built.SupplierID = supplierID
		built.PurchaseDate, _ = time.Parse("2006-01-02", date)
		built.DocumentReference = reference
		if _, err := f.repository.Register(ctx, built, key(name)); err != nil {
			t.Fatal(err)
		}
	}
	register("a", f.supplier.ID, "2026-11-01", "FAC-100")
	register("b", other.ID, "2026-11-05", "100%_ABC")
	register("c", f.supplier.ID, "2026-11-09", "FAC-300")

	date := func(value string) *time.Time { parsed, _ := time.Parse("2006-01-02", value); return &parsed }
	tests := []struct {
		name   string
		filter ports.PurchaseFilter
		want   []string
		total  int
	}{
		{"newest first", ports.PurchaseFilter{Limit: 10}, []string{"FAC-300", "100%_ABC", "FAC-100"}, 3},
		{"by supplier", ports.PurchaseFilter{SupplierID: f.supplier.ID, Limit: 10}, []string{"FAC-300", "FAC-100"}, 2},
		{"unknown supplier", ports.PurchaseFilter{SupplierID: "00000000-0000-0000-0000-000000000000", Limit: 10}, nil, 0},
		{"malformed supplier id", ports.PurchaseFilter{SupplierID: "nope", Limit: 10}, nil, 0},
		{"from date inclusive", ports.PurchaseFilter{From: date("2026-11-05"), Limit: 10}, []string{"FAC-300", "100%_ABC"}, 2},
		{"to date inclusive", ports.PurchaseFilter{To: date("2026-11-05"), Limit: 10}, []string{"100%_ABC", "FAC-100"}, 2},
		{"date range", ports.PurchaseFilter{From: date("2026-11-02"), To: date("2026-11-08"), Limit: 10}, []string{"100%_ABC"}, 1},
		{"search reference", ports.PurchaseFilter{Query: "fac-3", Limit: 10}, []string{"FAC-300"}, 1},
		{"search supplier name", ports.PurchaseFilter{Query: "textiles", Limit: 10}, []string{"100%_ABC"}, 1},
		{"wildcards are literal", ports.PurchaseFilter{Query: "100%_", Limit: 10}, []string{"100%_ABC"}, 1},
		{"paging", ports.PurchaseFilter{Limit: 1, Offset: 1}, []string{"100%_ABC"}, 3},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			page, err := f.repository.List(ctx, test.filter)
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			var got []string
			for _, item := range page.Items {
				got = append(got, item.DocumentReference)
			}
			if fmt.Sprint(got) != fmt.Sprint(test.want) && !(len(got) == 0 && len(test.want) == 0) {
				t.Fatalf("references = %v, want %v", got, test.want)
			}
			if page.Total != test.total {
				t.Errorf("total = %d, want %d", page.Total, test.total)
			}
		})
	}

	page, _ := f.repository.List(ctx, ports.PurchaseFilter{Limit: 10})
	first := page.Items[0]
	if first.SupplierName != "Plásticos del Sur" || first.LineCount != 1 || first.Total.String() != "10.00" {
		t.Errorf("summary = %+v", first)
	}
}
