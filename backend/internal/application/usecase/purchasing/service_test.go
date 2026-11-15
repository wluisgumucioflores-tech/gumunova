package purchasing_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"gumunova/backend/internal/application/ports"
	"gumunova/backend/internal/application/usecase/purchasing"
	"gumunova/backend/internal/domain/product"
	"gumunova/backend/internal/domain/purchase"
	"gumunova/backend/internal/domain/shared/decimal"
	"gumunova/backend/internal/domain/shared/validation"
	"gumunova/backend/internal/domain/supplier"
	"gumunova/backend/internal/testsupport"
)

var fixedNow = time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)

const key = "key-0001-abcdef"

type fixture struct {
	service   *purchasing.Service
	purchases *testsupport.MemoryPurchaseRepository
	suppliers *testsupport.MemorySupplierRepository
	products  *testsupport.MemoryProductRepository
	supplier  supplier.Supplier
	pvc       product.Product
	cloth     product.Product
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	ctx := context.Background()
	suppliers := testsupport.NewMemorySupplierRepository()
	products := testsupport.NewMemoryProductRepository()
	purchases := testsupport.NewMemoryPurchaseRepository(suppliers)
	purchases.Products = products

	owner, _ := supplier.New(supplier.Draft{Name: "Plásticos del Sur"})
	owner, _ = suppliers.Create(ctx, owner)

	pvc, err := product.New(product.Draft{SKU: "PVC-01", Name: "PVC floral", Type: product.TypePVC, PVC: &product.PVCDetails{
		WidthM: decimal.MustParse("1.40"), NominalLengthM: decimal.MustParse("20"),
		ReferencePricePerMeter: decimal.MustParse("15"), ReferencePricePerRoll: decimal.MustParse("280"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	pvc, _ = products.Create(ctx, pvc)
	cloth, _ := product.New(product.Draft{SKU: "MAN-01", Name: "Mantel", Type: product.TypeTablecloth,
		Tablecloth: &product.TableclothDetails{UnitPrice: decimal.MustParse("60")}})
	cloth, _ = products.Create(ctx, cloth)

	service := purchasing.NewService(purchases, suppliers, products, func() time.Time { return fixedNow })
	return fixture{service: service, purchases: purchases, suppliers: suppliers, products: products, supplier: owner, pvc: pvc, cloth: cloth}
}

func (f fixture) draft() purchase.Draft {
	return purchase.Draft{
		SupplierID:   f.supplier.ID,
		PurchaseDate: time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC),
		Lines: []purchase.LineDraft{
			{ProductID: f.pvc.ID, Rolls: []purchase.RollDraft{
				{LengthM: decimal.MustParse("19.80"), Cost: decimal.MustParse("280")},
				{LengthM: decimal.MustParse("20"), Cost: decimal.MustParse("300")},
			}},
			{ProductID: f.cloth.ID, Quantity: 10, UnitCost: decimal.MustParse("45")},
		},
		AdditionalCosts: decimal.MustParse("103"),
	}
}

func fieldCodes(t *testing.T, err error) map[string]string {
	t.Helper()
	var problem *validation.ValidationError
	if !errors.As(err, &problem) {
		t.Fatalf("expected a validation error, got %v", err)
	}
	result := map[string]string{}
	for _, field := range problem.Fields {
		result[field.Field] = field.Code
	}
	return result
}

func TestRegisterPurchaseTakesLineKindsFromTheCatalog(t *testing.T) {
	f := newFixture(t)

	result, err := f.service.RegisterPurchase(context.Background(), f.draft(), key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	view := result.View
	if result.Replayed || view.Purchase.ID == "" || view.Supplier == nil || view.Supplier.Name != "Plásticos del Sur" {
		t.Errorf("unexpected result: %+v", result)
	}
	if view.Purchase.Lines[0].ProductType != product.TypePVC || len(view.Purchase.Lines[0].Rolls) != 2 {
		t.Errorf("pvc line = %+v", view.Purchase.Lines[0])
	}
	if view.Purchase.Lines[0].Rolls[0].Code == "" {
		t.Error("each roll must get its code")
	}
	if view.Purchase.Lines[1].ProductType != product.TypeTablecloth || view.Purchase.Lines[1].Quantity != 10 {
		t.Errorf("cloth line = %+v", view.Purchase.Lines[1])
	}
	if view.Products[f.pvc.ID].SKU != "PVC-01" || view.Products[f.cloth.ID].SKU != "MAN-01" {
		t.Errorf("products = %+v", view.Products)
	}
	// 280 + 300 + 450 = 1030 of goods, plus 103 of additional costs.
	if view.Purchase.Total.String() != "1133.00" {
		t.Errorf("total = %s", view.Purchase.Total)
	}
}

func TestAClientCannotForceTheLineKind(t *testing.T) {
	f := newFixture(t)
	draft := f.draft()
	draft.Lines[0].ProductType = product.TypeTablecloth // the client lies; the catalog says PVC

	result, err := f.service.RegisterPurchase(context.Background(), draft, key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.View.Purchase.Lines[0].ProductType != product.TypePVC {
		t.Error("the catalog, not the client, decides the product type")
	}
}

func TestARollsLineOnATableclothProductIsRejected(t *testing.T) {
	f := newFixture(t)
	draft := f.draft()
	draft.Lines[0].ProductID = f.cloth.ID // rolls listed for a tablecloth SKU

	_, err := f.service.RegisterPurchase(context.Background(), draft, key)

	codes := fieldCodes(t, err)
	if codes["lines[0].rolls"] != validation.CodeNotAllowed || codes["lines[0].quantity"] != validation.CodeMustBePositive {
		t.Errorf("codes = %v", codes)
	}
	if f.purchases.RegisterCalls != 0 {
		t.Error("nothing may be saved")
	}
}

func TestUnknownOrInactiveReferencesAreReportedByField(t *testing.T) {
	f := newFixture(t)
	f.suppliers.Suppliers[f.supplier.ID] = func() supplier.Supplier { s := f.supplier; s.Deactivate(); return s }()
	off := f.cloth
	off.Deactivate()
	f.products.Products[f.cloth.ID] = off

	draft := f.draft()
	draft.Lines = append(draft.Lines, purchase.LineDraft{ProductID: "missing", Quantity: 1, UnitCost: decimal.MustParse("1")})

	_, err := f.service.RegisterPurchase(context.Background(), draft, key)

	codes := fieldCodes(t, err)
	if codes["supplier_id"] != validation.CodeInactive || codes["lines[1].product_id"] != validation.CodeInactive ||
		codes["lines[2].product_id"] != validation.CodeNotFound {
		t.Errorf("codes = %v", codes)
	}
	if f.purchases.RegisterCalls != 0 {
		t.Error("nothing may be saved")
	}
}

func TestUnknownSupplier(t *testing.T) {
	f := newFixture(t)
	draft := f.draft()
	draft.SupplierID = "nobody"

	_, err := f.service.RegisterPurchase(context.Background(), draft, key)

	if got := fieldCodes(t, err)["supplier_id"]; got != validation.CodeNotFound {
		t.Errorf("supplier_id = %q", got)
	}
}

func TestPurchaseWithoutSupplier(t *testing.T) {
	f := newFixture(t)
	draft := f.draft()
	draft.SupplierID = ""

	result, err := f.service.RegisterPurchase(context.Background(), draft, key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.View.Supplier != nil || result.View.Purchase.SupplierID != "" {
		t.Errorf("supplier = %+v, want none", result.View.Supplier)
	}

	loaded, err := f.service.GetPurchase(context.Background(), result.View.Purchase.ID)
	if err != nil || loaded.Supplier != nil {
		t.Errorf("GetPurchase = %+v, %v", loaded.Supplier, err)
	}
}

func TestDomainRulesStillApply(t *testing.T) {
	f := newFixture(t)
	draft := f.draft()
	draft.Lines[0].Rolls[0].Cost = decimal.MustParse("0")
	draft.PurchaseDate = fixedNow.AddDate(0, 0, 5)

	_, err := f.service.RegisterPurchase(context.Background(), draft, key)

	codes := fieldCodes(t, err)
	if codes["lines[0].rolls[0].cost"] != validation.CodeMustBePositive || codes["purchase_date"] != validation.CodeInFuture {
		t.Errorf("codes = %v", codes)
	}
}

func TestRepeatingTheSameRequestReturnsTheSamePurchase(t *testing.T) {
	f := newFixture(t)
	first, err := f.service.RegisterPurchase(context.Background(), f.draft(), key)
	if err != nil {
		t.Fatal(err)
	}

	second, err := f.service.RegisterPurchase(context.Background(), f.draft(), key)
	if err != nil {
		t.Fatalf("a retry must succeed: %v", err)
	}

	if !second.Replayed || second.View.Purchase.ID != first.View.Purchase.ID {
		t.Errorf("replayed=%v ids %q/%q", second.Replayed, first.View.Purchase.ID, second.View.Purchase.ID)
	}
	if f.purchases.RegisterCalls != 1 {
		t.Errorf("registered %d times, want exactly 1", f.purchases.RegisterCalls)
	}
	if second.View.Purchase.Lines[0].Rolls[0].Code != first.View.Purchase.Lines[0].Rolls[0].Code {
		t.Error("a retry must not create new rolls")
	}
}

func TestReusingAKeyForADifferentPurchaseIsRejected(t *testing.T) {
	f := newFixture(t)
	if _, err := f.service.RegisterPurchase(context.Background(), f.draft(), key); err != nil {
		t.Fatal(err)
	}
	different := f.draft()
	different.DocumentReference = "FAC-999"

	_, err := f.service.RegisterPurchase(context.Background(), different, key)

	if !errors.Is(err, purchase.ErrIdempotencyKeyReused) {
		t.Errorf("error = %v, want ErrIdempotencyKeyReused", err)
	}
	if f.purchases.RegisterCalls != 1 {
		t.Error("the second purchase must not be saved")
	}
}

func TestDifferentKeysRegisterSeparatePurchases(t *testing.T) {
	f := newFixture(t)
	first, _ := f.service.RegisterPurchase(context.Background(), f.draft(), "key-0001-aaaaaa")
	second, err := f.service.RegisterPurchase(context.Background(), f.draft(), "key-0002-bbbbbb")
	if err != nil {
		t.Fatal(err)
	}
	if second.View.Purchase.ID == first.View.Purchase.ID || f.purchases.RegisterCalls != 2 {
		t.Error("two keys are two purchases")
	}
}

func TestInvalidIdempotencyKeys(t *testing.T) {
	f := newFixture(t)
	for _, bad := range []string{"", "short", "has space in it", "ñandú-0000001", "x\n0123456789", string(make([]byte, 101))} {
		_, err := f.service.RegisterPurchase(context.Background(), f.draft(), bad)
		if !errors.Is(err, purchasing.ErrInvalidIdempotencyKey) {
			t.Errorf("key %q: error = %v, want ErrInvalidIdempotencyKey", bad, err)
		}
	}
	if f.purchases.RegisterCalls != 0 {
		t.Error("nothing may be saved without a valid key")
	}
}

func TestStorageFailuresAreNotHiddenAsSuccess(t *testing.T) {
	f := newFixture(t)
	f.purchases.FailRegister = errors.New("database down")

	if _, err := f.service.RegisterPurchase(context.Background(), f.draft(), key); err == nil {
		t.Fatal("a failed save must be reported")
	}
}

func TestGetPurchaseAttachesSupplierAndProducts(t *testing.T) {
	f := newFixture(t)
	registered, _ := f.service.RegisterPurchase(context.Background(), f.draft(), key)

	view, err := f.service.GetPurchase(context.Background(), registered.View.Purchase.ID)
	if err != nil {
		t.Fatal(err)
	}
	if view.Supplier.ID != f.supplier.ID || len(view.Products) != 2 {
		t.Errorf("view = %+v", view)
	}
	if _, err := f.service.GetPurchase(context.Background(), "missing"); !errors.Is(err, purchase.ErrNotFound) {
		t.Errorf("missing: error = %v", err)
	}
}

func TestListPurchasesAppliesPagingDefaultsAndFilters(t *testing.T) {
	f := newFixture(t)
	f.service.RegisterPurchase(context.Background(), f.draft(), "key-0001-aaaaaa")
	later := f.draft()
	later.PurchaseDate = time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)
	later.DocumentReference = "FAC-2"
	f.service.RegisterPurchase(context.Background(), later, "key-0002-bbbbbb")

	all, err := f.service.ListPurchases(context.Background(), ports.PurchaseFilter{Limit: 10_000, Offset: -1})
	if err != nil {
		t.Fatal(err)
	}
	if all.Limit != 200 || all.Offset != 0 || all.Total != 2 || all.Items[0].DocumentReference != "FAC-2" {
		t.Errorf("all = %+v", all)
	}
	if all.Items[0].SupplierName != "Plásticos del Sur" || all.Items[0].LineCount != 2 {
		t.Errorf("summary = %+v", all.Items[0])
	}

	onlyOne, _ := f.service.ListPurchases(context.Background(), ports.PurchaseFilter{Query: "fac-2"})
	if onlyOne.Limit != 50 || len(onlyOne.Items) != 1 {
		t.Errorf("filtered = %+v", onlyOne)
	}
}

func (f fixture) newProductDraft() purchase.Draft {
	return purchase.Draft{
		PurchaseDate: time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC),
		Lines: []purchase.LineDraft{{
			NewProduct: &product.Draft{SKU: "pvc-nuevo", Name: "PVC nuevo", Type: product.TypePVC, PVC: &product.PVCDetails{
				WidthM: decimal.MustParse("1.40"), NominalLengthM: decimal.MustParse("20"),
				ReferencePricePerMeter: decimal.MustParse("15"), ReferencePricePerRoll: decimal.MustParse("280"),
			}},
			Rolls: []purchase.RollDraft{{LengthM: decimal.MustParse("19.80"), Cost: decimal.MustParse("280")}},
		}},
	}
}

func TestPurchaseCreatesTheNewProduct(t *testing.T) {
	f := newFixture(t)
	before := len(f.products.Products)

	result, err := f.service.RegisterPurchase(context.Background(), f.newProductDraft(), key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	line := result.View.Purchase.Lines[0]
	created, ok := f.products.Products[line.ProductID]
	if !ok || created.SKU != "PVC-NUEVO" || !created.IsActive || len(f.products.Products) != before+1 {
		t.Fatalf("created product = %+v (known: %v)", created, ok)
	}
	if result.View.Products[line.ProductID].SKU != "PVC-NUEVO" || line.ProductType != product.TypePVC {
		t.Errorf("view = %+v", result.View.Products)
	}
}

func TestRepeatingARequestThatCreatesAProductDoesNotCreateItTwice(t *testing.T) {
	f := newFixture(t)
	before := len(f.products.Products)
	if _, err := f.service.RegisterPurchase(context.Background(), f.newProductDraft(), key); err != nil {
		t.Fatal(err)
	}

	result, err := f.service.RegisterPurchase(context.Background(), f.newProductDraft(), key)

	if err != nil || !result.Replayed || len(f.products.Products) != before+1 {
		t.Errorf("replayed %v, err %v, products %d (want %d)", result.Replayed, err, len(f.products.Products), before+1)
	}
}

func TestNewProductWithATakenSKU(t *testing.T) {
	f := newFixture(t)
	draft := f.newProductDraft()
	draft.Lines[0].NewProduct.SKU = f.pvc.SKU

	_, err := f.service.RegisterPurchase(context.Background(), draft, key)

	if got := fieldCodes(t, err)["lines[0].new_product.sku"]; got != validation.CodeAlreadyExists {
		t.Errorf("sku = %q", got)
	}
	if len(f.purchases.Purchases) != 0 {
		t.Error("nothing may be saved")
	}
}

func TestUpdatePurchaseWithTheSameItemsOnlyEditsTheData(t *testing.T) {
	f := newFixture(t)
	created, _ := f.service.RegisterPurchase(context.Background(), f.draft(), key)

	draft := f.draft()
	draft.Notes = "  corregida  "
	view, err := f.service.UpdatePurchase(context.Background(), created.View.Purchase.ID, draft)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.purchases.LastReplaceItems || view.Purchase.Notes != "corregida" || view.Purchase.Revision != 1 {
		t.Errorf("replace=%v notes=%q revision=%d", f.purchases.LastReplaceItems, view.Purchase.Notes, view.Purchase.Revision)
	}
}

func TestUpdatePurchaseWithDifferentItemsReplacesThem(t *testing.T) {
	f := newFixture(t)
	created, _ := f.service.RegisterPurchase(context.Background(), f.draft(), key)

	for name, change := range map[string]func(*purchase.Draft){
		"quantity":   func(d *purchase.Draft) { d.Lines[1].Quantity = 12 },
		"roll cost":  func(d *purchase.Draft) { d.Lines[0].Rolls[0].Cost = decimal.MustParse("281") },
		"freight":    func(d *purchase.Draft) { d.AdditionalCosts = decimal.MustParse("50") },
		"extra roll": func(d *purchase.Draft) { d.Lines[0].Rolls = append(d.Lines[0].Rolls, d.Lines[0].Rolls[0]) },
	} {
		draft := f.draft()
		change(&draft)
		f.purchases.LastReplaceItems = false
		if _, err := f.service.UpdatePurchase(context.Background(), created.View.Purchase.ID, draft); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !f.purchases.LastReplaceItems {
			t.Errorf("%s: the items should have been replaced", name)
		}
	}
}

func TestUpdatePurchaseReportsLockedItemsAndMissingPurchases(t *testing.T) {
	f := newFixture(t)
	created, _ := f.service.RegisterPurchase(context.Background(), f.draft(), key)
	f.purchases.ItemsLocked = true

	draft := f.draft()
	draft.Lines[1].Quantity = 12
	if _, err := f.service.UpdatePurchase(context.Background(), created.View.Purchase.ID, draft); !errors.Is(err, purchase.ErrItemsLocked) {
		t.Errorf("err = %v, want ErrItemsLocked", err)
	}
	if _, err := f.service.UpdatePurchase(context.Background(), "missing", f.draft()); !errors.Is(err, purchase.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
	view, _ := f.service.GetPurchase(context.Background(), created.View.Purchase.ID)
	if view.ItemsEditable {
		t.Error("ItemsEditable must be false when stock was sold")
	}
}

func TestUpdatePurchaseValidatesLikeRegistering(t *testing.T) {
	f := newFixture(t)
	created, _ := f.service.RegisterPurchase(context.Background(), f.draft(), key)

	draft := f.draft()
	draft.Lines = nil
	if codes := fieldCodes(t, func() error {
		_, err := f.service.UpdatePurchase(context.Background(), created.View.Purchase.ID, draft)
		return err
	}()); codes["lines"] != validation.CodeRequired {
		t.Errorf("codes = %v", codes)
	}
}
