package postgres_test

import (
	"context"
	"errors"
	"os"
	"regexp"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"gumunova/backend/internal/adapters/outbound/postgres"
	"gumunova/backend/internal/application/ports"
	"gumunova/backend/internal/domain/product"
	"gumunova/backend/internal/domain/shared/decimal"
	"gumunova/backend/migrations"
)

// newRepository connects to TEST_DATABASE_URL, applies migrations and empties the catalog.
// The tests are skipped when the variable is not set.
func newRepository(t *testing.T) (*postgres.ProductRepository, *pgxpool.Pool) {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set; skipping PostgreSQL integration test")
	}
	if err := postgres.MigrateUp(databaseURL, migrations.Files); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := postgres.Connect(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	// CASCADE also empties everything that points at products or suppliers (purchases, rolls,
	// stock and movements). The roll code sequence restarts so codes are predictable.
	_, err = pool.Exec(context.Background(),
		`TRUNCATE products, suppliers, sales CASCADE; ALTER SEQUENCE roll_code_seq RESTART WITH 1`)
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return postgres.NewProductRepository(pool), pool
}

func pvcProduct(sku, name string) product.Product {
	created, err := product.New(product.Draft{
		SKU: sku, Name: name, Type: product.TypePVC,
		PVC: &product.PVCDetails{
			Material: "PVC", Design: "Floral", Color: "Rojo", Texture: "Lisa",
			WidthM:                 decimal.MustParse("1.40"),
			NominalLengthM:         decimal.MustParse("20"),
			ReferencePricePerMeter: decimal.MustParse("15.5"),
			ReferencePricePerRoll:  decimal.MustParse("280"),
		},
	})
	if err != nil {
		panic(err)
	}
	return created
}

func tableclothProduct(sku, name string) product.Product {
	created, err := product.New(product.Draft{
		SKU: sku, Name: name, Type: product.TypeTablecloth,
		Tablecloth: &product.TableclothDetails{
			Design: "Liso", Color: "Blanco", Dimensions: "1.40 x 2.00 m",
			UnitPrice: decimal.MustParse("60"),
		},
	})
	if err != nil {
		panic(err)
	}
	return created
}

func TestCreateAndGetPVC(t *testing.T) {
	repository, _ := newRepository(t)
	ctx := context.Background()

	created, err := repository.Create(ctx, pvcProduct("PVC-01", "PVC floral"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ID == "" || created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Errorf("generated fields missing: %+v", created)
	}

	loaded, err := repository.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if loaded.SKU != "PVC-01" || loaded.Type != product.TypePVC || !loaded.IsActive || loaded.Tablecloth != nil {
		t.Errorf("unexpected product: %+v", loaded)
	}
	details := loaded.PVC
	if details == nil {
		t.Fatal("pvc details missing")
	}
	if details.WidthM.String() != "1.40" || details.NominalLengthM.String() != "20.00" ||
		details.ReferencePricePerMeter.String() != "15.50" || details.ReferencePricePerRoll.String() != "280.00" {
		t.Errorf("decimals not preserved exactly: %+v", details)
	}
	if details.ThicknessMM != nil {
		t.Errorf("thickness should be NULL, got %v", details.ThicknessMM)
	}
	if details.Design != "Floral" || details.Color != "Rojo" || details.Texture != "Lisa" || details.Material != "PVC" {
		t.Errorf("text attributes not preserved: %+v", details)
	}
}

func TestCreateAndGetTablecloth(t *testing.T) {
	repository, _ := newRepository(t)
	cost := decimal.MustParse("40.25")
	cloth := tableclothProduct("MAN-01", "Mantel blanco")
	cloth.Tablecloth.ReferenceCost = &cost

	created, err := repository.Create(context.Background(), cloth)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	loaded, err := repository.GetByID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if loaded.Tablecloth == nil || loaded.PVC != nil {
		t.Fatalf("unexpected shape: %+v", loaded)
	}
	if loaded.Tablecloth.UnitPrice.String() != "60.00" || loaded.Tablecloth.ReferenceCost == nil ||
		loaded.Tablecloth.ReferenceCost.String() != "40.25" || loaded.Tablecloth.Dimensions != "1.40 x 2.00 m" {
		t.Errorf("unexpected details: %+v", loaded.Tablecloth)
	}
}

func TestCreateDuplicateSKU(t *testing.T) {
	repository, pool := newRepository(t)
	ctx := context.Background()
	if _, err := repository.Create(ctx, pvcProduct("PVC-01", "First")); err != nil {
		t.Fatal(err)
	}

	_, err := repository.Create(ctx, tableclothProduct("PVC-01", "Second"))
	if !errors.Is(err, product.ErrSKUAlreadyExists) {
		t.Fatalf("error = %v, want ErrSKUAlreadyExists", err)
	}

	// The failed attempt must not leave partial rows behind.
	var products int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM products`).Scan(&products); err != nil || products != 1 {
		t.Errorf("products = %d (err %v), want 1", products, err)
	}
}

func TestUpdate(t *testing.T) {
	repository, _ := newRepository(t)
	ctx := context.Background()
	created, _ := repository.Create(ctx, pvcProduct("PVC-01", "Old name"))

	thickness := decimal.MustParse("0.30")
	created.Name = "New name"
	created.PVC.Color = "Azul"
	created.PVC.ThicknessMM = &thickness
	created.PVC.ReferencePricePerMeter = decimal.MustParse("17.75")
	created.Deactivate()

	updated, err := repository.Update(ctx, created)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "New name" || updated.IsActive || updated.PVC.Color != "Azul" ||
		updated.PVC.ThicknessMM == nil || updated.PVC.ThicknessMM.String() != "0.30" ||
		updated.PVC.ReferencePricePerMeter.String() != "17.75" {
		t.Errorf("unexpected product: %+v", updated)
	}
	if !updated.UpdatedAt.After(created.CreatedAt) && !updated.UpdatedAt.Equal(created.CreatedAt) {
		t.Error("updated_at must not go backwards")
	}
	if updated.SKU != "PVC-01" {
		t.Error("SKU must not change")
	}
}

func TestUpdateNotFound(t *testing.T) {
	repository, _ := newRepository(t)
	missing := pvcProduct("PVC-01", "x")
	missing.ID = "00000000-0000-0000-0000-000000000000"
	if _, err := repository.Update(context.Background(), missing); !errors.Is(err, product.ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestGetByIDNotFound(t *testing.T) {
	repository, _ := newRepository(t)
	for _, id := range []string{"00000000-0000-0000-0000-000000000000", "not-a-uuid", ""} {
		if _, err := repository.GetByID(context.Background(), id); !errors.Is(err, product.ErrNotFound) {
			t.Errorf("GetByID(%q) error = %v, want ErrNotFound", id, err)
		}
	}
}

func TestSetImageKey(t *testing.T) {
	repository, _ := newRepository(t)
	ctx := context.Background()
	created, _ := repository.Create(ctx, pvcProduct("PVC-01", "x"))

	if err := repository.SetImageKey(ctx, created.ID, "products/a/b.png"); err != nil {
		t.Fatal(err)
	}
	loaded, _ := repository.GetByID(ctx, created.ID)
	if loaded.ImageKey != "products/a/b.png" {
		t.Errorf("ImageKey = %q", loaded.ImageKey)
	}

	if err := repository.SetImageKey(ctx, created.ID, ""); err != nil {
		t.Fatal(err)
	}
	loaded, _ = repository.GetByID(ctx, created.ID)
	if loaded.ImageKey != "" {
		t.Errorf("ImageKey = %q, want empty", loaded.ImageKey)
	}

	if err := repository.SetImageKey(ctx, "00000000-0000-0000-0000-000000000000", "k"); !errors.Is(err, product.ErrNotFound) {
		t.Errorf("missing product: error = %v", err)
	}
}

func TestListFiltersAndPaging(t *testing.T) {
	repository, _ := newRepository(t)
	ctx := context.Background()
	repository.Create(ctx, pvcProduct("PVC-A", "Alpha PVC"))
	second, _ := repository.Create(ctx, pvcProduct("PVC-B", "Beta PVC"))
	repository.Create(ctx, tableclothProduct("MAN-C", "Charlie 100%_mantel"))
	second.Deactivate()
	repository.Update(ctx, second)

	active := true
	inactive := false

	tests := []struct {
		name   string
		filter ports.ProductFilter
		want   []string
		total  int
	}{
		{"all ordered by name", ports.ProductFilter{Limit: 10}, []string{"PVC-A", "PVC-B", "MAN-C"}, 3},
		{"only pvc", ports.ProductFilter{Type: product.TypePVC, Limit: 10}, []string{"PVC-A", "PVC-B"}, 2},
		{"only active", ports.ProductFilter{IsActive: &active, Limit: 10}, []string{"PVC-A", "MAN-C"}, 2},
		{"only inactive", ports.ProductFilter{IsActive: &inactive, Limit: 10}, []string{"PVC-B"}, 1},
		{"search by name, case-insensitive", ports.ProductFilter{Query: "beta", Limit: 10}, []string{"PVC-B"}, 1},
		{"search by sku", ports.ProductFilter{Query: "man-c", Limit: 10}, []string{"MAN-C"}, 1},
		{"wildcards are literal", ports.ProductFilter{Query: "100%_", Limit: 10}, []string{"MAN-C"}, 1},
		{"percent alone matches nothing else", ports.ProductFilter{Query: "%", Limit: 10}, []string{"MAN-C"}, 1},
		{"paging first", ports.ProductFilter{Limit: 2}, []string{"PVC-A", "PVC-B"}, 3},
		{"paging second", ports.ProductFilter{Limit: 2, Offset: 2}, []string{"MAN-C"}, 3},
		{"no results", ports.ProductFilter{Query: "zzz", Limit: 10}, nil, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			page, err := repository.List(ctx, test.filter)
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			var got []string
			for _, item := range page.Items {
				got = append(got, item.SKU)
			}
			if len(got) != len(test.want) {
				t.Fatalf("skus = %v, want %v", got, test.want)
			}
			for i := range got {
				if got[i] != test.want[i] {
					t.Fatalf("skus = %v, want %v", got, test.want)
				}
			}
			if page.Total != test.total {
				t.Errorf("total = %d, want %d", page.Total, test.total)
			}
		})
	}
}

func TestMinimumStockIsStoredAndUpdated(t *testing.T) {
	repository, _ := newRepository(t)
	ctx := context.Background()

	pvc := pvcProduct("PVC-MIN", "PVC con minimo")
	pvc.PVC.MinimumStockM = decimal.MustParse("50.5")
	cloth := tableclothProduct("MAN-MIN", "Mantel con minimo")
	cloth.Tablecloth.MinimumUnits = 8

	createdPVC, err := repository.Create(ctx, pvc)
	if err != nil {
		t.Fatal(err)
	}
	createdCloth, err := repository.Create(ctx, cloth)
	if err != nil {
		t.Fatal(err)
	}
	if createdPVC.PVC.MinimumStockM.String() != "50.50" || createdCloth.Tablecloth.MinimumUnits != 8 {
		t.Fatalf("created minimums = %s / %d", createdPVC.PVC.MinimumStockM, createdCloth.Tablecloth.MinimumUnits)
	}

	createdPVC.PVC.MinimumStockM = decimal.MustParse("0")
	createdCloth.Tablecloth.MinimumUnits = 12
	if _, err := repository.Update(ctx, createdPVC); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Update(ctx, createdCloth); err != nil {
		t.Fatal(err)
	}
	loadedPVC, _ := repository.GetByID(ctx, createdPVC.ID)
	loadedCloth, _ := repository.GetByID(ctx, createdCloth.ID)
	if !loadedPVC.PVC.MinimumStockM.IsZero() || loadedCloth.Tablecloth.MinimumUnits != 12 {
		t.Errorf("updated minimums = %s / %d", loadedPVC.PVC.MinimumStockM, loadedCloth.Tablecloth.MinimumUnits)
	}
}

func TestEveryProductGetsAUniqueLabelCodeThatCanBeReset(t *testing.T) {
	repository, _ := newRepository(t)
	ctx := context.Background()
	first, err := repository.Create(ctx, pvcProduct("PVC-A", "PVC A"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := repository.Create(ctx, tableclothProduct("MAN-A", "Mantel A"))
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^P-[0-9A-F]{12}$`).MatchString(first.LabelCode) || first.LabelCode == second.LabelCode {
		t.Fatalf("label codes %q and %q", first.LabelCode, second.LabelCode)
	}

	reset, err := repository.ResetLabelCode(ctx, first.ID)
	if err != nil {
		t.Fatalf("reset: %v", err)
	}
	if reset.LabelCode == first.LabelCode || reset.SKU != first.SKU || reset.Name != first.Name {
		t.Errorf("reset = %+v", reset)
	}
	if other, _ := repository.GetByID(ctx, second.ID); other.LabelCode != second.LabelCode {
		t.Error("resetting one product must not change another")
	}

	// Editing the data keeps the code.
	edited := reset
	edited.Name = "PVC A corregido"
	updated, err := repository.Update(ctx, edited)
	if err != nil || updated.LabelCode != reset.LabelCode {
		t.Errorf("update: %v, code %q -> %q", err, reset.LabelCode, updated.LabelCode)
	}

	for _, id := range []string{"00000000-0000-0000-0000-000000000000", "nope"} {
		if _, err := repository.ResetLabelCode(ctx, id); !errors.Is(err, product.ErrNotFound) {
			t.Errorf("reset %q: %v, want ErrNotFound", id, err)
		}
	}
}
